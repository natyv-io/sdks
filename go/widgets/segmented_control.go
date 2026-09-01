package widgets

import (
	"encoding/json"

	"github.com/extism/go-pdk"

	"github.com/natyv-io/sdks/go/widgets/internal"
)

//go:wasmimport extism:host/user natyv_clay_create_segmented_control
func natyvClayCreateSegmentedControlHost(uint64) uint64

// SegmentedControl is a fixed row of mutually-exclusive labeled segments,
// one focus stop (see SegmentedControl.zig's file doc comment for why this
// is a real host widget kind rather than guest-composed Buttons). Same
// host-authoritative model as Slider/NumericStepper.
type SegmentedControl uint32

func CreateSegmentedControl(layout Layout, segments []string, selectedIndex int) (SegmentedControl, error) {
	body, err := json.Marshal(struct {
		Layout        Layout   `json:"layout"`
		Segments      []string `json:"segments"`
		SelectedIndex int      `json:"selected_index"`
	}{layout, segments, selectedIndex})
	if err != nil {
		return 0, err
	}
	resp, err := internal.DecodeWidgetResponse(natyvClayCreateSegmentedControlHost(pdk.ResultBytes(body)))
	if err != nil {
		return 0, err
	}
	return SegmentedControl(resp.WidgetID), nil
}

func (sc SegmentedControl) SelectedIndex() (int, error) {
	v, err := internal.GetValue(uint32(sc))
	return int(v), err
}
func (sc SegmentedControl) Select(index int) error {
	return internal.SetValue(uint32(sc), float32(index))
}
func (sc SegmentedControl) OnChange(handler func(index int) error) {
	internal.RegisterChange(uint32(sc), func(v float32) error { return handler(int(v)) })
}
func (sc SegmentedControl) Destroy() { internal.DestroyWidget(uint32(sc)) }
