package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"zenith/desktop/internal/theme"
)

const themeUsage = `zenith theme — take the desktop's colours from one image

Usage:
  zenith theme generate IMAGE   write a palette from IMAGE
  zenith theme current          print the palette in use
  zenith theme list IMAGEDIR    what each image in a directory would produce
  zenith theme check            assert the palette in use is readable

The palette is written to $XDG_CACHE_HOME/zenith/colors.json. The shell reads
that file rather than deriving colours itself, so picking a wallpaper repaints
the system and nothing has to restart.
`

func zenithTheme(args []string) int {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, themeUsage)
		return exitUsage
	}

	switch args[0] {
	case "generate":
		return themeGenerate(args[1:])
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
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "zenith theme generate: needs one image")
		return exitUsage
	}
	path := args[0]

	palette, err := theme.Extract(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "zenith theme:", err)
		return exitFail
	}

	// Checked before the write, not after. A palette that is written and then
	// found unreadable has already been adopted by whatever watches the file.
	if problems := palette.Check(); len(problems) != 0 {
		fmt.Fprintf(os.Stderr,
			"zenith theme: refusing to write a palette that is not readable:\n")
		for _, problem := range problems {
			fmt.Fprintf(os.Stderr, "  %s\n", problem)
		}
		return exitFail
	}

	out := themeOutputPath()
	if err := theme.Write(out, palette); err != nil {
		fmt.Fprintln(os.Stderr, "zenith theme:", err)
		return exitFail
	}

	printPalette(palette)
	fmt.Printf("\nwritten to %s\n", out)
	return exitOK
}

func themeCurrent() int {
	path := themeOutputPath()
	palette, err := theme.Load(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "zenith theme current:", err)
		fmt.Fprintf(os.Stderr,
			"  no palette has been generated yet; run `zenith theme generate IMAGE`\n")
		return exitFail
	}
	printPalette(palette)
	return exitOK
}

// themeCheck exists so the shell check can call the same assertion the generator
// does, rather than reimplementing the thresholds.
func themeCheck() int {
	path := themeOutputPath()
	palette, err := theme.Load(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "zenith theme check:", err)
		return exitFail
	}

	if problems := palette.Check(); len(problems) != 0 {
		fmt.Fprintf(os.Stderr, "zenith theme check: %d problem(s):\n", len(problems))
		for _, problem := range problems {
			fmt.Fprintf(os.Stderr, "  %s\n", problem)
		}
		return exitFail
	}

	fmt.Printf("palette from %s is readable at every pair\n", palette.Source)
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

func printPalette(palette theme.Palette) {
	dark := "dark"
	if palette.Luminance >= 0.35 {
		dark = "light"
	}
	fmt.Printf("from      %s  (%s, mean luminance %.3f)\n",
		palette.Source, dark, palette.Luminance)
	rows := []struct{ name, value string }{
		{"background", palette.Background.Hex()},
		{"surface", palette.Surface.Hex()},
		{"text", palette.Text.Hex()},
		{"dim text", palette.TextDim.Hex()},
		{"border", palette.Border.Hex()},
		{"accent", palette.Accent.Hex()},
		{"on accent", palette.OnAccent.Hex()},
	}
	for _, row := range rows {
		fmt.Printf("%-10s %s\n", row.name, row.value)
	}
}
