package widgets

// Dialog is Modal (Container with Layout.Modal set) plus a semantic content
// convention -- a title (optional), a message, and a row of action buttons
// -- rather than a new primitive of its own. Needs no new host mechanism at
// all: everything it composes from (CreateContainer's Modal/background,
// CreateLabel, CreateButton, Container.OnDismiss) already exists.
//
// Every widget Dialog creates (title/message labels, the button row, every
// button) is a real Clay descendant of root, so destroying root alone
// cascades through all of them -- see close()'s own doc comment.
type Dialog struct {
	root    uint32
	buttons []Button
	labels  []string // labels[i] is buttons[i]'s label
}

// CreateDialog builds a centered, backdrop-blocking Modal (see Layout.Modal)
// containing an optional title, a message, and one Button per buttonLabels
// entry in a right-aligned row. title == "" omits the title Label entirely.
//
// Escape closes the dialog silently (no OnResult callback -- see OnResult's
// doc comment) -- wired here automatically, not left to the caller, so
// every Dialog gets correct cancel-on-Escape behavior for free. A backdrop
// click only blocks, never closes -- inherited from Modal itself, same as
// every other Modal in this SDK.
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
		if _, err := CreateLabel(Layout{
			ParentID: &rootID,
			// Fixed, not Fit -- natyv has no real font-driven Fit-height
			// measurement for text widgets yet (see SizingAxis.Fit's own
			// doc comment: a Label sized Fit collapses text height toward 0).
			Sizing: Sizing{Width: Grow(), Height: Fixed(20)},
		}, title); err != nil {
			return Dialog{}, err
		}
	}

	if _, err := CreateLabel(Layout{
		ParentID: &rootID,
		Sizing:   Sizing{Width: Grow(), Height: Fixed(20)},
	}, message); err != nil {
		return Dialog{}, err
	}

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
	}

	root.OnDismiss(func() error {
		d.close()
		return nil
	})

	return d, nil
}

// close destroys the whole dialog -- root alone cascades through every
// label/row/button it created, see the type doc comment. Harmless if
// called more than once (e.g. Escape racing a button click that already
// closed it), since a destroy against an already-gone widget_id just
// fails silently rather than crashing.
func (d Dialog) close() {
	Container(d.root).Destroy()
}

// OnResult registers the single handler for whichever button was clicked --
// called with that button's own label. Only fires on a real button click;
// Escape (wired automatically in CreateDialog) closes the dialog without
// ever calling this, so a cancelled-via-Escape dialog is silent by default,
// matching Modal's own "backdrop only blocks, doesn't invoke anything"
// precedent. Register before the dialog could plausibly be interacted with
// (i.e. right after CreateDialog returns), same ordering every other
// OnClick/OnChange/OnDismiss registration in this SDK expects.
func (d Dialog) OnResult(handler func(buttonLabel string) error) {
	for i, btn := range d.buttons {
		label := d.labels[i] // captured per-iteration, not the loop variable
		btn.OnClick(func() error {
			d.close()
			return handler(label)
		})
	}
}
