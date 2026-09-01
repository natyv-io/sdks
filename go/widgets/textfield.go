package widgets

import (
	"encoding/json"

	"github.com/extism/go-pdk"

	"github.com/natyv-io/sdks/go/widgets/internal"
)

//go:wasmimport extism:host/user natyv_clay_create_textfield
func natyvClayCreateTextFieldHost(uint64) uint64

type TextField uint32

func CreateTextField(layout Layout, placeholder string) (TextField, error) {
	body, err := json.Marshal(struct {
		Layout      Layout `json:"layout"`
		Placeholder string `json:"placeholder"`
	}{layout, placeholder})
	if err != nil {
		return 0, err
	}
	resp, err := internal.DecodeWidgetResponse(natyvClayCreateTextFieldHost(pdk.ResultBytes(body)))
	if err != nil {
		return 0, err
	}
	return TextField(resp.WidgetID), nil
}

func (t TextField) Text() (string, error)     { return internal.GetText(uint32(t)) }
func (t TextField) SetText(text string) error { return internal.SetText(uint32(t), text) }
func (t TextField) Clear() error              { return t.SetText("") }
func (t TextField) Destroy()                  { internal.DestroyWidget(uint32(t)) }

// OnChange fires on every keystroke (append or backspace) -- the first
// event a TextField reports live, not just on-demand via Text().
func (t TextField) OnChange(handler func(text string) error) {
	internal.RegisterTextChange(uint32(t), handler)
}

// OnBlur fires once, when this widget loses focus. Fired for any widget
// kind host-side, but only meaningful to register here today since
// TextField is the only kind with a real use for it (closing a Combobox's
// options panel). The handler receives newFocusID (0 = none) -- which
// widget focus moved *to* -- see internal.RegisterBlur's doc comment for
// why this matters (a widget within your own composed panel gaining focus
// isn't the same as a real click-away).
func (t TextField) OnBlur(handler func(newFocusID uint32) error) {
	internal.RegisterBlur(uint32(t), handler)
}

// OnKeyNav fires on Up/Down/Enter while this TextField is focused -- built
// for Combobox: move a highlighted option / select it.
func (t TextField) OnKeyNav(handler func(key string) error) {
	internal.RegisterKeyNav(uint32(t), handler)
}
