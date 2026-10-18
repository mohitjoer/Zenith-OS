package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	"zenith/desktop/internal/adopt"
)

const adoptUsage = `Usage: zenith adopt --check [flags]

Preflight. Reports what adoption would need and changes nothing. Needs no root
and is safe to run as often as you like.

  --json    emit the same report machine-readably

Exit codes:
  0  ready, no blockers
  1  blocked — zenith adopt will refuse
  2  ready with warnings — proceed, but read them
`

// zenithAdopt is the adoption entry point. Only the preflight exists so far;
// `zenith adopt` without --check refuses rather than pretending.
func zenithAdopt(args []string) int {
	set := newFlags("adopt", adoptUsage)
	check := set.Bool("check", false, "run the preflight and change nothing")
	asJSON := set.Bool("json", false, "emit the report machine-readably")
	if err := set.Parse(args); err != nil {
		return exitUsage
	}

	if !*check {
		fmt.Fprintln(os.Stderr,
			"zenith adopt: only --check exists so far. It reports what adoption "+
				"would need and changes nothing; `zenith adopt` itself is not "+
				"implemented.")
		return exitUsage
	}

	env := adoptEnv()
	report, err := adopt.Check(env)
	if err != nil {
		fmt.Fprintln(os.Stderr, "zenith adopt --check:", err)
		return exitFail
	}

	if *asJSON {
		if err := emitAdoptJSON(report); err != nil {
			fmt.Fprintln(os.Stderr, "zenith adopt --check:", err)
			return exitFail
		}
	} else {
		printAdoptReport(report)
	}

	return report.ExitCode()
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
	// Empty slices rather than null: a consumer iterating `blockers` should not
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
		for _, finding := range report.Blockers() {
			fmt.Printf("  blocker: %s — %s\n", finding.Check, finding.Detail)
		}
	case warnings > 0:
		fmt.Printf("%d warning(s), 0 blockers.  `zenith adopt` will proceed.\n", warnings)
	default:
		fmt.Println("0 warnings, 0 blockers.  `zenith adopt` will proceed.")
	}
}
