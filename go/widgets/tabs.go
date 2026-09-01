package widgets

import (
	"encoding/json"

	"github.com/extism/go-pdk"

	"github.com/natyv-io/sdks/go/widgets/internal"
)

//go:wasmimport extism:host/user natyv_clay_create_tabs
func natyvClayCreateTabsHost(uint64) uint64

//go:wasmimport extism:host/user natyv_clay_create_tab_panel
func natyvClayCreateTabPanelHost(uint64) uint64

// Tabs is a header row of mutually-exclusive labeled tabs paired with real
// Clay-managed panel children, exactly one shown at a time (see host-side
// Tabs.zig's file doc comment). Host-authoritative, same model as
// SegmentedControl -- CreateTabPanel below registers each panel's id with
// the host, which flips their `visible` flags on switch, so panel content
// (typed text, scroll position, ...) survives a switch instead of being
// destroyed and recreated.
type Tabs uint32

func CreateTabs(layout Layout, labels []string, selectedIndex int) (Tabs, error) {
	body, err := json.Marshal(struct {
		Layout        Layout   `json:"layout"`
		Labels        []string `json:"labels"`
		SelectedIndex int      `json:"selected_index"`
	}{layout, labels, selectedIndex})
	if err != nil {
		return 0, err
	}
	resp, err := internal.DecodeWidgetResponse(natyvClayCreateTabsHost(pdk.ResultBytes(body)))
	if err != nil {
		return 0, err
	}
	return Tabs(resp.WidgetID), nil
}

// CreateTabPanel adds one panel Container to an existing Tabs widget --
// call it once per tab, in the same order as CreateTabs' labels, right
// after CreateTabs returns. `layout.ParentID` is always overwritten with
// `tabs`' own id, regardless of what the caller sets it to -- the host
// rejects any parent that isn't the Tabs widget it's called on anyway, so
// accepting a ParentID a caller could get wrong here would only ever
// produce a confusing error, never a useful one.
//
// The returned Container's own initial visibility is decided by the host
// (whichever panel matches Tabs' current selected index), not by anything
// in `layout`.
func CreateTabPanel(tabs Tabs, layout Layout) (Container, error) {
	id := uint32(tabs)
	layout.ParentID = &id
	body, err := json.Marshal(struct {
		Layout Layout `json:"layout"`
	}{layout})
	if err != nil {
		return 0, err
	}
	resp, err := internal.DecodeWidgetResponse(natyvClayCreateTabPanelHost(pdk.ResultBytes(body)))
	if err != nil {
		return 0, err
	}
	return Container(resp.WidgetID), nil
}

func (t Tabs) SelectedIndex() (int, error) {
	v, err := internal.GetValue(uint32(t))
	return int(v), err
}
func (t Tabs) Select(index int) error { return internal.SetValue(uint32(t), float32(index)) }
func (t Tabs) OnChange(handler func(index int) error) {
	internal.RegisterChange(uint32(t), func(v float32) error { return handler(int(v)) })
}

// Destroy destroys only the Tabs widget itself, not its panels -- natyv has
// no cascading delete (see WidgetHost.zig's destroySubtreeLocked doc
// comment for the one narrow exception, which doesn't apply here). Destroy
// every panel Container CreateTabPanel returned before or after calling
// this, same explicit-per-child pattern Dialog.close already establishes
// for its own multi-widget composition.
func (t Tabs) Destroy() { internal.DestroyWidget(uint32(t)) }
