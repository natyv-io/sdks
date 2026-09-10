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
