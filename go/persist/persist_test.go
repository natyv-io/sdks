package persist

import (
	"errors"
	"testing"
)

// fakeHost models just enough of the real host-side store to test Registry
// in isolation: a value map plus a separate freed set (mirroring
// core/src/PersistStore.zig's own two-map design exactly), and a call
// counter per key so tests can confirm Registry never over- or
// under-calls it.
type fakeHost struct {
	values       map[string][]byte
	freed        map[string]bool
	getOrInitLog []string
	setLog       []string
	freeLog      []string
}

func newFakeHost() *fakeHost {
	return &fakeHost{values: map[string][]byte{}, freed: map[string]bool{}}
}

func (h *fakeHost) getOrInit(key string, def []byte) ([]byte, bool, error) {
	h.getOrInitLog = append(h.getOrInitLog, key)
	if h.freed[key] {
		return nil, true, nil
	}
	if v, ok := h.values[key]; ok {
		return v, false, nil
	}
	h.values[key] = def
	return def, false, nil
}

func (h *fakeHost) set(key string, value []byte) error {
	h.setLog = append(h.setLog, key)
	delete(h.freed, key)
	h.values[key] = value
	return nil
}

func (h *fakeHost) free(key string) error {
	h.freeLog = append(h.freeLog, key)
	delete(h.values, key)
	h.freed[key] = true
	return nil
}

func newTestRegistry(h *fakeHost) *Registry {
	return NewRegistry(h.getOrInit, h.set, h.free)
}

func TestRegisterAndFlushResolvesEveryEntryOnce(t *testing.T) {
	h := newFakeHost()
	r := newTestRegistry(h)
	a := r.Register("a", []byte(`1`))
	b := r.Register("b", []byte(`2`))

	if err := r.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if !a.Resolved || string(a.Cached) != "1" {
		t.Fatalf("a not resolved to its default: %+v", a)
	}
	if !b.Resolved || string(b.Cached) != "2" {
		t.Fatalf("b not resolved to its default: %+v", b)
	}
	if len(h.getOrInitLog) != 2 {
		t.Fatalf("expected exactly 2 getOrInit calls, got %d: %v", len(h.getOrInitLog), h.getOrInitLog)
	}
}

func TestFlushIsANoOpOnceEverythingIsAlreadyResolved(t *testing.T) {
	h := newFakeHost()
	r := newTestRegistry(h)
	r.Register("a", []byte(`1`))

	if err := r.Flush(); err != nil {
		t.Fatalf("first Flush: %v", err)
	}
	callsAfterFirst := len(h.getOrInitLog)

	if err := r.Flush(); err != nil {
		t.Fatalf("second Flush: %v", err)
	}
	if len(h.getOrInitLog) != callsAfterFirst {
		t.Fatalf("second Flush made new host calls: %v", h.getOrInitLog)
	}
}

func TestGetLazilyResolvesADynamicallyRegisteredKey(t *testing.T) {
	h := newFakeHost()
	r := newTestRegistry(h)
	// Registered *after* a Flush already ran -- the dynamic-key case, never
	// swept by the automatic startup flush.
	if err := r.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	e := r.Register("item-42-checked", []byte(`false`))

	value, err := r.Get(e)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(value) != "false" {
		t.Fatalf("got %q, want %q", value, "false")
	}
	if len(h.getOrInitLog) != 1 {
		t.Fatalf("expected exactly 1 getOrInit call, got %d", len(h.getOrInitLog))
	}
}

func TestResumeSupersession_ExistingHostValueWinsOverTheFreshDefault(t *testing.T) {
	h := newFakeHost()
	h.values["currentFolder"] = []byte(`"Sent"`) // simulates a value that survived a prior instance

	r := newTestRegistry(h)
	e := r.Register("currentFolder", []byte(`"Inbox"`)) // the compile-time default, re-registered fresh

	if err := r.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if string(e.Cached) != `"Sent"` {
		t.Fatalf("resumed value = %q, want the real survived value %q, not the default", e.Cached, `"Sent"`)
	}
}

func TestGetOnAFreedEntryReturnsErrPersistedFreedWithoutTouchingTheHostAgain(t *testing.T) {
	h := newFakeHost()
	r := newTestRegistry(h)
	e := r.Register("x", []byte(`"Inbox"`))
	if err := r.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if err := r.Free(e); err != nil {
		t.Fatalf("Free: %v", err)
	}

	callsBeforeGet := len(h.getOrInitLog)
	_, err := r.Get(e)
	if !errors.Is(err, ErrPersistedFreed) {
		t.Fatalf("Get after Free: got err=%v, want ErrPersistedFreed", err)
	}
	if len(h.getOrInitLog) != callsBeforeGet {
		t.Fatalf("Get on a freed entry made a host call it shouldn't have: %v", h.getOrInitLog)
	}
}

func TestFlushDoesNotResurrectAFreedKey(t *testing.T) {
	// The exact scenario this whole mechanism exists to prevent: a
	// package-level var's Persisted[T](key, default) unconditionally
	// re-registers on every instance boot, including a resume right after
	// the key was freed on the previous instance.
	h := newFakeHost()
	h.freed["draftMessage"] = true // simulates: freed on the instance that just recycled

	r := newTestRegistry(h)
	e := r.Register("draftMessage", []byte(`""`))
	if err := r.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if !e.Freed {
		t.Fatalf("Flush should have left the entry Freed, got %+v", e)
	}
	if e.Resolved {
		t.Fatalf("a freed entry must not also be Resolved: %+v", e)
	}

	_, err := r.Get(e)
	if !errors.Is(err, ErrPersistedFreed) {
		t.Fatalf("Get after a resume-time flush of a freed key: got err=%v, want ErrPersistedFreed", err)
	}
}

func TestSetClearsAFreedEntryAndTheNextGetReflectsTheNewValue(t *testing.T) {
	h := newFakeHost()
	r := newTestRegistry(h)
	e := r.Register("x", []byte(`"a"`))
	if err := r.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if err := r.Free(e); err != nil {
		t.Fatalf("Free: %v", err)
	}
	if err := r.Set(e, []byte(`"b"`)); err != nil {
		t.Fatalf("Set: %v", err)
	}

	value, err := r.Get(e)
	if err != nil {
		t.Fatalf("Get after Set: %v", err)
	}
	if string(value) != `"b"` {
		t.Fatalf("got %q, want %q", value, `"b"`)
	}
}

func TestResetClearsAFreedEntryBackToItsOwnDefault(t *testing.T) {
	h := newFakeHost()
	r := newTestRegistry(h)
	e := r.Register("x", []byte(`"default-value"`))
	if err := r.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if err := r.Set(e, []byte(`"changed"`)); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := r.Free(e); err != nil {
		t.Fatalf("Free: %v", err)
	}

	if err := r.Reset(e); err != nil {
		t.Fatalf("Reset: %v", err)
	}
	value, err := r.Get(e)
	if err != nil {
		t.Fatalf("Get after Reset: %v", err)
	}
	if string(value) != `"default-value"` {
		t.Fatalf("got %q, want the original default %q", value, `"default-value"`)
	}
}

func TestFreeThenGetThenSetThenGetProducesTheRightHostCallSequence(t *testing.T) {
	h := newFakeHost()
	r := newTestRegistry(h)
	e := r.Register("x", []byte(`"a"`))

	if _, err := r.Get(e); err != nil { // 1st getOrInit -- resolves the default
		t.Fatalf("first Get: %v", err)
	}
	if err := r.Free(e); err != nil { // 1st free
		t.Fatalf("Free: %v", err)
	}
	if _, err := r.Get(e); !errors.Is(err, ErrPersistedFreed) { // no host call -- local Freed check
		t.Fatalf("Get after Free: got err=%v, want ErrPersistedFreed", err)
	}
	if err := r.Set(e, []byte(`"b"`)); err != nil { // 1st set
		t.Fatalf("Set: %v", err)
	}
	value, err := r.Get(e) // no host call -- already Resolved from Set
	if err != nil {
		t.Fatalf("final Get: %v", err)
	}
	if string(value) != `"b"` {
		t.Fatalf("got %q, want %q", value, `"b"`)
	}

	if len(h.getOrInitLog) != 1 {
		t.Fatalf("expected exactly 1 getOrInit call total, got %d: %v", len(h.getOrInitLog), h.getOrInitLog)
	}
	if len(h.setLog) != 1 {
		t.Fatalf("expected exactly 1 set call, got %d: %v", len(h.setLog), h.setLog)
	}
	if len(h.freeLog) != 1 {
		t.Fatalf("expected exactly 1 free call, got %d: %v", len(h.freeLog), h.freeLog)
	}
}
