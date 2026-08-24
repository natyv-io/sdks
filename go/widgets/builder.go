package widgets

// Builder is the shared callback type for a children/slot-accepting
// component (`.ntx` tooling Stage 6a, ~/.claude/plans/lexical-wishing-penguin.md
// -- `<Card>...</Card>`-style reuse). Defined once here, in the base SDK,
// rather than per component, so a wrapper component never needs to import
// its caller's package just to accept content.
//
// Takes a raw `parentID uint32`, not a `Container` -- matching every other
// widget-creation entry point's real convention (`Layout.ParentID
// *uint32`, `ParentID(id uint32) Layout`): any widget can be a parent, not
// just Container, and `Container` is its own named `uint32` type with no
// implicit conversion to a bare `uint32`. A composer meant to be called as
// a component (from `.ntx` codegen or by hand) should declare its own
// leading parameter the same way, for the identical reason.
//
// Returns `error`, matching every real composer's own signature
// (`func Name(...) error`) -- a Builder's body is just as capable of
// hitting a real `widgets.CreateX` failure as any other composer body, and
// needs the same way to propagate it.
type Builder func(parentID uint32) error
