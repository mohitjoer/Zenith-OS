package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"zenith/desktop/internal/adopt"
)

const adoptUsage = `Usage: zenith adopt [flags]

Preflight (default). Reports what adoption would need and changes nothing.
Needs no root and is safe to run as often as you like.

  --check    run the preflight and change nothing (default when no other flag)
  --json     emit the preflight report machine-readably
  --dry-run  build and print the plan without executing

Execute (needs root):
  --prune    include the GNOME removal step (opt-in, not reversed by --undo)

Verify / undo:
  --verify   check that a previous adoption is intact
  --undo     reverse a previous adoption using its saved undo script

Exit codes:
  0  ready / success
  1  blocked — zenith adopt will refuse
  2  ready with warnings — proceed, but read them
`

func zenithAdopt(args []string) int {
	set := newFlags("adopt", adoptUsage)
	check := set.Bool("check", false, "run the preflight and change nothing")
	asJSON := set.Bool("json", false, "emit the report machine-readably (with --check)")
	prune := set.Bool("prune", false, "include the GNOME removal step")
	verify := set.Bool("verify", false, "verify a previous adoption is intact")
	undo := set.Bool("undo", false, "reverse a previous adoption")
	dryRun := set.Bool("dry-run", false, "build and print the plan without executing")
	if err := set.Parse(args); err != nil {
		return exitUsage
	}

	if *verify {
		if err := adopt.Verify(); err != nil {
			fmt.Fprintln(os.Stderr, "zenith adopt --verify:", err)
			return exitFail
		}
		fmt.Println("adoption verified: intact")
		return exitOK
	}

	if *undo {
		if err := adopt.Undo(); err != nil {
			fmt.Fprintln(os.Stderr, "zenith adopt --undo:", err)
			return exitFail
		}
		fmt.Println("adoption undone")
		return exitOK
	}

	env := adoptEnv()
	report, err := adopt.Check(env)
	if err != nil {
		fmt.Fprintln(os.Stderr, "zenith adopt:", err)
		return exitFail
	}

	if *check || *asJSON || *dryRun {
		if *asJSON {
			if err := emitAdoptJSON(report); err != nil {
				fmt.Fprintln(os.Stderr, "zenith adopt:", err)
				return exitFail
			}
		} else {
			printAdoptReport(report)
		}
		if *dryRun {
			if !report.Ready() {
				return exitFail
			}
			planEnv := adoptPlanEnv(env, *prune)
			plan := adopt.BuildPlan(planEnv)
			fmt.Println()
			fmt.Print(plan.String())
		}
		return report.ExitCode()
	}

	// Full execution path.
	if !report.Ready() {
		fmt.Fprintln(os.Stderr, "zenith adopt: preflight blocked — run `zenith adopt --check` first")
		for _, f := range report.Blockers() {
			fmt.Fprintf(os.Stderr, "  blocker: %s — %s\n", f.Check, f.Detail)
		}
		return exitFail
	}

	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "zenith adopt:", err)
		return exitFail
	}

	uid := os.Getuid()
	if uid != 0 {
		fmt.Fprintln(os.Stderr, "zenith adopt: needs root; run with sudo")
		return exitFail
	}

	backupDir := filepath.Join("/etc/zenith", "adopt", "backup",
		time.Now().UTC().Format("20060102T150405"))

	planEnv := adoptPlanEnv(env, *prune)
	plan := adopt.BuildPlan(planEnv)

	user := adopt.User{Name: os.Getenv("SUDO_USER"), Home: home, UID: uid}
	if user.Name == "" {
		user.Name = "root"
	}
	base := adopt.Base{ID: "ubuntu-26.04", VersionID: "26.04"}
	machine := adopt.Machine{DisplayManagerBefore: "gdm3"}

	fmt.Printf("zenith adopt: %d step(s), %d need root. Proceeding.\n",
		len(plan.Actions), plan.RootActions)

	if err := adopt.Execute(planEnv, user, base, machine, backupDir); err != nil {
		fmt.Fprintln(os.Stderr, "zenith adopt:", err)
		return exitFail
	}

	fmt.Println("adoption complete. Log out and back in to pick up the Zenith session.")
	return exitOK
}

// adoptPlanEnv builds the PlanEnv from the adoption environment.
func adoptPlanEnv(env adopt.Env, prune bool) adopt.PlanEnv {
	home, _ := os.UserHomeDir()
	backupDir := filepath.Join("/etc/zenith", "adopt", "backup",
		time.Now().UTC().Format("20060102T150405"))
	return adopt.PlanEnv{
		Home:      home,
		BackupDir: backupDir,
		Prune:     prune,
		GSettings: func(schema, key string) (string, bool) {
			out, err := exec.Command("gsettings", "get", schema, key).Output()
			if err != nil {
				return "", false
			}
			return strings.TrimSpace(string(out)), true
		},
	}
}

// adoptEnv builds the environment the preflight reads.
func adoptEnv() adopt.Env {
	home, err := os.UserHomeDir()
	if err != nil {
		home = ""
	}

	return adopt.Env{
		Desktop: os.Getenv("XDG_CURRENT_DESKTOP"),
		Home:    home,
		Run: func(name string, args ...string) (string, error) {
			command := exec.Command(name, args...)
			output, err := command.Output()
			if err != nil {
				return "", err
			}
			return string(output), nil
		},
	}
}

// adoptReportJSON is the machine-readable shape.
//
// It exists because `zenith adopt` re-runs the preflight as its own step zero
// rather than reimplementing any of it, and because a bug report about a
// preflight result needs the exact findings rather than a screenshot of text.
type adoptReportJSON struct {
	Ready    bool           `json:"ready"`
	ExitCode int            `json:"exit_code"`
	Blockers []adoptFinding `json:"blockers"`
	Warnings []adoptFinding `json:"warnings"`
	Checks   []adoptFinding `json:"checks"`
}

type adoptFinding struct {
	Check    string `json:"check"`
	Severity string `json:"severity"`
	Detail   string `json:"detail,omitempty"`
	Note     string `json:"note,omitempty"`
}

func emitAdoptJSON(report adopt.Report) error {
	// Empty slices rather than null: a consumer iterating `blocks` should not
	// have to handle both a null and an empty list for "no blockers".
	document := adoptReportJSON{
		Ready:    report.Ready(),
		ExitCode: report.ExitCode(),
		Blockers: []adoptFinding{},
		Warnings: []adoptFinding{},
		Checks:   []adoptFinding{},
	}

	for _, finding := range report.Findings {
		entry := adoptFinding{
			Check:    finding.Check,
			Severity: string(finding.Severity),
			Detail:   finding.Detail,
			Note:     finding.Note,
		}
		document.Checks = append(document.Checks, entry)
		switch finding.Severity {
		case adopt.Blocker:
			document.Blockers = append(document.Blockers, entry)
		case adopt.Warning:
			document.Warnings = append(document.Warnings, entry)
		}
	}

	encoded, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(encoded))
	return nil
}

// printAdoptReport renders the human form.
//
// The layout is one check per line with the value first and the verdict at the
// right, because the question a user has is "is this machine one I can switch",
// and that is answered by scanning a column rather than reading prose.
func printAdoptReport(report adopt.Report) {
	labelWidth := 0
	for _, finding := range report.Findings {
		if len(finding.Check) > labelWidth {
			labelWidth = len(finding.Check)
		}
	}

	for _, finding := range report.Findings {
		detail := finding.Detail
		if detail == "" {
			detail = string(finding.Severity)
		}

		line := fmt.Sprintf("%-*s %s", labelWidth, finding.Check, detail)

		switch finding.Severity {
		case adopt.Blocker:
			line += "  BLOCKED"
		case adopt.Warning:
			line += "  warning"
		default:
			line += "  ok"
		}

		if finding.Note != "" {
			line += "  (" + finding.Note + ")"
		}
		fmt.Println(line)
	}

	fmt.Println()
	blockers := len(report.Blockers())
	warnings := len(report.Warnings())

	switch {
	case blockers > 0:
		fmt.Printf("%d blocker(s), %d warning(s).  `zenith adopt` will refuse.\n",
			blockers, warnings)
		for _, f := range report.Blockers() {
			fmt.Printf("  blocker: %s — %s\n", f.Check, f.Detail)
		}
	case warnings > 0:
		fmt.Printf("%d warning(s), 0 blockers.  `zenith adopt` will proceed.\n", warnings)
	default:
		fmt.Println("0 warnings, 0 blockers.  `zenith adopt` will proceed.")
	}
}
