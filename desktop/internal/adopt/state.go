package adopt

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// StatePath is where adoption records what it did.
//
// One path, one owner, one format version. `--undo`, `zenith doctor` and every
// failure path read it, so it is written atomically and validated on read: a
// state file that was half-written is a state file that says adoption removed a
// package it never touched.
const StatePath = "/etc/zenith/adopt/state.json"

// StateVersion is the schema version. It exists because `--undo` reads a file
// this tool wrote possibly weeks earlier, possibly with a different meaning for
// a field, and acting on a misread field is how an undo removes the wrong thing.
const StateVersion = 1

// State is the record of one adoption.
//
// The shape is the one in docs/adopting-zenith.md. It is not negotiable: the
// document is the contract, and a state file that does not match it means the
// documented undo procedure cannot be followed by hand.
type State struct {
	Version       int            `json:"version"`
	AdoptedAt     time.Time      `json:"adopted_at"`
	User          User           `json:"user"`
	Base          Base           `json:"base"`
	Machine       Machine        `json:"machine"`
	Registrations []Registration `json:"registrations"`
	UserWrites    []UserWrite    `json:"user_writes"`
	Imports       []Import       `json:"imports"`
	Packages      Packages       `json:"packages"`
	Prune         Prune          `json:"prune"`
	BackupDir     string         `json:"backup_dir"`
}

// User is the adopting user.
//
// Named explicitly rather than relying on $HOME, because `sudo` sets $HOME to
// /root and a tool that trusted it would put every user-scoped path in the wrong
// place. That is the single most common way an adoption tool ruins a machine.
type User struct {
	Name string `json:"name"`
	Home string `json:"home"`
	UID  int    `json:"uid"`
}

// Base is the base layer adoption found.
type Base struct {
	ID        string `json:"id"`
	VersionID string `json:"version_id"`
}

// Machine is what adoption found, recorded so `--undo` can restore a specific
// thing rather than "whatever the display manager is now".
type Machine struct {
	DisplayManagerBefore string `json:"display_manager_before"`
	GNOMEShellVersion    string `json:"gnome_shell_version"`
	GPU                  GPU    `json:"gpu"`
}

// GPU is the detected graphics hardware and driver.
type GPU struct {
	Vendor   string `json:"vendor"`
	Driver   string `json:"driver"`
	Discrete bool   `json:"discrete"`
}

// Registration is a pointer adoption materialized outside /usr/share/zenith/.
//
// `ByteIdenticalToBase` is recorded rather than assumed: a registration that
// has drifted from its source is not a registration any more, it is a second
// copy of a file, and `--verify` needs to be able to say so.
type Registration struct {
	Path                string `json:"path"`
	MaterializedFrom    string `json:"materialized_from"`
	ByteIdenticalToBase bool   `json:"byte_identical_to_base"`
}

// UserWrite is a change adoption made inside the user's own tree.
type UserWrite struct {
	Path         string `json:"path"`
	ArchivedTo   string `json:"archived_to,omitempty"`
	Action       string `json:"action,omitempty"`
	Dconf        string `json:"dconf,omitempty"`
	ValueBefore  string `json:"value_before,omitempty"`
	Alternatives string `json:"alternatives,omitempty"`
}

// Import is a value carried across from a GNOME key to a Zenith one.
type Import struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Value string `json:"value"`
}

// Packages is what apt did.
type Packages struct {
	Installed []string `json:"installed"`
	Removed   []string `json:"removed"`
}

// Prune is the GNOME removal step, which is separable from adoption itself
// because it is the only irreversible-feeling part and users opt out of it.
type Prune struct {
	Performed               bool     `json:"performed"`
	Removed                 []string `json:"removed"`
	Purged                  []string `json:"purged"`
	SkippedBecauseGDMActive []string `json:"skipped_because_gdm_active"`
}

// StateDir is the directory the state file lives in.
func StateDir() string { return filepath.Dir(StatePath) }

// Save writes the state file atomically.
//
// Atomic because the file is read by `--undo` on a machine that is already in
// trouble, and a truncated state file read as an empty one makes undo do
// nothing and report success. Write to a sibling, fsync, rename: rename within
// a directory is atomic, so a reader sees either the old file or the new one.
func Save(state *State) error {
	if err := os.MkdirAll(StateDir(), 0o755); err != nil {
		return fmt.Errorf("cannot create %s: %w", StateDir(), err)
	}

	body, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')

	temp, err := os.CreateTemp(StateDir(), ".state.json.*")
	if err != nil {
		return fmt.Errorf("cannot create a temporary state file: %w", err)
	}
	tempName := temp.Name()

	// Any failure from here has to remove the temporary file, or a run of
	// adoption that fails leaves litter in /etc.
	defer os.Remove(tempName)

	if _, err := temp.Write(body); err != nil {
		temp.Close()
		return err
	}
	// fsync before rename: rename is atomic about the directory entry, not about
	// the bytes, and a power cut between the two leaves a correctly-named empty
	// file.
	if err := temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tempName, 0o644); err != nil {
		return err
	}
	return os.Rename(tempName, StatePath)
}

// Load reads the state file.
func Load(path string) (*State, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var state State
	if err := json.Unmarshal(body, &state); err != nil {
		return nil, fmt.Errorf("%s is not readable as adoption state: %w", path, err)
	}
	if state.Version != StateVersion {
		return nil, fmt.Errorf(
			"%s is version %d and this tool understands version %d; "+
				"refusing to undo an adoption whose record it cannot read",
			path, state.Version, StateVersion)
	}
	return &state, nil
}

// Exists reports whether this machine has been adopted.
func Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// SortedPackages returns installed and removed in a stable order, so a diff of
// two state files shows what changed rather than what was written in a
// different order.
func (p Packages) SortedPackages() Packages {
	sort.Strings(p.Installed)
	sort.Strings(p.Removed)
	return p
}

// Empty reports whether nothing was changed, which is what a `--dry-run` that
// then wrote a state file would produce — and a state file recording nothing is
// a state file that makes `--undo` a no-op that reports success.
func (s State) Empty() bool {
	return len(s.Registrations) == 0 &&
		len(s.UserWrites) == 0 &&
		len(s.Imports) == 0 &&
		len(s.Packages.Installed) == 0 &&
		len(s.Packages.Removed) == 0 &&
		!s.Prune.Performed
}
