package clay

import "natyv/sdk"

// Dialog is Modal (W5) plus a semantic content convention -- a title
// (optional), a message, and a row of action buttons -- rather than a new
// primitive of its own. Needs no new host mechanism at all: everything it
// composes from (CreateContainer's Modal/background, CreateLabel,
// CreateButton, Container.OnDismiss) already existed.
//
// Container.Destroy has no cascading delete (deliberate, unchanged --
// see WidgetHost.zig's destroyExpiredWidgets doc comment for the one
// narrow, host-driven exception to that rule, which doesn't apply here).
// Dialog tracks every widget it created so closing one explicitly destroys
// all of them, the same `closeModal`-style explicit list every other
// multi-widget panel in this project already uses.
type Dialog struct {
	root uint32
	// Every child widget id created for this dialog (title/message
	// labels, the button row, every button) -- destroyed before root
	// itself on close. Order doesn't matter; natyv has no parent-must-
	// outlive-children constraint on destroy order.
	extraIDs []uint32
	buttons  []natyv.Button
	labels   []string // labels[i] is buttons[i]'s label
}

// CreateDialog builds a centered, backdrop-blocking Modal (see Layout.Modal)
// containing an optional title, a message, and one Button per
// buttonLabels entry in a right-aligned row. title == "" omits the title
// Label entirely.
//
// Escape closes the dialog silently (no OnResult callback -- see
// OnResult's doc comment) -- wired here automatically, not left to the
// caller, so every Dialog gets correct cancel-on-Escape behavior for free.
// A backdrop click only blocks, never closes -- inherited from Modal
// itself, same as every other Modal in this project.
func CreateDialog(title, message string, buttonLabels []string) (Dialog, error) {
	root, err := CreateContainer(Layout{
		Sizing:    Sizing{Width: Fixed(280), Height: Fit()},
		Padding:   Padding{Left: 16, Right: 16, Top: 16, Bottom: 16},
		ChildGap:  12,
		Direction: TopToBottom,
		Modal:     true,
	}, true, 0)
	if err != nil {
		return Dialog{}, err
	}
	rootID := uint32(root)
	d := Dialog{root: rootID}

	if title != "" {
		titleLabel, err := CreateLabel(Layout{
			ParentID: &rootID,
			// Fixed, not Fit -- natyv has no real font-driven Fit-height
			// measurement for text widgets yet (see SizingAxis.Fit's own
			// doc comment: a Label sized Fit collapses toward 0 height).
			// Fixed(20) matches the existing precedent bookstore's own
			// title Label already uses.
			Sizing: Sizing{Width: Grow(), Height: Fixed(20)},
		}, title)
		if err != nil {
			return Dialog{}, err
		}
		d.extraIDs = append(d.extraIDs, uint32(titleLabel))
	}

	msgLabel, err := CreateLabel(Layout{
		ParentID: &rootID,
		Sizing:   Sizing{Width: Grow(), Height: Fixed(20)},
	}, message)
	if err != nil {
		return Dialog{}, err
	}
	d.extraIDs = append(d.extraIDs, uint32(msgLabel))

	row, err := CreateContainer(Layout{
		ParentID:       &rootID,
		Sizing:         Sizing{Width: Grow(), Height: Fixed(36)},
		Direction:      LeftToRight,
		ChildAlignment: Alignment{X: AlignXRight},
		ChildGap:       8,
	}, false, 0)
	if err != nil {
		return Dialog{}, err
	}
	rowID := uint32(row)
	d.extraIDs = append(d.extraIDs, rowID)

	for _, label := range buttonLabels {
		btn, err := CreateButton(Layout{
			ParentID: &rowID,
			Sizing:   Sizing{Width: Fixed(80), Height: Fixed(28)},
		}, label)
		if err != nil {
			return Dialog{}, err
		}
		d.buttons = append(d.buttons, btn)
		d.labels = append(d.labels, label)
		d.extraIDs = append(d.extraIDs, uint32(btn))
	}

	root.OnDismiss(func() error {
		d.close()
		return nil
	})

	return d, nil
}

// close destroys every widget this dialog created, explicitly -- see the
// type doc comment for why this can't be a single Destroy() call on root.
// Best-effort per widget (same as every other Destroy() in this SDK) --
// harmless if called more than once (e.g. Escape racing a button click
// that already closed it), since a destroy against an already-gone
// widget_id just fails silently rather than crashing.
func (d Dialog) close() {
	for _, id := range d.extraIDs {
		Container(id).Destroy()
	}
	Container(d.root).Destroy()
}

// OnResult registers the single handler for whichever button was clicked
// -- called with that button's own label. Only fires on a real button
// click; Escape (wired automatically in CreateDialog) closes the dialog
// without ever calling this, so a cancelled-via-Escape dialog is silent by
// default, matching Modal's own "backdrop only blocks, doesn't invoke
// anything" precedent. Register before the dialog could plausibly be
// interacted with (i.e. right after CreateDialog returns), same ordering
// every other OnClick/OnChange/OnDismiss registration in this SDK expects.
func (d Dialog) OnResult(handler func(buttonLabel string) error) {
	for i, btn := range d.buttons {
		label := d.labels[i] // captured per-iteration, not the loop variable
		btn.OnClick(func() error {
			d.close()
			return handler(label)
		})
	}
}
