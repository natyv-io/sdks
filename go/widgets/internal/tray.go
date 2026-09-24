package internal

import (
	"encoding/json"
	"errors"

	pdk "github.com/extism/go-pdk"
)

// System tray host calls. Split into their own file rather than added to
// value.go, which is already the catch-all for every widget accessor --
// nothing here touches a widget.
//
// Every call answers `{}` or `{"error":"..."}`, except the two that return
// an id and the one that returns a checkbox's state. An id comes back
// before the tray or entry actually exists on screen: natyv-core assigns it
// synchronously and materializes the real SDL object on its next frame, so
// a handler can be registered and children added immediately. Same contract
// CreateWindow already offers.

//go:wasmimport extism:host/user natyv_tray_create
func natyvTrayCreateHost(uint64) uint64

//go:wasmimport extism:host/user natyv_tray_destroy
func natyvTrayDestroyHost(uint64) uint64

//go:wasmimport extism:host/user natyv_tray_set_tooltip
func natyvTraySetTooltipHost(uint64) uint64

//go:wasmimport extism:host/user natyv_tray_insert_entry
func natyvTrayInsertEntryHost(uint64) uint64

//go:wasmimport extism:host/user natyv_tray_remove_entry
func natyvTrayRemoveEntryHost(uint64) uint64

//go:wasmimport extism:host/user natyv_tray_set_entry_label
func natyvTraySetEntryLabelHost(uint64) uint64

//go:wasmimport extism:host/user natyv_tray_set_entry_checked
func natyvTraySetEntryCheckedHost(uint64) uint64

//go:wasmimport extism:host/user natyv_tray_set_entry_enabled
func natyvTraySetEntryEnabledHost(uint64) uint64

//go:wasmimport extism:host/user natyv_tray_entry_checked
func natyvTrayEntryCheckedHost(uint64) uint64

type trayErrResp struct {
	Error string `json:"error,omitempty"`
}

type trayIDResp struct {
	ID    uint32 `json:"id"`
	Error string `json:"error,omitempty"`
}

type trayCheckedResp struct {
	Checked bool   `json:"checked"`
	Error   string `json:"error,omitempty"`
}

// trayResult decodes the `{}`/`{"error":"..."}` reply every mutating tray
// call shares.
//
// Deliberately takes the *result* of the host call rather than the host
// function itself: TinyGo rejects using a //go:wasmimport function as a
// value ("cannot use an exported function as value"), so a helper that took
// `fn func(uint64) uint64` would compile under plain Go and fail under the
// only toolchain that actually builds a guest. Every call site below makes
// its own import call inline for that reason, matching value.go.
func trayResult(raw uint64) error {
	var resp trayErrResp
	if err := json.Unmarshal(pdk.ParamBytes(raw), &resp); err != nil {
		return err
	}
	if resp.Error != "" {
		return errors.New(resp.Error)
	}
	return nil
}

func TrayCreate(tooltip string) (uint32, error) {
	body, err := json.Marshal(struct {
		Tooltip string `json:"tooltip"`
	}{tooltip})
	if err != nil {
		return 0, err
	}
	var resp trayIDResp
	if err := json.Unmarshal(pdk.ParamBytes(natyvTrayCreateHost(pdk.ResultBytes(body))), &resp); err != nil {
		return 0, err
	}
	if resp.Error != "" {
		return 0, errors.New(resp.Error)
	}
	return resp.ID, nil
}

func TrayDestroy(id uint32) error {
	body, err := json.Marshal(struct {
		ID uint32 `json:"id"`
	}{id})
	if err != nil {
		return err
	}
	return trayResult(natyvTrayDestroyHost(pdk.ResultBytes(body)))
}

func TraySetTooltip(id uint32, tooltip string) error {
	body, err := json.Marshal(struct {
		ID      uint32 `json:"id"`
		Tooltip string `json:"tooltip"`
	}{id, tooltip})
	if err != nil {
		return err
	}
	return trayResult(natyvTraySetTooltipHost(pdk.ResultBytes(body)))
}

// TrayInsertEntry adds an entry to parentID's menu -- a tray id for a
// top-level entry, or a submenu entry's id for a nested one. pos follows
// SDL's own convention: -1 appends.
func TrayInsertEntry(parentID uint32, pos int32, kind, label string, checked, enabled bool) (uint32, error) {
	body, err := json.Marshal(struct {
		ParentID uint32 `json:"parent_id"`
		Pos      int32  `json:"pos"`
		Kind     string `json:"kind"`
		Label    string `json:"label"`
		Checked  bool   `json:"checked"`
		Enabled  bool   `json:"enabled"`
	}{parentID, pos, kind, label, checked, enabled})
	if err != nil {
		return 0, err
	}
	var resp trayIDResp
	if err := json.Unmarshal(pdk.ParamBytes(natyvTrayInsertEntryHost(pdk.ResultBytes(body))), &resp); err != nil {
		return 0, err
	}
	if resp.Error != "" {
		return 0, errors.New(resp.Error)
	}
	return resp.ID, nil
}

func TrayRemoveEntry(id uint32) error {
	body, err := json.Marshal(struct {
		ID uint32 `json:"id"`
	}{id})
	if err != nil {
		return err
	}
	return trayResult(natyvTrayRemoveEntryHost(pdk.ResultBytes(body)))
}

func TraySetEntryLabel(id uint32, label string) error {
	body, err := json.Marshal(struct {
		ID    uint32 `json:"id"`
		Label string `json:"label"`
	}{id, label})
	if err != nil {
		return err
	}
	return trayResult(natyvTraySetEntryLabelHost(pdk.ResultBytes(body)))
}

func TraySetEntryChecked(id uint32, checked bool) error {
	body, err := json.Marshal(struct {
		ID      uint32 `json:"id"`
		Checked bool   `json:"checked"`
	}{id, checked})
	if err != nil {
		return err
	}
	return trayResult(natyvTraySetEntryCheckedHost(pdk.ResultBytes(body)))
}

func TraySetEntryEnabled(id uint32, enabled bool) error {
	body, err := json.Marshal(struct {
		ID      uint32 `json:"id"`
		Enabled bool   `json:"enabled"`
	}{id, enabled})
	if err != nil {
		return err
	}
	return trayResult(natyvTraySetEntryEnabledHost(pdk.ResultBytes(body)))
}

// TrayEntryChecked reads a checkbox entry's current state. Answered from
// natyv-core's own mirror of it, not by asking SDL: the user clicking a
// checkbox flips it inside SDL with no guest involvement, and reading it
// back is a main-thread-only call, so the host records what SDL reports at
// click time and serves it from there.
func TrayEntryChecked(id uint32) (bool, error) {
	body, err := json.Marshal(struct {
		ID uint32 `json:"id"`
	}{id})
	if err != nil {
		return false, err
	}
	var resp trayCheckedResp
	if err := json.Unmarshal(pdk.ParamBytes(natyvTrayEntryCheckedHost(pdk.ResultBytes(body))), &resp); err != nil {
		return false, err
	}
	if resp.Error != "" {
		return false, errors.New(resp.Error)
	}
	return resp.Checked, nil
}

// -- App lifecycle --
//
// Not tray calls, but they arrived with the tray and share its reason for
// existing: an app that keeps running with no window on screen. See
// widgets/window_lifecycle.go.

//go:wasmimport extism:host/user natyv_set_window_visible
func natyvSetWindowVisibleHost(uint64) uint64

//go:wasmimport extism:host/user natyv_set_quit_on_last_window_close
func natyvSetQuitOnLastWindowCloseHost(uint64) uint64

func SetWindowVisible(windowID uint32, visible bool) error {
	body, err := json.Marshal(struct {
		WindowID uint32 `json:"window_id"`
		Visible  bool   `json:"visible"`
	}{windowID, visible})
	if err != nil {
		return err
	}
	return trayResult(natyvSetWindowVisibleHost(pdk.ResultBytes(body)))
}

func SetQuitOnLastWindowClose(quit bool, notifyID uint32) error {
	body, err := json.Marshal(struct {
		Quit           bool   `json:"quit"`
		NotifyWidgetID uint32 `json:"notify_widget_id"`
	}{quit, notifyID})
	if err != nil {
		return err
	}
	return trayResult(natyvSetQuitOnLastWindowCloseHost(pdk.ResultBytes(body)))
}
