// Handler-reattachment bindings -- the "resume without recreate" mechanism's
// per-widget counterpart to region.go's own per-subtree recipe replay (see
// that file's own doc comment for the mechanism it wraps).
// RegisterHandlerFunc/RegisterBinding/SnapshotBindings/RestoreBindings below
// wrap one package-level binding.Registry (see binding/binding.go for the
// real logic and its own doc comment on why it's a separate package, and
// Restore's own doc comment for why it re-populates its own active-bindings
// state on every restore -- a real, load-bearing difference from how
// region.Registry.Restore behaves). Package-level state persists across
// calls into the same plugin instance, same as widgets/internal's own
// event-handler tables -- registrations made during natyv_init are still
// there when a later natyv_checkpoint/natyv_resume call needs them.
//
// Built 2026-09-10 per ~/.claude/plans/adaptive-painting-seal.md, after a
// live Step 0 test (see project_natyv_render_loop_fix memory) confirmed
// region's own mass-rebuild-on-every-resume burst is the likely trigger for
// a real SDL3-Metal-WindowServer rendering bug -- resume should reattach
// handlers to already-existing widget ids instead of destroying and
// recreating them.
package natyv

import (
	"encoding/json"

	"github.com/natyv-io/sdks/go/binding"
	"github.com/natyv-io/sdks/go/widgets"
)

var bindingRegistry = binding.NewRegistry()

// RegisterHandlerFunc names a function so a later RestoreBindings can call
// it again by name. Call this once, e.g. during natyv_init -- registering
// the same name twice replaces the earlier registration. fn's own widgetID
// argument is whichever pre-existing widget id it's being asked to
// reattach to.
func RegisterHandlerFunc(kind string, fn func(widgetID uint32, args json.RawMessage) error) {
	bindingRegistry.RegisterHandlerFunc(kind, fn)
}

// RegisterBinding records widgetID's current reattachment recipe: on
// resume, RestoreBindings will call the function registered under kind
// (via RegisterHandlerFunc) with (widgetID, args) to reattach a real
// handler to it. Prefer the BindXClick helpers below, which call this
// atomically alongside the real handler registration -- call this
// directly only for a widget kind those don't cover yet. args must be
// JSON-marshalable; this only serializes it, it never inspects it.
func RegisterBinding(widgetID uint32, kind string, args any) error {
	return bindingRegistry.RegisterBinding(widgetID, kind, args)
}

// SnapshotBindings serializes every currently-active binding -- called
// from natyv_checkpoint alongside whatever else the app also persists.
func SnapshotBindings() (json.RawMessage, error) {
	return bindingRegistry.Snapshot()
}

// RestoreBindings reattaches every binding SnapshotBindings captured.
// Called from natyv_resume -- unlike RestoreRegions, this touches no
// widget creation/destruction/visibility at all, only in-guest handler
// registrations, so it's safe to call regardless of whether the app's
// widget tree itself was recreated this resume or not.
func RestoreBindings(data json.RawMessage) error {
	return bindingRegistry.Restore(data)
}

// BindButtonClick registers handler as btn's real OnClick AND records the
// reattachment recipe (RegisterBinding) in one call, so a dev never has
// two separate calls to keep in sync by hand -- see
// ~/.claude/plans/adaptive-painting-seal.md's own note on why the combined
// form ships instead of two adjacent calls.
func BindButtonClick(btn widgets.Button, kind string, args any, handler func() error) error {
	btn.OnClick(handler)
	return RegisterBinding(uint32(btn), kind, args)
}

// BindCheckboxClick is BindButtonClick for Checkbox.
func BindCheckboxClick(cb widgets.Checkbox, kind string, args any, handler func() error) error {
	cb.OnClick(handler)
	return RegisterBinding(uint32(cb), kind, args)
}

// BindContainerClick is BindButtonClick for Container (e.g. a whole
// clickable row).
func BindContainerClick(c widgets.Container, kind string, args any, handler func() error) error {
	c.OnClick(handler)
	return RegisterBinding(uint32(c), kind, args)
}
