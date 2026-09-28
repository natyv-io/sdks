package widgets

import (
	"encoding/json"

	"github.com/extism/go-pdk"

	"github.com/natyv-io/sdks/go/widgets/internal"
)

//go:wasmimport extism:host/user natyv_clay_create_textarea
func natyvClayCreateTextAreaHost(uint64) uint64

// TextArea is TextField's multi-line sibling. Both support a real host-side
// cursor and selection (click/click-drag/shift+arrow to select, Cmd/Ctrl+
// C/X/V to copy/cut/paste) -- entirely host-side, invisible to the guest
// beyond OnChange firing with whatever the resulting text is. Enter inserts
// a literal newline instead of TextField's "select the highlighted combobox
// option" meaning, handled entirely host-side -- nothing extra for the
// guest to do, a newline just arrives as part of the same OnChange text a
// keystroke would.
type TextArea uint32

func CreateTextArea(layout Layout, placeholder string) (TextArea, error) {
	body, err := json.Marshal(struct {
		Layout      Layout `json:"layout"`
		Placeholder string `json:"placeholder"`
	}{layout, placeholder})
	if err != nil {
		return 0, err
	}
	resp, err := internal.DecodeWidgetResponse(natyvClayCreateTextAreaHost(pdk.ResultBytes(body)))
	if err != nil {
		return 0, err
	}
	return TextArea(resp.WidgetID), nil
}

func (t TextArea) Text() (string, error)     { return internal.GetText(uint32(t)) }
func (t TextArea) SetText(text string) error { return internal.SetText(uint32(t), text) }
func (t TextArea) Clear() error              { return t.SetText("") }
func (t TextArea) Destroy()                  { internal.DestroyWidget(uint32(t)) }

// OnChange fires on every mutation (typed character, backspace, or Enter
// inserting a newline) -- same shape as TextField's. No OnKeyNav: a
// TextArea never fires `.key_nav` (no highlight-navigation semantics).
func (t TextArea) OnChange(handler func(text string) error) {
	internal.RegisterTextChange(uint32(t), handler)
}

// OnBlur fires once, when this widget loses focus -- same shape as
// TextField's.
func (t TextArea) OnBlur(handler func(newFocusID uint32) error) {
	internal.RegisterBlur(uint32(t), handler)
}

// WrapTextArea returns a typed handle for a host-assigned widget id the
// guest didn't just create -- see widgets.WrapLabel's own doc comment for
// the real use case and caveats.
func WrapTextArea(id uint32) TextArea {
	return TextArea(id)
}
