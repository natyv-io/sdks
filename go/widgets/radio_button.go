package widgets

import (
	"encoding/json"

	"github.com/extism/go-pdk"

	"github.com/natyv-io/sdks/go/widgets/internal"
)

//go:wasmimport extism:host/user natyv_clay_create_radio_button
func natyvClayCreateRadioButtonHost(uint64) uint64

type RadioButton uint32

// groupID is an arbitrary tag the app picks -- every radio button sharing
// the same groupID is mutually exclusive (selecting one deselects the
// others in its group). Not derived from anything else natyv tracks, so
// any uint32 the app finds convenient works.
func CreateRadioButton(layout Layout, groupID uint32, label string) (RadioButton, error) {
	body, err := json.Marshal(struct {
		Layout  Layout `json:"layout"`
		Label   string `json:"label"`
		GroupID uint32 `json:"group_id"`
	}{layout, label, groupID})
	if err != nil {
		return 0, err
	}
	resp, err := internal.DecodeWidgetResponse(natyvClayCreateRadioButtonHost(pdk.ResultBytes(body)))
	if err != nil {
		return 0, err
	}
	return RadioButton(resp.WidgetID), nil
}

func (r RadioButton) SetLabel(label string) error  { return internal.SetText(uint32(r), label) }
func (r RadioButton) Label() (string, error)       { return internal.GetText(uint32(r)) }
func (r RadioButton) Checked() (bool, error)       { return internal.GetChecked(uint32(r)) }
func (r RadioButton) Select() error                { return internal.SetChecked(uint32(r), true) }
func (r RadioButton) OnClick(handler func() error) { internal.RegisterClick(uint32(r), handler) }
func (r RadioButton) Destroy()                     { internal.DestroyWidget(uint32(r)) }

// WrapRadioButton returns a typed handle for a host-assigned widget id the
// guest didn't just create -- see widgets.WrapLabel's own doc comment for
// the real use case and caveats.
func WrapRadioButton(id uint32) RadioButton {
	return RadioButton(id)
}
