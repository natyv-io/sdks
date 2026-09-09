package region

import (
	"encoding/json"
	"errors"
	"math"
	"testing"

	"github.com/natyv-io/sdks/go/persistjson"
)

// fakeWorld models just enough of a widget tree for these tests: which
// fake id is parented under which, and whether each is currently visible
// -- mirrors the real host's own actual data model (a flat parent_id per
// widget, not a maintained parent->children list), so a destroy can never
// leave a stale entry in some other id's own children list the way a
// separately-tracked list could.
type fakeWorld struct {
	nextID  uint32
	parent  map[uint32]uint32
	visible map[uint32]bool
	// lastCreateChildContent records the ContentLayout passed to the most
	// recent createChild call -- lets a test confirm SetActiveRegion's own
	// content actually reaches CreateChild via Restore, not just that
	// Restore runs at all.
	lastCreateChildContent ContentLayout
}

func newFakeWorld(startID uint32) *fakeWorld {
	return &fakeWorld{nextID: startID, parent: map[uint32]uint32{}, visible: map[uint32]bool{}}
}

func (w *fakeWorld) create(parentID uint32) uint32 {
	w.nextID++
	id := w.nextID
	w.parent[id] = parentID
	w.visible[id] = true
	return id
}

// createChild mirrors the real CreateChild callback: a fresh child,
// already hidden, recording the content it was asked to build with.
func (w *fakeWorld) createChild(parentID uint32, content ContentLayout) (uint32, error) {
	id := w.create(parentID)
	w.visible[id] = false
	w.lastCreateChildContent = content
	return id, nil
}

func (w *fakeWorld) setVisible(id uint32, visible bool) error {
	w.visible[id] = visible
	return nil
}

// children returns id's current direct children, in creation order.
func (w *fakeWorld) children(id uint32) []uint32 {
	var out []uint32
	for child := uint32(2); child <= w.nextID; child++ {
		if p, ok := w.parent[child]; ok && p == id {
			out = append(out, child)
		}
	}
	return out
}

func (w *fakeWorld) exists(id uint32) bool {
	_, ok := w.visible[id]
	return ok
}

func (w *fakeWorld) destroyWidget(id uint32) {
	for _, child := range w.children(id) {
		w.destroyWidget(child)
	}
	delete(w.parent, id)
	delete(w.visible, id)
}

func (w *fakeWorld) destroyChildrenExcept(parentID, exceptID uint32) error {
	for _, child := range w.children(parentID) {
		if child != exceptID {
			w.destroyWidget(child)
		}
	}
	return nil
}

// noopRegistry builds a Registry over fakeWorld's four real callbacks --
// the shared constructor call every test below needs.
func noopRegistry(world *fakeWorld) *Registry {
	return NewRegistry(world.createChild, world.setVisible, world.destroyWidget, world.destroyChildrenExcept)
}

// TestRegionRoundTrip is the plan's own specified scenario: construct a
// fake multi-widget region, force a real recycle (snapshot into a fresh
// registry, mirroring a fresh guest instance's own natyv_resume),
// confirm the region's widgets are destroyed and rebuilt with fresh ids,
// its rebuild function runs for real with the right persisted args (not
// just data restored), and a second, non-active cached subtree under the
// same parent is correctly gone rather than orphaned.
func TestRegionRoundTrip(t *testing.T) {
	const rootID = uint32(1)
	world := newFakeWorld(rootID)

	var rebuildCalls []string
	var lastBuiltInto uint32
	renderFolder := func(parentID uint32, args json.RawMessage) error {
		var a struct{ Folder string }
		if err := json.Unmarshal(args, &a); err != nil {
			return err
		}
		rebuildCalls = append(rebuildCalls, a.Folder)
		lastBuiltInto = parentID
		// A real multi-widget region: two fresh child widgets per build,
		// e.g. a row and its own scrollbar -- built into whatever parent
		// Restore actually gave this call, not a fixed id.
		world.create(parentID)
		world.create(parentID)
		return nil
	}

	registry := noopRegistry(world)
	registry.RegisterRebuildFunc("renderFolder", renderFolder)

	wantContent := ContentLayout{Direction: "top_to_bottom", ChildGap: 8}
	if err := registry.SetActiveRegion(rootID, "renderFolder", struct{ Folder string }{"Inbox"}, wantContent); err != nil {
		t.Fatalf("SetActiveRegion: %v", err)
	}
	// The region's own initial build -- exactly where a real app would
	// have just called SetActiveRegion, right after building this.
	widA := world.create(rootID)
	widB := world.create(rootID)

	// A second, non-active cached subtree under the SAME parent -- e.g. a
	// previously-viewed folder kept alive for instant back-navigation.
	// Nothing in this package tracks it specially; it must still be gone
	// after a real recycle, not orphaned.
	orphan := world.create(rootID)

	if got := len(world.children(rootID)); got != 3 {
		t.Fatalf("expected 3 children before recycle, got %d", got)
	}

	// "Force a real recycle": snapshot the live registry (mirroring
	// natyv_checkpoint's own SnapshotRegions call), then restore into a
	// completely fresh registry with its own freshly-registered rebuild
	// funcs (mirroring a brand-new guest instance's own natyv_resume,
	// including init() re-registering everything from scratch -- a new
	// instance's own rebuildFuncs map starts genuinely empty).
	snapshot, err := registry.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	fresh := noopRegistry(world)
	fresh.RegisterRebuildFunc("renderFolder", renderFolder)

	if err := fresh.Restore(snapshot); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	if len(rebuildCalls) != 1 || rebuildCalls[0] != "Inbox" {
		t.Fatalf("expected exactly one renderFolder(\"Inbox\") call, got %v", rebuildCalls)
	}

	// rootID's only surviving child is the fresh staging container the
	// rebuild function was actually given -- every old child (region-owned
	// or orphaned) is gone, not just the ones this registry tracked.
	rootChildren := world.children(rootID)
	if len(rootChildren) != 1 || rootChildren[0] != lastBuiltInto {
		t.Fatalf("expected rootID's only child to be the staging container %d, got %v", lastBuiltInto, rootChildren)
	}
	if !world.visible[lastBuiltInto] {
		t.Fatalf("staging container %d should be visible after a successful restore", lastBuiltInto)
	}
	for _, old := range []uint32{widA, widB, orphan} {
		if world.exists(old) {
			t.Fatalf("child %d survived the recycle -- every old child (region-owned or orphaned) should have been destroyed and replaced", old)
		}
	}

	// The rebuild function's own 2 fresh widgets live one level deeper, as
	// children of the staging container, not of rootID directly.
	staged := world.children(lastBuiltInto)
	if len(staged) != 2 {
		t.Fatalf("expected 2 widgets built into the staging container, got %d: %v", len(staged), staged)
	}

	// The real regression this arc found live: a staging container built
	// with no layout information falls back to the host's own
	// left_to_right leaf-widget default, silently breaking a TopToBottom
	// region's arrangement on its first post-recycle rebuild. Confirms the
	// content SetActiveRegion was given on the OLD (pre-recycle) registry
	// actually reaches CreateChild via the snapshot/restore round trip into
	// the fresh one, not just that Restore runs.
	if world.lastCreateChildContent != wantContent {
		t.Fatalf("expected CreateChild to receive %+v, got %+v", wantContent, world.lastCreateChildContent)
	}
}

// TestRestoreKeepsOldContentUntilRebuildSucceeds confirms the
// make-before-break order: the previous content is still fully present
// and visible while the rebuild function runs, not torn down first -- a
// rebuild function that inspects the tree mid-call sees exactly what a
// user would still be looking at, and a rebuild that does anything slow
// (a real network reconnect, for a real app) never leaves the region
// visibly empty while it works.
func TestRestoreKeepsOldContentUntilRebuildSucceeds(t *testing.T) {
	const rootID = uint32(1)
	world := newFakeWorld(rootID)
	oldChild := world.create(rootID) // a real, live-before-recycle child

	var childrenAtRebuildTime []uint32
	var oldChildStillVisible bool
	rebuildFn := func(parentID uint32, args json.RawMessage) error {
		childrenAtRebuildTime = world.children(rootID)
		oldChildStillVisible = world.visible[oldChild]
		return nil
	}

	registry := noopRegistry(world)
	registry.RegisterRebuildFunc("f", rebuildFn)
	if err := registry.SetActiveRegion(rootID, "f", struct{}{}, ContentLayout{}); err != nil {
		t.Fatalf("SetActiveRegion: %v", err)
	}

	snapshot, err := registry.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	fresh := noopRegistry(world)
	fresh.RegisterRebuildFunc("f", rebuildFn)
	if err := fresh.Restore(snapshot); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	found := false
	for _, id := range childrenAtRebuildTime {
		if id == oldChild {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected oldChild %d still present under rootID during rebuild, children were %v", oldChild, childrenAtRebuildTime)
	}
	if !oldChildStillVisible {
		t.Fatal("expected oldChild to still be visible during rebuild -- it should only be destroyed after the rebuild succeeds")
	}
}

// TestRestoreFailedRebuildLeavesOldContentIntact confirms the real
// robustness improvement make-before-break gives for free: a rebuild
// function that fails leaves the previous content completely untouched,
// with only the incomplete staging container cleaned up -- never a region
// left empty because its replacement couldn't be built.
func TestRestoreFailedRebuildLeavesOldContentIntact(t *testing.T) {
	const rootID = uint32(1)
	world := newFakeWorld(rootID)
	oldChild := world.create(rootID)

	wantErr := errors.New("boom")
	var stagingID uint32
	rebuildFn := func(parentID uint32, args json.RawMessage) error {
		stagingID = parentID
		world.create(parentID) // partially built before failing
		return wantErr
	}

	registry := noopRegistry(world)
	registry.RegisterRebuildFunc("f", rebuildFn)
	if err := registry.SetActiveRegion(rootID, "f", struct{}{}, ContentLayout{}); err != nil {
		t.Fatalf("SetActiveRegion: %v", err)
	}

	snapshot, err := registry.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	fresh := noopRegistry(world)
	fresh.RegisterRebuildFunc("f", rebuildFn)

	if err := fresh.Restore(snapshot); !errors.Is(err, wantErr) {
		t.Fatalf("expected the rebuild function's own error, got %v", err)
	}

	if !world.visible[oldChild] {
		t.Fatalf("oldChild %d should still exist and be visible after a failed rebuild", oldChild)
	}
	if world.exists(stagingID) {
		t.Fatalf("incomplete staging container %d should have been destroyed after the rebuild failed", stagingID)
	}
	rootChildren := world.children(rootID)
	if len(rootChildren) != 1 || rootChildren[0] != oldChild {
		t.Fatalf("expected rootID's only child to still be oldChild %d after a failed rebuild, got %v", oldChild, rootChildren)
	}
}

func TestRestoreUnregisteredFuncIsHardError(t *testing.T) {
	const rootID = uint32(1)
	world := newFakeWorld(rootID)

	registry := noopRegistry(world)
	registry.RegisterRebuildFunc("f", func(uint32, json.RawMessage) error { return nil })
	if err := registry.SetActiveRegion(rootID, "f", struct{}{}, ContentLayout{}); err != nil {
		t.Fatalf("SetActiveRegion: %v", err)
	}
	snapshot, err := registry.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	// A fresh registry that forgot to (re-)register "f" -- exactly the
	// real failure mode of a resumed instance whose own init() doesn't
	// register everything it might need.
	fresh := noopRegistry(world)
	err = fresh.Restore(snapshot)
	if err == nil {
		t.Fatal("expected an error for an unregistered rebuild function, got nil")
	}
}

func TestRestoreEmptyDataIsNoop(t *testing.T) {
	registry := NewRegistry(
		func(uint32, ContentLayout) (uint32, error) {
			t.Fatal("CreateChild should never be called for empty data")
			return 0, nil
		},
		func(uint32, bool) error {
			t.Fatal("SetVisible should never be called for empty data")
			return nil
		},
		func(uint32) {
			t.Fatal("DestroyWidget should never be called for empty data")
		},
		func(uint32, uint32) error {
			t.Fatal("DestroyChildrenExcept should never be called for empty data")
			return nil
		},
	)
	if err := registry.Restore(nil); err != nil {
		t.Fatalf("Restore(nil): %v", err)
	}
	if err := registry.Restore(json.RawMessage{}); err != nil {
		t.Fatalf("Restore(empty): %v", err)
	}
}

// TestSnapshotIsSortedByParentID confirms the documented determinism
// property directly -- map iteration order is unspecified, and
// SnapshotRegions's own generated output must not depend on it.
func TestSnapshotIsSortedByParentID(t *testing.T) {
	registry := NewRegistry(
		func(uint32, ContentLayout) (uint32, error) { return 0, nil },
		func(uint32, bool) error { return nil },
		func(uint32) {},
		func(uint32, uint32) error { return nil },
	)
	registry.RegisterRebuildFunc("f", func(uint32, json.RawMessage) error { return nil })
	// Registered out of order on purpose.
	for _, id := range []uint32{30, 10, 20} {
		if err := registry.SetActiveRegion(id, "f", struct{}{}, ContentLayout{}); err != nil {
			t.Fatalf("SetActiveRegion(%d): %v", id, err)
		}
	}

	snapshot, err := registry.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	var list []ActiveRegion
	if err := json.Unmarshal(snapshot, &list); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("expected 3 regions, got %d", len(list))
	}
	for i := 1; i < len(list); i++ {
		if list[i-1].ParentID >= list[i].ParentID {
			t.Fatalf("snapshot not sorted by ParentID: %+v", list)
		}
	}
}

// TestSetActiveRegionUsesSafeMarshal confirms SetActiveRegion really goes
// through persistjson.Marshal, not stdlib encoding/json.Marshal -- args
// is arbitrary dev-authored data, the exact risk shape the plan's own
// "Safe serialization" section is about. A NaN float used to be silently
// accepted by encoding/json.Marshal only to panic deep inside it; here it
// must surface as an ordinary error instead, never a panic, and never a
// silently-corrupted snapshot.
func TestSetActiveRegionUsesSafeMarshal(t *testing.T) {
	registry := NewRegistry(
		func(uint32, ContentLayout) (uint32, error) { return 0, nil },
		func(uint32, bool) error { return nil },
		func(uint32) {},
		func(uint32, uint32) error { return nil },
	)
	err := registry.SetActiveRegion(1, "f", struct {
		Ratio float64 `json:"ratio"`
	}{Ratio: math.NaN()}, ContentLayout{})
	if err == nil {
		t.Fatal("expected an error for a NaN arg field, got nil")
	}
	if !errors.Is(err, persistjson.ErrUnsupportedValue) {
		t.Fatalf("expected persistjson.ErrUnsupportedValue, got %v", err)
	}
}
