// Package region holds the rebuildable-regions registry logic itself --
// the memory-recycle ergonomics layer's real mechanism for widget/handler
// state (see the plan's own "Mechanism B" section,
// ~/.claude/plans/lexical-wishing-penguin.md). A region is a widget
// subtree rooted at a stable parent (one whose id never changes across a
// recycle); RegisterRebuildFunc/SetActiveRegion let a dev name the current
// recipe for reconstructing it, so a resumed guest instance can rebuild it
// by calling that same real function again, rather than trying to capture
// and replay what it did.
//
// Deliberately its own package, separate from natyv's own root package
// (which wraps one package-level Registry as RegisterRebuildFunc/
// SetActiveRegion/SnapshotRegions/RestoreRegions, see ../region.go): the
// root package's own widget calls go through
// github.com/natyv-io/sdks/go/widgets, which transitively imports go-pdk,
// whose //go:wasmimport declarations don't compile for a native target at
// all -- keeping this file's own registry/snapshot/restore logic here,
// with the real widget primitives injected as plain function values
// rather than imported directly, is what makes it possible to write real,
// natively-runnable go test coverage for the mechanism itself (construct
// a fake multi-widget region, force a real recycle, confirm handler
// re-wiring and orphan cleanup) without needing a real host at all.
//
// Restore's own swap is make-before-break, not destroy-then-rebuild: a
// region's replacement content is built as a hidden new child of its
// stable parent *before* the previous content is touched, and the
// previous content is only destroyed after the replacement is confirmed
// fully built and revealed. This is what keeps a region whose rebuild does
// anything slow (a network reconnect, say) from ever showing a visibly
// empty or half-built state -- see Restore's own doc comment for the full
// sequence.
package region

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/natyv-io/sdks/go/persistjson"
)

// ContentLayout carries the subset of widgets.Layout that determines how a
// region's own content is arranged -- captured at SetActiveRegion time
// (the caller already knows this, since it just built the real content
// under this same layout) and threaded through to Restore's staging
// container so it matches what the region's real content actually needs,
// instead of CreateChild's own generic caller deciding on a fixed default
// with no way to know the region's real Direction/Padding/etc. Real,
// live-caught bug this fixes (2026-09-09, spikes/memory-recycle-phase2):
// a staging container built with no Direction set falls back to the host's
// own left_to_right leaf-widget default, silently re-laying out a
// TopToBottom region's content as a row the first time it's rebuilt after
// a recycle -- see project_natyv_ergonomics_layer memory for the full
// finding.
//
// Deliberately NOT the real widgets.Layout -- this package stays free of
// any widgets import (see this file's own doc comment on why), and most of
// Layout's fields are meaningless or actively wrong for a staging
// container: its ParentID is always CreateChild's own parentID, its Sizing
// is always Grow/Grow (a region fills whatever space its stable parent
// leaves it, unchanged by this), and it can never itself be a
// floating/modal/toast layer.
type ContentLayout struct {
	Direction        string
	PaddingLeft      uint16
	PaddingRight     uint16
	PaddingTop       uint16
	PaddingBottom    uint16
	ChildGap         uint16
	ChildAlignX      string
	ChildAlignY      string
	ScrollVertical   bool
	ScrollHorizontal bool
}

// ActiveRegion is one currently-active region: the stable parent widget id
// it's rooted at, which registered function currently describes its
// contents, the exact arguments that function needs to reproduce them, and
// the layout its content actually needs (see ContentLayout's own doc
// comment).
type ActiveRegion struct {
	ParentID uint32          `json:"parent_id"`
	FuncName string          `json:"func_name"`
	Args     json.RawMessage `json:"args"`
	Content  ContentLayout   `json:"content_layout"`
}

// Registry holds the name->rebuild-function map and the currently-active
// region recipes. natyv's own root package wraps exactly one package-level
// Registry, injecting the real widgets.Container primitives below; tests
// construct their own with fakes to verify the mechanism without a real
// host.
type Registry struct {
	// CreateChild creates a new, already-hidden child of parentID, laid out
	// per content (see ContentLayout's own doc comment), and returns its id
	// -- the make-before-break swap's staging container. Never nil once
	// constructed via NewRegistry.
	CreateChild func(parentID uint32, content ContentLayout) (uint32, error)

	// SetVisible shows or hides id's whole subtree without destroying it.
	// Never nil once constructed via NewRegistry.
	SetVisible func(id uint32, visible bool) error

	// DestroyWidget destroys id and its whole subtree -- used to clean up
	// an incomplete staging container when its rebuild function fails.
	// Never nil once constructed via NewRegistry.
	DestroyWidget func(id uint32)

	// DestroyChildrenExcept destroys every current child of parentID
	// except exceptID's own subtree. Called once per active region during
	// Restore, after its registered rebuild function has already built a
	// full replacement as exceptID. Never nil once constructed via
	// NewRegistry.
	DestroyChildrenExcept func(parentID, exceptID uint32) error

	// rebuildFuncs is the name->function registry a dev populates once
	// (e.g. at natyv_init time) via RegisterRebuildFunc. Keyed by a plain
	// string the dev chooses -- deliberately not a Go identifier/
	// reflection lookup, since closures can't survive a recycle (the whole
	// guest instance, including every closure it ever held, is destroyed
	// by the host) and only a name plus serializable arguments can cross
	// that boundary via natyv_checkpoint's own JSON bytes. Takes the
	// parent to build into as its first argument -- Restore passes a
	// hidden staging container here, not the region's own real stable
	// parent, so the previous content stays alive and visible while this
	// runs.
	rebuildFuncs map[string]func(parentID uint32, args json.RawMessage) error

	// activeRegions is keyed by parent widget id -- a region's stable
	// parent is unique, and a later SetActiveRegion call for the same
	// parent replaces its previous recipe wholesale, exactly what a dev
	// calling it again from a normal in-app rebuild already means: "here's
	// the new current recipe for this region."
	activeRegions map[uint32]ActiveRegion
}

// NewRegistry constructs an empty Registry over the four real widget
// primitives Restore's make-before-break swap needs -- see each field's
// own doc comment above for what it does.
func NewRegistry(
	createChild func(parentID uint32, content ContentLayout) (uint32, error),
	setVisible func(id uint32, visible bool) error,
	destroyWidget func(id uint32),
	destroyChildrenExcept func(parentID, exceptID uint32) error,
) *Registry {
	return &Registry{
		CreateChild:           createChild,
		SetVisible:            setVisible,
		DestroyWidget:         destroyWidget,
		DestroyChildrenExcept: destroyChildrenExcept,
		rebuildFuncs:          map[string]func(parentID uint32, args json.RawMessage) error{},
		activeRegions:         map[uint32]ActiveRegion{},
	}
}

// RegisterRebuildFunc names a function so a later Restore can call it
// again by name. Call this once, e.g. during natyv_init -- registering the
// same name twice replaces the earlier registration. fn's own parentID
// argument is whatever container Restore actually built it into (a hidden
// staging container during a resume-triggered rebuild) -- fn shouldn't
// assume it's the region's own real stable parent, and in practice almost
// never needs to, since it's just forwarding whatever it was given.
func (r *Registry) RegisterRebuildFunc(name string, fn func(parentID uint32, args json.RawMessage) error) {
	r.rebuildFuncs[name] = fn
}

// SetActiveRegion records the current recipe for reconstructing whatever
// is inside parentID's subtree: on Restore, fn (already registered under
// funcName) gets called with args to build a fresh replacement before the
// current children are touched at all, and only then are they destroyed
// (see Restore's own doc comment for the full make-before-break sequence).
// Call this every time the app rebuilds this region normally -- exactly
// where it already does its own "I just rebuilt this" bookkeeping today.
// args must be JSON-marshalable; this only serializes it, it never
// inspects it. content is the same layout the caller just used to build
// its own real content (see ContentLayout's own doc comment for why this
// needs to be captured here rather than guessed by CreateChild later).
//
// Uses persistjson.Marshal, not stdlib encoding/json.Marshal: args is
// arbitrary, dev-authored data (the same risk shape as a persisted var --
// see the plan's "Safe serialization" section), and stdlib's own
// encode-side panic-based error conversion doesn't survive TinyGo's
// wasmexport/asyncify context.
func (r *Registry) SetActiveRegion(parentID uint32, funcName string, args any, content ContentLayout) error {
	encoded, err := persistjson.Marshal(args)
	if err != nil {
		return err
	}
	r.activeRegions[parentID] = ActiveRegion{
		ParentID: parentID,
		FuncName: funcName,
		Content:  content,
		Args:     encoded,
	}
	return nil
}

// Snapshot serializes every currently-active region's recipe -- called
// from natyv_checkpoint (generated) alongside whatever plain data the app
// also persists.
func (r *Registry) Snapshot() (json.RawMessage, error) {
	list := make([]ActiveRegion, 0, len(r.activeRegions))
	for _, region := range r.activeRegions {
		list = append(list, region)
	}
	// Sorted for deterministic output -- map iteration order is
	// unspecified, and there's no reason to leave the checkpoint's own
	// byte layout order-dependent for no benefit.
	sort.Slice(list, func(i, j int) bool { return list[i].ParentID < list[j].ParentID })
	return json.Marshal(list)
}

// Restore rebuilds every region Snapshot captured, using a
// make-before-break swap so the region's previous content stays fully
// visible and intact for the entire time its replacement is being built:
// for each region, a new hidden child of its stable parent is created
// first, the registered rebuild function builds the replacement into
// *that* (not the real parent directly), and only once it returns
// successfully is the new subtree revealed and the previous one destroyed.
// A rebuild function that fails leaves the previous content completely
// untouched -- only the incomplete staging subtree is cleaned up -- rather
// than today's alternative of a destroy-first sequence leaving the parent
// empty on any failure.
//
// The rebuild function is expected to redo whatever bookkeeping (including
// its own SetActiveRegion call) a normal rebuild already does -- Restore
// itself never re-populates activeRegions, trusting the same "one function
// owns this region" discipline SetActiveRegion's own doc comment
// describes. Called from natyv_resume. A region whose function wasn't
// (re-)registered by the time this runs is a real, reportable error, not a
// silent skip -- natyv_resume is responsible for calling
// RegisterRebuildFunc for everything it might need before calling this.
//
// DestroyChildrenExcept destroying *every* other current child of a
// region's parent -- not just the ones this Registry happens to know
// about -- is deliberate, not a gap: nothing outside a region's own
// registered recipe ever holds a standalone, independently-persisted
// reference into that region's contents, so anything else a parent
// happened to be caching-but-not-showing (a previously viewed folder kept
// alive for instant back-navigation, say) is a descendant of the same
// stable parent and gets destroyed along with everything else, correctly
// -- the framework was never tracking those entries, and the app's own
// existing cold-cache-miss path already handles rebuilding one on demand.
func (r *Registry) Restore(data json.RawMessage) error {
	if len(data) == 0 {
		return nil
	}
	var list []ActiveRegion
	if err := json.Unmarshal(data, &list); err != nil {
		return err
	}
	for _, region := range list {
		fn, ok := r.rebuildFuncs[region.FuncName]
		if !ok {
			return fmt.Errorf("natyv: no rebuild function registered for %q", region.FuncName)
		}
		staging, err := r.CreateChild(region.ParentID, region.Content)
		if err != nil {
			return err
		}
		if err := fn(staging, region.Args); err != nil {
			r.DestroyWidget(staging)
			return err
		}
		if err := r.SetVisible(staging, true); err != nil {
			return err
		}
		if err := r.DestroyChildrenExcept(region.ParentID, staging); err != nil {
			return err
		}
	}
	return nil
}
