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

//go:wasmimport extism:host/user natyv_set_range
func natyvSetRangeHost(uint64) uint64

//go:wasmimport extism:host/user natyv_get_range
func natyvGetRangeHost(uint64) uint64

//go:wasmimport extism:host/user natyv_set_text
func natyvSetTextHost(uint64) uint64

//go:wasmimport extism:host/user natyv_get_text
func natyvGetTextHost(uint64) uint64

//go:wasmimport extism:host/user natyv_destroy_widget
func natyvDestroyWidgetHost(uint64) uint64

//go:wasmimport extism:host/user natyv_set_visible
func natyvSetVisibleHost(uint64) uint64

//go:wasmimport extism:host/user natyv_set_size
func natyvSetSizeHost(uint64) uint64

//go:wasmimport extism:host/user natyv_set_style
func natyvSetStyleHost(uint64) uint64

//go:wasmimport extism:host/user natyv_get_scroll_position
func natyvGetScrollPositionHost(uint64) uint64

//go:wasmimport extism:host/user natyv_scroll_into_view
func natyvScrollIntoViewHost(uint64) uint64

//go:wasmimport extism:host/user natyv_show_open_file_dialog
func natyvShowOpenFileDialogHost(uint64) uint64

//go:wasmimport extism:host/user natyv_show_save_file_dialog
func natyvShowSaveFileDialogHost(uint64) uint64

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

// SetRange sets both ends of a RangeSlider's span at once -- see host-side
// WidgetHostFunctions.zig's setRangeHostFn doc comment for why this is one
// call, not two separate SetValue-style calls (setting each end
// independently would clamp against whatever the *other* one still is at
// that moment, which can reject a legitimate new pair depending on call
// order).
func SetRange(id uint32, min, max float32) error {
	body, err := json.Marshal(struct {
		WidgetID uint32  `json:"widget_id"`
		Min      float32 `json:"min"`
		Max      float32 `json:"max"`
	}{id, min, max})
	if err != nil {
		return err
	}
	var resp struct {
		Error string `json:"error,omitempty"`
	}
	if err := json.Unmarshal(pdk.ParamBytes(natyvSetRangeHost(pdk.ResultBytes(body))), &resp); err != nil {
		return err
	}
	if resp.Error != "" {
		return errors.New(resp.Error)
	}
	return nil
}

// GetRange reads a RangeSlider's current {min, max} -- the RangeSlider
// counterpart to GetValue.
func GetRange(id uint32) (min, max float32, err error) {
	body, marshalErr := json.Marshal(struct {
		WidgetID uint32 `json:"widget_id"`
	}{id})
	if marshalErr != nil {
		return 0, 0, marshalErr
	}
	var resp struct {
		Min   float32 `json:"min"`
		Max   float32 `json:"max"`
		Error string  `json:"error,omitempty"`
	}
	if err := json.Unmarshal(pdk.ParamBytes(natyvGetRangeHost(pdk.ResultBytes(body))), &resp); err != nil {
		return 0, 0, err
	}
	if resp.Error != "" {
		return 0, 0, errors.New(resp.Error)
	}
	return resp.Min, resp.Max, nil
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

// SetHeight resizes id in place to a Fixed height (min=max=height,
// regardless of whatever sizing type it was created with) -- see host-side
// WidgetHostFunctions.zig's setSizeHostFn doc comment. Built for Tree
// view's virtualized spacer Containers (tree.go composes this instead of
// destroying and recreating them on every scroll tick) but generic to any
// widget id. Width is left untouched -- no current caller needs both axes.
func SetHeight(id uint32, height float32) error {
	body, err := json.Marshal(struct {
		WidgetID uint32  `json:"widget_id"`
		Height   float32 `json:"height"`
	}{id, height})
	if err != nil {
		return err
	}
	var resp struct {
		Error string `json:"error,omitempty"`
	}
	if err := json.Unmarshal(pdk.ParamBytes(natyvSetSizeHost(pdk.ResultBytes(body))), &resp); err != nil {
		return err
	}
	if resp.Error != "" {
		return errors.New(resp.Error)
	}
	return nil
}

// ColorValue/PaddingValue are plain wire-shaped mirrors of
// WidgetHostFunctions.zig's SetStyleRequest -- 0..1 floats for color,
// matching the stylesheet resolver's own convention (src/styling/
// Resolver.zig), not the 0-255 range some other Go color types use.
// Deliberately local to this package rather than reusing the sibling
// `widgets` package's own nicer `Color`/`Padding` types: `internal` can't
// import `widgets` (that package already imports `internal` -- Go doesn't
// allow the cycle), so `widgets.ApplyStyle` converts down to these at the
// call site instead.
type ColorValue struct {
	R float32 `json:"r"`
	G float32 `json:"g"`
	B float32 `json:"b"`
	A float32 `json:"a"`
}

type PaddingValue struct {
	Left   uint16 `json:"left"`
	Right  uint16 `json:"right"`
	Top    uint16 `json:"top"`
	Bottom uint16 `json:"bottom"`
}

// SetStyle applies already-resolved style values to an existing widget --
// bg/padding are nil when the caller (widgets.ApplyStyle) didn't resolve
// that property from any of the applied tokens, omitted from the wire JSON
// entirely via `omitempty` rather than sent as an explicit zero/null, so
// WidgetHostFunctions.zig's SetStyleRequest sees "leave this property
// unchanged" (its own `?T = null` default), not "set it to zero." See
// setStyleHostFn's doc comment for why this never takes a style-token name.
func SetStyle(id uint32, bg *ColorValue, padding *PaddingValue) error {
	body, err := json.Marshal(struct {
		WidgetID        uint32        `json:"widget_id"`
		BackgroundColor *ColorValue   `json:"background_color,omitempty"`
		Padding         *PaddingValue `json:"padding,omitempty"`
	}{id, bg, padding})
	if err != nil {
		return err
	}
	var resp struct {
		Error string `json:"error,omitempty"`
	}
	if err := json.Unmarshal(pdk.ParamBytes(natyvSetStyleHost(pdk.ResultBytes(body))), &resp); err != nil {
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

// ShowOpenFileDialog queues an "open file" native OS dialog request for the
// host's main thread to actually show next frame -- see
// WidgetHostFunctions.zig's showOpenFileDialogHostFn doc comment. The real
// result (chosen path(s), or none) arrives later as a real event through
// RegisterFileSelected(id, ...), not a return value here -- register that
// before calling this.
func ShowOpenFileDialog(id uint32, allowMany bool) error {
	body, err := json.Marshal(struct {
		WidgetID  uint32 `json:"widget_id"`
		AllowMany bool   `json:"allow_many"`
	}{id, allowMany})
	if err != nil {
		return err
	}
	var resp struct {
		Error string `json:"error,omitempty"`
	}
	if err := json.Unmarshal(pdk.ParamBytes(natyvShowOpenFileDialogHost(pdk.ResultBytes(body))), &resp); err != nil {
		return err
	}
	if resp.Error != "" {
		return errors.New(resp.Error)
	}
	return nil
}

// ShowSaveFileDialog is ShowOpenFileDialog's save-dialog counterpart -- no
// allowMany (SDL_ShowSaveFileDialog has no such parameter), same
// RegisterFileSelected result delivery.
func ShowSaveFileDialog(id uint32) error {
	body, err := json.Marshal(struct {
		WidgetID uint32 `json:"widget_id"`
	}{id})
	if err != nil {
		return err
	}
	var resp struct {
		Error string `json:"error,omitempty"`
	}
	if err := json.Unmarshal(pdk.ParamBytes(natyvShowSaveFileDialogHost(pdk.ResultBytes(body))), &resp); err != nil {
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
