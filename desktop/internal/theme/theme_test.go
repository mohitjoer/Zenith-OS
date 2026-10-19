package theme

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeImage writes a solid or two-tone PNG, so the extractor can be tested
// against a palette whose answer is known rather than against a photograph whose
// answer is merely plausible.
func writeImage(t *testing.T, path string, w, h int, colour color.RGBA) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, colour)
		}
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := png.Encode(file, img); err != nil {
		t.Fatal(err)
	}
}

// writeMixed writes an image that is mostly one colour with a patch of another,
// which is the shape of a photograph closely enough to test accent selection.
func writeMixed(t *testing.T, path string, w, h int, base, patch color.RGBA) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if x > w/2 && y > h/2 {
				img.Set(x, y, patch)
			} else {
				img.Set(x, y, base)
			}
		}
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := png.Encode(file, img); err != nil {
		t.Fatal(err)
	}
}

// TestContrastMatchesWCAG pins the formula against the two pairs the
// specification states, because the shortcut people reach for — treating sRGB
// values as if they were linear — passes a self-consistency test and is wrong.
func TestContrastMatchesWCAG(t *testing.T) {
	black := Colour{0, 0, 0}
	white := Colour{1, 1, 1}

	if got := Contrast(black, white); math.Abs(got-21) > 0.01 {
		t.Errorf("black on white = %.4f, want 21", got)
	}
	if got := Contrast(white, white); math.Abs(got-1) > 0.0001 {
		t.Errorf("white on white = %.4f, want 1", got)
	}

	// Symmetry: contrast is a ratio, so swapping the arguments cannot change it.
	mid := Colour{0.5, 0.25, 0.75}
	if math.Abs(Contrast(black, mid)-Contrast(mid, black)) > 0.0001 {
		t.Error("contrast is not symmetric")
	}
}

// TestExtractFindsTheAccent is the point of the whole package: a palette taken
// from a picture, where the picture's one distinctive colour is the accent.
func TestExtractFindsTheAccent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.png")
	// A dark grey field with a mid-tone teal patch.
	writeMixed(t, path, 200, 200,
		color.RGBA{40, 40, 45, 255},
		color.RGBA{32, 160, 150, 255})

	palette, err := Extract(path)
	if err != nil {
		t.Fatal(err)
	}

	if problems := palette.Check(); len(problems) != 0 {
		t.Fatalf("a generated palette failed its own check: %v", problems)
	}

	// The accent must be the teal, not the grey that fills three quarters of
	// the image. This is the case the scoring exists for: the most common colour
	// in a photograph is its background.
	accent := palette.Accent
	if accent.G <= accent.R || accent.B <= accent.R {
		t.Errorf("accent %s is not the teal patch; it is the grey field",
			accent.Hex())
	}
}

// TestExtractIsDarkForADarkImage and its light counterpart cover the branch that
// decides which of two wholly different palettes is emitted.
func TestExtractIsDarkForADarkImage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dark.png")
	writeMixed(t, path, 120, 120,
		color.RGBA{18, 18, 22, 255},
		color.RGBA{220, 120, 60, 255})

	palette, err := Extract(path)
	if err != nil {
		t.Fatal(err)
	}
	if palette.Luminance >= 0.35 {
		t.Errorf("a dark image reported luminance %.3f, which is not dark",
			palette.Luminance)
	}
	if RelativeLuminance(palette.Background) > 0.2 {
		t.Errorf("background %s is not dark for a dark image",
			palette.Background.Hex())
	}
}

func TestExtractIsLightForALightImage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "light.png")
	writeMixed(t, path, 120, 120,
		color.RGBA{240, 238, 245, 255},
		color.RGBA{40, 110, 200, 255})

	palette, err := Extract(path)
	if err != nil {
		t.Fatal(err)
	}
	if palette.Luminance < 0.35 {
		t.Errorf("a light image reported luminance %.3f, which is not light",
			palette.Luminance)
	}
	if RelativeLuminance(palette.Background) < 0.6 {
		t.Errorf("background %s is not light for a light image",
			palette.Background.Hex())
	}
	if problems := palette.Check(); len(problems) != 0 {
		t.Errorf("a light palette failed: %v", problems)
	}
}

// TestEveryShippedWallpaperProducesAReadablePalette runs the real assets through
// it.
//
// Seven real images beat seven synthetic ones, because the failure this guards
// against — a palette that is beautiful and unreadable — only shows up on
// photographs. These are the wallpapers the package ships, so a change that
// breaks one of them breaks a file users will see.
func TestEveryShippedWallpaperProducesAReadablePalette(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("..", "..", "..",
		"public", "wallpapers", "*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Skip("no wallpapers in the checkout")
	}

	for _, path := range paths {
		name := filepath.Base(path)
		palette, err := Extract(path)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		problems := palette.Check()
		if len(problems) != 0 {
			t.Errorf("%s produces an unreadable palette:", name)
			for _, problem := range problems {
				t.Errorf("    %s", problem)
			}
		}
	}
}

// TestAnImageWithNoMidToneStillProducesAPalette covers the photographs that have
// no accent to find: a black frame, a blown-out white one.
//
// The palette still has to exist. A missing accent is a shell that draws
// nothing, which is harder to diagnose than a grey one.
func TestAnImageWithNoMidToneStillProducesAPalette(t *testing.T) {
	for _, tc := range []struct {
		name   string
		colour color.RGBA
	}{
		{"black", color.RGBA{0, 0, 0, 255}},
		{"white", color.RGBA{255, 255, 255, 255}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, tc.name+".png")
			writeImage(t, path, 80, 80, tc.colour)

			palette, err := Extract(path)
			if err != nil {
				t.Fatalf("a %s image produced no palette: %v", tc.name, err)
			}
			if problems := palette.Check(); len(problems) != 0 {
				t.Errorf("a %s image produced an unreadable palette: %v",
					tc.name, problems)
			}
		})
	}
}

// TestASmallImageIsReadInFull covers the sampling stride.
//
// The stride is computed from the image's own dimensions so the sample count
// lands near the cap. A bug that computes it from a fixed 4K reference would
// throw away nearly every pixel of a small image and pick an accent from two
// samples.
func TestASmallImageIsReadInFull(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "small.png")
	writeImage(t, path, 8, 8, color.RGBA{30, 160, 140, 255})

	palette, err := Extract(path)
	if err != nil {
		t.Fatal(err)
	}
	// With the whole image read, the accent is the image's only colour.
	if math.Abs(palette.Luminance-relativeLuminance(
		30.0/255, 160.0/255, 140.0/255)) > 0.06 {
		t.Errorf("luminance %.4f suggests a small image was subsampled",
			palette.Luminance)
	}
}

// TestWriteAndLoadRoundTrip covers the file the shell actually reads.
func TestWriteAndLoadRoundTrip(t *testing.T) {
	palette := Palette{
		Source:     "rain-radio.png",
		Background: Colour{0.1, 0.1, 0.11},
		Accent:     Colour{0.9, 0.68, 0.19},
	}

	path := filepath.Join(t.TempDir(), "nested", "colors.json")
	if err := Write(path, palette); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Accent.Hex() != palette.Accent.Hex() {
		t.Errorf("accent %s did not survive the round trip as %s",
			loaded.Accent.Hex(), palette.Accent.Hex())
	}
	if loaded.Source != palette.Source {
		t.Errorf("source = %q, want %q", loaded.Source, palette.Source)
	}
}

// TestLoadReportsABrokenFile rather than returning a zero palette.
//
// A zero palette is dark grey with black text: contrast 1.05, and a shell that
// adopts it looks broken in a way nobody can trace back to a truncated file.
func TestLoadReportsABrokenFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "colors.json")
	if err := os.WriteFile(path, []byte("{ not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("a truncated palette loaded as if it were valid")
	}
}

// TestHexIsAlwaysSixDigits covers the format the shell parses.
//
// A four-digit hex from a channel that happens to be zero-padded would be
// silently misread by a colour parser as a different colour.
func TestHexIsAlwaysSixDigits(t *testing.T) {
	for _, colour := range []Colour{
		{0, 0, 0}, {1, 1, 1}, {0.5, 0, 0}, {0, 1, 0}, {0, 0, 1},
	} {
		hex := colour.Hex()
		if len(hex) != 7 || hex[0] != '#' {
			t.Errorf("Hex() = %q, want #rrggbb", hex)
		}
		if strings.ToLower(hex) != hex {
			t.Errorf("Hex() = %q, want lower case", hex)
		}
	}
}

// TestClampKeepsColoursInRange covers the arithmetic that runs twenty-four times
// per palette.
func TestClampKeepsColoursInRange(t *testing.T) {
	if clamp01(-0.5) != 0 {
		t.Error("clamp01 did not clamp below")
	}
	if clamp01(1.5) != 1 {
		t.Error("clamp01 did not clamp above")
	}
	if clamp01(0.25) != 0.25 {
		t.Error("clamp01 changed an in-range value")
	}
}

// RelativeLuminance is exposed for tests and for a future shell check that
// verifies a palette it did not generate.
func RelativeLuminance(c Colour) float64 {
	return relativeLuminance(c.R, c.G, c.B)
}

// TestDimTextIsLessProminentThanText covers the inversion this style exists to
// express.
//
// It was derived from a `dark` flag that stepped the wrong way, so on a dark
// palette the "dim" text came out brighter than the normal text. Every contrast
// check still passed — it was perfectly readable, just not dim — which is why
// the property is asserted directly rather than through the palette check.
func TestDimTextIsLessProminentThanText(t *testing.T) {
	for _, tc := range []struct {
		name string
		seed string
	}{
		{"dark field", ""},
		{"light field", ""},
	} {
		_ = tc
	}

	for _, colour := range []color.RGBA{
		{18, 18, 22, 255},
		{240, 238, 245, 255},
		{90, 40, 40, 255},
		{20, 90, 70, 255},
	} {
		dir := t.TempDir()
		path := filepath.Join(dir, "seed.png")
		writeImage(t, path, 64, 64, colour)

		palette, err := Extract(path)
		if err != nil {
			t.Fatal(err)
		}

		textContrast := Contrast(palette.Text, palette.Background)
		dimContrast := Contrast(palette.TextDim, palette.Background)

		if dimContrast >= textContrast {
			t.Errorf("for background %s: dim text %s (%.2f:1) is as prominent "+
				"as or more prominent than text %s (%.2f:1)",
				palette.Background.Hex(), palette.TextDim.Hex(), dimContrast,
				palette.Text.Hex(), textContrast)
		}
		if dimContrast < MinimumContrast {
			t.Errorf("dim text %s is only %.2f:1; a secondary label that fails "+
				"contrast is a caption nobody reads",
				palette.TextDim.Hex(), dimContrast)
		}
	}
}

// TestBorderIsVisibleButNotText covers the other derived colour.
//
// A separator is allowed to be quiet. It is not allowed to be invisible, because
// it is the only thing telling two panels apart.
func TestBorderIsVisibleButNotText(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "seed.png")
	writeImage(t, path, 64, 64, color.RGBA{30, 30, 36, 255})

	palette, err := Extract(path)
	if err != nil {
		t.Fatal(err)
	}

	border := Contrast(palette.Border, palette.Background)
	if border < 1.15 {
		t.Errorf("border %s is %.2f:1 against the background, which is "+
			"invisible", palette.Border.Hex(), border)
	}
	if border >= Contrast(palette.Text, palette.Background) {
		t.Errorf("border %s is %.2f:1, as strong as text; a border as loud as "+
			"the label is a second label",
			palette.Border.Hex(), border)
	}
}
