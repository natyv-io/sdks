package widgets

import "github.com/natyv-io/sdks/go/widgets/internal"

// App lifecycle. Neither of these is a widget, and neither is specific to
// the system tray -- but both exist because of it: a tray is usually there
// so an app can keep running with nothing on screen, which natyv had no way
// to express before. An app with no tray can use both.

// StartupWindow is the id to pass to SetWindowVisible for the window natyv
// opens on its own at launch. That window has no root widget id of its own,
// unlike one opened with CreateWindow, so 0 is how it is named.
const StartupWindow uint32 = 0

// SetWindowVisible shows or hides a real OS window. Hiding is not closing:
// the window keeps its widgets, its scroll positions and its text, and
// showing it again brings all of that back, so this is what "minimize to
// tray" should use rather than destroying and rebuilding the window.
//
// windowID is a window's own ID(), or StartupWindow.
func SetWindowVisible(windowID uint32, visible bool) error {
	return internal.SetWindowVisible(windowID, visible)
}

// SetQuitOnLastWindowClose controls what the startup window's close button
// does. The default is true -- closing it quits -- and that is what every
// app gets unless it calls this.
//
// Passing false turns that close into an ordinary OnCloseRequested event on
// notifyID instead, exactly like a window opened with CreateWindow already
// gets, and the app keeps running. natyv still destroys nothing by itself:
// a handler that does nothing leaves the window open.
//
// notifyID must be a real widget the app owns -- typically its root
// container. It cannot be 0: natyv dispatches its own internal no-op events
// to id 0, so a handler registered there would receive those too.
//
// Register the handler before calling this -- the user can reach the close
// button at any moment, and a close with nothing registered is silently
// ignored. OnStartupWindowClose below does both in the right order and is
// the better call unless you need them apart.
func SetQuitOnLastWindowClose(quit bool, notifyID uint32) error {
	return internal.SetQuitOnLastWindowClose(quit, notifyID)
}

// OnStartupWindowClose makes the startup window's close button run handler
// instead of quitting the app. This is the whole "minimize to tray" gesture
// in one call:
//
//	widgets.OnStartupWindowClose(uint32(root), func() error {
//	    return widgets.SetWindowVisible(widgets.StartupWindow, false)
//	})
//
// notifyID is any widget the app owns -- its root container is the natural
// choice. The startup window has no id of its own to address, which is why
// one has to be supplied.
//
// Doing it in one call is not just sugar: it registers the handler before
// turning off quit-on-close, which is the order that cannot leave the app
// with a close button that does nothing.
func OnStartupWindowClose(notifyID uint32, handler func() error) error {
	WrapWindow(notifyID).OnCloseRequested(handler)
	return SetQuitOnLastWindowClose(false, notifyID)
}
