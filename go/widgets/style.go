package widgets

import (
	"fmt"

	"natyv/sdk/widgets/internal"
)

// Color is a resolved style color -- 0..1 floats, matching the stylesheet
// resolver's own convention (src/styling/Resolver.zig) and the SDF
// shader's eventual uniform layout. Deliberately not called RGBA/SDLColor
// -- this is the type `natyv prepare`'s codegen (src/styling/Codegen.zig)
// emits literal values into, so its name is the one guest code actually
// writes/reads.
type Color struct {
	R, G, B, A float32
}

// ResolvedStyle is one style token's resolved property values, exactly the
// shape `natyv prepare`'s codegen emits into the generated `StyleTokens`
// map (see Codegen.zig's own doc comment) -- pointer fields distinguish "a
// guest never referenced this property" (nil) from "referenced and
// resolved" (non-nil), needed for ApplyStyle's later-wins merge across
// multiple token names.
//
// Only BackgroundColor/Padding are consumed by ApplyStyle today.
// CornerRadius/Border/Gradient/Texture parse and resolve correctly
// end-to-end (src/styling/Resolver.zig) but have no rendering path to
// apply to yet -- that's Stage 3's SDF-shader migration. Kept out of this
// struct until Stage 3 defines their real Go-side shape, rather than
// guessing at it now.
type ResolvedStyle struct {
	BackgroundColor *Color
	Padding         *Padding
}

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
	resolved := make([]ResolvedStyle, 0, len(tokenNames))
	for _, name := range tokenNames {
		style, ok := tokens[name]
		if !ok {
			return fmt.Errorf("natyv: unknown style token %q", name)
		}
		resolved = append(resolved, style)
	}
	merged := mergeStyles(resolved)

	var bg *internal.ColorValue
	if merged.BackgroundColor != nil {
		bg = &internal.ColorValue{R: merged.BackgroundColor.R, G: merged.BackgroundColor.G, B: merged.BackgroundColor.B, A: merged.BackgroundColor.A}
	}
	var padding *internal.PaddingValue
	if merged.Padding != nil {
		padding = &internal.PaddingValue{Left: merged.Padding.Left, Right: merged.Padding.Right, Top: merged.Padding.Top, Bottom: merged.Padding.Bottom}
	}
	return internal.SetStyle(widgetID, bg, padding)
}
