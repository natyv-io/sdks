// Package clay is the Clay-backed layout sibling to sdk/go's plain
// (absolute-pixel) widget API -- wraps the natyv_clay_* host functions
// (L3) with the same typed-handle ergonomics as the base package's
// widgets.go, so app code reads as `clay.CreateButton(layout, "Add")`
// instead of hand-building the wire JSON itself. An app opts into this by
// setting conf.natyv.json's `ui.backend` to "clay" (see Config.UiConfig on
// the host side) -- nothing about the base `natyv` package changes or goes
// away, the two are meant to coexist in the same guest: widgets created
// here get their x/y/w/h computed by the host's per-frame Clay layout pass
// (L4), widgets created via the base package keep the guest-supplied
// absolute pixels they always had.
//
// Button/TextField/Label handles returned from this package are the exact
// same types as the base `natyv` package's -- natyv_set_text/
// natyv_get_text/natyv_destroy_widget work identically regardless of which
// create call originally produced a widget_id, so there's no need for
// clay-specific variants of those. Container is the one handle type that
// only exists here, since layout-only grouping nodes have no equivalent in
// the absolute-pixel API.
package clay

import (
	"encoding/json"
	"errors"

	"github.com/extism/go-pdk"

	"natyv/sdk"
)

//go:wasmimport extism:host/user natyv_clay_create_container
func natyvClayCreateContainerHost(uint64) uint64

//go:wasmimport extism:host/user natyv_clay_create_button
func natyvClayCreateButtonHost(uint64) uint64

//go:wasmimport extism:host/user natyv_clay_create_textfield
func natyvClayCreateTextFieldHost(uint64) uint64

//go:wasmimport extism:host/user natyv_clay_create_textarea
func natyvClayCreateTextAreaHost(uint64) uint64

//go:wasmimport extism:host/user natyv_clay_create_label
func natyvClayCreateLabelHost(uint64) uint64

//go:wasmimport extism:host/user natyv_clay_create_checkbox
func natyvClayCreateCheckboxHost(uint64) uint64

//go:wasmimport extism:host/user natyv_clay_create_radio_button
func natyvClayCreateRadioButtonHost(uint64) uint64

//go:wasmimport extism:host/user natyv_clay_create_progressbar
func natyvClayCreateProgressBarHost(uint64) uint64

//go:wasmimport extism:host/user natyv_clay_create_divider
func natyvClayCreateDividerHost(uint64) uint64

//go:wasmimport extism:host/user natyv_clay_create_slider
func natyvClayCreateSliderHost(uint64) uint64

//go:wasmimport extism:host/user natyv_destroy_widget
func natyvDestroyWidgetHost(uint64) uint64

// Container is a layout-only Clay node -- position/size only, nothing
// drawn -- used to group other Clay-managed widgets under a shared
// direction/padding/gap/alignment. See host-side widgets/Container.zig.
type Container uint32

// OnDismiss registers the handler for a modal's "please close" request --
// fired on Escape or a backdrop click while this Container is the topmost
// open modal (Layout.Modal was true at CreateContainer time). The host
// never force-closes; the handler decides whether to actually destroy the
// modal's subtree, same as any other guest-owned destroy/recreate widget.
// Only meaningful on a modal root -- registering it on a plain Container
// is harmless but never fires, since only a modal root's own widget id is
// ever the target of a `.dismiss` event.
func (c Container) OnDismiss(handler func() error) { natyv.RegisterDismiss(uint32(c), handler) }

// SizingAxis mirrors Clay's own Clay_SizingAxis -- the four constructors
// below (Fixed/Grow/Fit/Percent) match the ergonomics of Clay's real
// CLAY_SIZING_FIXED/CLAY_SIZING_GROW/CLAY_SIZING_FIT/CLAY_SIZING_PERCENT
// macros, just as Go functions instead of C macros. `Type` values must be
// exactly "fixed"/"grow"/"fit"/"percent" -- the host parses these directly
// into Clay's real sizing-type enum (see WidgetHost.zig's toSizingAxis).
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
// GROW-width text fields in a row split the row's leftover width evenly,
// the same behavior L1's toolchain proof demonstrated with two children.
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

// Layout is the parent/style a Clay-managed widget is created with --
// passed to every Create* function in this package. `ParentID` is nil for
// a top-level node (attached under natyv's synthetic per-frame root, see
// ClayLayout.zig); pass an existing Container's, Button's, etc. widget_id
// otherwise (any Clay-managed widget can be a parent, not just Container,
// though a leaf with children is an unusual layout to build on purpose).
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
	// W2: clips this container's children to its own bounds and lets the
	// mouse wheel scroll them -- only meaningful on a container with a
	// bounded (non-Fit) size. Plain bools, unlike Direction/ChildAlignment
	// above: an explicit `false` and an absent key both parse to the same
	// Zig-side default, so no omitempty is needed here.
	ScrollVertical   bool `json:"scroll_vertical"`
	ScrollHorizontal bool `json:"scroll_horizontal"`
	// W4: layers this element (and its own children) on top of normal
	// content instead of taking part in its parent's normal flex flow --
	// e.g. a dropdown's options panel, positioned directly below whatever
	// widget ParentID names. Same "plain bool, no omitempty" reasoning as
	// ScrollVertical/ScrollHorizontal above.
	Floating bool `json:"floating"`
	// W5: implies floating-style positioning on its own (don't also set
	// Floating) but centers against the whole window instead of attaching
	// below ParentID -- the host also gives it a backdrop and blocks input
	// to everything outside it while open. A backdrop click never fires
	// OnDismiss -- it only blocks (Quinn: "more idiomatic of modals").
	// Escape and a guest-declared close Button are the only two ways to
	// close one, see OnDismiss.
	Modal bool `json:"modal"`
	// W7: implies floating-style positioning on its own (don't also set
	// Floating or Modal) -- anchored to a fixed screen corner
	// (bottom-right, not configurable in v1) instead of below ParentID or
	// centered. Set once, on a single persistent toast-stack container;
	// individual toasts are plain children of it (see
	// CreateContainer's durationMs param), stacking via ordinary flex
	// layout, not their own Toast flag.
	Toast bool `json:"toast"`
}

// ParentID builds a Layout whose ParentID points at an existing
// Clay-managed widget -- pass `uint32(x)` for any handle (Container,
// natyv.Button, natyv.TextField, natyv.Label), since all of them are just a
// widget_id underneath. Convenience over writing out
// `id := uint32(x); Layout{ParentID: &id}` at every call site. Not a
// generic function deliberately -- TinyGo's wasip1/c-shared target is new
// enough ground in this project's toolchain that avoiding generics here
// isn't worth the risk for what a one-line explicit cast already covers.
func ParentID(id uint32) Layout {
	return Layout{ParentID: &id}
}

type widgetResponse struct {
	WidgetID uint32 `json:"widget_id"`
	Error    string `json:"error,omitempty"`
}

// W5: background is a fixed host-drawn panel fill/border, not a
// guest-chosen color -- see WidgetHost.zig's ClayContainerRequest doc
// comment for why this isn't the start of a guest-controllable styling
// system. Existing callers pass false for a plain layout-only container.
// W7: durationMs is 0 (the default) for a Container that never expires --
// see WidgetHost.zig's Slot.expires_at_ms doc comment. Non-zero destroys
// this Container (and every descendant -- host-driven expiry cascades,
// unlike Destroy(), which stays explicit-per-child-only) automatically
// once that many milliseconds have passed, no guest polling/timer needed.
func CreateContainer(layout Layout, background bool, durationMs uint32) (Container, error) {
	body, err := json.Marshal(struct {
		Layout     Layout `json:"layout"`
		Background bool   `json:"background"`
		DurationMs uint32 `json:"duration_ms"`
	}{layout, background, durationMs})
	if err != nil {
		return 0, err
	}
	resp, err := decodeWidgetResponse(natyvClayCreateContainerHost(pdk.ResultBytes(body)))
	if err != nil {
		return 0, err
	}
	return Container(resp.WidgetID), nil
}

func CreateButton(layout Layout, label string) (natyv.Button, error) {
	body, err := json.Marshal(struct {
		Layout Layout `json:"layout"`
		Label  string `json:"label"`
	}{layout, label})
	if err != nil {
		return 0, err
	}
	resp, err := decodeWidgetResponse(natyvClayCreateButtonHost(pdk.ResultBytes(body)))
	if err != nil {
		return 0, err
	}
	return natyv.Button(resp.WidgetID), nil
}

func CreateTextField(layout Layout, placeholder string) (natyv.TextField, error) {
	body, err := json.Marshal(struct {
		Layout      Layout `json:"layout"`
		Placeholder string `json:"placeholder"`
	}{layout, placeholder})
	if err != nil {
		return 0, err
	}
	resp, err := decodeWidgetResponse(natyvClayCreateTextFieldHost(pdk.ResultBytes(body)))
	if err != nil {
		return 0, err
	}
	return natyv.TextField(resp.WidgetID), nil
}

// CreateTextArea -- W10, mirrors CreateTextField's shape.
func CreateTextArea(layout Layout, placeholder string) (natyv.TextArea, error) {
	body, err := json.Marshal(struct {
		Layout      Layout `json:"layout"`
		Placeholder string `json:"placeholder"`
	}{layout, placeholder})
	if err != nil {
		return 0, err
	}
	resp, err := decodeWidgetResponse(natyvClayCreateTextAreaHost(pdk.ResultBytes(body)))
	if err != nil {
		return 0, err
	}
	return natyv.TextArea(resp.WidgetID), nil
}

func CreateLabel(layout Layout, text string) (natyv.Label, error) {
	body, err := json.Marshal(struct {
		Layout Layout `json:"layout"`
		Text   string `json:"text"`
	}{layout, text})
	if err != nil {
		return 0, err
	}
	resp, err := decodeWidgetResponse(natyvClayCreateLabelHost(pdk.ResultBytes(body)))
	if err != nil {
		return 0, err
	}
	return natyv.Label(resp.WidgetID), nil
}

// W1: Checkbox/RadioButton/ProgressBar handles returned here are the exact
// same base `natyv` package types Button/TextField/Label already are --
// natyv_set_checked/natyv_get_checked/natyv_set_value/natyv_get_value work
// identically regardless of which create call produced the widget_id.
func CreateCheckbox(layout Layout, label string) (natyv.Checkbox, error) {
	body, err := json.Marshal(struct {
		Layout Layout `json:"layout"`
		Label  string `json:"label"`
	}{layout, label})
	if err != nil {
		return 0, err
	}
	resp, err := decodeWidgetResponse(natyvClayCreateCheckboxHost(pdk.ResultBytes(body)))
	if err != nil {
		return 0, err
	}
	return natyv.Checkbox(resp.WidgetID), nil
}

// groupID -- see the base package's CreateRadioButton doc comment.
func CreateRadioButton(layout Layout, groupID uint32, label string) (natyv.RadioButton, error) {
	body, err := json.Marshal(struct {
		Layout  Layout `json:"layout"`
		Label   string `json:"label"`
		GroupID uint32 `json:"group_id"`
	}{layout, label, groupID})
	if err != nil {
		return 0, err
	}
	resp, err := decodeWidgetResponse(natyvClayCreateRadioButtonHost(pdk.ResultBytes(body)))
	if err != nil {
		return 0, err
	}
	return natyv.RadioButton(resp.WidgetID), nil
}

func CreateProgressBar(layout Layout, value float32) (natyv.ProgressBar, error) {
	body, err := json.Marshal(struct {
		Layout Layout  `json:"layout"`
		Value  float32 `json:"value"`
	}{layout, value})
	if err != nil {
		return 0, err
	}
	resp, err := decodeWidgetResponse(natyvClayCreateProgressBarHost(pdk.ResultBytes(body)))
	if err != nil {
		return 0, err
	}
	return natyv.ProgressBar(resp.WidgetID), nil
}

// W3: Slider handle returned here is the same base `natyv` package type --
// natyv_set_value/natyv_get_value/natyv_destroy_widget, and the OnChange
// registration, all work identically regardless of which create call
// produced the widget_id.
func CreateSlider(layout Layout, value float32) (natyv.Slider, error) {
	body, err := json.Marshal(struct {
		Layout Layout  `json:"layout"`
		Value  float32 `json:"value"`
	}{layout, value})
	if err != nil {
		return 0, err
	}
	resp, err := decodeWidgetResponse(natyvClayCreateSliderHost(pdk.ResultBytes(body)))
	if err != nil {
		return 0, err
	}
	return natyv.Slider(resp.WidgetID), nil
}

// CreateDivider -- W11, mirrors CreateSlider's shape minus the value.
func CreateDivider(layout Layout) (natyv.Divider, error) {
	body, err := json.Marshal(struct {
		Layout Layout `json:"layout"`
	}{layout})
	if err != nil {
		return 0, err
	}
	resp, err := decodeWidgetResponse(natyvClayCreateDividerHost(pdk.ResultBytes(body)))
	if err != nil {
		return 0, err
	}
	return natyv.Divider(resp.WidgetID), nil
}

// decodeWidgetResponse factors out the shared response-decoding boilerplate
// (every natyv_clay_create_* host function replies the same
// {"widget_id":N}|{"error":...} shape) -- but deliberately does *not* also
// take the host function itself as a `func(uint64) uint64` parameter the
// way a first pass at this did: TinyGo's wasip1/c-shared target rejects
// using a //go:wasmimport-declared function as a value ("cannot use an
// exported function as value"), so each Create* call above calls its own
// host import directly and only hands this the raw result to decode.
func decodeWidgetResponse(resultOffset uint64) (widgetResponse, error) {
	var resp widgetResponse
	if err := json.Unmarshal(pdk.ParamBytes(resultOffset), &resp); err != nil {
		return widgetResponse{}, err
	}
	if resp.Error != "" {
		return widgetResponse{}, errors.New(resp.Error)
	}
	return resp, nil
}

func (c Container) Destroy() {
	// Best-effort, same as the base natyv package's widget Destroy methods
	// -- nothing meaningful for app code to do if this fails.
	body, err := json.Marshal(struct {
		WidgetID uint32 `json:"widget_id"`
	}{uint32(c)})
	if err != nil {
		return
	}
	_ = natyvDestroyWidgetHost(pdk.ResultBytes(body))
}
