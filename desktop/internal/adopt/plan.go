package adopt

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ActionKind is what one step of adoption does.
//
// The kind is the point. `--dry-run` prints a plan, the undo script is
// generated from the same plan, and both have to agree about what happened — so
// the ordering and the reversal live here rather than being written twice, once
// in the executor and once in the undo's reverse-engineering of it.
type ActionKind string

const (
	// ActionInstall runs apt-get install. Needs root.
	ActionInstall ActionKind = "install"
	// ActionRemove runs apt-get remove. Needs root.
	ActionRemove ActionKind = "remove"
	// ActionRegister copies content from /usr/share/zenith/ to a registration
	// path. Needs root, and is the only kind that writes outside the user's own
	// tree.
	ActionRegister ActionKind = "register"
	// ActionSchemaCompile recompiles the GSettings schemas after a registration
	// lands. Needs root, and is separate because a schema file that is on disk
	// but not compiled is a setting that silently falls back to its default.
	ActionSchemaCompile ActionKind = "compile-schemas"
	// ActionPortal writes the user's portal preference. The only kind that has
	// to exist per-user rather than per-machine.
	ActionPortal ActionKind = "write-portals"
	// ActionImport carries a value from a GNOME key to a Zenith one.
	ActionImport ActionKind = "import-setting"
	// ActionAlternative repoints an update-alternatives group, which is the
	// only way to change what x-terminal-emulator means without editing the
	// alternatives database by hand.
	ActionAlternative ActionKind = "set-alternative"
	// ActionArchive copies a file adoption is about to overwrite into the backup
	// directory. Runs before the write it protects, always.
	ActionArchive ActionKind = "archive"
	// ActionPrune removes GNOME. Separate from adoption itself and opt-out,
	// because it is the only step a user cannot get back with --undo alone.
	ActionPrune ActionKind = "prune"
)

// NeedsRoot says whether an action has to run as root.
//
// Kept as a method rather than a field so it cannot drift from the kind: an
// action that needs root and does not say so is an action that fails halfway
// through adoption, which is the one outcome the plan exists to prevent.
func (k ActionKind) NeedsRoot() bool {
	switch k {
	case ActionInstall, ActionRemove, ActionRegister, ActionSchemaCompile,
		ActionPrune:
		return true
	default:
		return false
	}
}

// Reverses says whether --undo handles this kind, and is false for the steps
// that undo restores rather than replays backwards.
func (k ActionKind) Reverses() bool { return k != ActionPrune }

// Action is one step.
type Action struct {
	Kind ActionKind
	// What is acted on: a package name, a path, a "schema key" pair.
	Target string
	// Detail is the argument, when the kind takes one.
	Detail string
	// Archive is where a previous value is copied to, for ActionArchive.
	Archive string
	// Why is the sentence shown to the user in a dry run.
	//
	// It is required rather than optional because a plan the user cannot read is
	// a plan they either trust blindly or refuse. Every line has to be able to
	// say why it is in the plan, and "because" is the part people read.
	Why string
}

func (a Action) String() string {
	line := string(a.Kind) + " " + a.Target
	if a.Detail != "" {
		line += " " + a.Detail
	}
	if a.Why != "" {
		line += "\n      — " + a.Why
	}
	return line
}

// Plan is the ordered set of steps adoption would take.
//
// Ordering is not cosmetic. An archive runs before the write it protects; a
// registration runs before the schema compile that makes it readable; the
// portal preference is written last because it is the switch that decides the
// session reads it. A plan that is merely a set gets the ordering wrong.
type Plan struct {
	// Actions, in the order they must run.
	Actions []Action
	// Packages to install, for the summary and for the state's package list.
	Install []string
	// Registrations, in the same form the state file records.
	Registrations []Registration
	// Imports are the settings carried across.
	Imports []Import
	// UserWrites are the user's-tree changes.
	UserWrites []UserWrite
	// BackupDir is where archives go.
	BackupDir string
	// RootActions counts what needs root, so the summary can say whether this
	// is going to ask for a password.
	RootActions int
}

// NeedsRoot reports whether executing this plan requires root.
func (p Plan) NeedsRoot() bool { return p.RootActions > 0 }

// String renders the plan for a dry run.
func (p Plan) String() string {
	var out strings.Builder
	fmt.Fprintf(&out, "zenith adopt would take %d step(s):\n\n", len(p.Actions))
	for i, action := range p.Actions {
		fmt.Fprintf(&out, "  %2d. %s\n", i+1, action)
	}
	if p.NeedsRoot() {
		fmt.Fprintf(&out, "\n%d step(s) need root. Nothing has been changed.",
			p.RootActions)
	} else {
		fmt.Fprintf(&out, "\nNothing has been changed.")
	}
	return out.String()
}

// registrationSources maps a registration path to the packaged content it
// points at.
//
// The whole design in one table: every path here is a fixed directory that a
// third-party binary scans and nowhere else, and every target is content under
// /usr/share/zenith/. Adoption writes pointers; it never writes content outside
// its own tree, and never edits a shipped file in place.
var registrationSources = []Registration{
	{Path: "/usr/share/wayland-sessions/zenith.desktop",
		MaterializedFrom: "/usr/share/zenith/session/zenith.desktop"},
	{Path: "/usr/lib/systemd/user/zenith-shell.service",
		MaterializedFrom: "/usr/share/zenith/session/units/zenith-shell.service"},
	{Path: "/usr/lib/systemd/user/zenith-portal.service",
		MaterializedFrom: "/usr/share/zenith/session/units/zenith-portal.service"},
	{Path: "/usr/lib/systemd/user/hyprland-session.target.d/10-zenith.conf",
		MaterializedFrom: "/usr/share/zenith/session/units/hyprland-session.target.d-10-zenith.conf"},
}

// sessionPackages is what adoption installs.
//
// The session stack, and nothing else. Quickshell is Zenith-built and arrives
// with the package; the rest come from the base layer's repository, because rule
// 2 in the manifest header is that anything the archive carries is not
// repackaged — and a session that installs its own compositor would be a
// session nobody tested against the base layer.
var sessionPackages = []string{
	"zenith",
	"hyprland",
	"xdg-desktop-portal",
	"xdg-desktop-portal-hyprland",
	"xdg-desktop-portal-gtk",
	"xdg-desktop-portal-lxqt",
}

// settingImports carries values across from GNOME to Zenith.
//
// The `to` half is a key this project owns and the Hub has a page for. An
// import whose destination has no page would write a setting the user cannot
// see or change, which is worse than not importing it.
var settingImports = []Import{
	{From: "org.gnome.desktop.interface color-scheme",
		To: "org.zenith.desktop color-scheme"},
	{From: "org.gnome.desktop.background picture-uri",
		To: "org.zenith.shell wallpaper-path"},
	{From: "org.gnome.shell favorite-apps",
		To: "org.zenith.shell favorites"},
	{From: "org.gnome.desktop.wm.preferences button-layout",
		To: "org.zenith.shell dock-position"},
}

// PlanEnv is what the plan is built from.
type PlanEnv struct {
	// Root prefixes every system path, so a plan can be built against a fixture.
	Root string
	// Home is the adopting user's home. Named, never inherited from $HOME.
	Home string
	// BackupDir is where archives go.
	BackupDir string
	// DisplayManager is what is running, recorded so undo can restore a
	// specific one rather than "whatever it is now".
	DisplayManager string
	// Terminal is the current x-terminal-emulator alternative.
	Terminal string
	// Prune asks for the GNOME removal step.
	Prune bool
	// gsettings reports the current value of a "schema key" pair, and whether
	// the key exists at all. Injected so a plan can be built without a session.
	gsettings func(schema, key string) (string, bool)
}

// BuildPlan assembles the steps, in the order they must run.
//
// Every step is derived from what is actually on the machine rather than from a
// fixed script. A registration whose file is already byte-identical to its
// source is not a step, because writing it again is a change that changes
// nothing and makes the state's "what adoption did" list a worse record.
func BuildPlan(env PlanEnv) Plan {
	plan := Plan{BackupDir: env.BackupDir}

	// 1. Archive anything that is about to be overwritten. First, always: an
	//    archive taken after the write it protects is a copy of the new value.
	for _, registration := range registrationSources {
		path := env.resolve(registration.Path)
		if _, err := os.Stat(path); err != nil {
			continue
		}
		plan.Actions = append(plan.Actions, Action{
			Kind:   ActionArchive,
			Target: registration.Path,
			Archive: filepath.Join(env.BackupDir,
				filepath.Base(registration.Path)+".pre-adopt"),
			Why: "a file is already here that adoption would overwrite, and " +
				"undoing into the unknown is not undoing",
		})
	}

	// 2. Install the session stack.
	for _, pkg := range sessionPackages {
		if installed(env, pkg) {
			continue
		}
		plan.Install = append(plan.Install, pkg)
		plan.Actions = append(plan.Actions, Action{
			Kind:   ActionInstall,
			Target: pkg,
			Why: "the session cannot start without it, and the base layer " +
				"already carries it, so it is installed rather than repackaged",
		})
	}

	// 3. Materialize the registrations, as pointers to packaged content.
	for _, registration := range registrationSources {
		source := env.resolve(registration.MaterializedFrom)
		if _, err := os.Stat(source); err != nil {
			// The package is not installed yet at plan time on a clean machine,
			// so a missing source is expected and is not a reason to skip the
			// step. It becomes a real failure at execute time, loudly.
			plan.Actions = append(plan.Actions, Action{
				Kind:   ActionRegister,
				Target: registration.Path,
				Detail: registration.MaterializedFrom,
				Why:    "a pointer from a fixed directory; the content stays under /usr/share/zenith/",
			})
			plan.Registrations = append(plan.Registrations, registration)
			continue
		}
		if identical(source, env.resolve(registration.Path)) {
			continue
		}
		plan.Actions = append(plan.Actions, Action{
			Kind:   ActionRegister,
			Target: registration.Path,
			Detail: registration.MaterializedFrom,
			Why:    "a pointer from a fixed directory; the content stays under /usr/share/zenith/",
		})
		plan.Registrations = append(plan.Registrations, registration)
	}

	// 4. Recompile the schemas. Separate from the registration above because a
	//    schema file that is on disk and not compiled is a setting that reads
	//    as its default with nothing wrong on screen to explain it.
	plan.Actions = append(plan.Actions, Action{
		Kind:   ActionSchemaCompile,
		Target: "/usr/share/glib-2.0/schemas",
		Why: "GSettings reads gschemas.compiled, not the XML beside it; " +
			"without this every Zenith setting falls back to its default",
	})

	// 5. Point x-terminal-emulator at the shipped terminal. The archive is a
	//    user-tree write, so it needs no root and is still recorded.
	if env.Terminal != "" {
		plan.Actions = append(plan.Actions, Action{
			Kind:    ActionArchive,
			Target:  "x-terminal-emulator",
			Archive: filepath.Join(env.BackupDir, "x-terminal-emulator.path"),
			Why:     "undo restores the alternative the user actually had",
		})
		plan.Actions = append(plan.Actions, Action{
			Kind:   ActionAlternative,
			Target: "x-terminal-emulator",
			Detail: "/usr/bin/ghostty",
			Why: "an alternative, not an edit: the alternatives database is " +
				"a shipped file",
		})
		plan.UserWrites = append(plan.UserWrites, UserWrite{
			Alternatives: "x-terminal-emulator",
			ValueBefore:  env.Terminal,
		})
	}

	// 6. Carry settings across.
	if env.gsettings != nil {
		for _, mapping := range settingImports {
			schema, key, _ := strings.Cut(mapping.To, " ")
			value, exists := env.gsettings(schema, key)
			if !exists {
				// A GNOME key that is not set is not an import. Writing the
				// Zenith default over it would be indistinguishable from
				// having imported nothing, and would show up in the state file
				// as a change that did not change anything.
				continue
			}
			imported := Import{From: mapping.From, To: mapping.To, Value: value}
			plan.Imports = append(plan.Imports, imported)
			plan.Actions = append(plan.Actions, Action{
				Kind:   ActionImport,
				Target: mapping.To,
				Detail: value,
				Why:    "the setting already exists on this machine and has a page in the Hub",
			})
		}
	}

	// 7. Write the user's portal preference. Last of the user-scoped steps,
	//    because it is the switch that decides the session reads it, and a
	//    switch written before the packages it names are installed selects
	//    nothing.
	portalPath := filepath.Join(env.Home, ".config", "xdg-desktop-portal",
		"portals.conf")
	if _, err := os.Stat(portalPath); err == nil {
		plan.Actions = append(plan.Actions, Action{
			Kind:    ActionArchive,
			Target:  portalPath,
			Archive: filepath.Join(env.BackupDir, "portals.conf.gnome"),
			Why:     "the GNOME session's own portal preference, so undo puts it back",
		})
		plan.UserWrites = append(plan.UserWrites, UserWrite{
			Path:       portalPath,
			ArchivedTo: filepath.Join(env.BackupDir, "portals.conf.gnome"),
		})
	}
	plan.Actions = append(plan.Actions, Action{
		Kind:   ActionPortal,
		Target: portalPath,
		Why: "written last, because it is the switch that decides the " +
			"session reads the preference at all",
	})

	// 8. Prune, only if asked. Last, and separate, because it is the only step
	//    `--undo` does not reverse on its own.
	if env.Prune {
		plan.Actions = append(plan.Actions, Action{
			Kind:   ActionPrune,
			Target: "gnome-desktop",
			Why: "the GNOME metapackage, so an apt upgrade does not put the " +
				"old desktop back. GNOME stays installed until the login " +
				"manager is switched, and --undo does not reverse this step",
		})
	}

	for _, action := range plan.Actions {
		if action.Kind.NeedsRoot() {
			plan.RootActions++
		}
	}

	sort.Strings(plan.Install)
	return plan
}

func (env PlanEnv) resolve(absolute string) string {
	if env.Root == "" {
		return absolute
	}
	return filepath.Join(env.Root, absolute)
}

// installed reports whether a package is already present.
//
// dpkg-query, not `dpkg -l`, because -l prints a header and a table and a
// grep for the name also matches the description of a different package.
func installed(env PlanEnv, pkg string) bool {
	query := env.resolve("/var/lib/dpkg/status")
	if _, err := os.Stat(query); err != nil {
		// No dpkg database in the fixture: assume not installed, so the plan
		// includes the install rather than silently omitting it.
		return false
	}
	body, err := os.ReadFile(query)
	if err != nil {
		return false
	}
	needle := "\nPackage: " + pkg + "\n"
	return strings.Contains("\n"+string(body), needle)
}

func identical(a, b string) bool {
	first, err := os.ReadFile(a)
	if err != nil {
		return false
	}
	second, err := os.ReadFile(b)
	if err != nil {
		return false
	}
	return string(first) == string(second)
}
