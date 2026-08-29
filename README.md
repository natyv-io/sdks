# natyv-io/sdks

Guest-side SDKs for natyv apps -- real, idiomatic packages over each guest language's own official
Extism PDK, one per language subdirectory.

- `go/` -- the Go SDK (`natyv/sdk`), the only one built so far. Real guest apps depend on it via a
  `replace natyv/sdk => <path>/go` directive in their own `go.mod` (see any example under
  [natyv-io/natyv](https://github.com/natyv-io/natyv)'s own `examples/`).

Future language SDKs (Rust, Zig, C/C++, JS/TS, C#, F#, Haskell, AssemblyScript, Python) land as
sibling subdirectories here, demand-driven.
