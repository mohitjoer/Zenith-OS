package adopt

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Execute runs an adoption plan against a machine.
//
// It is the thing the roadmap says does not exist: the plan, the state and the
// undo script are all generated; this function is what actually makes the
// changes. It runs as root, because every step that writes outside the user's
// own tree needs root, and a plan that needs root and does not ask for it is a
// plan that fails halfway through.
//
// The order is the plan's order, and the undo script is written before the first
// write, so a failure at any step leaves a way back.
func Execute(env PlanEnv, user User, base Base, machine Machine, backupDir string) error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("zenith adopt: needs root; run with sudo")
	}

	// Step zero: the preflight. The same check `--check` runs, because a plan
	// built on a machine that is not ready is a plan that fails halfway.
	report, err := Check(Env{
		Desktop: os.Getenv("XDG_CURRENT_DESKTOP"),
		Home:    user.Home,
		Run: func(name string, args ...string) (string, error) {
			out, err := exec.Command(name, args...).Output()
			return string(out), err
		},
	})
	if err != nil {
		return fmt.Errorf("zenith adopt: preflight: %w", err)
	}
	if !report.Ready() {
		return fmt.Errorf("zenith adopt: preflight blocked — run `zenith adopt --check` first")
	}

	// Build the plan.
	plan := BuildPlan(env)
	if len(plan.Actions) == 0 {
		return fmt.Errorf("zenith adopt: the plan is empty; nothing to do")
	}

	// Write the undo script before the first write. If adoption fails halfway,
	// the script exists and the machine has a way back.
	undoPath := filepath.Join(backupDir, "undo.sh")
	state := &State{
		Version:       StateVersion,
		AdoptedAt:     envNow(),
		User:          user,
		Base:          base,
		Machine:       machine,
		BackupDir:     backupDir,
		Packages:      Packages{Installed: plan.Install},
		Registrations: plan.Registrations,
		UserWrites:    plan.UserWrites,
		Imports:       plan.Imports,
		Prune:         Prune{Performed: env.Prune},
	}
	if err := WriteUndoScript(undoPath, plan, state); err != nil {
		return fmt.Errorf("zenith adopt: writing the undo script: %w", err)
	}

	// Execute each action in order.
	for i, action := range plan.Actions {
		if err := executeAction(action, env, state); err != nil {
			return fmt.Errorf("zenith adopt: step %d (%s %s): %w", i+1, action.Kind, action.Target, err)
		}
	}

	// Save the state.
	if err := Save(state); err != nil {
		return fmt.Errorf("zenith adopt: saving state: %w", err)
	}

	return nil
}

// executeAction runs one step of the plan.
func executeAction(action Action, env PlanEnv, state *State) error {
	switch action.Kind {
	case ActionArchive:
		return execArchive(action, env)
	case ActionInstall:
		return execInstall(action.Target)
	case ActionRemove:
		return execRemove(action.Target)
	case ActionRegister:
		return execRegister(action.Target, action.Detail)
	case ActionSchemaCompile:
		return execSchemaCompile(action.Target)
	case ActionPortal:
		return execPortal(action.Target)
	case ActionImport:
		return execImport(action.Target, action.Detail)
	case ActionAlternative:
		return execAlternative(action.Target, action.Detail)
	case ActionPrune:
		return execPrune(action.Target)
	default:
		return fmt.Errorf("unknown action kind %q", action.Kind)
	}
}

func execArchive(action Action, env PlanEnv) error {
	source := action.Target
	if env.Root != "" {
		source = filepath.Join(env.Root, source)
	}
	if _, err := os.Stat(source); err != nil {
		return nil // nothing to archive
	}
	if err := os.MkdirAll(filepath.Dir(action.Archive), 0o755); err != nil {
		return err
	}
	return copyFile(source, action.Archive)
}

func execInstall(pkg string) error {
	cmd := exec.Command("apt-get", "install", "-y", pkg)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Run()
}

func execRemove(pkg string) error {
	cmd := exec.Command("apt-get", "remove", "-y", pkg)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Run()
}

func execRegister(target, source string) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	return copyFile(source, target)
}

func execSchemaCompile(schemaDir string) error {
	cmd := exec.Command("glib-compile-schemas", schemaDir)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Run()
}

func execPortal(path string) error {
	// Write the portal preference file.
	content := `[portal]
preferred=hyprland;gtk
`
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

func execImport(target, value string) error {
	// target is "schema key", value is the value to set
	schema, key, ok := strings.Cut(target, " ")
	if !ok {
		return fmt.Errorf("bad import target %q", target)
	}
	cmd := exec.Command("gsettings", "set", schema, key, value)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Run()
}

func execAlternative(group, path string) error {
	cmd := exec.Command("update-alternatives", "--set", group, path)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Run()
}

func execPrune(target string) error {
	cmd := exec.Command("apt-get", "remove", "-y", target)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Run()
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}

func envNow() time.Time {
	return time.Now().UTC()
}

// Verify checks that an adoption was performed correctly.
func Verify() error {
	state, err := Load(StatePath)
	if err != nil {
		return fmt.Errorf("no adoption state found: %w", err)
	}
	if state.Empty() {
		return fmt.Errorf("adoption state is empty")
	}
	// Check that registrations exist
	for _, reg := range state.Registrations {
		if _, err := os.Stat(reg.Path); err != nil {
			return fmt.Errorf("registration %s is missing: %w", reg.Path, err)
		}
	}
	return nil
}

// Undo runs the undo script for a previous adoption.
func Undo() error {
	state, err := Load(StatePath)
	if err != nil {
		return fmt.Errorf("no adoption state found: %w", err)
	}
	if state.Empty() {
		return fmt.Errorf("adoption state is empty; nothing to undo")
	}
	undoPath := filepath.Join(state.BackupDir, "undo.sh")
	if _, err := os.Stat(undoPath); err != nil {
		return fmt.Errorf("undo script not found: %w", err)
	}
	cmd := exec.Command("sh", undoPath)
	cmd.Stdout, cmd.Stderr, cmd.Stdin = os.Stdout, os.Stderr, os.Stdin
	return cmd.Run()
}
