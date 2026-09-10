// Package widgets is natyv's guest-side widget SDK -- typed handles over
// natyv's Clay-managed widget kinds, wrapping every natyv_clay_create_*
// host function with the same ergonomic shape (app code reads as
// `button.OnClick(handleAdd)` instead of tracking which uint32 means what,
// and `widgets.CreateButton(layout, "Add")` instead of hand-building the
// wire JSON itself). Clay-based layout is the only creation path this
// package exposes -- every widget is created via a `Layout` (parent_id,
// sizing, padding, etc.), computed and positioned by the host's per-frame
// Clay layout pass, not guest-supplied absolute pixels.
//
// One file per widget kind (button.go, textfield.go, ...), each holding
// that widget's own wasmimport, its Create function, its typed handle, and
// its methods. Shared plumbing that isn't specific to any one widget kind
// -- natyv_dispatch's event registries, and the generic
// natyv_set_value/natyv_get_value/natyv_set_checked/natyv_get_checked/
// natyv_set_text/natyv_get_text/natyv_destroy_widget round trips -- lives
// in the sibling `widgets/internal` package instead, not duplicated per
// file. `Layout` and its supporting types (this file) are the one
// exception kept in the public package rather than `internal`: every
// widget's own Create function takes a `Layout`, so app code needs to be
// able to construct one directly.
package widgets

// SizingAxis mirrors Clay's own Clay_SizingAxis -- the four constructors
// below (Fixed/Grow/Fit/Percent) match the ergonomics of Clay's real
// CLAY_SIZING_FIXED/CLAY_SIZING_GROW/CLAY_SIZING_FIT/CLAY_SIZING_PERCENT
// macros, just as Go functions instead of C macros.
type SizingAxis struct {
	// `omitempty` matters here, not just on Direction/Alignment below: a
	// zero-value SizingAxis{} built without one of the Fixed/Grow/Fit/
	// Percent constructors would otherwise serialize Type as `""`, which
	// the host's ClaySizingType enum has no matching tag for -- Zig's
	// std.json only applies a struct field's own default value
	// (ClaySizingAxisRequest.type defaults to .fit) when the JSON key is
	// *absent*, not when it's present as an empty string.
	Type    string  `json:"type,omitempty"`
	Min     float32 `json:"min"`
	Max     float32 `json:"max"`
	Percent float32 `json:"percent"`
}

// Fixed pins an axis to exactly px, never growing or shrinking.
func Fixed(px float32) SizingAxis { return SizingAxis{Type: "fixed", Min: px, Max: px} }

// Grow expands to fill whatever space its siblings leave behind, sharing
// evenly with any other GROW siblings along the same axis -- e.g. three
// GROW-width text fields in a row split the row's leftover width evenly.
func Grow() SizingAxis { return SizingAxis{Type: "grow", Min: 0, Max: 1e9} }

// Fit sizes an axis to its content/children -- the default if a SizingAxis
// is left zero-valued. Leaf widgets (Button/TextField/Label) sized Fit
// without any Clay text content collapse toward their min size (0 here)
// until natyv has real font-driven text measurement; prefer Fixed or Grow
// for leaf widgets until then.
func Fit() SizingAxis { return SizingAxis{Type: "fit"} }

// Percent sizes an axis to a fraction (0-1) of the parent's content size
// along that axis, minus the parent's own padding/child gaps.
func Percent(fraction float32) SizingAxis { return SizingAxis{Type: "percent", Percent: fraction} }

type Sizing struct {
	Width  SizingAxis `json:"width"`
	Height SizingAxis `json:"height"`
}

type Padding struct {
	Left   uint16 `json:"left"`
	Right  uint16 `json:"right"`
	Top    uint16 `json:"top"`
	Bottom uint16 `json:"bottom"`
}

type Alignment struct {
	// `omitempty` for the same reason as SizingAxis.Type above -- a Layout
	// that never sets ChildAlignment (every leaf widget in a typical tree)
	// must not send `"x":""`/`"y":""`, or the host's enum parse fails
	// instead of falling back to its documented left/top default.
	X string `json:"x,omitempty"` // "left" | "right" | "center"
	Y string `json:"y,omitempty"` // "top" | "bottom" | "center"
}

const (
	LeftToRight = "left_to_right"
	TopToBottom = "top_to_bottom"

	AlignXLeft   = "left"
	AlignXRight  = "right"
	AlignXCenter = "center"

	AlignYTop    = "top"
	AlignYBottom = "bottom"
	AlignYCenter = "center"
)

// Layout is the parent/style a widget is created with -- passed to every
// Create* function in this package. `ParentID` is nil for a top-level node
// (attached under natyv's synthetic per-frame root, see ClayLayout.zig);
// pass an existing widget's own widget_id otherwise (any widget can be a
// parent, not just Container, though a leaf with children is an unusual
// layout to build on purpose).
type Layout struct {
	ParentID *uint32 `json:"parent_id,omitempty"`
	Sizing   Sizing  `json:"sizing"`
	Padding  Padding `json:"padding"`
	ChildGap uint16  `json:"child_gap"`
	// `omitempty`: a Layout that never sets Direction (any leaf widget with
	// no children of its own) must not send `"direction":""` -- see
	// SizingAxis.Type's doc comment for why an empty string breaks the
	// host's enum parse instead of falling back to its default.
	Direction      string    `json:"direction,omitempty"`
	ChildAlignment Alignment `json:"child_alignment"`
	// Clips this container's children to its own bounds and lets the mouse
	// wheel scroll them -- only meaningful on a container with a bounded
	// (non-Fit) size. Plain bools, unlike Direction/ChildAlignment above:
	// an explicit `false` and an absent key both parse to the same
	// Zig-side default, so no omitempty is needed here.
	ScrollVertical   bool `json:"scroll_vertical"`
	ScrollHorizontal bool `json:"scroll_horizontal"`
	// Layers this element (and its own children) on top of normal content
	// instead of taking part in its parent's normal flex flow -- e.g. a
	// dropdown's options panel, positioned directly below whatever widget
	// ParentID names. Same "plain bool, no omitempty" reasoning as
	// ScrollVertical/ScrollHorizontal above.
	Floating bool `json:"floating"`
	// Implies floating-style positioning on its own (don't also set
	// Floating) but centers against the whole window instead of attaching
	// below ParentID -- the host also gives it a backdrop and blocks input
	// to everything outside it while open. A backdrop click never fires
	// OnDismiss -- it only blocks. Escape and a guest-declared close Button
	// are the only two ways to close one, see Container.OnDismiss.
	Modal bool `json:"modal"`
	// Implies floating-style positioning on its own (don't also set
	// Floating or Modal) -- anchored to a fixed screen corner
	// (bottom-right, not configurable in v1) instead of below ParentID or
	// centered. Set once, on a single persistent toast-stack container;
	// individual toasts are plain children of it (see CreateContainer's
	// durationMs param), stacking via ordinary flex layout, not their own
	// Toast flag.
	Toast bool `json:"toast"`
	// The five fields below let a widget be created with its visual style
	// already fully resolved, in the same wire request as its creation --
	// set them via ApplyStyleToLayout (or ApplyStyleToLayoutWithTexture),
	// never by hand; see ApplyStyleToLayout's own doc comment in style.go
	// for the real, live-caught race this exists to close. nil/omitted
	// means "no style resolved for this property," the same convention
	// every other optional field on this struct already uses.
	BackgroundColor *Color      `json:"background_color,omitempty"`
	CornerRadius    *[4]float32 `json:"corner_radius,omitempty"`
	Border          *Border     `json:"border,omitempty"`
	Gradient        *Gradient   `json:"gradient,omitempty"`
	TextureID       *uint32     `json:"texture,omitempty"`
}

// ParentID builds a Layout whose ParentID points at an existing widget --
// pass `uint32(x)` for any handle (Container, Button, TextField, ...),
// since all of them are just a widget_id underneath. Convenience over
// writing out `id := uint32(x); Layout{ParentID: &id}` at every call site.
// Not a generic function deliberately -- TinyGo's wasip1/c-shared target is
// new enough ground in this project's toolchain that avoiding generics here
// isn't worth the risk for what a one-line explicit cast already covers.
func ParentID(id uint32) Layout {
	return Layout{ParentID: &id}
}
