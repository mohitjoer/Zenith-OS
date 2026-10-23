package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"zenith/desktop/internal/theme"
)

const themeUsage = `zenith theme — take the desktop's colours from one image

Usage:
  zenith theme generate [flags] IMAGE   write the shell's palette from IMAGE
  zenith theme render --config CONFIG --palette CARRIER
                                fill the templates CONFIG names from CARRIER
  zenith theme current          print the palette in use
  zenith theme list IMAGEDIR    what each image in a directory would produce
  zenith theme check            assert the palette in use is readable

generate flags (matugen's names; the daemon passes them):
  --image PATH             the image, instead of the argument
  --mode dark|light|smart  smart (default) follows the image's brightness
  --scheme NAME            monochrome (default), tonal-spot, neutral, vibrant,
                           expressive, fruit-salad, rainbow, fidelity, content
  --contrast N             -1 to 1
  --lightness-dark N       --lightness-light N
  --source-color-index N   which wallpaper colour seeds the scheme, 0 is the best
  --json                   print matugen's JSON on stdout and write nothing
  --prefer, --dry-run      accepted and ignored: an explicit index wins, as in
                           matugen, and --json already writes nothing

The palette is written to $XDG_CACHE_HOME/zenith/colors.json, with the tonal
ramps beside it in tones.json. The shell reads those files rather than deriving
colours itself, so picking a wallpaper repaints the system and nothing has to
restart. docs/theming.md has the keys and who writes them.
`

const renderUsage = `Usage: zenith theme render --config CONFIG --palette CARRIER

Fills every [templates.NAME] block in CONFIG (matugen's file format) from the
colours in CARRIER, a JSON file shaped {"colors": {NAME: {"default": {"hex":
"#rrggbb", ...}, "dark": ..., "light": ...}}}, and writes each output_path.
A file that is not ours, or that has a user edit, is reported and left alone.
matugen's own spelling, -c CONFIG [json] CARRIER, is accepted too.
`

func zenithTheme(args []string) int {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, themeUsage)
		return exitUsage
	}

	switch args[0] {
	case "generate":
		return themeGenerate(args[1:])
	case "render":
		return themeRender(args[1:])
	case "current":
		return themeCurrent()
	case "list":
		return themeList(args[1:])
	case "check":
		return themeCheck()
	default:
		fmt.Fprintf(os.Stderr, "zenith theme: unknown subcommand %q\n", args[0])
		return exitUsage
	}
}

// themeOutputPath is where the palette goes.
//
// $XDG_CACHE_HOME with a documented fallback, because a shell that reads an
// absolute path the tool wrote to a relative one is a shell with no theme and no
// way to find out why.
func themeOutputPath() string {
	if dir := os.Getenv("XDG_CACHE_HOME"); dir != "" {
		return filepath.Join(dir, "zenith", "colors.json")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "zenith-colors.json")
	}
	return filepath.Join(home, ".cache", "zenith", "colors.json")
}

func themeGenerate(args []string) int {
	set := newFlags("theme generate", themeUsage)
	var (
		image      = set.String("image", "", "")
		mode       = set.String("mode", "smart", "")
		scheme     = set.String("scheme", theme.DefaultScheme, "")
		contrast   = set.Float64("contrast", 0, "")
		lightDark  = set.Float64("lightness-dark", 0, "")
		lightLight = set.Float64("lightness-light", 0, "")
		index      = set.Int("source-color-index", 0, "")
		asJSON     = set.Bool("json", false, "")
	)
	set.String("prefer", "", "")
	set.Bool("dry-run", false, "")
	positional, err := parseFlags(set, args)
	if err != nil {
		return exitUsage
	}
	path := *image
	if path == "" && len(positional) == 1 {
		path, positional = positional[0], nil
	}
	if path == "" || len(positional) > 0 {
		fmt.Fprintln(os.Stderr, "zenith theme generate: needs one image")
		return exitUsage
	}

	palette, err := theme.Extract(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "zenith theme:", err)
		return exitFail
	}
	var dark bool
	switch *mode {
	case "dark", "light":
		dark = *mode == "dark"
	case "smart":
		dark = palette.Dark()
	default:
		fmt.Fprintf(os.Stderr, "zenith theme generate: --mode is dark, light or smart, not %q\n", *mode)
		return exitUsage
	}

	seed := palette.Ranked[max(0, min(*index, len(palette.Ranked)-1))]
	generated, err := theme.Generate(seed, theme.Options{
		Scheme: *scheme, Contrast: *contrast, LightnessDark: *lightDark, LightnessLight: *lightLight,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "zenith theme generate:", err)
		return exitUsage
	}

	if *asJSON {
		body, err := json.MarshalIndent(generated.Matugen(dark), "", "  ")
		if err != nil {
			fmt.Fprintln(os.Stderr, "zenith theme:", err)
			return exitFail
		}
		fmt.Println(string(body))
		return exitOK
	}

	out := themeOutputPath()
	if err := theme.Write(out, theme.ShellColors(generated.Flat(dark))); err != nil {
		fmt.Fprintln(os.Stderr, "zenith theme:", err)
		return exitFail
	}
	if err := theme.Write(filepath.Join(filepath.Dir(out), "tones.json"), generated.Tones()); err != nil {
		fmt.Fprintln(os.Stderr, "zenith theme:", err)
		return exitFail
	}

	modeName := "light"
	if dark {
		modeName = "dark"
	}
	fmt.Printf("from      %s  (%s, %s, seed %s)\n", palette.Source, modeName, generated.Name, seed.Hex())
	if roles, err := theme.Load(out); err == nil {
		printRoles(roles)
	}
	fmt.Printf("\nwritten to %s\n", out)
	return exitOK
}

// themeRender is matugen's templating pass. The daemon calls it with --config and
// --palette; matugen's own `-c CONFIG json CARRIER` works too, so a script written
// for matugen needs only the executable swapped.
func themeRender(args []string) int {
	set := newFlags("theme render", renderUsage)
	config := set.String("c", "", "")
	set.StringVar(config, "config", "", "")
	palette := set.String("palette", "", "")
	positional, err := parseFlags(set, args)
	if err != nil {
		return exitUsage
	}
	if len(positional) > 0 && positional[0] == "json" {
		positional = positional[1:]
	}
	carrierPath := *palette
	if carrierPath == "" && len(positional) == 1 {
		carrierPath, positional = positional[0], nil
	}
	if *config == "" || carrierPath == "" || len(positional) > 0 {
		fmt.Fprint(os.Stderr, renderUsage)
		return exitUsage
	}

	body, err := os.ReadFile(carrierPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "zenith theme render:", err)
		return exitFail
	}
	var carrier map[string]any
	if err := json.Unmarshal(body, &carrier); err != nil {
		fmt.Fprintf(os.Stderr, "zenith theme render: %s: %v\n", carrierPath, err)
		return exitFail
	}

	done, err := theme.Render(*config, carrier)
	for _, r := range done {
		fmt.Printf("%-14s %-22s %s\n", r.Block, r.Status, r.Dest)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "zenith theme render:", err)
		return exitFail
	}
	return exitOK
}

func themeCurrent() int {
	path := themeOutputPath()
	roles, err := theme.Load(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "zenith theme current:", err)
		fmt.Fprintf(os.Stderr,
			"  no palette has been generated yet; run `zenith theme generate IMAGE`\n")
		return exitFail
	}
	printRoles(roles)
	return exitOK
}

// themeCheck exists so the shell check can call the same assertion the generator
// is built to satisfy, rather than reimplementing the thresholds.
func themeCheck() int {
	path := themeOutputPath()
	roles, err := theme.Load(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "zenith theme check:", err)
		return exitFail
	}

	if problems := theme.CheckRoles(roles); len(problems) != 0 {
		fmt.Fprintf(os.Stderr, "zenith theme check: %d problem(s):\n", len(problems))
		for _, problem := range problems {
			fmt.Fprintf(os.Stderr, "  %s\n", problem)
		}
		return exitFail
	}

	fmt.Printf("palette in %s is readable at every pair\n", path)
	return exitOK
}

// themeList previews a whole directory.
//
// It is there because "which of these wallpapers should I use" is a question with
// a preview, and generating a palette for each one and then deleting fourteen of
// them is the alternative.
func themeList(args []string) int {
	dir := "."
	if len(args) == 1 {
		dir = args[0]
	}
	if len(args) > 1 {
		fmt.Fprintln(os.Stderr, "zenith theme list: takes at most one directory")
		return exitUsage
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "zenith theme list:", err)
		return exitFail
	}

	found := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		switch strings.ToLower(filepath.Ext(entry.Name())) {
		case ".png", ".jpg", ".jpeg", ".webp":
		default:
			continue
		}

		path := filepath.Join(dir, entry.Name())
		palette, err := theme.Extract(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  %-20s %v\n", entry.Name(), err)
			continue
		}
		found++

		status := "ok"
		if problems := palette.Check(); len(problems) > 0 {
			status = fmt.Sprintf("UNREADABLE (%d)", len(problems))
		}
		fmt.Printf("  %-20s accent %s  bg %s  text %s  lum %.2f  %s\n",
			entry.Name(), palette.Accent.Hex(), palette.Background.Hex(),
			palette.Text.Hex(), palette.Luminance, status)
	}

	if found == 0 {
		fmt.Fprintf(os.Stderr, "zenith theme list: no images in %s\n", dir)
		return exitFail
	}
	return exitOK
}

func printRoles(roles map[string]theme.Colour) {
	for _, name := range []string{
		"surface", "onSurface", "onSurfaceVariant", "primary", "onPrimary",
		"secondary", "tertiary", "outline", "error",
	} {
		fmt.Printf("%-18s %s\n", name, roles[name].Hex())
	}
}
