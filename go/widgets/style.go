package widgets

import (
	"fmt"

	"github.com/natyv-io/sdks/go/widgets/internal"
)

// Color is a resolved style color -- 0..1 floats, matching the stylesheet
// resolver's own convention (src/styling/Resolver.zig) and the SDF
// shader's eventual uniform layout. Deliberately not called RGBA/SDLColor
// -- this is the type `natyv prepare`'s codegen (src/styling/Codegen.zig)
// emits literal values into, so its name is the one guest code actually
// writes/reads.
//
// JSON tags exist so ApplyStyleToLayout can embed this directly into a
// Layout and let json.Marshal(layout) serialize it correctly on the wire
// (matching WidgetHostFunctions.zig's ColorRequest field names) --
// applyMergedStyle's own conversion to internal.ColorValue (used by the
// separate ApplyStyle(id, ...) path) never went through these tags at all,
// so adding them changes nothing about that existing path.
type Color struct {
	R float32 `json:"r"`
	G float32 `json:"g"`
	B float32 `json:"b"`
	A float32 `json:"a"`
}

// CornerRadius is one style token's resolved per-corner radius -- named
// fields (not a bare [4]float32) for the same readability reason Padding
// uses Left/Right/Top/Bottom instead of a raw array. Order is TL/TR/BR/BL,
// matching the stylesheet's real CSS-clockwise convention
// (src/styling/Resolver.zig). No JSON tags of its own -- the wire shape for
// corner radius is always a raw [4]float32 (see SetStyleRequest.corner_radius
// and Layout.CornerRadius), never a named object, so this type is only ever
// converted positionally, both here and in applyMergedStyle.
type CornerRadius struct {
	TopLeft, TopRight, BottomRight, BottomLeft float32
}

// Border is one style token's resolved border -- width + color are always
// both present together (Resolver.zig requires both when a `border` block
// is used at all), so Color is a plain value here, not a pointer like
// ResolvedStyle's own BackgroundColor (whose absence is meaningful on its
// own). JSON tags: see Color's own doc comment.
type Border struct {
	Width float32 `json:"width"`
	Color Color   `json:"color"`
}

// Gradient is one style token's resolved linear gradient. StartPos/EndPos
// are already-resolved 0..1 shape-space positions -- `natyv prepare`'s
// codegen bakes the stylesheet's named anchor (`topLeft`, etc.) into this
// plain coordinate at prepare time (Codegen.zig's own `anchorUV`), the
// same way a hex color is baked into floats, so nothing guest-facing here
// needs to know the anchor vocabulary exists. JSON tags: see Color's own
// doc comment.
type Gradient struct {
	StartPos   [2]float32 `json:"start_pos"`
	StartColor Color      `json:"start_color"`
	EndPos     [2]float32 `json:"end_pos"`
	EndColor   Color      `json:"end_color"`
}

// ResolvedStyle is one style token's resolved property values, exactly the
// shape `natyv prepare`'s codegen emits into the generated `StyleTokens`
// map (see Codegen.zig's own doc comment) -- pointer fields distinguish "a
// guest never referenced this property" (nil) from "referenced and
// resolved" (non-nil), needed for ApplyStyle's later-wins merge across
// multiple token names.
//
// TextureID is a real asset id, not a path -- `natyv prepare`'s codegen
// resolves the stylesheet's `texture: "logo.png"` string into this at
// generate time (see Codegen.zig's own doc comment), an index into the
// per-app `TextureAssets.data` embedded-bytes array the host reads from.
// Guest code never constructs one of these by hand; it only ever flows
// through from a generated `StyleTokens` entry.
type ResolvedStyle struct {
	BackgroundColor *Color
	Padding         *Padding
	CornerRadius    *CornerRadius
	Border          *Border
	Gradient        *Gradient
	TextureID       *uint32
}

// TextureIDPtr exists only so `natyv prepare`'s generated code can populate
// ResolvedStyle.TextureID -- Go doesn't allow taking the address of a bare
// integer literal (`&5` is a compile error), unlike CornerRadius/Border/
// Gradient's composite-literal fields, which can be addressed directly.
// Not expected to be called from hand-written guest code.
func TextureIDPtr(id uint32) *uint32 { return &id }

// mergeStyles applies later-wins precedence across multiple resolved
// tokens, field by field -- matches the token model's own "plain
// field-merge, not CSS specificity" convention (CLAUDE.md's styling
// section). Order matches the order names were passed to ApplyStyle.
func mergeStyles(styles []ResolvedStyle) ResolvedStyle {
	var out ResolvedStyle
	for _, s := range styles {
		if s.BackgroundColor != nil {
			out.BackgroundColor = s.BackgroundColor
		}
		if s.Padding != nil {
			out.Padding = s.Padding
		}
		if s.CornerRadius != nil {
			out.CornerRadius = s.CornerRadius
		}
		if s.Border != nil {
			out.Border = s.Border
		}
		if s.Gradient != nil {
			out.Gradient = s.Gradient
		}
		if s.TextureID != nil {
			out.TextureID = s.TextureID
		}
	}
	return out
}

// ApplyStyle resolves tokenNames against `tokens` (the `StyleTokens` map
// `natyv prepare`'s codegen generates into *the app's own guest package* --
// see Codegen.zig's own doc comment -- passed in explicitly rather than
// assumed as a package-level global here, since each app has its own
// distinct stylesheet; this SDK package can't bake in any one app's
// tokens). Merges later-wins on conflicting properties, then calls the
// host with the fully resolved values -- the host itself never sees a
// token name, see WidgetHostFunctions.zig's setStyleHostFn doc comment for
// why. Returns a real error (not a silent no-op) if any name isn't in
// `tokens`, same "always surface real failures" posture as every other
// natyv guest-facing call.
//
// Takes a plain widget id rather than a typed widget handle -- a generic
// `interface{ widgetID() uint32 }` would read more naturally at call
// sites, but would require adding that method to every widget type in
// this package for a first, prove-it-works pass. Worth revisiting once
// this has a real caller beyond the fixture demo.
func ApplyStyle(widgetID uint32, tokens map[string]ResolvedStyle, tokenNames ...string) error {
	merged, err := resolveTokenNames(tokens, tokenNames)
	if err != nil {
		return err
	}
	return applyMergedStyle(widgetID, merged)
}

// ApplyStyleWithTexture merges tokenNames exactly like ApplyStyle, then
// overrides the merged result's TextureID with textureID directly --
// `<Image src="...">` sugar's own real mechanism (see ntx/Codegen.zig's
// emitApplyStyleWithTexture): src's own resolved asset id always wins over
// whatever texture (if any) the named tokens themselves carry, per Quinn's
// own explicit call, while every other merged field from tokenNames still
// applies normally. tokenNames may be empty -- a bare `<Image src="..."/>`
// with no styles={} at all still needs its texture applied.
func ApplyStyleWithTexture(widgetID uint32, tokens map[string]ResolvedStyle, textureID uint32, tokenNames ...string) error {
	merged, err := resolveTokenNames(tokens, tokenNames)
	if err != nil {
		return err
	}
	merged.TextureID = &textureID
	return applyMergedStyle(widgetID, merged)
}

// ApplyStyleToLayout resolves tokenNames exactly like ApplyStyle, but
// writes the result directly into layout instead of making a separate,
// later host call against an already-created widget's id -- call this
// BEFORE the matching CreateX(layout, ...) call, not after. Real bug this
// exists to close (found live 2026-09-09, mail-natyv's own real click-
// through -- see WidgetHostFunctions.zig's ClayLayoutRequest doc comment
// and project_natyv_render_loop_fix memory for the full finding):
// CreateX and the separate ApplyStyle(id, ...) call are two entirely
// independently-locked host calls, leaving a real window where a freshly
// created widget exists with no visual style at all -- long enough for an
// unrelated main-thread redraw (any real SDL event, not just a click) to
// render it that way. Embedding the resolved style directly into the same
// create request closes the gap outright, since the host applies both
// under the one lock that inserts the widget.
//
// ApplyStyle(id, ...) remains real and correct for genuinely restyling an
// already-existing widget later (a runtime state change) -- this doesn't
// replace it, it's the atomic alternative for the create-time case, which
// `natyv prepare`'s own codegen should prefer whenever a widget's full
// style is already known before it's created (the overwhelming majority
// of real `.ntx` markup).
//
// A resolved Padding (if any) is written directly into layout.Padding,
// overwriting whatever the caller already set there -- the same later-
// wins precedence ApplyStyle(id, ...) already has today (setStyle
// unconditionally overwrites), just resolved before creation instead of
// after.
func ApplyStyleToLayout(layout *Layout, tokens map[string]ResolvedStyle, tokenNames ...string) error {
	merged, err := resolveTokenNames(tokens, tokenNames)
	if err != nil {
		return err
	}
	mergeStyleIntoLayout(layout, merged)
	return nil
}

// ApplyStyleToLayoutWithTexture mirrors ApplyStyleWithTexture -- src's own
// resolved asset id always wins over whatever texture (if any) the named
// tokens themselves carry. See ApplyStyleToLayout's own doc comment for
// why this exists at all.
func ApplyStyleToLayoutWithTexture(layout *Layout, tokens map[string]ResolvedStyle, textureID uint32, tokenNames ...string) error {
	merged, err := resolveTokenNames(tokens, tokenNames)
	if err != nil {
		return err
	}
	merged.TextureID = &textureID
	mergeStyleIntoLayout(layout, merged)
	return nil
}

func mergeStyleIntoLayout(layout *Layout, merged ResolvedStyle) {
	if merged.Padding != nil {
		layout.Padding = *merged.Padding
	}
	layout.BackgroundColor = merged.BackgroundColor
	if merged.CornerRadius != nil {
		cr := merged.CornerRadius
		layout.CornerRadius = &[4]float32{cr.TopLeft, cr.TopRight, cr.BottomRight, cr.BottomLeft}
	}
	layout.Border = merged.Border
	layout.Gradient = merged.Gradient
	layout.TextureID = merged.TextureID
}

func resolveTokenNames(tokens map[string]ResolvedStyle, tokenNames []string) (ResolvedStyle, error) {
	resolved := make([]ResolvedStyle, 0, len(tokenNames))
	for _, name := range tokenNames {
		style, ok := tokens[name]
		if !ok {
			return ResolvedStyle{}, fmt.Errorf("natyv: unknown style token %q", name)
		}
		resolved = append(resolved, style)
	}
	return mergeStyles(resolved), nil
}

func applyMergedStyle(widgetID uint32, merged ResolvedStyle) error {
	var bg *internal.ColorValue
	if merged.BackgroundColor != nil {
		bg = &internal.ColorValue{R: merged.BackgroundColor.R, G: merged.BackgroundColor.G, B: merged.BackgroundColor.B, A: merged.BackgroundColor.A}
	}
	var padding *internal.PaddingValue
	if merged.Padding != nil {
		padding = &internal.PaddingValue{Left: merged.Padding.Left, Right: merged.Padding.Right, Top: merged.Padding.Top, Bottom: merged.Padding.Bottom}
	}
	var cornerRadius *[4]float32
	if merged.CornerRadius != nil {
		cr := merged.CornerRadius
		cornerRadius = &[4]float32{cr.TopLeft, cr.TopRight, cr.BottomRight, cr.BottomLeft}
	}
	var border *internal.BorderValue
	if merged.Border != nil {
		border = &internal.BorderValue{
			Width: merged.Border.Width,
			Color: internal.ColorValue{R: merged.Border.Color.R, G: merged.Border.Color.G, B: merged.Border.Color.B, A: merged.Border.Color.A},
		}
	}
	var gradient *internal.GradientValue
	if merged.Gradient != nil {
		g := merged.Gradient
		gradient = &internal.GradientValue{
			StartPos:   g.StartPos,
			StartColor: internal.ColorValue{R: g.StartColor.R, G: g.StartColor.G, B: g.StartColor.B, A: g.StartColor.A},
			EndPos:     g.EndPos,
			EndColor:   internal.ColorValue{R: g.EndColor.R, G: g.EndColor.G, B: g.EndColor.B, A: g.EndColor.A},
		}
	}
	return internal.SetStyle(widgetID, bg, padding, cornerRadius, border, gradient, merged.TextureID)
}
