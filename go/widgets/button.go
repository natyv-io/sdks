package widgets

import (
	"encoding/json"

	"github.com/extism/go-pdk"

	"github.com/natyv-io/sdks/go/widgets/internal"
)

//go:wasmimport extism:host/user natyv_clay_create_button
func natyvClayCreateButtonHost(uint64) uint64

// Button is a typed widget handle (over the host-assigned widget_id)
// rather than a raw int, so app code reads as `addButton.OnClick(handleAdd)`
// instead of tracking which uint32 means what.
type Button uint32

func CreateButton(layout Layout, label string) (Button, error) {
	body, err := json.Marshal(struct {
		Layout Layout `json:"layout"`
		Label  string `json:"label"`
	}{layout, label})
	if err != nil {
		return 0, err
	}
	resp, err := internal.DecodeWidgetResponse(natyvClayCreateButtonHost(pdk.ResultBytes(body)))
	if err != nil {
		return 0, err
	}
	return Button(resp.WidgetID), nil
}

func (b Button) SetLabel(label string) error  { return internal.SetText(uint32(b), label) }
func (b Button) Label() (string, error)       { return internal.GetText(uint32(b)) }
func (b Button) OnClick(handler func() error) { internal.RegisterClick(uint32(b), handler) }

// OnHover is the tooltip primitive -- fired true once the host's hover-hold
// timer crosses its threshold over this Button, and false the moment it
// stops being hovered. See internal.RegisterHover's doc comment for the
// usual create-on-true/destroy-on-false shape.
func (b Button) OnHover(handler func(hovering bool) error) {
	internal.RegisterHover(uint32(b), handler)
}

// OnBlur fires once, when this Button loses focus -- fired for any widget
// kind host-side (see internal.RegisterBlur's own doc comment), exposed
// here for Menu-shaped composition (a trigger/submenu-trigger Button
// closing its own cascade on a real click-away). The handler receives
// newFocusID (0 = none) -- which widget focus moved *to* -- since a widget
// within your own composed panel gaining focus isn't the same as a real
// click-away.
func (b Button) OnBlur(handler func(newFocusID uint32) error) {
	internal.RegisterBlur(uint32(b), handler)
}

// OnKeyNav fires on Up/Down/Enter while this Button is focused -- exposed
// for Menu-shaped composition (moving a keyboard highlight across a
// cascading menu's items).
func (b Button) OnKeyNav(handler func(key string) error) {
	internal.RegisterKeyNav(uint32(b), handler)
}

// SetVisible shows/hides this Button without destroying it -- see
// internal.SetVisible's own doc comment. Built for Tree view's fixed row
// pool (tree.go hides whichever pool slots the current window doesn't
// need, rather than destroying/recreating rows on every render).
func (b Button) SetVisible(visible bool) error { return internal.SetVisible(uint32(b), visible) }

// SetEnabled disables this Button -- it ignores clicks (no flash, no
// OnClick, no hover/pointer-cursor affordance) and renders a fixed muted
// gray regardless of any styles= backgroundColor, until re-enabled. Built
// for a real "< N >" pager (disabled at either end of the page range)
// instead of hiding the buttons entirely. See internal.SetEnabled's own
// doc comment.
func (b Button) SetEnabled(enabled bool) error { return internal.SetEnabled(uint32(b), enabled) }

func (b Button) Destroy() { internal.DestroyWidget(uint32(b)) }

// WrapButton returns a typed handle for a host-assigned widget id the guest
// didn't just create -- see widgets.WrapLabel's own doc comment for the
// real use case (natyv_resume reattaching to a pre-existing on-screen
// widget instead of recreating it) and caveats (no host-side validation,
// fails with the host's own "no such widget" error if id isn't alive).
func WrapButton(id uint32) Button {
	return Button(id)
}
