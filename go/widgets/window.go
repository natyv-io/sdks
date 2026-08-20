package widgets

import (
	"encoding/json"
	"errors"

	"github.com/extism/go-pdk"

	"natyv/sdk/widgets/internal"
)

//go:wasmimport extism:host/user natyv_clay_create_window
func natyvClayCreateWindowHost(uint64) uint64

//go:wasmimport extism:host/user natyv_destroy_window
func natyvDestroyWindowHost(uint64) uint64

// Window is a typed widget handle over a real second OS window's own root
// widget_id -- see host-side WidgetHost.ClayStyle.window_root's own doc
// comment. Unlike every other widget kind, a Window has no parent of its
// own at all (a real OS window can't be a Clay child of anything) -- its
// own RootID() is what every child widget's own Layout.ParentID names to
// put content inside it, the same idiom any other Clay parent (Container,
// Tabs, ...) already uses.
type Window uint32

// CreateWindow opens a real second OS window. The returned Window's own
// widget_id is real and usable immediately -- child widgets can be
// parented under RootID() right away -- even though the actual
// SDL_Window/SDL_Renderer/TTF_TextEngine only materializes a frame or so
// later, once the host's main thread drains its own pending-window-request
// queue. See host-side WidgetHost.pending_window_requests' own doc comment
// for the full two-part story.
func CreateWindow(title string, width, height float32) (Window, error) {
	body, err := json.Marshal(struct {
		Title  string  `json:"title"`
		Width  float32 `json:"width"`
		Height float32 `json:"height"`
	}{title, width, height})
	if err != nil {
		return 0, err
	}
	resp, err := internal.DecodeWidgetResponse(natyvClayCreateWindowHost(pdk.ResultBytes(body)))
	if err != nil {
		return 0, err
	}
	return Window(resp.WidgetID), nil
}

// RootID is what every child widget's own Layout.ParentID names to put
// content inside this window.
func (w Window) RootID() uint32 { return uint32(w) }

// OnCloseRequested registers the handler for this window's own "please
// close" request -- fired when the user clicks its real OS close button.
// The host never force-closes it: the handler decides whether to actually
// call Close(), the same discretion Container's own OnDismiss already
// gives a modal over Escape/a backdrop click. A handler that only
// sometimes calls Close() (e.g. guarded on some "has unsaved changes"
// flag) is a complete "are you sure?" veto flow at zero extra host-side
// cost -- and if this is never registered at all, the window simply never
// closes from its own OS button, same as an unhandled modal Escape
// already does nothing.
func (w Window) OnCloseRequested(handler func() error) {
	internal.RegisterWindowCloseRequested(uint32(w), handler)
}

// Close destroys this window's own widget subtree (every child, cascading)
// and its real OS resources -- deliberately not internal.DestroyWidget,
// whose contract is explicitly no-cascade (every other composed widget
// destroys its own children manually, which is untenable for "close this
// whole window"). See host-side destroyWindowHostFn's own doc comment.
func (w Window) Close() error {
	body, err := json.Marshal(struct {
		WidgetID uint32 `json:"widget_id"`
	}{uint32(w)})
	if err != nil {
		return err
	}
	var resp struct {
		Error string `json:"error,omitempty"`
	}
	if err := json.Unmarshal(pdk.ParamBytes(natyvDestroyWindowHost(pdk.ResultBytes(body))), &resp); err != nil {
		return err
	}
	if resp.Error != "" {
		return errors.New(resp.Error)
	}
	return nil
}
