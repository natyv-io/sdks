package widgets

import (
	"encoding/json"

	"github.com/extism/go-pdk"

	"natyv/sdk/widgets/internal"
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
func (cb Checkbox) Destroy()                      { internal.DestroyWidget(uint32(cb)) }
