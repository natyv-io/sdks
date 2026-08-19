// Package internal holds the wire-format plumbing every widget in the
// sibling `widgets` package shares -- the generic natyv_set_value/
// natyv_get_value/natyv_set_checked/natyv_get_checked/natyv_set_text/
// natyv_get_text/natyv_destroy_widget round trips, the shared
// {"widget_id":N}|{"error":...} response decoding, and natyv_dispatch's
// event registries (dispatch.go). None of this is meant to be called by
// app code directly -- only by the `widgets` package's own per-widget
// files, which is exactly what Go's `internal` import-path rule enforces:
// nothing outside `natyv/sdk/widgets` can import this package at all.
package internal

import (
	"encoding/json"
	"errors"

	"github.com/extism/go-pdk"
)

//go:wasmimport extism:host/user natyv_set_checked
func natyvSetCheckedHost(uint64) uint64

//go:wasmimport extism:host/user natyv_get_checked
func natyvGetCheckedHost(uint64) uint64

//go:wasmimport extism:host/user natyv_set_value
func natyvSetValueHost(uint64) uint64

//go:wasmimport extism:host/user natyv_get_value
func natyvGetValueHost(uint64) uint64

//go:wasmimport extism:host/user natyv_set_text
func natyvSetTextHost(uint64) uint64

//go:wasmimport extism:host/user natyv_get_text
func natyvGetTextHost(uint64) uint64

//go:wasmimport extism:host/user natyv_destroy_widget
func natyvDestroyWidgetHost(uint64) uint64

//go:wasmimport extism:host/user natyv_set_visible
func natyvSetVisibleHost(uint64) uint64

//go:wasmimport extism:host/user natyv_get_scroll_position
func natyvGetScrollPositionHost(uint64) uint64

//go:wasmimport extism:host/user natyv_scroll_into_view
func natyvScrollIntoViewHost(uint64) uint64

// WidgetResponse is the shared {"widget_id":N}|{"error":...} shape every
// natyv_clay_create_* host function replies with.
type WidgetResponse struct {
	WidgetID uint32 `json:"widget_id"`
	Error    string `json:"error,omitempty"`
}

// DecodeWidgetResponse factors out the shared response-decoding boilerplate
// every widget's own Create function needs -- callers marshal their own
// request body and pass the raw host-call result here to decode.
func DecodeWidgetResponse(resultOffset uint64) (WidgetResponse, error) {
	var resp WidgetResponse
	if err := json.Unmarshal(pdk.ParamBytes(resultOffset), &resp); err != nil {
		return WidgetResponse{}, err
	}
	if resp.Error != "" {
		return WidgetResponse{}, errors.New(resp.Error)
	}
	return resp, nil
}

func SetChecked(id uint32, checked bool) error {
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

func GetChecked(id uint32) (bool, error) {
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

func SetValue(id uint32, value float32) error {
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

func GetValue(id uint32) (float32, error) {
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

func SetText(id uint32, text string) error {
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

func GetText(id uint32) (string, error) {
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

// SetVisible shows/hides id's whole subtree without destroying it -- see
// host-side WidgetHostFunctions.zig's setVisibleHostFn doc comment. Built
// for Accordion (accordion.go composes this with an existing Button/
// Container rather than a new widget kind) but generic to any widget id.
func SetVisible(id uint32, visible bool) error {
	body, err := json.Marshal(struct {
		WidgetID uint32 `json:"widget_id"`
		Visible  bool   `json:"visible"`
	}{id, visible})
	if err != nil {
		return err
	}
	var resp struct {
		Error string `json:"error,omitempty"`
	}
	if err := json.Unmarshal(pdk.ParamBytes(natyvSetVisibleHost(pdk.ResultBytes(body))), &resp); err != nil {
		return err
	}
	if resp.Error != "" {
		return errors.New(resp.Error)
	}
	return nil
}

// ScrollPosition mirrors WidgetHostFunctions.zig's getScrollPositionHostFn
// response -- Clay's live scroll offset/dimensions for a scroll container,
// as of the most recent real layout pass (see host-side
// WidgetHost.Slot.scroll_data's own doc comment for the staleness note).
type ScrollPosition struct {
	ScrollOffsetX float32 `json:"scroll_offset_x"`
	ScrollOffsetY float32 `json:"scroll_offset_y"`
	ContainerW    float32 `json:"container_w"`
	ContainerH    float32 `json:"container_h"`
	ContentW      float32 `json:"content_w"`
	ContentH      float32 `json:"content_h"`
}

// GetScrollPosition errors if id doesn't name a scroll container (Layout's
// ScrollVertical/ScrollHorizontal weren't set at create time).
func GetScrollPosition(id uint32) (ScrollPosition, error) {
	body, err := json.Marshal(struct {
		WidgetID uint32 `json:"widget_id"`
	}{id})
	if err != nil {
		return ScrollPosition{}, err
	}
	var resp struct {
		ScrollPosition
		Error string `json:"error,omitempty"`
	}
	if err := json.Unmarshal(pdk.ParamBytes(natyvGetScrollPositionHost(pdk.ResultBytes(body))), &resp); err != nil {
		return ScrollPosition{}, err
	}
	if resp.Error != "" {
		return ScrollPosition{}, errors.New(resp.Error)
	}
	return resp.ScrollPosition, nil
}

// ScrollIntoView scrolls id's nearest scrollable ancestor just enough to
// bring id back within its viewport -- a harmless no-op if id has no
// scrollable ancestor or is already fully visible. Built for Accordion
// (accordion.go calls this after expanding a section), but generic to any
// widget id -- see WidgetHostFunctions.zig's scrollIntoViewHostFn.
func ScrollIntoView(id uint32) error {
	body, err := json.Marshal(struct {
		WidgetID uint32 `json:"widget_id"`
	}{id})
	if err != nil {
		return err
	}
	var resp struct {
		Error string `json:"error,omitempty"`
	}
	if err := json.Unmarshal(pdk.ParamBytes(natyvScrollIntoViewHost(pdk.ResultBytes(body))), &resp); err != nil {
		return err
	}
	if resp.Error != "" {
		return errors.New(resp.Error)
	}
	return nil
}

func DestroyWidget(id uint32) {
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
