// Package persistjson is a hand-rolled, encode-only JSON walker that
// replaces stdlib encoding/json.Marshal for genuinely arbitrary,
// dev-authored data (persisted vars, region rebuild args) -- see the
// plan's "Safe serialization" section
// (~/.claude/plans/lexical-wishing-penguin.md) for the full root-cause
// finding. encoding/json's decode.go has no recover() anywhere; only
// encode.go's (*encodeState).marshal uses panic as its error-conversion
// mechanism, and TinyGo's asyncify-wrapped wasmexport context doesn't
// survive that recover. Decode stays plain stdlib encoding/json.Unmarshal
// everywhere -- only encode needed replacing, and only for arbitrary data
// (the widget SDK's own MarshalJSON methods keep calling stdlib
// json.Marshal internally on their own always-valid wire structs; this
// package's own Marshaler passthrough below calls them directly,
// unchanged).
//
// Deliberately its own package, separate from natyv's own root package
// (which re-exports Marshal as natyv.MarshalSafe, see ../persistjson.go):
// the root package's region.go imports natyv/sdks/go/widgets, which
// transitively imports go-pdk, whose //go:wasmimport declarations don't
// compile for a native target at all -- putting this file there too would
// have silently defeated the whole point of a plain, natively-testable
// go test loop for this package's own correctness.
package persistjson

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// ErrUnsupportedValue is returned -- never panicked -- when Marshal meets
// a NaN/infinite float, a non-string-keyed map, or a Go value it has no
// encoding for (chan, func, complex, unsafe.Pointer). stdlib
// encoding/json converts each of these into a panic deep in its own
// reflective walk and recovers at the top; that recover doesn't survive
// TinyGo's wasmexport/asyncify context, so this walker never panics at
// all and returns an ordinary error the moment it would otherwise need to
// unwind.
var ErrUnsupportedValue = errors.New("persistjson: unsupported value")

type jsonMarshaler interface {
	MarshalJSON() ([]byte, error)
}

// Marshal encodes v as JSON with ordinary Go error returns at every
// recursion level -- no panic anywhere in its own code, so there is
// nothing for a broken recover() to fail to catch. Handles bool, all
// int/uint widths, string (own JSON escaping), float32/64 (NaN/+-Inf
// rejected as an ordinary error), slice/array, map[string]V (keys sorted
// before iterating), struct (exported fields via their "json" tag),
// pointer/interface (nil -> null), and any type implementing
// MarshalJSON() ([]byte, error) -- called directly and its result/error
// propagated normally, which is exactly the widget SDK's already-shipped
// method shape.
func Marshal(v any) ([]byte, error) {
	return marshalValue(nil, reflect.ValueOf(v))
}

func marshalValue(buf []byte, rv reflect.Value) ([]byte, error) {
	if !rv.IsValid() {
		return append(buf, "null"...), nil
	}
	if (rv.Kind() == reflect.Pointer || rv.Kind() == reflect.Interface) && rv.IsNil() {
		return append(buf, "null"...), nil
	}
	if m, ok := asMarshaler(rv); ok {
		data, err := m.MarshalJSON()
		if err != nil {
			return nil, err
		}
		return append(buf, data...), nil
	}

	switch rv.Kind() {
	case reflect.Pointer, reflect.Interface:
		return marshalValue(buf, rv.Elem())
	case reflect.Bool:
		if rv.Bool() {
			return append(buf, "true"...), nil
		}
		return append(buf, "false"...), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.AppendInt(buf, rv.Int(), 10), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return strconv.AppendUint(buf, rv.Uint(), 10), nil
	case reflect.Float32:
		return marshalFloat(buf, rv.Float(), 32)
	case reflect.Float64:
		return marshalFloat(buf, rv.Float(), 64)
	case reflect.String:
		return marshalString(buf, rv.String()), nil
	case reflect.Slice:
		if rv.IsNil() {
			return append(buf, "null"...), nil
		}
		return marshalSequence(buf, rv)
	case reflect.Array:
		return marshalSequence(buf, rv)
	case reflect.Map:
		return marshalMap(buf, rv)
	case reflect.Struct:
		return marshalStruct(buf, rv)
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedValue, rv.Type())
	}
}

// asMarshaler checks both value- and pointer-receiver MarshalJSON
// implementations, matching Go's own method-set promotion rules -- the
// same reasoning already confirmed real for the widget SDK's own
// MarshalJSON methods (Card's value-receiver method promotes onto
// *Card). Only ever called on a value already known non-nil (see the nil
// check above): calling through on a nil pointer whose value-receiver
// method dereferences it would panic, exactly the failure mode stdlib's
// own marshalerEncoder avoids by special-casing nil first.
func asMarshaler(rv reflect.Value) (jsonMarshaler, bool) {
	if !rv.CanInterface() {
		return nil, false
	}
	if m, ok := rv.Interface().(jsonMarshaler); ok {
		return m, true
	}
	if rv.CanAddr() {
		if m, ok := rv.Addr().Interface().(jsonMarshaler); ok {
			return m, true
		}
	}
	return nil, false
}

func marshalFloat(buf []byte, f float64, bitSize int) ([]byte, error) {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return nil, fmt.Errorf("%w: %s is not representable in JSON", ErrUnsupportedValue, strconv.FormatFloat(f, 'g', -1, bitSize))
	}
	return strconv.AppendFloat(buf, f, 'g', -1, bitSize), nil
}

func marshalString(buf []byte, s string) []byte {
	buf = append(buf, '"')
	for _, r := range s {
		switch {
		case r == '"':
			buf = append(buf, '\\', '"')
		case r == '\\':
			buf = append(buf, '\\', '\\')
		case r == '\n':
			buf = append(buf, '\\', 'n')
		case r == '\r':
			buf = append(buf, '\\', 'r')
		case r == '\t':
			buf = append(buf, '\\', 't')
		case r < 0x20:
			buf = append(buf, '\\', 'u')
			buf = appendHex4(buf, uint16(r))
		default:
			// Invalid UTF-8 in s decodes as utf8.RuneError via range,
			// same as a genuine U+FFFD -- both re-encode identically,
			// matching encoding/json's own documented "invalid UTF-8 is
			// replaced by the Unicode replacement character" behavior.
			buf = utf8.AppendRune(buf, r)
		}
	}
	return append(buf, '"')
}

func appendHex4(buf []byte, v uint16) []byte {
	const hexDigits = "0123456789abcdef"
	return append(buf, hexDigits[(v>>12)&0xf], hexDigits[(v>>8)&0xf], hexDigits[(v>>4)&0xf], hexDigits[v&0xf])
}

func marshalSequence(buf []byte, rv reflect.Value) ([]byte, error) {
	buf = append(buf, '[')
	n := rv.Len()
	for i := 0; i < n; i++ {
		if i > 0 {
			buf = append(buf, ',')
		}
		var err error
		buf, err = marshalValue(buf, rv.Index(i))
		if err != nil {
			return nil, err
		}
	}
	return append(buf, ']'), nil
}

func marshalMap(buf []byte, rv reflect.Value) ([]byte, error) {
	if rv.IsNil() {
		return append(buf, "null"...), nil
	}
	if rv.Type().Key().Kind() != reflect.String {
		return nil, fmt.Errorf("%w: map key type %s (only string-keyed maps are supported)", ErrUnsupportedValue, rv.Type().Key())
	}
	keys := rv.MapKeys()
	sort.Slice(keys, func(i, j int) bool { return keys[i].String() < keys[j].String() })

	buf = append(buf, '{')
	for i, k := range keys {
		if i > 0 {
			buf = append(buf, ',')
		}
		buf = marshalString(buf, k.String())
		buf = append(buf, ':')
		var err error
		buf, err = marshalValue(buf, rv.MapIndex(k))
		if err != nil {
			return nil, err
		}
	}
	return append(buf, '}'), nil
}

func marshalStruct(buf []byte, rv reflect.Value) ([]byte, error) {
	t := rv.Type()
	buf = append(buf, '{')
	wrote := false
	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)
		if sf.PkgPath != "" && !sf.Anonymous {
			continue // unexported, non-embedded -- matches encoding/json's own silent skip
		}
		name, omitempty, skip := fieldJSONName(sf)
		if skip {
			continue
		}
		fv := rv.Field(i)
		if omitempty && fv.IsZero() {
			continue
		}
		if wrote {
			buf = append(buf, ',')
		}
		buf = marshalString(buf, name)
		buf = append(buf, ':')
		var err error
		buf, err = marshalValue(buf, fv)
		if err != nil {
			return nil, err
		}
		wrote = true
	}
	return append(buf, '}'), nil
}

func fieldJSONName(sf reflect.StructField) (name string, omitempty bool, skip bool) {
	name = sf.Name
	tag := sf.Tag.Get("json")
	if tag == "" {
		return name, false, false
	}
	parts := strings.Split(tag, ",")
	if parts[0] == "-" && len(parts) == 1 {
		return "", false, true
	}
	if parts[0] != "" {
		name = parts[0]
	}
	for _, opt := range parts[1:] {
		if opt == "omitempty" {
			omitempty = true
		}
	}
	return name, omitempty, false
}
