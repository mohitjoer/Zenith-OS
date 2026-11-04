package theme

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

// A scheme is the role set the shell and the app templates are drawn from: the
// Material 3 names, in the JSON shape matugen prints, built from one seed colour.
//
// It is not matugen's arithmetic. matugen solves colours in CAM16 (HCT); this
// builds each palette in CIE LCh, which shares the one property the roles depend
// on: a tone is L*, so the tone distance between a role and the surface it sits
// on fixes its contrast, whatever the hue. Hues and chromas therefore differ a
// little from matugen's for the same wallpaper, and every contrast pair holds.

// The six palettes of a scheme, in the order roles refer to them.
const (
	palPrimary = iota
	palSecondary
	palTertiary
	palNeutral
	palNeutralVariant
	palError
)

var paletteNames = [...]string{"primary", "secondary", "tertiary", "neutral", "neutral_variant", "error"}

// toneStops are the tones each palette publishes as a ramp.
var toneStops = [...]int{0, 5, 10, 15, 20, 25, 30, 35, 40, 50, 60, 70, 80, 90, 95, 98, 99, 100}

// hueChroma is one palette: where it sits on the hue wheel, relative to the seed,
// and how colourful it is.
type hueChroma struct{ turn, chroma float64 }

// schemes lists the variants the Hub offers. Each builds the five seed-derived
// palettes from the seed's own chroma. Variants whose hue rotation matugen
// reads off a table (vibrant, expressive) use one rotation here, and fidelity
// and content approximate the complement and analogous hues.
var schemes = map[string]func(seed float64) [5]hueChroma{
	"monochrome": func(float64) [5]hueChroma { return [5]hueChroma{} },
	"tonal-spot": func(float64) [5]hueChroma {
		return [5]hueChroma{{0, 36}, {0, 16}, {60, 24}, {0, 6}, {0, 8}}
	},
	"neutral": func(float64) [5]hueChroma {
		return [5]hueChroma{{0, 12}, {0, 8}, {0, 16}, {0, 2}, {0, 2}}
	},
	"vibrant": func(float64) [5]hueChroma {
		return [5]hueChroma{{0, 200}, {15, 24}, {30, 32}, {0, 10}, {0, 10}}
	},
	"expressive": func(float64) [5]hueChroma {
		return [5]hueChroma{{240, 40}, {60, 24}, {120, 32}, {15, 8}, {15, 12}}
	},
	"fruit-salad": func(float64) [5]hueChroma {
		return [5]hueChroma{{-50, 48}, {-50, 36}, {0, 36}, {0, 10}, {0, 16}}
	},
	"rainbow": func(float64) [5]hueChroma {
		return [5]hueChroma{{0, 48}, {0, 16}, {60, 24}, {0, 0}, {0, 0}}
	},
	"fidelity": func(c float64) [5]hueChroma {
		return [5]hueChroma{{0, c}, {0, math.Max(c-32, c*0.5)}, {180, c}, {0, c / 8}, {0, c/8 + 4}}
	},
	"content": func(c float64) [5]hueChroma {
		return [5]hueChroma{{0, c}, {0, math.Max(c-32, c*0.5)}, {45, c}, {0, c / 8}, {0, c/8 + 4}}
	},
}

// SchemeNames is every scheme `Generate` accepts, sorted.
func SchemeNames() []string {
	names := make([]string, 0, len(schemes))
	for name := range schemes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// DefaultScheme is the one used when nobody asks for another. The design system
// is black, grey and white, so the wallpaper drives tone (light or dark, contrast,
// surface lift) and no hue; a coloured scheme is something a person picks.
const DefaultScheme = "monochrome"

// The error palette is red in every scheme: error is one of the design system's
// two permitted colours. Hue and chroma (LCh) are those of Material's T40 error,
// #ba1a1a, so the ramp lands on its known tones.
const (
	errorHue    = 35
	errorChroma = 74
)

// role is one Material colour: which palette it is drawn from and its tone in a
// dark and a light scheme.
//
// kind is how the contrast knob treats it: 's' for a surface, 'f' for a
// foreground that sits on surfaces, 0 for a pair member (an "on" colour, a
// container) whose tone is tied to its partner and stays put.
type role struct {
	name        string
	palette     int
	dark, light float64
	kind        byte
}

var roles = buildRoles()

func buildRoles() []role {
	var rs []role
	// i is the palette: palPrimary, palSecondary, palTertiary.
	for i, n := range []string{"primary", "secondary", "tertiary"} {
		rs = append(rs,
			role{n, i, 80, 40, 'f'}, role{"on_" + n, i, 20, 100, 0},
			role{n + "_container", i, 30, 90, 0}, role{"on_" + n + "_container", i, 90, 10, 0},
			role{n + "_fixed", i, 90, 90, 0}, role{n + "_fixed_dim", i, 80, 80, 0},
			role{"on_" + n + "_fixed", i, 10, 10, 0}, role{"on_" + n + "_fixed_variant", i, 30, 30, 0})
	}
	return append(rs,
		role{"inverse_primary", palPrimary, 40, 80, 0},
		role{"surface_tint", palPrimary, 80, 40, 'f'},
		role{"error", palError, 80, 40, 'f'}, role{"on_error", palError, 20, 100, 0},
		role{"error_container", palError, 30, 90, 0}, role{"on_error_container", palError, 90, 10, 0},
		role{"background", palNeutral, 6, 98, 's'}, role{"on_background", palNeutral, 90, 10, 'f'},
		role{"surface", palNeutral, 6, 98, 's'}, role{"surface_dim", palNeutral, 6, 87, 's'},
		role{"surface_bright", palNeutral, 24, 98, 's'},
		role{"surface_container_lowest", palNeutral, 4, 100, 's'},
		role{"surface_container_low", palNeutral, 10, 96, 's'},
		role{"surface_container", palNeutral, 12, 94, 's'},
		role{"surface_container_high", palNeutral, 17, 92, 's'},
		role{"surface_container_highest", palNeutral, 22, 90, 's'},
		role{"surface_variant", palNeutralVariant, 30, 90, 's'},
		role{"on_surface", palNeutral, 90, 10, 'f'},
		role{"on_surface_variant", palNeutralVariant, 80, 30, 'f'},
		role{"outline", palNeutralVariant, 60, 50, 'f'},
		role{"outline_variant", palNeutralVariant, 30, 80, 'f'},
		role{"inverse_surface", palNeutral, 90, 20, 0},
		role{"inverse_on_surface", palNeutral, 20, 95, 0},
		role{"shadow", palNeutral, 0, 0, 0}, role{"scrim", palNeutral, 0, 0, 0})
}

// adjustTone applies the contrast knob (-1..1) to a tone. More contrast pushes
// surfaces toward the edge and foregrounds away from them; less pulls both toward
// the middle. Pair members are left alone so an "on" colour keeps its distance
// from the colour it is read on.
func adjustTone(tone float64, kind byte, dark bool, contrast float64) float64 {
	if contrast == 0 || kind == 0 {
		return tone
	}
	if contrast < 0 {
		return tone + -contrast*0.35*(50-tone)
	}
	edge := 0.0
	if (kind == 's') == !dark {
		edge = 100
	}
	return tone + contrast*0.5*(edge-tone)
}

// Options are the knobs matugen takes, under the same names.
type Options struct {
	Scheme         string  // "tonal-spot", "scheme-monochrome", ...; empty is DefaultScheme
	Contrast       float64 // -1..1
	LightnessDark  float64 // matugen's --lightness-dark
	LightnessLight float64 // matugen's --lightness-light
}

// Scheme is a generated scheme: every role in a dark and a light variant.
type Scheme struct {
	Name        string
	Source      Colour
	Dark, Light map[string]Colour
	ramps       [6][]Colour // one per palette, a colour for each of toneStops
}

// Generate builds a scheme from a seed colour. The same seed and options always
// give the same colours.
func Generate(seed Colour, o Options) (*Scheme, error) {
	name := strings.TrimPrefix(o.Scheme, "scheme-")
	if name == "" {
		name = DefaultScheme
	}
	build, ok := schemes[name]
	if !ok {
		return nil, fmt.Errorf("unknown scheme %q (want one of %s)", o.Scheme, strings.Join(SchemeNames(), ", "))
	}

	_, seedChroma, seedHue := lch(seed)
	var pal [6]hueChroma
	// A grey wallpaper has no hue to take; without this every scheme would
	// invent one from the seed's rounding noise.
	if seedChroma >= 4 {
		built := build(seedChroma)
		copy(pal[:5], built[:])
	}
	pal[palError] = hueChroma{0, errorChroma}

	hueOf := func(i int) float64 {
		if i == palError {
			return errorHue
		}
		return seedHue + pal[i].turn
	}

	s := &Scheme{
		Name:   name,
		Source: seed,
		Dark:   make(map[string]Colour, len(roles)+1),
		Light:  make(map[string]Colour, len(roles)+1),
	}
	for _, r := range roles {
		p := pal[r.palette]
		s.Dark[r.name] = toneColour(hueOf(r.palette), p.chroma, adjustTone(r.dark, r.kind, true, o.Contrast))
		s.Light[r.name] = toneColour(hueOf(r.palette), p.chroma, adjustTone(r.light, r.kind, false, o.Contrast))
	}
	for i := range pal {
		for _, t := range toneStops {
			s.ramps[i] = append(s.ramps[i], toneColour(hueOf(i), pal[i].chroma, float64(t)))
		}
	}
	for name, c := range s.Dark {
		s.Dark[name] = lightness(c, o.LightnessDark, true)
	}
	for name, c := range s.Light {
		s.Light[name] = lightness(c, o.LightnessLight, false)
	}
	s.Dark["source_color"], s.Light["source_color"] = seed, seed
	return s, nil
}

// lightness is matugen's --lightness-dark/--lightness-light, as it computes it on
// the 8-bit channels: dark schemes rotate the identity line about white, light
// schemes scale toward it, and black stays black.
func lightness(c Colour, amount float64, dark bool) Colour {
	if amount == 0 {
		return c
	}
	r, g, b := math.Round(c.R*255), math.Round(c.G*255), math.Round(c.B*255)
	mean := (r + g + b) / 3
	if mean == 0 {
		return c
	}
	adj := 1 + amount
	if dark {
		adj = (mean/255*(1-amount) + amount) / mean * 255
	}
	scale := func(v float64) float64 { return math.Trunc(math.Min(255, math.Max(0, v*adj))) / 255 }
	return Colour{scale(r), scale(g), scale(b)}
}

// Matugen is the document `matugen --json hex` prints, which is what the daemon
// parses: every role under dark, light and default (the requested mode), and the
// six tonal ramps.
func (s *Scheme) Matugen(dark bool) map[string]any {
	entry := func(c Colour) map[string]string { return map[string]string{"color": c.Hex()} }
	colors := make(map[string]any, len(s.Dark))
	for name, d := range s.Dark {
		def := s.Light[name]
		if dark {
			def = d
		}
		colors[name] = map[string]any{"dark": entry(d), "light": entry(s.Light[name]), "default": entry(def)}
	}
	palettes := make(map[string]any, len(s.ramps))
	for i, ramp := range s.ramps {
		tones := make(map[string]any, len(ramp))
		for j, c := range ramp {
			tones[strconv.Itoa(toneStops[j])] = entry(c)
		}
		palettes[paletteNames[i]] = tones
	}
	mode := "light"
	if dark {
		mode = "dark"
	}
	return map[string]any{
		"colors": colors, "palettes": palettes,
		"mode": mode, "is_dark_mode": dark, "scheme": "scheme-" + s.Name,
	}
}

// Flat is one mode's roles as role name to "#rrggbb": the palette the base16
// and shell conversions below start from.
func (s *Scheme) Flat(dark bool) map[string]string {
	src := s.Light
	if dark {
		src = s.Dark
	}
	flat := make(map[string]string, len(src))
	for name, c := range src {
		flat[name] = c.Hex()
	}
	return flat
}

// Tones is the file Ink.qml reads: ramp name (camelCase) to tone to "#rrggbb".
func (s *Scheme) Tones() map[string]map[string]string {
	out := make(map[string]map[string]string, len(s.ramps))
	for i, ramp := range s.ramps {
		name := paletteNames[i]
		if name == "neutral_variant" {
			name = "neutralVariant"
		}
		tones := make(map[string]string, len(ramp))
		for j, c := range ramp {
			tones[strconv.Itoa(toneStops[j])] = c.Hex()
		}
		out[name] = tones
	}
	return out
}

// shellRoleKeys maps the snake_case Material roles to the camelCase keys the
// shell's Theme, Scheme and Tokens singletons read. Ported from Ryoku's
// matugenRoleKeys.
var shellRoleKeys = [][2]string{
	{"surface", "surface"},
	{"surface_variant", "surfaceVariant"},
	{"surface_container_lowest", "surfaceContainerLowest"},
	{"surface_container_low", "surfaceContainerLow"},
	{"surface_container", "surfaceContainer"},
	{"surface_container_high", "surfaceContainerHigh"},
	{"surface_container_highest", "surfaceContainerHighest"},
	{"inverse_surface", "inverseSurface"},
	{"inverse_on_surface", "inverseOnSurface"},
	{"surface_tint", "surfaceTint"},
	{"primary", "primary"},
	{"primary_container", "primaryContainer"},
	{"secondary", "secondary"},
	{"secondary_container", "secondaryContainer"},
	{"tertiary", "tertiary"},
	{"tertiary_container", "tertiaryContainer"},
	{"error", "error"},
	{"error_container", "errorContainer"},
	{"outline", "outline"},
	{"outline_variant", "outlineVariant"},
	{"on_surface", "onSurface"},
	{"on_surface_variant", "onSurfaceVariant"},
	{"on_primary", "onPrimary"},
	{"on_primary_container", "onPrimaryContainer"},
	{"on_secondary", "onSecondary"},
	{"on_secondary_container", "onSecondaryContainer"},
	{"on_tertiary", "onTertiary"},
	{"on_tertiary_container", "onTertiaryContainer"},
	{"on_error", "onError"},
	{"on_error_container", "onErrorContainer"},
	{"shadow", "shadow"},
	{"scrim", "scrim"},
}

// baseSixteen maps the Material palette onto the sixteen terminal-style keys the
// border and terminal templates read. Ported from Ryoku's matugenBase16: the
// accent slots take the main accent roles, never a container, which inverts
// between modes and washes out on a light surface.
func baseSixteen(pal map[string]string) map[string]string {
	pick := func(k, fallback string) string {
		if v := pal[k]; v != "" {
			return v
		}
		return fallback
	}
	bg := pick("surface", pick("background", "#121212"))
	fg := pick("on_surface", pick("on_background", "#e6e6e6"))
	primary := pick("primary", "#a8c7fa")
	secondary := pick("secondary", "#7cacf8")
	tertiary := pick("tertiary", "#ffb4a9")
	errc := pick("error", "#ffb4ab")
	outline := pick("outline", "#8e918f")
	return map[string]string{
		"background": bg, "foreground": fg, "cursor": fg,
		"color0": bg, "color1": errc, "color2": primary, "color3": tertiary,
		"color4": secondary, "color5": primary,
		"color6": tertiary, "color7": fg,
		"color8": outline, "color9": errc,
		"color10": primary, "color11": tertiary, "color12": secondary,
		"color13": primary, "color14": outline,
		"color15": pick("on_primary_container", fg),
	}
}

// ShellColors is colors.json: the base16 keys plus the camelCase roles. This is
// the file the shell reads (see docs/theming.md); the daemon writes the same
// shape from the same input.
func ShellColors(pal map[string]string) map[string]string {
	out := baseSixteen(pal)
	for _, kv := range shellRoleKeys {
		if v := pal[kv[0]]; v != "" {
			out[kv[1]] = v
		}
	}
	return out
}

// CheckRoles reports every foreground/surface pair in the shell's colors.json
// below the design system's floor: 4.5:1 for text, 3:1 for shapes.
func CheckRoles(palette map[string]Colour) []string {
	pairs := []struct {
		fg, bg string
		min    float64
	}{
		{"onSurface", "surface", MinimumContrast},
		{"onSurfaceVariant", "surface", MinimumContrast},
		{"onPrimary", "primary", MinimumContrast},
		{"onSecondary", "secondary", MinimumContrast},
		{"onTertiary", "tertiary", MinimumContrast},
		{"onError", "error", MinimumContrast},
		{"onPrimaryContainer", "primaryContainer", MinimumContrast},
		{"onSecondaryContainer", "secondaryContainer", MinimumContrast},
		{"onTertiaryContainer", "tertiaryContainer", MinimumContrast},
		{"onErrorContainer", "errorContainer", MinimumContrast},
		{"inverseOnSurface", "inverseSurface", MinimumContrast},
		{"primary", "surface", 3.0},
		{"outline", "surface", 3.0},
	}
	var problems []string
	for _, p := range pairs {
		fg, haveFg := palette[p.fg]
		bg, haveBg := palette[p.bg]
		if !haveFg || !haveBg {
			problems = append(problems, fmt.Sprintf("%s on %s: a role is missing", p.fg, p.bg))
			continue
		}
		if ratio := Contrast(fg, bg); ratio < p.min {
			problems = append(problems, fmt.Sprintf("%s on %s is %.2f:1, below the %.1f:1 required (%s on %s)",
				p.fg, p.bg, ratio, p.min, fg.Hex(), bg.Hex()))
		}
	}
	return problems
}

// lch reads a colour as CIE L*, chroma and hue (degrees).
func lch(c Colour) (l, chroma, hue float64) {
	r, g, b := linearise(c.R), linearise(c.G), linearise(c.B)
	x := (0.4124564*r + 0.3575761*g + 0.1804375*b) / 0.95047
	y := 0.2126729*r + 0.7151522*g + 0.0721750*b
	z := (0.0193339*r + 0.1191920*g + 0.9503041*b) / 1.08883
	f := func(t float64) float64 {
		if t > 216.0/24389.0 {
			return math.Cbrt(t)
		}
		return (24389.0/27.0*t + 16) / 116
	}
	fx, fy, fz := f(x), f(y), f(z)
	a, bb := 500*(fx-fy), 200*(fy-fz)
	hue = math.Atan2(bb, a) * 180 / math.Pi
	if hue < 0 {
		hue += 360
	}
	return 116*fy - 16, math.Hypot(a, bb), hue
}

// toneColour is the sRGB colour at CIE L* = tone with the given hue (degrees) and
// the largest chroma up to `chroma` the gamut holds at that tone. Lightness is
// never traded for chroma, so the contrast a tone promises survives.
func toneColour(hue, chroma, tone float64) Colour {
	if tone <= 0 {
		return Colour{}
	}
	if tone >= 100 {
		return Colour{1, 1, 1}
	}
	labInverse := func(t float64) float64 {
		if t3 := t * t * t; t3 > 216.0/24389.0 {
			return t3
		}
		return (116*t - 16) / (24389.0 / 27.0)
	}
	if chroma < 1e-6 {
		v := encode(labInverse((tone + 16) / 116))
		return Colour{v, v, v}
	}
	h := hue * math.Pi / 180
	at := func(c float64) (r, g, b float64, inGamut bool) {
		fy := (tone + 16) / 116
		x := 0.95047 * labInverse(fy+c*math.Cos(h)/500)
		y := labInverse(fy)
		z := 1.08883 * labInverse(fy-c*math.Sin(h)/200)
		r = 3.2404542*x - 1.5371385*y - 0.4985314*z
		g = -0.9692660*x + 1.8760108*y + 0.0415560*z
		b = 0.0556434*x - 0.2040259*y + 1.0572252*z
		const slack = 1e-4
		return r, g, b, r >= -slack && r <= 1+slack && g >= -slack && g <= 1+slack && b >= -slack && b <= 1+slack
	}
	r, g, b, ok := at(chroma)
	if !ok {
		lo, hi := 0.0, chroma
		for i := 0; i < 24; i++ {
			mid := (lo + hi) / 2
			if _, _, _, fits := at(mid); fits {
				lo = mid
			} else {
				hi = mid
			}
		}
		r, g, b, _ = at(lo)
	}
	return Colour{encode(clamp01(r)), encode(clamp01(g)), encode(clamp01(b))}
}

// encode is the sRGB transfer function, the inverse of linearise.
func encode(v float64) float64 {
	if v <= 0.0031308 {
		return v * 12.92
	}
	return 1.055*math.Pow(v, 1/2.4) - 0.055
}
