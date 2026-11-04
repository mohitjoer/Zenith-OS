// Package theme derives a colour palette from a wallpaper.
//
// The idea is that a desktop should take its colours from one image, so picking
// a wallpaper repaints the system instead of leaving a blue desktop with a green
// wallpaper. That is a small idea and most of the difficulty is in not getting it
// wrong: a palette extracted from a picture is usually unreadable, because a
// photograph has no idea what a foreground colour is.
//
// So this does not simply sort pixels by frequency. It picks an accent for being
// saturated and mid-tone, derives surfaces from the image's actual luminance
// distribution, and then *enforces* contrast: every foreground/background pair
// it emits is checked against WCAG and adjusted until it passes. A palette that
// is pretty and unreadable is worse than no palette, because it is adopted
// before anyone notices and then every label is the wrong shade of grey.
package theme

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Colour is a sRGB colour with components in 0..1.
//
// Components are floats rather than bytes because every derivation here is a
// computation in linear light, and rounding to bytes between each step is how
// palettes come out muddy. It is converted to bytes only on the way out.
type Colour struct {
	R, G, B float64
}

// MarshalJSON writes the colour as the string "#rrggbb", which is what a
// colour property in QML is.
//
// The struct's fields are three floats, so without this the file the shell reads
// carries `{"R":0.08,"G":0.09,"B":0.12}` for every colour, and Palette.apply
// assigns an object to a property declared as a string. QML reports that in a
// runtime nobody watches, and the fallback palette stays — the same failure the
// readonly assignments below were already causing, from the other end of the
// same file. The contract is in the doc comment on Palette: this is what the
// shell reads, so it is written the way the shell needs it.
func (c Colour) MarshalJSON() ([]byte, error) {
	return json.Marshal(c.Hex())
}

// UnmarshalJSON reads "#rrggbb", and also the object form this file used to
// carry — a palette generated before the writer changed is still a palette, and
// telling somebody to regenerate theirs because of a serialisation detail is a
// failure they cannot act on quickly.
func (c *Colour) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) > 0 && trimmed[0] == '"' {
		var hex string
		if err := json.Unmarshal(trimmed, &hex); err != nil {
			return err
		}
		parsed, err := parseHex(hex)
		if err != nil {
			return err
		}
		*c = parsed
		return nil
	}

	var raw struct{ R, G, B float64 }
	if err := json.Unmarshal(trimmed, &raw); err != nil {
		return err
	}
	*c = Colour{R: raw.R, G: raw.G, B: raw.B}
	return nil
}

// parseHex reads the "#rgb" and "#rrggbb" forms a colour property accepts.
func parseHex(s string) (Colour, error) {
	digits := strings.TrimPrefix(s, "#")
	if len(digits) == 3 {
		digits = string([]byte{
			digits[0], digits[0], digits[1], digits[1], digits[2], digits[2],
		})
	}
	if len(digits) != 6 {
		return Colour{}, fmt.Errorf("colour %q is not #rgb or #rrggbb", s)
	}
	var channels [3]float64
	for i := range channels {
		value, err := strconv.ParseUint(digits[i*2:i*2+2], 16, 8)
		if err != nil {
			return Colour{}, fmt.Errorf("colour %q: %w", s, err)
		}
		channels[i] = float64(value) / 255
	}
	return Colour{R: channels[0], G: channels[1], B: channels[2]}, nil
}

// Hex is the colour as "#rrggbb".
func (c Colour) Hex() string {
	return fmt.Sprintf("#%02x%02x%02x",
		byte(clamp01(c.R)*255+0.5),
		byte(clamp01(c.G)*255+0.5),
		byte(clamp01(c.B)*255+0.5))
}

// Palette is what the shell reads.
type Palette struct {
	// Source is the image it came from, for the report and for a stale check.
	Source string `json:"source"`

	// Background and Surface are the window colours. Surface sits above
	// Background, which is why it is derived separately rather than being a shade
	// of it applied at draw time: two panels at the same elevation need to be
	// told apart by colour here, and a shell that computes the difference has to
	// get the arithmetic right at every call site.
	Background Colour `json:"background"`
	Surface    Colour `json:"surface"`

	// Text, TextDim and Border are the foregrounds.
	Text    Colour `json:"text"`
	TextDim Colour `json:"text_dim"`
	Border  Colour `json:"border"`

	// Accent is the one colour that is allowed to be the image's own.
	Accent Colour `json:"accent"`
	// OnAccent is the only colour permitted on top of Accent.
	OnAccent Colour `json:"on_accent"`
	// Ranked is every candidate seed, best first, one per hue. Accent is
	// Ranked[0]; a scheme seeded from the nth wallpaper colour uses Ranked[n].
	Ranked []Colour `json:"-"`

	// Luminance is the image's mean relative luminance, kept because it decides
	// whether the palette is a dark one or a light one, and re-deriving it from
	// the emitted colours would not give the same answer.
	Luminance float64 `json:"luminance"`
}

// darkBelow is the mean relative luminance under which an image is a dark one.
const darkBelow = 0.35

// Dark reports whether the image is dark enough for a dark scheme.
func (p Palette) Dark() bool { return p.Luminance < darkBelow }

// Minimum contrast for any pair the palette emits.
//
// WCAG AA is 4.5:1 for body text. It is used rather than a lower figure because
// the alternative is discovering the threshold by looking at a screen with a
// bright wallpaper and squinting.
const MinimumContrast = 4.5

// maxSamples caps how many pixels are read.
//
// A 5760x3240 wallpaper is 18.7 million pixels. Sampling every one of them to
// build a histogram is minutes of work for a histogram that stops changing after
// the first few hundred thousand. The stride is chosen from the image's own
// dimensions so the sample count lands near the cap whatever the resolution, and
// so a small image is read in full rather than having its only few pixels thrown
// away by a stride computed for a 4K one.
const maxSamples = 24000

// decode reads what the standard library can, and hands anything else (webp,
// avif) to ffmpeg when it is installed: a wallpaper whose palette cannot be read
// leaves the desktop in the last wallpaper's colours without saying so.
func decode(path string) (image.Image, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err == nil {
		return img, nil
	}
	name := filepath.Base(path)
	ffmpeg, lookErr := exec.LookPath("ffmpeg")
	if lookErr != nil {
		return nil, fmt.Errorf("%s: %w (install ffmpeg to read this format)", name, err)
	}
	// Width capped so a 8K wallpaper does not become a 100 MB pipe; 2048 is
	// far more than the sampler below ever reads.
	frame, runErr := exec.Command(ffmpeg, "-v", "error", "-i", path, "-frames:v", "1",
		"-vf", `scale=min(iw\,2048):-2`, "-f", "image2pipe", "-c:v", "png", "-").Output()
	if runErr != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	if img, _, err = image.Decode(bytes.NewReader(frame)); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return img, nil
}

// Extract derives a palette from an image file.
//
// PNG and JPEG are the two formats that matter; both are in the standard
// library, so this needs no image dependency on a base layer that has no reason
// to carry one.
func Extract(path string) (Palette, error) {
	image, err := decode(path)
	if err != nil {
		return Palette{}, err
	}

	bounds := image.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	if width <= 0 || height <= 0 {
		return Palette{}, fmt.Errorf("%s has no pixels", filepath.Base(path))
	}

	stride := 1
	if width*height > maxSamples {
		stride = int(math.Ceil(math.Sqrt(float64(width*height) / maxSamples)))
		if stride < 1 {
			stride = 1
		}
	}

	histogram := map[uint32]int{}
	var sumLuminance, count float64

	for y := bounds.Min.Y; y < bounds.Max.Y; y += stride {
		for x := bounds.Min.X; x < bounds.Max.X; x += stride {
			r, g, b, _ := image.At(x, y).RGBA()
			// RGBA() returns 16-bit values already alpha-premultiplied.
			red := float64(r) / 0xffff
			green := float64(g) / 0xffff
			blue := float64(b) / 0xffff

			sumLuminance += relativeLuminance(red, green, blue)
			count++

			histogram[quantise(red, green, blue)]++
		}
	}

	if count == 0 {
		return Palette{}, fmt.Errorf("%s yielded no samples", filepath.Base(path))
	}

	mean := sumLuminance / count
	palette := Palette{
		Source:    filepath.Base(path),
		Luminance: round(mean, 4),
	}

	palette.Ranked = rankAccents(histogram, mean)
	palette.Accent = palette.Ranked[0]
	palette.OnAccent = onColour(palette.Accent)

	dark := palette.Dark()
	palette.Background = backgroundFor(mean, palette.Accent, dark)
	palette.Surface = surfaceFor(palette.Background, dark)

	palette.Text = readableOn(palette.Background)
	palette.TextDim = dim(palette.Text, palette.Background, dark)
	palette.Border = borderBetween(palette.Surface, palette.Background, dark)

	return palette, nil
}

// quantise packs a colour into a 5-bit-per-channel histogram key.
//
// Five bits rather than eight because a 4K photograph has millions of distinct
// colours and almost all of them occur exactly once, so an 8-bit histogram is
// mostly singleton buckets and "the most common colour" becomes whichever
// quantisation artefact appeared most often. Five bits merges close colours into
// the same bucket, which is what makes the count mean something.
func quantise(r, g, b float64) uint32 {
	const levels = 32
	ri := uint32(clamp01(r) * (levels - 1))
	gi := uint32(clamp01(g) * (levels - 1))
	bi := uint32(clamp01(b) * (levels - 1))
	return (ri << 10) | (gi << 5) | bi
}

// rankAccents ranks the colours the palette could take from the image, best
// first, one per hue. The first is the accent.
//
// Three things are weighed, and each rules out a whole class of bad accent:
//
//   - Saturation. A near-grey bucket is never the accent, however common it is,
//     because a grey accent is indistinguishable from the borders.
//   - Mid-tone brightness. The darkest and lightest buckets are usually the
//     shadow and the highlight of the picture; an accent taken from either is
//     unreadable on the surface it is put on.
//   - Population, as a tiebreak rather than as the primary term. The most common
//     colour in a photograph is its background, and an accent that matches the
//     background is not an accent.
func rankAccents(histogram map[uint32]int, meanLuminance float64) []Colour {
	type candidate struct {
		colour Colour
		score  float64
		key    uint32
	}

	var candidates []candidate
	total := 0
	for _, count := range histogram {
		total += count
	}

	for key, count := range histogram {
		red := float64((key>>10)&31) / 31
		green := float64((key>>5)&31) / 31
		blue := float64(key&31) / 31

		luminance := relativeLuminance(red, green, blue)
		chroma := maxOf(red, green, blue) - minOf(red, green, blue)

		// Distance from mid-grey, 0 at the extremes and 1 at the middle.
		midTone := 1 - math.Abs(luminance-0.5)*2

		// Skew toward the side of the image's own brightness, so a dark
		// photograph does not produce an accent that disappears on its own dark
		// surface. The weight is small: this is a preference, not a rule.
		brightnessBias := 0.75 + 0.25*midTone

		population := float64(count) / float64(total)

		score := chroma * midTone * brightnessBias *
			(0.5 + 0.5*math.Sqrt(population))

		// A very dark or very light candidate is rejected outright rather than
		// scored down, because there are photographs where every bucket is one
		// or the other and scoring would pick the least-bad of them.
		if luminance < 0.08 || luminance > 0.92 {
			continue
		}

		candidates = append(candidates, candidate{
			colour: Colour{red, green, blue},
			score:  score,
			key:    key,
		})
	}

	if len(candidates) == 0 {
		// A photograph with no mid-tone at all — a black frame, a white one.
		// The answer has to be stated rather than left empty: the shell would
		// otherwise read a missing accent and draw nothing. Grey, not an
		// invented blue: the picture has no hue to take.
		if meanLuminance >= 0.5 {
			return []Colour{{0.15, 0.15, 0.18}}
		}
		return []Colour{{0.62, 0.62, 0.62}}
	}

	// The histogram is a map, so equal scores arrive in a different order on
	// every run; the key makes the order total and the output reproducible.
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		return candidates[i].key < candidates[j].key
	})

	// Neighbouring buckets of one hue are one colour to the eye, so the list
	// keeps the best of each hue and `--source-color-index` walks distinct ones.
	var ranked []Colour
	var hues []float64
pick:
	for _, c := range candidates {
		_, _, hue := lch(c.colour)
		for _, h := range hues {
			if d := math.Abs(hue - h); math.Min(d, 360-d) < 30 {
				continue pick
			}
		}
		ranked = append(ranked, c.colour)
		hues = append(hues, hue)
		if len(ranked) == 5 {
			break
		}
	}
	return ranked
}

// backgroundFor derives the window background from the image's brightness.
//
// Tinted toward the accent rather than toward the image's average, because a
// background at the image's own average luminance is muddy: photographs average
// out to a mid-grey that reads as neither light nor dark, and a desktop built on
// it looks broken rather than tinted.
func backgroundFor(meanLuminance float64, accent Colour, dark bool) Colour {
	tint := accent
	if dark {
		// Keep a trace of the hue without letting it lift the background into
		// something that is no longer dark.
		return Colour{
			0.055 + 0.05*tint.R,
			0.060 + 0.05*tint.G,
			0.075 + 0.05*tint.B,
		}
	}
	return Colour{
		0.965 - 0.04*tint.R,
		0.955 - 0.04*tint.G,
		0.975 - 0.04*tint.B,
	}
}

// surfaceFor is background lifted or dropped by one step.
//
// One step, not a computed difference: the gap has to be visible at a glance and
// the arithmetic for "visible" is a judgement, not a formula.
func surfaceFor(background Colour, dark bool) Colour {
	delta := 0.045
	if !dark {
		delta = -0.035
	}
	return Colour{
		clamp01(background.R + delta),
		clamp01(background.G + delta),
		clamp01(background.B + delta),
	}
}

// readableOn returns whichever of near-black and near-white contrasts better
// with the given background.
//
// Both candidates are then walked toward the background until they pass, so the
// answer is never "the best of two options, neither of which is readable".
func readableOn(background Colour) Colour {
	dark := Colour{0.055, 0.058, 0.065}
	light := Colour{0.96, 0.96, 0.97}

	if Contrast(dark, background) >= Contrast(light, background) {
		return ensureContrast(dark, background, false)
	}
	return ensureContrast(light, background, true)
}

// onColour is readableOn for the accent.
//
// A palette that passes on its background and fails on its accent produces a
// button nobody can read, and a button is the one place the accent is used at
// full strength.
func onColour(accent Colour) Colour {
	dark := Colour{0.03, 0.03, 0.04}
	light := Colour{0.99, 0.99, 1.0}
	if Contrast(dark, accent) >= Contrast(light, accent) {
		return ensureContrast(dark, accent, false)
	}
	return ensureContrast(light, accent, true)
}

// ensureContrast walks a colour away from its background until it passes.
//
// The step is in the background's own direction, so a colour on a dark surface
// gets lighter rather than getting a hue bolted on. Ten steps of 0.04 is enough
// to reach a usable pair from any background a photograph can produce, and the
// fallback at the end is a colour that is contrast-maximal by construction.
func ensureContrast(foreground, background Colour, light bool) Colour {
	candidate := foreground
	for i := 0; i < 24; i++ {
		if Contrast(candidate, background) >= MinimumContrast {
			return candidate
		}
		if light {
			candidate = Colour{
				clamp01(candidate.R + 0.04),
				clamp01(candidate.G + 0.04),
				clamp01(candidate.B + 0.04),
			}
		} else {
			candidate = Colour{
				clamp01(candidate.R - 0.04),
				clamp01(candidate.G - 0.04),
				clamp01(candidate.B - 0.04),
			}
		}
	}
	// Unreachable for any real pair, but a function that can return something
	// unreadable must not do so quietly.
	if light {
		return Colour{1, 1, 1}
	}
	return Colour{0, 0, 0}
}

// dim is Text, moved toward the background but never past the threshold.
//
// Direction is computed from the two colours rather than taken from a `dark`
// flag. It was a flag, and it was wrong on the dark palette: it stepped *away*
// from the background, which made the dim text brighter than the normal text and
// inverted the one distinction the style is for. The sign of the luminance
// difference says which way "toward" is, and there is no branch to get backwards.
func dim(text, background Colour, dark bool) Colour {
	// The step points from text towards the background, so its sign is the sign
	// of the luminance difference and nothing else. Nearly-white text on a dark
	// background steps down; nearly-black text on a light background steps up.
	textLuminance := relativeLuminance(text.R, text.G, text.B)
	backgroundLuminance := relativeLuminance(background.R, background.G, background.B)

	step := -0.05
	if textLuminance < backgroundLuminance {
		step = 0.05
	}

	candidate := Colour{
		clamp01(text.R + step),
		clamp01(text.G + step),
		clamp01(text.B + step),
	}
	for i := 0; i < 24; i++ {
		if Contrast(candidate, background) >= MinimumContrast {
			return candidate
		}
		candidate = Colour{
			clamp01(candidate.R + step),
			clamp01(candidate.G + step),
			clamp01(candidate.B + step),
		}
	}
	return text
}

// borderBetween is a subtle separation that is not a text colour.
//
// Borders sit between two surfaces rather than on a background, so they are
// measured against the surface they are drawn on.
func borderBetween(surface, background Colour, dark bool) Colour {
	delta := 0.03
	if !dark {
		delta = -0.025
	}
	candidate := Colour{
		clamp01(surface.R + delta),
		clamp01(surface.G + delta),
		clamp01(surface.B + delta),
	}
	// A border is allowed to be quieter than text, but not invisible: it is the
	// only thing separating two panels, and a separator that cannot be seen is
	// not one.
	for Contrast(candidate, background) < 1.15 && delta > 0 {
		delta *= 1.6
		candidate = Colour{
			clamp01(surface.R + delta),
			clamp01(surface.G + delta),
			clamp01(surface.B + delta),
		}
	}
	return candidate
}

// Contrast is the WCAG 2.1 relative contrast ratio between two colours.
//
// The ratio of relative luminances, not a Euclidean distance: contrast is about
// how much light each colour emits, and two colours that are far apart in RGB
// can be very close in luminance.
func Contrast(a, b Colour) float64 {
	la := relativeLuminance(a.R, a.G, a.B)
	lb := relativeLuminance(b.R, b.G, b.B)

	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

// relativeLuminance is the WCAG definition, in linear light.
//
// The sRGB transfer function has to be undone first. Skipping it — treating the
// encoded values as linear, which is the tempting shortcut — makes mid-tones far
// too bright and produces palettes that pass every contrast test on paper and
// fail on screen.
func relativeLuminance(r, g, b float64) float64 {
	return 0.2126*linearise(r) + 0.7152*linearise(g) + 0.0722*linearise(b)
}

func linearise(c float64) float64 {
	c = clamp01(c)
	if c <= 0.04045 {
		return c / 12.92
	}
	return math.Pow((c+0.055)/1.055, 2.4)
}

// Write saves a JSON document where the shell will find it.
//
// Written even when the file already says the same thing, because the shell
// watches the path rather than the content, and a shell that has to be restarted
// to see its own theme is a shell whose theming looks broken. Renamed into place
// for the same reason: a half-written file is a palette that fails to parse.
func Write(path string, document any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	body, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(body, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Load reads the shell's palette: role name to colour.
func Load(path string) (map[string]Colour, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var roles map[string]Colour
	if err := json.Unmarshal(body, &roles); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return roles, nil
}

// Check reports every pair in the palette that does not meet the threshold.
//
// Exported because the shell check calls it: a palette can be written by
// something other than this package, and the property that matters is about the
// colours, not about who produced them.
func (p Palette) Check() []string {
	var problems []string

	pairs := []struct {
		name       string
		foreground Colour
		background Colour
		minimum    float64
	}{
		{"text on background", p.Text, p.Background, MinimumContrast},
		{"text on surface", p.Text, p.Surface, MinimumContrast},
		{"dim text on background", p.TextDim, p.Background, MinimumContrast},
		{"on-accent on accent", p.OnAccent, p.Accent, MinimumContrast},
		{"accent on background", p.Accent, p.Background, 3.0},
	}

	for _, pair := range pairs {
		ratio := Contrast(pair.foreground, pair.background)
		if ratio < pair.minimum {
			problems = append(problems, fmt.Sprintf(
				"%s is %.2f:1, below the %.1f:1 required (%s on %s)",
				pair.name, ratio, pair.minimum,
				pair.foreground.Hex(), pair.background.Hex()))
		}
	}
	return problems
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func maxOf(values ...float64) float64 {
	best := values[0]
	for _, v := range values[1:] {
		if v > best {
			best = v
		}
	}
	return best
}

func minOf(values ...float64) float64 {
	best := values[0]
	for _, v := range values[1:] {
		if v < best {
			best = v
		}
	}
	return best
}

func round(v float64, places int) float64 {
	factor := math.Pow(10, float64(places))
	return math.Round(v*factor) / factor
}
