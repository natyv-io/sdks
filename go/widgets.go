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

//go:wasmimport extism:host/user natyv_create_label
func natyvCreateLabelHost(uint64) uint64

//go:wasmimport extism:host/user natyv_set_text
func natyvSetTextHost(uint64) uint64

//go:wasmimport extism:host/user natyv_get_text
func natyvGetTextHost(uint64) uint64

//go:wasmimport extism:host/user natyv_destroy_widget
func natyvDestroyWidgetHost(uint64) uint64

// Button, TextField, and Label are typed widget handles (over the same
// underlying host-assigned widget_id) rather than raw ints, so app code
// reads as `addButton.OnClick(handleAdd)` instead of tracking which uint32
// means what.
type Button uint32
type TextField uint32
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
func (b Button) Destroy()                     { destroyWidget(uint32(b)) }

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
