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
// layout carries sizing/padding/gap (`.ntx`'s own codegen supplies today's
// same 280x-Fit/16-pad/12-gap defaults when nothing overrides them) and,
// crucially, any style resolved via `styles={...}` -- BackgroundColor/
// CornerRadius/Border/Gradient/TextureID all flow straight through to the
// real CreateContainer call below, since ApplyStyleToLayout (called by
// generated code *before* CreateDialog, see that function's own doc
// comment) mutates this same layout value pre-creation. Direction and
// Modal are always forced regardless of what layout carries -- both are
// load-bearing for what makes this a Dialog at all (vertical title/
// message/button-row stacking, backdrop-blocking centered modal), not a
// cosmetic default a caller should be able to override into something
// that no longer reads as a dialog.
func CreateDialog(layout Layout, title, message string, buttonLabels []string) (Dialog, error) {
	layout.Direction = TopToBottom
	layout.Modal = true
	root, err := CreateContainer(layout, true, 0)
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

// ID is the widget id `.ntx`'s own `styles={...}`/`ref={...}` codegen
// targets, since Dialog (unlike Container/Button/...) isn't itself a
// uint32-based type -- see ntx/Codegen.zig's `isStructBackedWidgetKind`.
func (d Dialog) ID() uint32 { return d.root }

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

// WrapDialog reconstructs a Dialog for a pre-existing root widget id and
// its ordered button ids/labels -- see widgets.WrapMenu's own doc comment
// for the general shape this serves. buttonIDs must be in the same order
// buttonLabels were originally given to CreateDialog -- no host-side
// "list this container's children" primitive exists to rediscover them,
// same real constraint WrapMenuBar's own triggerIDs param has. Re-wires
// root's own OnDismiss (the Escape/backdrop-click handler) directly,
// matching CreateDialog's own behavior -- OnResult must still be called
// separately afterward, same caller contract CreateDialog itself has.
func WrapDialog(rootID uint32, buttonIDs []uint32, buttonLabels []string) Dialog {
	d := Dialog{root: rootID}
	for i, label := range buttonLabels {
		if i >= len(buttonIDs) {
			break
		}
		d.buttons = append(d.buttons, Button(buttonIDs[i]))
		d.labels = append(d.labels, label)
	}
	Container(rootID).OnDismiss(func() error {
		d.close()
		return nil
	})
	return d
}

// ButtonIDs returns this dialog's own button ids, in the same order
// WrapDialog's own buttonIDs parameter expects them back in -- there's no
// host "list this container's children" primitive, so a caller
// reattaching this Dialog after a recycle must persist and supply this
// list itself, alongside buttonLabels.
func (d Dialog) ButtonIDs() []uint32 {
	ids := make([]uint32, len(d.buttons))
	for i, b := range d.buttons {
		ids[i] = uint32(b)
	}
	return ids
}
