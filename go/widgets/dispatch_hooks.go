package widgets

import "github.com/natyv-io/sdks/go/widgets/internal"

// SetPostDispatchHook wires fn to run at the very start of every real
// natyv_dispatch call. Exists so natyv's own root package (github.com/
// natyv-io/sdks/go) can hook region.Registry.FlushPendingReveals into the
// real dispatch router without needing direct access to the `internal`
// package itself -- Go's own internal-package visibility rule only allows
// this `widgets` package (internal's real parent) to import it, and the
// root package is a sibling, not a subpackage, of widgets.
func SetPostDispatchHook(fn func()) {
	internal.PostDispatchHook = fn
}

// HandleEventType routes every dispatched event of eventType to fn, with
// the id it was addressed to and its raw JSON payload. For sibling SDK
// packages whose events aren't widget events (the `hid` package's
// `hid_report`/`hid_disconnected`) -- same internal-package bridge as
// SetPostDispatchHook. App code has no reason to call this: those packages
// register themselves and expose typed handlers instead.
func HandleEventType(eventType string, fn func(id uint32, payload string) error) {
	internal.RegisterEventType(eventType, fn)
}
