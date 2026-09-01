package widgets

import (
	"encoding/json"

	"github.com/extism/go-pdk"

	"github.com/natyv-io/sdks/go/widgets/internal"
)

//go:wasmimport extism:host/user natyv_clay_create_slider
func natyvClayCreateSliderHost(uint64) uint64

// Slider is natyv's first host-authoritative interactive widget -- see
// WidgetHost.zig's file doc comment for the full explanation. Value() and
// SetValue() work the same as ProgressBar's (a guest can still set a
// default), but the common case is the *host* changing the value via drag
// or arrow-key nudge and telling the guest via OnChange, not the other way
// around.
type Slider uint32

func CreateSlider(layout Layout, value float32) (Slider, error) {
	body, err := json.Marshal(struct {
		Layout Layout  `json:"layout"`
		Value  float32 `json:"value"`
	}{layout, value})
	if err != nil {
		return 0, err
	}
	resp, err := internal.DecodeWidgetResponse(natyvClayCreateSliderHost(pdk.ResultBytes(body)))
	if err != nil {
		return 0, err
	}
	return Slider(resp.WidgetID), nil
}

func (s Slider) Value() (float32, error)      { return internal.GetValue(uint32(s)) }
func (s Slider) SetValue(value float32) error { return internal.SetValue(uint32(s), value) }
func (s Slider) OnChange(handler func(value float32) error) {
	internal.RegisterChange(uint32(s), handler)
}
func (s Slider) Destroy() { internal.DestroyWidget(uint32(s)) }
