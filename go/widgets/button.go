package widgets

import (
	"encoding/json"

	"github.com/extism/go-pdk"

	"natyv/sdk/widgets/internal"
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
func (b Button) Destroy() { internal.DestroyWidget(uint32(b)) }
