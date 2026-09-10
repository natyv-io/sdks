package binding

import (
	"encoding/json"
	"errors"
	"math"
	"testing"

	"github.com/natyv-io/sdks/go/persistjson"
)

// TestBindingRoundTrip is the plan's own specified scenario: register a
// binding, force a real recycle (snapshot into a fresh registry, mirroring
// a fresh guest instance's own natyv_resume), confirm the rebind function
// runs for real with the right widget id and persisted args -- not just
// that data was restored.
func TestBindingRoundTrip(t *testing.T) {
	type rebindCall struct {
		widgetID uint32
		seq      int
	}
	var calls []rebindCall
	rebindRowOpen := func(widgetID uint32, args json.RawMessage) error {
		var a struct {
			Seq int `json:"seq"`
		}
		if err := json.Unmarshal(args, &a); err != nil {
			return err
		}
		calls = append(calls, rebindCall{widgetID, a.Seq})
		return nil
	}

	registry := NewRegistry()
	registry.RegisterHandlerFunc("row_open", rebindRowOpen)
	if err := registry.RegisterBinding(42, "row_open", struct {
		Seq int `json:"seq"`
	}{Seq: 7}); err != nil {
		t.Fatalf("RegisterBinding: %v", err)
	}

	snapshot, err := registry.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	// A fresh registry with its own freshly-registered rebind funcs --
	// mirrors a brand-new guest instance's own natyv_resume, including
	// init() re-registering everything from scratch.
	fresh := NewRegistry()
	fresh.RegisterHandlerFunc("row_open", rebindRowOpen)
	if err := fresh.Restore(snapshot); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	if len(calls) != 1 || calls[0].widgetID != 42 || calls[0].seq != 7 {
		t.Fatalf("expected exactly one rebind call for widget 42 with seq=7, got %v", calls)
	}
}

// TestRestoreRepopulatesActiveBindings is the real correctness fix this
// package exists to ship, tested directly: unlike region.Registry.Restore
// (safe to leave activeRegions untouched, since normal navigation
// naturally re-triggers SetActiveRegion), most bindings are registered
// exactly once, at widget creation -- so a SECOND recycle with zero
// intervening RegisterBinding calls must still successfully restore,
// or the binding is silently, permanently lost on the second resume.
func TestRestoreRepopulatesActiveBindings(t *testing.T) {
	var calls int
	rebind := func(widgetID uint32, args json.RawMessage) error {
		calls++
		return nil
	}

	registry := NewRegistry()
	registry.RegisterHandlerFunc("nav_inbox", rebind)
	if err := registry.RegisterBinding(1, "nav_inbox", nil); err != nil {
		t.Fatalf("RegisterBinding: %v", err)
	}

	// First recycle.
	snapshot1, err := registry.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot 1: %v", err)
	}
	instance2 := NewRegistry()
	instance2.RegisterHandlerFunc("nav_inbox", rebind)
	if err := instance2.Restore(snapshot1); err != nil {
		t.Fatalf("Restore 1: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 rebind call after first restore, got %d", calls)
	}

	// Second recycle, with ZERO intervening RegisterBinding calls -- the
	// exact scenario the fix targets. instance2's own activeBindings must
	// have been repopulated by its own Restore call above for this
	// Snapshot to have anything to serialize at all.
	snapshot2, err := instance2.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot 2: %v", err)
	}
	var list []ActiveBinding
	if err := json.Unmarshal(snapshot2, &list); err != nil {
		t.Fatalf("Unmarshal snapshot2: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected the second snapshot to still contain 1 binding (proving Restore repopulated activeBindings), got %d: %+v", len(list), list)
	}

	instance3 := NewRegistry()
	instance3.RegisterHandlerFunc("nav_inbox", rebind)
	if err := instance3.Restore(snapshot2); err != nil {
		t.Fatalf("Restore 2: %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected 2 total rebind calls after the second restore, got %d -- the binding was silently dropped on the second recycle", calls)
	}
}

// TestRestoreContinuesPastUnregisteredKind confirms one binding's own
// unregistered kind doesn't sink every other, already-restorable binding
// -- a deliberate difference from region.Registry.Restore's hard-stop
// behavior (see binding.go's own doc comment on Restore for why).
func TestRestoreContinuesPastUnregisteredKind(t *testing.T) {
	var goodCalled bool
	registry := NewRegistry()
	registry.RegisterHandlerFunc("good", func(uint32, json.RawMessage) error {
		goodCalled = true
		return nil
	})
	if err := registry.RegisterBinding(1, "good", nil); err != nil {
		t.Fatalf("RegisterBinding(good): %v", err)
	}
	if err := registry.RegisterBinding(2, "missing", nil); err != nil {
		t.Fatalf("RegisterBinding(missing): %v", err)
	}

	snapshot, err := registry.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	// A fresh registry that forgot to (re-)register "missing" -- exactly
	// the real failure mode of a resumed instance whose own init() doesn't
	// register everything it might need.
	fresh := NewRegistry()
	fresh.RegisterHandlerFunc("good", func(uint32, json.RawMessage) error {
		goodCalled = true
		return nil
	})
	err = fresh.Restore(snapshot)
	if err == nil {
		t.Fatal("expected an error for the unregistered \"missing\" kind, got nil")
	}
	if !goodCalled {
		t.Fatal("expected the \"good\" binding to still be reattached despite \"missing\" failing")
	}
}

func TestRestoreEmptyDataIsNoop(t *testing.T) {
	registry := NewRegistry()
	registry.RegisterHandlerFunc("f", func(uint32, json.RawMessage) error {
		t.Fatal("rebind function should never be called for empty data")
		return nil
	})
	if err := registry.Restore(nil); err != nil {
		t.Fatalf("Restore(nil): %v", err)
	}
	if err := registry.Restore(json.RawMessage{}); err != nil {
		t.Fatalf("Restore(empty): %v", err)
	}
}

// TestSnapshotIsSortedByWidgetID confirms the documented determinism
// property directly -- map iteration order is unspecified, and
// SnapshotBindings's own generated output must not depend on it.
func TestSnapshotIsSortedByWidgetID(t *testing.T) {
	registry := NewRegistry()
	registry.RegisterHandlerFunc("f", func(uint32, json.RawMessage) error { return nil })
	// Registered out of order on purpose.
	for _, id := range []uint32{30, 10, 20} {
		if err := registry.RegisterBinding(id, "f", nil); err != nil {
			t.Fatalf("RegisterBinding(%d): %v", id, err)
		}
	}

	snapshot, err := registry.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	var list []ActiveBinding
	if err := json.Unmarshal(snapshot, &list); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("expected 3 bindings, got %d", len(list))
	}
	for i := 1; i < len(list); i++ {
		if list[i-1].WidgetID >= list[i].WidgetID {
			t.Fatalf("snapshot not sorted by WidgetID: %+v", list)
		}
	}
}

// TestRegisterBindingReplacesWholesale confirms a later RegisterBinding
// call for the same widget id replaces its earlier recipe entirely, not
// merges with it -- the same real-world case as re-rendering a widget
// whose handler's own captured args changed.
func TestRegisterBindingReplacesWholesale(t *testing.T) {
	registry := NewRegistry()
	if err := registry.RegisterBinding(1, "row_open", struct{ Seq int }{Seq: 1}); err != nil {
		t.Fatalf("RegisterBinding first: %v", err)
	}
	if err := registry.RegisterBinding(1, "row_open", struct{ Seq int }{Seq: 2}); err != nil {
		t.Fatalf("RegisterBinding second: %v", err)
	}

	snapshot, err := registry.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	var list []ActiveBinding
	if err := json.Unmarshal(snapshot, &list); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected exactly 1 binding for widget 1 (later call replaces, not merges), got %d", len(list))
	}
	var args struct{ Seq int }
	if err := json.Unmarshal(list[0].Args, &args); err != nil {
		t.Fatalf("Unmarshal args: %v", err)
	}
	if args.Seq != 2 {
		t.Fatalf("expected the second call's args (Seq=2) to have won, got Seq=%d", args.Seq)
	}
}

// TestRegisterBindingUsesSafeMarshal confirms RegisterBinding really goes
// through persistjson.Marshal, not stdlib encoding/json.Marshal -- args is
// arbitrary, dev-authored data, the exact risk shape
// region.Registry.SetActiveRegion's own equivalent test covers. A NaN
// float used to be silently accepted by encoding/json.Marshal only to
// panic deep inside it; here it must surface as an ordinary error
// instead, never a panic.
func TestRegisterBindingUsesSafeMarshal(t *testing.T) {
	registry := NewRegistry()
	err := registry.RegisterBinding(1, "f", struct {
		Ratio float64 `json:"ratio"`
	}{Ratio: math.NaN()})
	if err == nil {
		t.Fatal("expected an error for a NaN arg field, got nil")
	}
	if !errors.Is(err, persistjson.ErrUnsupportedValue) {
		t.Fatalf("expected persistjson.ErrUnsupportedValue, got %v", err)
	}
}
