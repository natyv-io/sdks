package widgets

import (
	"encoding/json"

	"github.com/extism/go-pdk"

	"github.com/natyv-io/sdks/go/widgets/internal"
)

//go:wasmimport extism:host/user natyv_clay_create_range_slider
func natyvClayCreateRangeSliderHost(uint64) uint64

// RangeSlider is Slider's two-handle sibling -- selects a `[min, max]` span
// instead of one value. Host-authoritative the same way Slider is: value
// changes originate from a host-owned drag or arrow-key nudge, not a guest
// call, delivered via OnChange. Both `Range()`/`SetRange()` still work (a
// guest can set/read a default), same "not the common case, but still
// works" precedent Slider's own Value()/SetValue() established.
type RangeSlider uint32

// step is the increment a drag or arrow-key nudge moves a handle by, in the
// same normalized `[0, 1]` units as min/max themselves (see
// RangeSlider.zig's own doc comment on the host side for why a real-world
// domain unit isn't a host concept here) -- `step <= 0` means continuous,
// no snapping at all.
func CreateRangeSlider(layout Layout, min, max, step float32) (RangeSlider, error) {
	body, err := json.Marshal(struct {
		Layout Layout  `json:"layout"`
		Min    float32 `json:"min"`
		Max    float32 `json:"max"`
		Step   float32 `json:"step"`
	}{layout, min, max, step})
	if err != nil {
		return 0, err
	}
	resp, err := internal.DecodeWidgetResponse(natyvClayCreateRangeSliderHost(pdk.ResultBytes(body)))
	if err != nil {
		return 0, err
	}
	return RangeSlider(resp.WidgetID), nil
}

// Range reads the current {min, max} -- see internal.GetRange's own doc
// comment.
func (r RangeSlider) Range() (min, max float32, err error) {
	return internal.GetRange(uint32(r))
}

// SetRange sets both ends at once -- see internal.SetRange's own doc
// comment for why not two separate calls.
func (r RangeSlider) SetRange(min, max float32) error {
	return internal.SetRange(uint32(r), min, max)
}

// OnChange fires whenever either handle's value actually changes (a drag or
// an arrow-key nudge) -- min/max are always both reported together, even
// though only one handle moved, since that's the widget's whole state.
func (r RangeSlider) OnChange(handler func(min, max float32) error) {
	internal.RegisterRangeChange(uint32(r), handler)
}

func (r RangeSlider) Destroy() { internal.DestroyWidget(uint32(r)) }

// WrapRangeSlider returns a typed handle for a host-assigned widget id the
// guest didn't just create -- see widgets.WrapLabel's own doc comment for
// the real use case and caveats.
func WrapRangeSlider(id uint32) RangeSlider {
	return RangeSlider(id)
}
