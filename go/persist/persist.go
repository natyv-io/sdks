// Package persist holds the host-owned persisted state registry itself --
// natyv.Persisted[T]'s real mechanism (see the plan's own "Host-owned
// persisted state" section, ~/.claude/plans/lexical-wishing-penguin.md, and
// project_natyv_ergonomics_layer memory for the full design history).
//
// Deliberately its own package, separate from natyv's own root package,
// mirroring exactly why sdks/go/region/region.go is split from
// sdks/go/region.go: the root package's own widget calls transitively
// import go-pdk, whose //go:wasmimport declarations don't compile for a
// native target at all -- keeping this file's own registry logic here, with
// the three real host primitives injected as plain function values rather
// than imported directly, is what makes it possible to write real,
// natively-runnable go test coverage for the mechanism itself without a
// real host at all.
package persist

import (
	"errors"
	"sort"
)

// ErrPersistedFreed is returned by Get when the key was explicitly freed
// (via Free) and has not been re-established since by an explicit Set or
// Reset. The automatic startup flush never clears this on its own -- see
// Entry's own doc comment for why a freed key must not be silently
// resurrected by the next resume.
var ErrPersistedFreed = errors.New("natyv: persisted key was freed")

// Entry is one registered key's own bookkeeping. Resolved and Freed are
// mutually exclusive and both start false -- a fresh entry has touched
// neither the host's create-if-absent primitive nor an explicit free.
type Entry struct {
	Key     string
	Default []byte
	// Resolved is true once GetOrInit has been called for this key and
	// returned a real value (not freed) -- Cached is only meaningful when
	// this is true.
	Resolved bool
	// Freed is true when the host reported this key as explicitly freed.
	// Deliberately not just "the absence of Resolved" -- a fresh,
	// never-touched entry and a freed one both have Resolved == false, but
	// only a freed one should refuse to be silently recreated by the
	// automatic flush. A plain bool (not a one-way latch like sync.Once) is
	// used specifically because Free must be reversible within the same
	// instance: Set/Reset both clear it back to false.
	Freed  bool
	Cached []byte
}

// Registry holds every key ever registered via Register, plus the three
// real host primitives injected at construction -- natyv's own root package
// wraps exactly one package-level Registry, injecting the real
// //go:wasmimport-backed calls; tests construct their own with fakes to
// verify the mechanism without a real host.
type Registry struct {
	// GetOrInit resolves key's current value: the real survived value if
	// the host already has it, or def (stored under key for the first
	// time) if the host has never seen key before. freed is true when the
	// host reports key was explicitly freed -- in that case value is
	// meaningless and must not be cached. Never nil once constructed via
	// NewRegistry.
	GetOrInit func(key string, def []byte) (value []byte, freed bool, err error)

	// SetHost is a plain, unconditional upsert -- creates key if absent,
	// overwrites if present, and (host-side) clears any freed marker.
	// Never nil once constructed via NewRegistry.
	SetHost func(key string, value []byte) error

	// FreeHost removes key from host state and marks it freed.
	// Free-if-absent is a clean no-op success on the host side. Never nil
	// once constructed via NewRegistry.
	FreeHost func(key string) error

	entries map[string]*Entry
}

// NewRegistry constructs an empty Registry over the three real host
// primitives above -- see each field's own doc comment for what it does.
func NewRegistry(
	getOrInit func(key string, def []byte) (value []byte, freed bool, err error),
	setHost func(key string, value []byte) error,
	freeHost func(key string) error,
) *Registry {
	return &Registry{
		GetOrInit: getOrInit,
		SetHost:   setHost,
		FreeHost:  freeHost,
		entries:   map[string]*Entry{},
	}
}

// Register records key/def and returns its Entry -- pure in-memory
// bookkeeping, no host call, safe to call from a package-level var
// initializer (i.e. before natyv_init has ever run) since it's the only
// part of this whole mechanism that must be. Calling Register again for a
// key already present creates a fresh Entry, replacing the old one --
// exactly what happens naturally every instance boot anyway, since
// Persisted[T] call sites are package-level vars that unconditionally
// re-run their own initializer.
func (r *Registry) Register(key string, def []byte) *Entry {
	e := &Entry{Key: key, Default: def}
	r.entries[key] = e
	return e
}

// ensureResolved is the create-if-absent dance, shared by Get (lazy, first
// real touch) and Flush (eager, once at startup) -- a no-op once e is
// already Resolved or Freed, so repeated calls (or a Flush touching an
// entry a live Get already resolved) never re-hit the host.
func (r *Registry) ensureResolved(e *Entry) error {
	if e.Resolved || e.Freed {
		return nil
	}
	value, freed, err := r.GetOrInit(e.Key, e.Default)
	if err != nil {
		return err
	}
	if freed {
		e.Freed = true
		return nil
	}
	e.Cached = value
	e.Resolved = true
	return nil
}

// Get returns e's current value, resolving it first if this is the first
// real touch. Returns ErrPersistedFreed, with no further host call, if e
// was explicitly freed and never re-established since.
func (r *Registry) Get(e *Entry) ([]byte, error) {
	if e.Freed {
		return nil, ErrPersistedFreed
	}
	if err := r.ensureResolved(e); err != nil {
		return nil, err
	}
	if e.Freed {
		return nil, ErrPersistedFreed
	}
	return e.Cached, nil
}

// Set unconditionally writes value -- an explicit Set always wins over any
// prior Free, on both the host and this Entry.
func (r *Registry) Set(e *Entry, value []byte) error {
	if err := r.SetHost(e.Key, value); err != nil {
		return err
	}
	e.Cached = value
	e.Resolved = true
	e.Freed = false
	return nil
}

// Reset is Set using e's own remembered default -- not a fourth host
// primitive.
func (r *Registry) Reset(e *Entry) error {
	return r.Set(e, e.Default)
}

// Free removes e's key from host state entirely (real host RSS
// reclamation -- native memory the host can actually free, unlike guest
// linear memory) and marks it freed on this Entry too, so a later Get on
// this same live instance also returns ErrPersistedFreed without a
// redundant host round trip.
func (r *Registry) Free(e *Entry) error {
	if err := r.FreeHost(e.Key); err != nil {
		return err
	}
	e.Freed = true
	e.Resolved = false
	e.Cached = nil
	return nil
}

// Flush eagerly resolves every registered entry that hasn't been touched
// yet, once, in one coherent pass -- called as the first line of both
// natyv_init and natyv_resume. Keys sorted before iterating for
// deterministic ordering, matching region.Registry.Snapshot's own
// determinism rationale (map iteration order is unspecified). A key
// already Resolved or Freed (e.g. a live Get already touched it) is a
// no-op via ensureResolved -- Flush never re-hits the host for it.
func (r *Registry) Flush() error {
	keys := make([]string, 0, len(r.entries))
	for k := range r.entries {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if err := r.ensureResolved(r.entries[k]); err != nil {
			return err
		}
	}
	return nil
}
