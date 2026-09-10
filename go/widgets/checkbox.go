package widgets

import (
	"encoding/json"

	"github.com/extism/go-pdk"

	"github.com/natyv-io/sdks/go/widgets/internal"
)

//go:wasmimport extism:host/user natyv_clay_create_checkbox
func natyvClayCreateCheckboxHost(uint64) uint64

type Checkbox uint32

func CreateCheckbox(layout Layout, label string) (Checkbox, error) {
	body, err := json.Marshal(struct {
		Layout Layout `json:"layout"`
		Label  string `json:"label"`
	}{layout, label})
	if err != nil {
		return 0, err
	}
	resp, err := internal.DecodeWidgetResponse(natyvClayCreateCheckboxHost(pdk.ResultBytes(body)))
	if err != nil {
		return 0, err
	}
	return Checkbox(resp.WidgetID), nil
}

func (cb Checkbox) SetLabel(label string) error   { return internal.SetText(uint32(cb), label) }
func (cb Checkbox) Label() (string, error)        { return internal.GetText(uint32(cb)) }
func (cb Checkbox) Checked() (bool, error)        { return internal.GetChecked(uint32(cb)) }
func (cb Checkbox) SetChecked(checked bool) error { return internal.SetChecked(uint32(cb), checked) }
func (cb Checkbox) OnClick(handler func() error)  { internal.RegisterClick(uint32(cb), handler) }

// OnBlur fires once, when this Checkbox loses focus -- fired for any widget
// kind host-side (see internal.RegisterBlur's own doc comment), exposed
// here for composed panels (e.g. a Popover) that need to know when focus
// left one of their own controls to decide whether to close.
func (cb Checkbox) OnBlur(handler func(newFocusID uint32) error) {
	internal.RegisterBlur(uint32(cb), handler)
}

func (cb Checkbox) Destroy() { internal.DestroyWidget(uint32(cb)) }

// WrapCheckbox returns a typed handle for a host-assigned widget id the
// guest didn't just create -- see widgets.WrapLabel's own doc comment for
// the real use case and caveats.
func WrapCheckbox(id uint32) Checkbox {
	return Checkbox(id)
}
