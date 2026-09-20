# natyv-io/sdks

Guest-side SDKs for natyv apps -- real, idiomatic packages over each guest language's own official
Extism PDK, one per language subdirectory.

- `go/` -- the Go SDK (`github.com/natyv-io/sdks/go`), the only one built so far. It's published on
  [pkg.go.dev](https://pkg.go.dev/github.com/natyv-io/sdks/go), so a guest app just imports it
  normally; a local `replace` directive is only needed when developing against an unreleased checkout.
  [natyv-io/mail-natyv](https://github.com/natyv-io/mail-natyv) is a real app built on it.

See the [natyv-io org page](https://github.com/natyv-io) for the project overview, install
instructions, and roadmap.

Future language SDKs (Rust, Zig, C/C++, JS/TS, C#, F#, Haskell, AssemblyScript, Python) land as
sibling subdirectories here, demand-driven.
