package widgets

import "errors"

// Breadcrumbs is a navigation trail composed entirely from existing
// primitives -- a LeftToRight row alternating clickable crumb Buttons with
// separator Labels, needing no new host mechanism at all (same "guest
// composes it from primitives" precedent Dialog already establishes for a
// floating widget, applied here to a normal-flow one). The final crumb (the
// current page) renders as a plain, non-clickable Label instead of a
// Button, matching how a real breadcrumb trail never lets you "navigate" to
// where you already are.
type Breadcrumbs struct {
	root    uint32
	buttons []Button
}

// approxTextWidth estimates a glyph-rendered text's pixel width from its
// length alone -- a heuristic, not real measurement. Fit sizing collapses
// leaf widgets toward 0 (see Fit's own doc comment: natyv has no
// font-driven text measurement yet), so Fixed is the only real option, and
// a single uniform Fixed width across every crumb would look wrong the
// moment crumb text lengths actually differ -- the normal case for a real
// breadcrumb trail ("Home" vs. "Settings"). ~11px/glyph, tuned against
// Inter (the bundled default font) at natyv's default text size -- rough
// by nature, revisit if/when real text measurement lands.
func approxTextWidth(text string) float32 {
	const pxPerChar = 11
	const minWidth = 12
	w := float32(len(text)) * pxPerChar
	if w < minWidth {
		return minWidth
	}
	return w
}

// approxButtonWidth adds Button.drawDecorations' own drawing contract on
// top of approxTextWidth: text is drawn at a fixed `rect.x + 10` left
// inset with no clipping and no centering (see Button.zig), so a Button's
// box needs that inset plus a right-side buffer of its own, not just the
// bare text width -- underestimating here means real, visible overflow
// past the box, not just a slightly-too-generous box.
func approxButtonWidth(text string) float32 {
	const leftInset = 10
	const rightBuffer = 10
	return approxTextWidth(text) + leftInset + rightBuffer
}

// approxLabelWidth is approxTextWidth plus a small buffer -- Label.zig
// draws at `rect.x` directly (no inset), but still needs a little headroom
// since the estimate is approximate, not exact.
func approxLabelWidth(text string) float32 {
	const buffer = 4
	return approxTextWidth(text) + buffer
}

// CreateBreadcrumbs builds a row of `crumbs` (in order, root page first),
// each followed by a `separator` Label (e.g. "/" or ">") except the last.
// `layout` places the row itself (ParentID/Sizing/etc., same as every other
// Create* in this package) -- Direction is forced to LeftToRight regardless
// of what `layout` sets, since a breadcrumb trail that wasn't a row
// wouldn't be one. crumbs must have at least one entry.
func CreateBreadcrumbs(layout Layout, crumbs []string, separator string) (Breadcrumbs, error) {
	if len(crumbs) == 0 {
		return Breadcrumbs{}, errors.New("widgets: CreateBreadcrumbs needs at least one crumb")
	}

	layout.Direction = LeftToRight
	layout.ChildAlignment.Y = AlignYCenter
	root, err := CreateContainer(layout, false, 0)
	if err != nil {
		return Breadcrumbs{}, err
	}
	rootID := uint32(root)
	b := Breadcrumbs{root: rootID}

	for i, crumb := range crumbs {
		last := i == len(crumbs)-1
		if last {
			if _, err := CreateLabel(Layout{
				ParentID: &rootID,
				Sizing:   Sizing{Width: Fixed(approxLabelWidth(crumb)), Height: Fixed(20)},
			}, crumb); err != nil {
				return Breadcrumbs{}, err
			}
			break
		}

		btn, err := CreateButton(Layout{
			ParentID: &rootID,
			Sizing:   Sizing{Width: Fixed(approxButtonWidth(crumb)), Height: Fixed(20)},
		}, crumb)
		if err != nil {
			return Breadcrumbs{}, err
		}
		b.buttons = append(b.buttons, btn)

		if _, err := CreateLabel(Layout{
			ParentID: &rootID,
			Sizing:   Sizing{Width: Fixed(approxLabelWidth(separator)), Height: Fixed(20)},
		}, separator); err != nil {
			return Breadcrumbs{}, err
		}
	}

	return b, nil
}

// OnCrumbClick registers the single handler for whichever non-final crumb
// was clicked, called with that crumb's own index into the `crumbs` slice
// originally passed to CreateBreadcrumbs -- index, not label, since crumb
// text isn't guaranteed unique (nothing stops two path segments sharing a
// name). Register before the trail could plausibly be interacted with,
// same ordering every other OnClick/OnChange/OnDismiss registration in this
// SDK expects.
func (b Breadcrumbs) OnCrumbClick(handler func(index int) error) {
	for i, btn := range b.buttons {
		index := i // captured per-iteration, not the loop variable
		btn.OnClick(func() error {
			return handler(index)
		})
	}
}

// ID is the widget id `.ntx`'s own `styles={...}`/`ref={...}` codegen
// targets, since Breadcrumbs (unlike Container/Button/...) isn't itself a
// uint32-based type -- see ntx/Codegen.zig's `isStructBackedWidgetKind`.
func (b Breadcrumbs) ID() uint32 { return b.root }

// Destroy destroys the whole trail -- every crumb Button and separator
// Label is a real Clay child of root, so destroying root alone cascades to
// all of them.
func (b Breadcrumbs) Destroy() {
	Container(b.root).Destroy()
}

// WrapBreadcrumbs reconstructs a Breadcrumbs for a pre-existing root
// widget id and its ordered clickable-crumb button ids (the final,
// non-clickable crumb has no button at all -- see CreateBreadcrumbs' own
// "last" branch -- so buttonIDs has one fewer entry than the original
// crumbs list). Same real constraint as WrapMenuBar/WrapDialog's own
// ordered-id params: no host-side "list children" primitive exists to
// rediscover them. OnCrumbClick must still be called separately
// afterward, same caller contract CreateBreadcrumbs itself has.
func WrapBreadcrumbs(rootID uint32, buttonIDs []uint32) Breadcrumbs {
	b := Breadcrumbs{root: rootID}
	for _, id := range buttonIDs {
		b.buttons = append(b.buttons, Button(id))
	}
	return b
}

// ButtonIDs returns this breadcrumb trail's own crumb-button ids, in the
// same order WrapBreadcrumbs' own buttonIDs parameter expects them back
// in -- there's no host "list this container's children" primitive, so a
// caller reattaching this Breadcrumbs after a recycle must persist and
// supply this list itself.
func (b Breadcrumbs) ButtonIDs() []uint32 {
	ids := make([]uint32, len(b.buttons))
	for i, btn := range b.buttons {
		ids[i] = uint32(btn)
	}
	return ids
}
