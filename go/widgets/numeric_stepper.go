package widgets

import (
	"encoding/json"

	"github.com/extism/go-pdk"

	"github.com/natyv-io/sdks/go/widgets/internal"
)

//go:wasmimport extism:host/user natyv_clay_create_numeric_stepper
func natyvClayCreateNumericStepperHost(uint64) uint64

// NumericStepper is Slider's integer-valued, click-zone-driven sibling
// (minus/plus zones instead of a drag track), with real min/max/step/wrap
// semantics instead of a normalized [0,1] value. Same host-authoritative
// model as Slider: the host owns the value (a click or a focused arrow-key
// press), the guest finds out via OnChange.
type NumericStepper uint32

// `wrap` true wraps at the edges (e.g. an hour stepper: 23->0, 0->23);
// false clamps -- the common "quantity picker" case.
func CreateNumericStepper(layout Layout, value, min, max, step int32, wrap bool) (NumericStepper, error) {
	body, err := json.Marshal(struct {
		Layout Layout `json:"layout"`
		Value  int32  `json:"value"`
		Min    int32  `json:"min"`
		Max    int32  `json:"max"`
		Step   int32  `json:"step"`
		Wrap   bool   `json:"wrap"`
	}{layout, value, min, max, step, wrap})
	if err != nil {
		return 0, err
	}
	resp, err := internal.DecodeWidgetResponse(natyvClayCreateNumericStepperHost(pdk.ResultBytes(body)))
	if err != nil {
		return 0, err
	}
	return NumericStepper(resp.WidgetID), nil
}

// Value/SetValue round-trip through the shared float32-valued
// natyv_get_value/natyv_set_value wire (same as ProgressBar/Slider) -- fine
// for the small integer ranges a stepper deals in. OnChange reuses the same
// change-dispatch machinery every other `.change`-emitting widget already
// shares, since the host's payload key is the same `"value"` field.
func (n NumericStepper) Value() (int32, error) {
	v, err := internal.GetValue(uint32(n))
	return int32(v), err
}
func (n NumericStepper) SetValue(value int32) error {
	return internal.SetValue(uint32(n), float32(value))
}
func (n NumericStepper) OnChange(handler func(value int32) error) {
	internal.RegisterChange(uint32(n), func(v float32) error { return handler(int32(v)) })
}

// OnBlur fires once, when this NumericStepper loses focus -- fired for any
// widget kind host-side (see internal.RegisterBlur's own doc comment),
// exposed here for composed panels (e.g. a Date & time picker) that need to
// know when focus left one of their own controls to decide whether to
// close.
func (n NumericStepper) OnBlur(handler func(newFocusID uint32) error) {
	internal.RegisterBlur(uint32(n), handler)
}

func (n NumericStepper) Destroy() { internal.DestroyWidget(uint32(n)) }
