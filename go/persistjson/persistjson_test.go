package persistjson

import (
	"encoding/json"
	"errors"
	"math"
	"testing"
)

func TestMarshalPrimitives(t *testing.T) {
	cases := []struct {
		in   any
		want string
	}{
		{true, "true"},
		{false, "false"},
		{42, "42"},
		{-7, "-7"},
		{uint32(9), "9"},
		{3.5, "3.5"},
		{"hello", `"hello"`},
		{[]int(nil), "null"},
		{(*int)(nil), "null"},
	}
	for _, c := range cases {
		got, err := Marshal(c.in)
		if err != nil {
			t.Fatalf("Marshal(%#v): %v", c.in, err)
		}
		if string(got) != c.want {
			t.Fatalf("Marshal(%#v) = %s, want %s", c.in, got, c.want)
		}
	}
}

func TestMarshalStringEscaping(t *testing.T) {
	in := "a\"b\\c\nd\te" + string(rune(1)) + "f"
	got, err := Marshal(in)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	want := `"a\"b\\c\nd\te\u0001f"`
	if string(got) != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestMarshalSliceAndArray(t *testing.T) {
	got, err := Marshal([]int{1, 2, 3})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if string(got) != "[1,2,3]" {
		t.Fatalf("got %s", got)
	}

	got, err = Marshal([2]string{"a", "b"})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if string(got) != `["a","b"]` {
		t.Fatalf("got %s", got)
	}
}

func TestMarshalMapKeysAreSorted(t *testing.T) {
	m := map[string]int{"zebra": 1, "apple": 2, "mango": 3}
	got, err := Marshal(m)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	want := `{"apple":2,"mango":3,"zebra":1}`
	if string(got) != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestMarshalNonStringMapKeyIsOrdinaryError(t *testing.T) {
	_, err := Marshal(map[int]int{1: 2})
	if !errors.Is(err, ErrUnsupportedValue) {
		t.Fatalf("expected ErrUnsupportedValue, got %v", err)
	}
}

func TestMarshalNestedStruct(t *testing.T) {
	type Inner struct {
		B string `json:"b"`
	}
	type Outer struct {
		A     int    `json:"a"`
		Inner Inner  `json:"inner"`
		Skip  string `json:"-"`
		unexp string
	}
	v := Outer{A: 1, Inner: Inner{B: "x"}, Skip: "gone", unexp: "also gone"}
	got, err := Marshal(v)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	want := `{"a":1,"inner":{"b":"x"}}`
	if string(got) != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestMarshalOmitempty(t *testing.T) {
	type S struct {
		A int    `json:"a,omitempty"`
		B string `json:"b,omitempty"`
	}
	got, err := Marshal(S{})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if string(got) != "{}" {
		t.Fatalf("got %s, want {}", got)
	}
}

func TestMarshalRejectsNaNAndInf(t *testing.T) {
	for _, f := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		_, err := Marshal(f)
		if !errors.Is(err, ErrUnsupportedValue) {
			t.Fatalf("Marshal(%v): expected ErrUnsupportedValue, got %v", f, err)
		}
	}
	// A NaN/Inf field nested inside a struct must surface the same
	// ordinary error, not silently succeed or panic -- this is exactly
	// the runtime-value hole a static "safe JSON types" allowlist alone
	// could never close (see the plan's "Safe serialization" section).
	type S struct {
		F float64 `json:"f"`
	}
	if _, err := Marshal(S{F: math.NaN()}); !errors.Is(err, ErrUnsupportedValue) {
		t.Fatalf("expected ErrUnsupportedValue for nested NaN, got %v", err)
	}
}

// erroringMarshaler is the case that used to crash the guest under
// stdlib encode.go's own defer-recover: a MarshalJSON implementation
// returning a non-nil error. Marshal must propagate it as an
// ordinary Go error, since its own walk never panics in the first place.
type erroringMarshaler struct{}

var errBoom = errors.New("boom")

func (erroringMarshaler) MarshalJSON() ([]byte, error) { return nil, errBoom }

func TestMarshalPropagatesMarshalerError(t *testing.T) {
	_, err := Marshal(erroringMarshaler{})
	if !errors.Is(err, errBoom) {
		t.Fatalf("expected errBoom, got %v", err)
	}

	// Also nested inside a struct field -- the error must propagate all
	// the way up through ordinary returns, not get swallowed.
	type Holder struct {
		M erroringMarshaler `json:"m"`
	}
	if _, err := Marshal(Holder{}); !errors.Is(err, errBoom) {
		t.Fatalf("expected errBoom from nested field, got %v", err)
	}
}

// nilableMarshaler has a value-receiver MarshalJSON, exactly like the
// widget SDK's own Card/Button methods -- a nil *nilableMarshaler must
// encode as "null" without calling the method (which would nil-deref),
// matching stdlib's own documented marshalerEncoder behavior.
type nilableMarshaler struct{ v int }

func (n nilableMarshaler) MarshalJSON() ([]byte, error) {
	return Marshal(n.v)
}

func TestMarshalNilMarshalerPointer(t *testing.T) {
	type Holder struct {
		M *nilableMarshaler `json:"m"`
	}
	got, err := Marshal(Holder{M: nil})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if string(got) != `{"m":null}` {
		t.Fatalf("got %s, want {\"m\":null}", got)
	}

	got, err = Marshal(Holder{M: &nilableMarshaler{v: 5}})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if string(got) != `{"m":5}` {
		t.Fatalf("got %s, want {\"m\":5}", got)
	}
}

// TestMarshalRoundTripsThroughStdlibDecode confirms the output is
// standard, unremarkable JSON -- decode never needed to change (see the
// plan's "Safe serialization" section: encoding/json's decode.go has no
// recover() anywhere, so it was never part of the TinyGo finding), and
// this proves Marshal's output is nothing stdlib's decoder needs to
// special-case.
func TestMarshalRoundTripsThroughStdlibDecode(t *testing.T) {
	type Payload struct {
		Name   string         `json:"name"`
		Counts []int          `json:"counts"`
		Meta   map[string]int `json:"meta"`
	}
	want := Payload{Name: "hello \"world\"", Counts: []int{1, 2, 3}, Meta: map[string]int{"a": 1, "b": 2}}

	data, err := Marshal(want)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got Payload
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("stdlib json.Unmarshal: %v", err)
	}
	if got.Name != want.Name || len(got.Counts) != len(want.Counts) || len(got.Meta) != len(want.Meta) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}
