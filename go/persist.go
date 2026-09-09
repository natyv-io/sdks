// Host-owned persisted state -- natyv.Persisted[T] below wraps one
// package-level persist.Registry (see sdks/go/persist/persist.go for the
// real registry logic, its own doc comment on why it's a separate package,
// and the plan's own "Host-owned persisted state" section,
// ~/.claude/plans/lexical-wishing-penguin.md, for the full design),
// injecting the three real host calls the registry needs. Package-level
// state persists across calls into the same plugin instance, same as
// region.go's own regionRegistry and widgets/internal's event-handler
// tables -- entries registered during a package-level var initializer are
// still there when natyv_init/natyv_resume calls FlushPersisted.
package natyv

import (
	"encoding/json"
	"errors"

	"github.com/extism/go-pdk"
	"github.com/natyv-io/sdks/go/persist"
)

//go:wasmimport extism:host/user natyv_persist_get_or_init
func persistGetOrInitHost(uint64) uint64

//go:wasmimport extism:host/user natyv_persist_set
func persistSetHost(uint64) uint64

//go:wasmimport extism:host/user natyv_persist_free
func persistFreeHost(uint64) uint64

func hostGetOrInit(key string, def []byte) ([]byte, bool, error) {
	body, err := json.Marshal(struct {
		Key     string `json:"key"`
		Default string `json:"default"`
	}{key, string(def)})
	if err != nil {
		return nil, false, err
	}
	var resp struct {
		// A pointer, not a bare string: JSON null (freed) must be
		// distinguishable from an absent field (an error response, which
		// never carries "value" at all) and from a real, possibly-empty
		// encoded value.
		Value *string `json:"value"`
		Error string  `json:"error,omitempty"`
	}
	if err := json.Unmarshal(pdk.ParamBytes(persistGetOrInitHost(pdk.ResultBytes(body))), &resp); err != nil {
		return nil, false, err
	}
	if resp.Error != "" {
		return nil, false, errors.New(resp.Error)
	}
	if resp.Value == nil {
		return nil, true, nil
	}
	return []byte(*resp.Value), false, nil
}

func hostSet(key string, value []byte) error {
	body, err := json.Marshal(struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}{key, string(value)})
	if err != nil {
		return err
	}
	var resp struct {
		Error string `json:"error,omitempty"`
	}
	if err := json.Unmarshal(pdk.ParamBytes(persistSetHost(pdk.ResultBytes(body))), &resp); err != nil {
		return err
	}
	if resp.Error != "" {
		return errors.New(resp.Error)
	}
	return nil
}

func hostFree(key string) error {
	body, err := json.Marshal(struct {
		Key string `json:"key"`
	}{key})
	if err != nil {
		return err
	}
	var resp struct {
		Error string `json:"error,omitempty"`
	}
	if err := json.Unmarshal(pdk.ParamBytes(persistFreeHost(pdk.ResultBytes(body))), &resp); err != nil {
		return err
	}
	if resp.Error != "" {
		return errors.New(resp.Error)
	}
	return nil
}

var persistRegistry = persist.NewRegistry(hostGetOrInit, hostSet, hostFree)

// ErrPersistedFreed is returned by a Persisted[T] handle's Get when its key
// was explicitly freed (via Free) and hasn't been re-established since by
// an explicit Set or Reset -- see persist.ErrPersistedFreed's own doc
// comment for why the automatic flush never clears this on its own.
var ErrPersistedFreed = persist.ErrPersistedFreed

// FlushPersisted eagerly resolves every registered Persisted[T] entry that
// hasn't been touched yet, once, in one coherent pass. Call this as the
// first line of both natyv_init and natyv_resume -- for natyv_resume
// specifically, it must run *before* RestoreRegions, since a region's own
// rebuild function reads persisted values to decide what to rebuild and
// needs them already resolved to the real survived state, not a stale,
// just-sent default.
func FlushPersisted() error {
	return persistRegistry.Flush()
}

// persistedHandle is Persisted[T]'s own concrete return type -- unexported
// since callers never need to spell it out explicitly (:= infers it from
// Persisted[T]'s own return value).
type persistedHandle[T any] struct {
	entry *persist.Entry
	// Set only if marshaling def itself failed at construction time (e.g. a
	// NaN/Inf default) -- a pure input-format error, not a host-communication
	// one, so it's simplest to store and surface on first real use rather
	// than needing Persisted[T] itself to return an error and break the
	// plain `var x = natyv.Persisted[T](...)` construction shape.
	marshalErr error
}

// Persisted registers key with default value def and returns a handle to
// it. Makes zero host calls -- pure in-memory bookkeeping, safe to call
// from a package-level var initializer (i.e. before natyv_init has ever
// run), which is the whole point: see this file's own doc comment and the
// plan's "Host-owned persisted state" section for why this specific
// property is empirically required, not just convenient.
//
// def is marshaled via MarshalSafe (not encoding/json.Marshal) since it's
// arbitrary, dev-authored data -- see MarshalSafe's own doc comment.
func Persisted[T any](key string, def T) *persistedHandle[T] {
	defBytes, err := MarshalSafe(def)
	if err != nil {
		return &persistedHandle[T]{marshalErr: err}
	}
	return &persistedHandle[T]{entry: persistRegistry.Register(key, defBytes)}
}

// Get returns the handle's current value -- host-side create-if-absent on
// first real touch (whether that's this call or an earlier FlushPersisted
// pass), a cheap local read every call after that. Returns
// ErrPersistedFreed if the key was explicitly freed and not since
// re-established by Set or Reset. Must not be called before natyv_init has
// been entered -- see this file's own doc comment.
func (p *persistedHandle[T]) Get() (T, error) {
	var zero T
	if p.marshalErr != nil {
		return zero, p.marshalErr
	}
	raw, err := persistRegistry.Get(p.entry)
	if err != nil {
		return zero, err
	}
	var v T
	if err := json.Unmarshal(raw, &v); err != nil {
		return zero, err
	}
	return v, nil
}

// Set unconditionally writes value -- creates the key if this is the first
// time it's ever been written, and always clears any prior Free.
func (p *persistedHandle[T]) Set(value T) error {
	if p.marshalErr != nil {
		return p.marshalErr
	}
	encoded, err := MarshalSafe(value)
	if err != nil {
		return err
	}
	return persistRegistry.Set(p.entry, encoded)
}

// Reset is Set using the handle's own original default (the value passed
// to Persisted when this handle was constructed) -- not a separate host
// primitive.
func (p *persistedHandle[T]) Reset() error {
	if p.marshalErr != nil {
		return p.marshalErr
	}
	return persistRegistry.Reset(p.entry)
}

// Free removes this key from host state entirely -- real host RSS
// reclamation, since host-side state lives in ordinary native memory that
// can actually be freed, unlike guest WASM linear memory. A freed key
// stays freed across a recycle (the next resume's FlushPersisted will not
// silently recreate it) until an explicit Set or Reset brings it back.
func (p *persistedHandle[T]) Free() error {
	if p.marshalErr != nil {
		return p.marshalErr
	}
	return persistRegistry.Free(p.entry)
}
