// Package binding holds the handler-reattachment registry -- the memory-
// recycle "resume without recreate" mechanism's per-widget counterpart to
// region's own per-subtree recipe replay (see region/region.go's own doc
// comment for that mechanism, and ~/.claude/plans/adaptive-painting-seal.md
// for the full design and why this exists: region's own mass-rebuild-on-
// every-resume burst is the likely trigger for a real SDL3-Metal-
// WindowServer rendering bug, confirmed via a live Step 0 test, 2026-09-10
// -- see project_natyv_render_loop_fix memory). A binding is a single
// widget id plus a named "kind" (a rebind recipe registered ahead of time)
// and small serializable args -- natyv_resume replays these to reattach a
// real handler to an already-existing widget, instead of region's own
// destroy-and-rebuild-the-subtree approach.
//
// Deliberately its own package, separate from natyv's own root package,
// mirroring region's own split rationale exactly (see region/region.go's
// doc comment) -- keeps this natively testable without a real host, since
// the widgets package's own real primitives transitively import go-pdk,
// whose //go:wasmimport declarations don't compile for a native target.
package binding

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/natyv-io/sdks/go/persistjson"
)

// ActiveBinding is one currently-registered handler reattachment: which
// widget id it's for, which registered "kind" describes how to reattach
// it, and the small serializable args that rebind function needs.
type ActiveBinding struct {
	WidgetID uint32          `json:"widget_id"`
	Kind     string          `json:"kind"`
	Args     json.RawMessage `json:"args"`
}

// Registry holds the kind->rebind-function map and the currently-active
// bindings. Unlike region.Registry, this has nothing to build, reveal, or
// destroy -- no widget primitives are injected here at all, since
// reattaching a handler touches nothing about a widget's own existence or
// visibility, only its in-guest handler-map entry.
type Registry struct {
	// rebindFuncs is the kind->function registry a dev populates once
	// (e.g. at natyv_init time) via RegisterHandlerFunc. Keyed by a plain
	// string, same rationale as region.Registry.rebuildFuncs: a captured
	// Go closure can't survive a recycle -- the whole guest instance,
	// including every closure it ever held, is destroyed by the host --
	// only a name plus serializable args can cross that boundary via
	// natyv_checkpoint's own bytes.
	rebindFuncs map[string]func(widgetID uint32, args json.RawMessage) error

	// activeBindings is keyed by widget id -- a later call for the same
	// id replaces its previous recipe wholesale, mirroring
	// region.Registry.activeRegions' own "later call is the current
	// truth" semantics.
	activeBindings map[uint32]ActiveBinding
}

// NewRegistry constructs an empty Registry.
func NewRegistry() *Registry {
	return &Registry{
		rebindFuncs:    map[string]func(widgetID uint32, args json.RawMessage) error{},
		activeBindings: map[uint32]ActiveBinding{},
	}
}

// RegisterHandlerFunc names a function so a later Restore can call it
// again by name. Call this once, e.g. during natyv_init -- registering
// the same name twice replaces the earlier registration.
func (r *Registry) RegisterHandlerFunc(kind string, fn func(widgetID uint32, args json.RawMessage) error) {
	r.rebindFuncs[kind] = fn
}

// RegisterBinding records widgetID's current reattachment recipe: on
// Restore, the function registered under kind gets called with
// (widgetID, args) to reattach a real handler to it. Call this every time
// a real handler is attached to a widget that should survive a recycle --
// exactly alongside the real .OnClick/.OnChange call, not instead of it
// (see the root natyv package's BindButtonClick and friends, which do
// both atomically -- the whole point being that a dev never has two
// separate calls to keep in sync by hand).
//
// Uses persistjson.Marshal, not stdlib encoding/json.Marshal, matching
// region.Registry.SetActiveRegion's own rationale exactly: args is
// arbitrary, dev-authored data.
func (r *Registry) RegisterBinding(widgetID uint32, kind string, args any) error {
	encoded, err := persistjson.Marshal(args)
	if err != nil {
		return err
	}
	r.activeBindings[widgetID] = ActiveBinding{WidgetID: widgetID, Kind: kind, Args: encoded}
	return nil
}

// Snapshot serializes every currently-active binding -- called from
// natyv_checkpoint alongside whatever else the app also persists.
func (r *Registry) Snapshot() (json.RawMessage, error) {
	list := make([]ActiveBinding, 0, len(r.activeBindings))
	for _, b := range r.activeBindings {
		list = append(list, b)
	}
	// Sorted for deterministic output, matching region.Registry.Snapshot's
	// own rationale -- map iteration order is unspecified.
	sort.Slice(list, func(i, j int) bool { return list[i].WidgetID < list[j].WidgetID })
	return json.Marshal(list)
}

// Restore reattaches every binding Snapshot captured: for each one, looks
// up the registered kind and calls it with (WidgetID, Args).
//
// Real, load-bearing difference from region.Registry.Restore: region
// safely leaves its own activeRegions untouched on restore, because the
// rebuild function it calls naturally re-triggers SetActiveRegion as an
// ordinary side effect of normal app navigation -- every real content
// change re-records the current recipe. Most bindings have no such
// natural re-trigger: they're registered exactly once, at widget
// creation, which under this whole mechanism's premise is once *ever* for
// the life of an instance. So Restore itself re-populates activeBindings
// with every successfully-processed entry -- safe here, unlike region,
// since a binding's kind+args don't change as a side effect of being
// restored. Without this, a second recycle with no intervening
// interaction that happens to re-touch a given binding would silently,
// permanently drop it on the next resume -- the exact failure mode this
// whole mechanism exists to eliminate, just deferred to the second
// recycle instead of the first.
//
// Also deliberately different from region.Registry.Restore's hard-stop-on-
// first-error behavior: a binding whose kind wasn't (re-)registered, or
// whose rebind function itself fails, doesn't abort every other binding's
// restore -- it's recorded (returned as the first error encountered) and
// the loop continues. With dozens of individual widget bindings instead
// of region's handful of subtree recipes, one broken binding sinking every
// other already-correct one is a much worse failure mode here than it
// would be for region -- matches region.Registry.FlushPendingReveals' own
// "continue past one failure" posture, not Restore's.
func (r *Registry) Restore(data json.RawMessage) error {
	if len(data) == 0 {
		return nil
	}
	var list []ActiveBinding
	if err := json.Unmarshal(data, &list); err != nil {
		return err
	}
	var firstErr error
	for _, b := range list {
		fn, ok := r.rebindFuncs[b.Kind]
		if !ok {
			if firstErr == nil {
				firstErr = fmt.Errorf("natyv: no rebind function registered for %q", b.Kind)
			}
			continue
		}
		if err := fn(b.WidgetID, b.Args); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		r.activeBindings[b.WidgetID] = b
	}
	return firstErr
}
