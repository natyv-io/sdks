// Package natyv is the first hand-written (not yet code-generated) guest
// SDK for natyv apps. It exists to prove the shape a real binding
// generator would eventually target: typed widget handles instead of raw
// widget_id ints, and OnClick-style handler registration instead of guest
// code hand-parsing natyv_dispatch payloads itself. Extracted from the
// bookstore example once it had working, hand-written low-level calls to
// generalize from -- this SDK doesn't invent anything the example hadn't
// already proven, it just gives it a reusable, ergonomic home.
package natyv

import (
	"encoding/json"
	"errors"

	"github.com/extism/go-pdk"
)

//go:wasmimport extism:host/user natyv_create_button
func natyvCreateButtonHost(uint64) uint64

//go:wasmimport extism:host/user natyv_create_textfield
func natyvCreateTextFieldHost(uint64) uint64

//go:wasmimport extism:host/user natyv_create_textarea
func natyvCreateTextAreaHost(uint64) uint64

//go:wasmimport extism:host/user natyv_create_label
func natyvCreateLabelHost(uint64) uint64

//go:wasmimport extism:host/user natyv_set_text
func natyvSetTextHost(uint64) uint64

//go:wasmimport extism:host/user natyv_get_text
func natyvGetTextHost(uint64) uint64

//go:wasmimport extism:host/user natyv_destroy_widget
func natyvDestroyWidgetHost(uint64) uint64

//go:wasmimport extism:host/user natyv_create_checkbox
func natyvCreateCheckboxHost(uint64) uint64

//go:wasmimport extism:host/user natyv_create_toggle
func natyvCreateToggleHost(uint64) uint64

//go:wasmimport extism:host/user natyv_create_radio_button
func natyvCreateRadioButtonHost(uint64) uint64

//go:wasmimport extism:host/user natyv_create_progressbar
func natyvCreateProgressBarHost(uint64) uint64

//go:wasmimport extism:host/user natyv_create_slider
func natyvCreateSliderHost(uint64) uint64

//go:wasmimport extism:host/user natyv_create_divider
func natyvCreateDividerHost(uint64) uint64

//go:wasmimport extism:host/user natyv_create_badge
func natyvCreateBadgeHost(uint64) uint64

//go:wasmimport extism:host/user natyv_set_checked
func natyvSetCheckedHost(uint64) uint64

//go:wasmimport extism:host/user natyv_get_checked
func natyvGetCheckedHost(uint64) uint64

//go:wasmimport extism:host/user natyv_set_value
func natyvSetValueHost(uint64) uint64

//go:wasmimport extism:host/user natyv_get_value
func natyvGetValueHost(uint64) uint64

// Button, TextField, and Label are typed widget handles (over the same
// underlying host-assigned widget_id) rather than raw ints, so app code
// reads as `addButton.OnClick(handleAdd)` instead of tracking which uint32
// means what.
type Button uint32
type TextField uint32
type TextArea uint32
type Label uint32

type widgetResponse struct {
	WidgetID uint32 `json:"widget_id"`
	Error    string `json:"error,omitempty"`
}

func CreateButton(x, y, w, h float32, label string) (Button, error) {
	body, err := json.Marshal(struct {
		X     float32 `json:"x"`
		Y     float32 `json:"y"`
		W     float32 `json:"w"`
		H     float32 `json:"h"`
		Label string  `json:"label"`
	}{x, y, w, h, label})
	if err != nil {
		return 0, err
	}
	var resp widgetResponse
	if err := json.Unmarshal(pdk.ParamBytes(natyvCreateButtonHost(pdk.ResultBytes(body))), &resp); err != nil {
		return 0, err
	}
	if resp.Error != "" {
		return 0, errors.New(resp.Error)
	}
	return Button(resp.WidgetID), nil
}

func (b Button) SetLabel(label string) error  { return setText(uint32(b), label) }
func (b Button) Label() (string, error)       { return getText(uint32(b)) }
func (b Button) OnClick(handler func() error) { registerClick(uint32(b), handler) }

// OnHover is W15's tooltip primitive -- fired true once the host's
// hover-hold timer crosses its threshold over this Button, and false the
// moment it stops being hovered. See registerHover's doc comment
// (dispatch.go) for the usual create-on-true/destroy-on-false shape.
func (b Button) OnHover(handler func(hovering bool) error) { registerHover(uint32(b), handler) }
func (b Button) Destroy()                                  { destroyWidget(uint32(b)) }

func CreateTextField(x, y, w, h float32, placeholder string) (TextField, error) {
	body, err := json.Marshal(struct {
		X           float32 `json:"x"`
		Y           float32 `json:"y"`
		W           float32 `json:"w"`
		H           float32 `json:"h"`
		Placeholder string  `json:"placeholder"`
	}{x, y, w, h, placeholder})
	if err != nil {
		return 0, err
	}
	var resp widgetResponse
	if err := json.Unmarshal(pdk.ParamBytes(natyvCreateTextFieldHost(pdk.ResultBytes(body))), &resp); err != nil {
		return 0, err
	}
	if resp.Error != "" {
		return 0, errors.New(resp.Error)
	}
	return TextField(resp.WidgetID), nil
}

func (t TextField) Text() (string, error)     { return getText(uint32(t)) }
func (t TextField) SetText(text string) error { return setText(uint32(t), text) }
func (t TextField) Clear() error              { return t.SetText("") }
func (t TextField) Destroy()                  { destroyWidget(uint32(t)) }

// OnChange fires on every keystroke (append or backspace) -- W6, the first
// event a TextField reports live, not just on-demand via Text(). See
// EventQueue.zig's `.text_changed` doc comment on the host side.
func (t TextField) OnChange(handler func(text string) error) { registerTextChange(uint32(t), handler) }

// OnBlur fires once, when this widget loses focus -- W6. Fired for any
// widget kind host-side, but only meaningful to register here today since
// TextField is the only kind with a real use for it (closing a Combobox's
// options panel). W9: handler receives newFocusID (0 = none) -- see
// registerBlur's doc comment for why this matters (a widget within your
// own composed panel gaining focus isn't the same as a real click-away).
func (t TextField) OnBlur(handler func(newFocusID uint32) error) { registerBlur(uint32(t), handler) }

// OnKeyNav fires on Up/Down/Enter while this TextField is focused -- W6,
// built for Combobox: move a highlighted option / select it.
func (t TextField) OnKeyNav(handler func(key string) error) { registerKeyNav(uint32(t), handler) }

// CreateTextArea -- W10, TextField's multi-line sibling. Same append/
// backspace-at-the-end-only model as TextField (no arbitrary cursor
// position); Enter inserts a literal newline instead of TextField's
// "select the highlighted combobox option" meaning, handled entirely
// host-side (see main.zig's SDLK_RETURN handling) -- nothing extra for
// the guest to do, a newline just arrives as part of the same OnChange
// text a keystroke would.
func CreateTextArea(x, y, w, h float32, placeholder string) (TextArea, error) {
	body, err := json.Marshal(struct {
		X           float32 `json:"x"`
		Y           float32 `json:"y"`
		W           float32 `json:"w"`
		H           float32 `json:"h"`
		Placeholder string  `json:"placeholder"`
	}{x, y, w, h, placeholder})
	if err != nil {
		return 0, err
	}
	var resp widgetResponse
	if err := json.Unmarshal(pdk.ParamBytes(natyvCreateTextAreaHost(pdk.ResultBytes(body))), &resp); err != nil {
		return 0, err
	}
	if resp.Error != "" {
		return 0, errors.New(resp.Error)
	}
	return TextArea(resp.WidgetID), nil
}

func (t TextArea) Text() (string, error)     { return getText(uint32(t)) }
func (t TextArea) SetText(text string) error { return setText(uint32(t), text) }
func (t TextArea) Clear() error              { return t.SetText("") }
func (t TextArea) Destroy()                  { destroyWidget(uint32(t)) }

// OnChange fires on every mutation (typed character, backspace, or Enter
// inserting a newline) -- same shape as TextField's. No OnKeyNav: a
// TextArea never fires `.key_nav` (no highlight-navigation semantics in
// v1 -- see the widget plan).
func (t TextArea) OnChange(handler func(text string) error) { registerTextChange(uint32(t), handler) }

// OnBlur fires once, when this widget loses focus -- same shape as
// TextField's.
func (t TextArea) OnBlur(handler func(newFocusID uint32) error) { registerBlur(uint32(t), handler) }

func CreateLabel(x, y float32, text string) (Label, error) {
	body, err := json.Marshal(struct {
		X    float32 `json:"x"`
		Y    float32 `json:"y"`
		Text string  `json:"text"`
	}{x, y, text})
	if err != nil {
		return 0, err
	}
	var resp widgetResponse
	if err := json.Unmarshal(pdk.ParamBytes(natyvCreateLabelHost(pdk.ResultBytes(body))), &resp); err != nil {
		return 0, err
	}
	if resp.Error != "" {
		return 0, errors.New(resp.Error)
	}
	return Label(resp.WidgetID), nil
}

func (l Label) SetText(text string) error { return setText(uint32(l), text) }
func (l Label) Text() (string, error)     { return getText(uint32(l)) }
func (l Label) Destroy()                  { destroyWidget(uint32(l)) }

// Checkbox, RadioButton, and ProgressBar are typed handles the same way
// Button/TextField/Label are -- W1 widget breadth, added once the keyboard
// interaction model (focus/Tab/Enter/Space) already covered how a checkbox
// or radio button gets activated, so nothing about dispatch needed to
// change: OnClick already works unchanged, since it's generic over any
// widget_id.
type Checkbox uint32
type RadioButton uint32
type ProgressBar uint32

// Toggle -- W12 widget breadth. Functionally identical to Checkbox (same
// bool-state/click/focus model, see WidgetHost.zig's Toggle.zig doc
// comment), just a different visual (pill track + thumb instead of a box +
// checkmark) -- reuses natyv_set_checked/natyv_get_checked the same way
// RadioButton does, no new host functions beyond its own create call.
type Toggle uint32

// Slider is natyv's first host-authoritative interactive widget -- see
// WidgetHost.zig's file doc comment for the full explanation. Value() and
// SetValue() work the same as ProgressBar's (a guest can still set a
// default), but the common case is the *host* changing the value via drag
// or arrow-key nudge and telling the guest via OnChange, not the other way
// around.
type Slider uint32

func CreateCheckbox(x, y, w, h float32, label string) (Checkbox, error) {
	body, err := json.Marshal(struct {
		X     float32 `json:"x"`
		Y     float32 `json:"y"`
		W     float32 `json:"w"`
		H     float32 `json:"h"`
		Label string  `json:"label"`
	}{x, y, w, h, label})
	if err != nil {
		return 0, err
	}
	var resp widgetResponse
	if err := json.Unmarshal(pdk.ParamBytes(natyvCreateCheckboxHost(pdk.ResultBytes(body))), &resp); err != nil {
		return 0, err
	}
	if resp.Error != "" {
		return 0, errors.New(resp.Error)
	}
	return Checkbox(resp.WidgetID), nil
}

func (cb Checkbox) SetLabel(label string) error   { return setText(uint32(cb), label) }
func (cb Checkbox) Label() (string, error)        { return getText(uint32(cb)) }
func (cb Checkbox) Checked() (bool, error)        { return getChecked(uint32(cb)) }
func (cb Checkbox) SetChecked(checked bool) error { return setChecked(uint32(cb), checked) }
func (cb Checkbox) OnClick(handler func() error)  { registerClick(uint32(cb), handler) }
func (cb Checkbox) Destroy()                      { destroyWidget(uint32(cb)) }

func CreateToggle(x, y, w, h float32, label string) (Toggle, error) {
	body, err := json.Marshal(struct {
		X     float32 `json:"x"`
		Y     float32 `json:"y"`
		W     float32 `json:"w"`
		H     float32 `json:"h"`
		Label string  `json:"label"`
	}{x, y, w, h, label})
	if err != nil {
		return 0, err
	}
	var resp widgetResponse
	if err := json.Unmarshal(pdk.ParamBytes(natyvCreateToggleHost(pdk.ResultBytes(body))), &resp); err != nil {
		return 0, err
	}
	if resp.Error != "" {
		return 0, errors.New(resp.Error)
	}
	return Toggle(resp.WidgetID), nil
}

func (tg Toggle) SetLabel(label string) error   { return setText(uint32(tg), label) }
func (tg Toggle) Label() (string, error)        { return getText(uint32(tg)) }
func (tg Toggle) Checked() (bool, error)        { return getChecked(uint32(tg)) }
func (tg Toggle) SetChecked(checked bool) error { return setChecked(uint32(tg), checked) }
func (tg Toggle) OnClick(handler func() error)  { registerClick(uint32(tg), handler) }
func (tg Toggle) Destroy()                      { destroyWidget(uint32(tg)) }

// CreateRadioButton's groupID is an arbitrary tag the app picks -- every
// radio button sharing the same groupID is mutually exclusive (selecting
// one deselects the others in its group). Not derived from anything else
// natyv tracks, so any uint32 the app finds convenient works.
func CreateRadioButton(x, y, w, h float32, groupID uint32, label string) (RadioButton, error) {
	body, err := json.Marshal(struct {
		X       float32 `json:"x"`
		Y       float32 `json:"y"`
		W       float32 `json:"w"`
		H       float32 `json:"h"`
		Label   string  `json:"label"`
		GroupID uint32  `json:"group_id"`
	}{x, y, w, h, label, groupID})
	if err != nil {
		return 0, err
	}
	var resp widgetResponse
	if err := json.Unmarshal(pdk.ParamBytes(natyvCreateRadioButtonHost(pdk.ResultBytes(body))), &resp); err != nil {
		return 0, err
	}
	if resp.Error != "" {
		return 0, errors.New(resp.Error)
	}
	return RadioButton(resp.WidgetID), nil
}

func (r RadioButton) SetLabel(label string) error  { return setText(uint32(r), label) }
func (r RadioButton) Label() (string, error)       { return getText(uint32(r)) }
func (r RadioButton) Checked() (bool, error)       { return getChecked(uint32(r)) }
func (r RadioButton) Select() error                { return setChecked(uint32(r), true) }
func (r RadioButton) OnClick(handler func() error) { registerClick(uint32(r), handler) }
func (r RadioButton) Destroy()                     { destroyWidget(uint32(r)) }

func CreateProgressBar(x, y, w, h float32, value float32) (ProgressBar, error) {
	body, err := json.Marshal(struct {
		X     float32 `json:"x"`
		Y     float32 `json:"y"`
		W     float32 `json:"w"`
		H     float32 `json:"h"`
		Value float32 `json:"value"`
	}{x, y, w, h, value})
	if err != nil {
		return 0, err
	}
	var resp widgetResponse
	if err := json.Unmarshal(pdk.ParamBytes(natyvCreateProgressBarHost(pdk.ResultBytes(body))), &resp); err != nil {
		return 0, err
	}
	if resp.Error != "" {
		return 0, errors.New(resp.Error)
	}
	return ProgressBar(resp.WidgetID), nil
}

func (p ProgressBar) Value() (float32, error)      { return getValue(uint32(p)) }
func (p ProgressBar) SetValue(value float32) error { return setValue(uint32(p), value) }
func (p ProgressBar) Destroy()                     { destroyWidget(uint32(p)) }

func CreateSlider(x, y, w, h float32, value float32) (Slider, error) {
	body, err := json.Marshal(struct {
		X     float32 `json:"x"`
		Y     float32 `json:"y"`
		W     float32 `json:"w"`
		H     float32 `json:"h"`
		Value float32 `json:"value"`
	}{x, y, w, h, value})
	if err != nil {
		return 0, err
	}
	var resp widgetResponse
	if err := json.Unmarshal(pdk.ParamBytes(natyvCreateSliderHost(pdk.ResultBytes(body))), &resp); err != nil {
		return 0, err
	}
	if resp.Error != "" {
		return 0, errors.New(resp.Error)
	}
	return Slider(resp.WidgetID), nil
}

func (s Slider) Value() (float32, error)                    { return getValue(uint32(s)) }
func (s Slider) SetValue(value float32) error               { return setValue(uint32(s), value) }
func (s Slider) OnChange(handler func(value float32) error) { registerChange(uint32(s), handler) }
func (s Slider) Destroy()                                   { destroyWidget(uint32(s)) }

// Divider is W11 -- a thin visual rule, purely decorative (no text, no
// focus, no events). Horizontal vs. vertical is entirely a function of
// the w/h given here (or the Layout sizing passed to the Clay variant),
// not a separate field -- see Divider.zig's doc comment on the host side.
type Divider uint32

func CreateDivider(x, y, w, h float32) (Divider, error) {
	body, err := json.Marshal(struct {
		X float32 `json:"x"`
		Y float32 `json:"y"`
		W float32 `json:"w"`
		H float32 `json:"h"`
	}{x, y, w, h})
	if err != nil {
		return 0, err
	}
	var resp widgetResponse
	if err := json.Unmarshal(pdk.ParamBytes(natyvCreateDividerHost(pdk.ResultBytes(body))), &resp); err != nil {
		return 0, err
	}
	if resp.Error != "" {
		return 0, errors.New(resp.Error)
	}
	return Divider(resp.WidgetID), nil
}

func (d Divider) Destroy() { destroyWidget(uint32(d)) }

// Badge is W14 -- a small filled pill with centered text, purely
// decorative (no focus/click of its own). A dismissible tag is
// guest-composed from a Badge plus an adjacent Button (OnClick), same
// "guest composes it from primitives" precedent Breadcrumbs/Dialog already
// established -- Badge itself has no dismiss button.
type Badge uint32

// BadgeTone is a small fixed set of semantic colors -- natyv has no way
// for a guest to pick an arbitrary widget color yet (every widget's
// palette is hardcoded host-side); see Badge.zig's own doc comment.
type BadgeTone string

const (
	BadgeTonePrimary BadgeTone = "primary"
	BadgeToneSuccess BadgeTone = "success"
	BadgeToneWarning BadgeTone = "warning"
	BadgeToneDanger  BadgeTone = "danger"
	BadgeToneNeutral BadgeTone = "neutral"
)

func CreateBadge(x, y, w, h float32, tone BadgeTone, label string) (Badge, error) {
	body, err := json.Marshal(struct {
		X     float32   `json:"x"`
		Y     float32   `json:"y"`
		W     float32   `json:"w"`
		H     float32   `json:"h"`
		Tone  BadgeTone `json:"tone"`
		Label string    `json:"label"`
	}{x, y, w, h, tone, label})
	if err != nil {
		return 0, err
	}
	var resp widgetResponse
	if err := json.Unmarshal(pdk.ParamBytes(natyvCreateBadgeHost(pdk.ResultBytes(body))), &resp); err != nil {
		return 0, err
	}
	if resp.Error != "" {
		return 0, errors.New(resp.Error)
	}
	return Badge(resp.WidgetID), nil
}

func (bd Badge) SetLabel(label string) error { return setText(uint32(bd), label) }
func (bd Badge) Label() (string, error)      { return getText(uint32(bd)) }
func (bd Badge) Destroy()                    { destroyWidget(uint32(bd)) }

func setChecked(id uint32, checked bool) error {
	body, err := json.Marshal(struct {
		WidgetID uint32 `json:"widget_id"`
		Checked  bool   `json:"checked"`
	}{id, checked})
	if err != nil {
		return err
	}
	var resp struct {
		Error string `json:"error,omitempty"`
	}
	if err := json.Unmarshal(pdk.ParamBytes(natyvSetCheckedHost(pdk.ResultBytes(body))), &resp); err != nil {
		return err
	}
	if resp.Error != "" {
		return errors.New(resp.Error)
	}
	return nil
}

func getChecked(id uint32) (bool, error) {
	body, err := json.Marshal(struct {
		WidgetID uint32 `json:"widget_id"`
	}{id})
	if err != nil {
		return false, err
	}
	var resp struct {
		Checked bool   `json:"checked"`
		Error   string `json:"error,omitempty"`
	}
	if err := json.Unmarshal(pdk.ParamBytes(natyvGetCheckedHost(pdk.ResultBytes(body))), &resp); err != nil {
		return false, err
	}
	if resp.Error != "" {
		return false, errors.New(resp.Error)
	}
	return resp.Checked, nil
}

func setValue(id uint32, value float32) error {
	body, err := json.Marshal(struct {
		WidgetID uint32  `json:"widget_id"`
		Value    float32 `json:"value"`
	}{id, value})
	if err != nil {
		return err
	}
	var resp struct {
		Error string `json:"error,omitempty"`
	}
	if err := json.Unmarshal(pdk.ParamBytes(natyvSetValueHost(pdk.ResultBytes(body))), &resp); err != nil {
		return err
	}
	if resp.Error != "" {
		return errors.New(resp.Error)
	}
	return nil
}

func getValue(id uint32) (float32, error) {
	body, err := json.Marshal(struct {
		WidgetID uint32 `json:"widget_id"`
	}{id})
	if err != nil {
		return 0, err
	}
	var resp struct {
		Value float32 `json:"value"`
		Error string  `json:"error,omitempty"`
	}
	if err := json.Unmarshal(pdk.ParamBytes(natyvGetValueHost(pdk.ResultBytes(body))), &resp); err != nil {
		return 0, err
	}
	if resp.Error != "" {
		return 0, errors.New(resp.Error)
	}
	return resp.Value, nil
}

func setText(id uint32, text string) error {
	body, err := json.Marshal(struct {
		WidgetID uint32 `json:"widget_id"`
		Text     string `json:"text"`
	}{id, text})
	if err != nil {
		return err
	}
	var resp struct {
		Error string `json:"error,omitempty"`
	}
	if err := json.Unmarshal(pdk.ParamBytes(natyvSetTextHost(pdk.ResultBytes(body))), &resp); err != nil {
		return err
	}
	if resp.Error != "" {
		return errors.New(resp.Error)
	}
	return nil
}

func getText(id uint32) (string, error) {
	body, err := json.Marshal(struct {
		WidgetID uint32 `json:"widget_id"`
	}{id})
	if err != nil {
		return "", err
	}
	var resp struct {
		Text  string `json:"text"`
		Error string `json:"error,omitempty"`
	}
	if err := json.Unmarshal(pdk.ParamBytes(natyvGetTextHost(pdk.ResultBytes(body))), &resp); err != nil {
		return "", err
	}
	if resp.Error != "" {
		return "", errors.New(resp.Error)
	}
	return resp.Text, nil
}

func destroyWidget(id uint32) {
	// Best-effort: nothing meaningful for app code to do if this fails, and
	// it only ever targets widgets this guest itself created.
	body, err := json.Marshal(struct {
		WidgetID uint32 `json:"widget_id"`
	}{id})
	if err != nil {
		return
	}
	_ = natyvDestroyWidgetHost(pdk.ResultBytes(body))
}
