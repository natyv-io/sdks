// MarshalSafe re-exports persistjson.Marshal under the natyv package --
// see sdks/go/persistjson/persistjson.go for the real implementation and
// its own doc comment for why it's a separate package (this package's own
// region.go transitively imports go-pdk via widgets, which would have
// silently defeated a plain, natively-testable go test loop for this
// logic's own correctness had it lived here directly). Exists so
// generated code and the SDK's own region.go can both reach it as
// natyv.MarshalSafe, matching PersistCodegen's existing "natyv." import
// allowlist entry.
package natyv

import "github.com/natyv-io/sdks/go/persistjson"

// MarshalSafe encodes v as JSON without ever using panic as part of its
// own error handling -- see the plan's "Safe serialization" section for
// why that matters under TinyGo's wasmexport/asyncify context, where a
// stdlib encoding/json.Marshal failure (an erroring custom MarshalJSON, a
// NaN/Inf float, an unsupported type) can't be recovered from and crashes
// the whole guest instead of returning an ordinary error.
func MarshalSafe(v any) ([]byte, error) {
	return persistjson.Marshal(v)
}
