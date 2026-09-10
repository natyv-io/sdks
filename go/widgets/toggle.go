package widgets

import (
	"encoding/json"

	"github.com/extism/go-pdk"

	"github.com/natyv-io/sdks/go/widgets/internal"
)

//go:wasmimport extism:host/user natyv_clay_create_toggle
func natyvClayCreateToggleHost(uint64) uint64

// Toggle is functionally identical to Checkbox (same bool-state/click/focus
// model, see WidgetHost.zig's Toggle.zig doc comment), just a different
// visual (pill track + thumb instead of a box + checkmark) -- reuses
// natyv_set_checked/natyv_get_checked the same way RadioButton does, no new
// host functions beyond its own create call.
type Toggle uint32

func CreateToggle(layout Layout, label string) (Toggle, error) {
	body, err := json.Marshal(struct {
		Layout Layout `json:"layout"`
		Label  string `json:"label"`
	}{layout, label})
	if err != nil {
		return 0, err
	}
	resp, err := internal.DecodeWidgetResponse(natyvClayCreateToggleHost(pdk.ResultBytes(body)))
	if err != nil {
		return 0, err
	}
	return Toggle(resp.WidgetID), nil
}

func (tg Toggle) SetLabel(label string) error   { return internal.SetText(uint32(tg), label) }
func (tg Toggle) Label() (string, error)        { return internal.GetText(uint32(tg)) }
func (tg Toggle) Checked() (bool, error)        { return internal.GetChecked(uint32(tg)) }
func (tg Toggle) SetChecked(checked bool) error { return internal.SetChecked(uint32(tg), checked) }
func (tg Toggle) OnClick(handler func() error)  { internal.RegisterClick(uint32(tg), handler) }
func (tg Toggle) Destroy()                      { internal.DestroyWidget(uint32(tg)) }

// WrapToggle returns a typed handle for a host-assigned widget id the
// guest didn't just create -- see widgets.WrapLabel's own doc comment for
// the real use case and caveats.
func WrapToggle(id uint32) Toggle {
	return Toggle(id)
}
