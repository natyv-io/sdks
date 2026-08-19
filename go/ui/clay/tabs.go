// W19: Tabs bindings. The `Tabs` handle type itself lives in the base
// `natyv` package, not here -- see widgets.go's own doc comment on why
// (its .Select()/.OnChange()/.Destroy() reuse that package's private
// setValue/registerChange/destroyWidget helpers, the same way
// SegmentedControl's do). This file only has what's genuinely Clay-only:
// the two new host imports and the two constructors that call them.
package clay

import (
	"encoding/json"

	"github.com/extism/go-pdk"

	"natyv/sdk"
)

//go:wasmimport extism:host/user natyv_clay_create_tabs
func natyvClayCreateTabsHost(uint64) uint64

//go:wasmimport extism:host/user natyv_clay_create_tab_panel
func natyvClayCreateTabPanelHost(uint64) uint64

// CreateTabs creates the header row -- see host-side Tabs.zig's file doc
// comment for why this single widget both draws its own header AND is the
// real Clay parent of whatever panel Containers CreateTabPanel adds to it.
func CreateTabs(layout Layout, labels []string, selectedIndex int) (natyv.Tabs, error) {
	body, err := json.Marshal(struct {
		Layout        Layout   `json:"layout"`
		Labels        []string `json:"labels"`
		SelectedIndex int      `json:"selected_index"`
	}{layout, labels, selectedIndex})
	if err != nil {
		return 0, err
	}
	resp, err := decodeWidgetResponse(natyvClayCreateTabsHost(pdk.ResultBytes(body)))
	if err != nil {
		return 0, err
	}
	return natyv.Tabs(resp.WidgetID), nil
}

// CreateTabPanel adds one panel Container to an existing Tabs widget --
// call it once per tab, in the same order as CreateTabs' labels, right
// after CreateTabs returns. `layout.ParentID` is always overwritten with
// `tabs`' own id, regardless of what the caller sets it to -- the host
// rejects any parent that isn't the Tabs widget it's called on anyway (see
// createClayTabPanelHostFn's doc comment), so accepting a ParentID a
// caller could get wrong here would only ever produce a confusing error,
// never a useful one.
//
// The returned Container's own initial visibility is decided by the host
// (whichever panel matches Tabs' current selected index), not by anything
// in `layout` -- see ClayStyle.visible's doc comment.
func CreateTabPanel(tabs natyv.Tabs, layout Layout) (Container, error) {
	id := uint32(tabs)
	layout.ParentID = &id
	body, err := json.Marshal(struct {
		Layout Layout `json:"layout"`
	}{layout})
	if err != nil {
		return 0, err
	}
	resp, err := decodeWidgetResponse(natyvClayCreateTabPanelHost(pdk.ResultBytes(body)))
	if err != nil {
		return 0, err
	}
	return Container(resp.WidgetID), nil
}
