package widgets

// Tooltip is a hover-triggered floating panel showing a short message over
// a trigger Button -- unlike every click-triggered floating widget
// elsewhere in this SDK (Menu, Dropdown, Popover), open/close is driven by
// the host's own hover-hold timer (Button.OnHover), not a guest-observed
// click. Needs no primitive of its own beyond Floating + OnHover, both of
// which already exist host-side.
//
// Productized from the fixture's own W15 demo.
type Tooltip struct {
	trigger       Button
	panel         Container
	label         Label
	width, height float32
	message       string
}

// CreateTooltip creates the trigger Button (from layout) wired to show/hide
// a fixed-size floating panel on hover. width/height are Fixed, not Fit --
// Clay's own "fit" sizing measures a Label's *unwrapped* natural text
// width/height, so a Fit-height box around real wrapped multi-line text
// sizes itself to one line and the rest visually overhangs it (a real bug
// the fixture's own click-through caught).
func CreateTooltip(layout Layout, triggerLabel string, width, height float32, message string) (*Tooltip, error) {
	trigger, err := CreateButton(layout, triggerLabel)
	if err != nil {
		return nil, err
	}
	t := &Tooltip{trigger: trigger, width: width, height: height, message: message}
	trigger.OnHover(t.onHover)
	return t, nil
}

func (t *Tooltip) onHover(hovering bool) error {
	if hovering {
		if t.panel != 0 {
			return nil
		}
		triggerID := uint32(t.trigger)
		panel, err := CreateContainer(Layout{
			ParentID: &triggerID,
			Sizing:   Sizing{Width: Fixed(t.width), Height: Fixed(t.height)},
			Padding:  Padding{Left: 8, Right: 8, Top: 6, Bottom: 6},
			Floating: true,
		}, true, 0)
		if err != nil {
			return err
		}
		t.panel = panel
		panelID := uint32(panel)
		label, err := CreateLabel(Layout{
			ParentID: &panelID,
			Sizing:   Sizing{Width: Grow(), Height: Grow()},
		}, t.message)
		if err != nil {
			return err
		}
		t.label = label
		return nil
	}
	// natyv has no cascading delete -- destroying the panel alone leaves the
	// label (its child) still registered and still drawn, so the box
	// disappears but the text doesn't.
	if t.panel != 0 {
		t.label.Destroy()
		t.panel.Destroy()
		t.panel = 0
		t.label = 0
	}
	return nil
}

// SetMessage changes the text a future hover will show -- has no effect on
// a panel that's already open.
func (t *Tooltip) SetMessage(message string) { t.message = message }

// Destroy closes the tooltip (if currently shown) and destroys the trigger
// itself.
func (t *Tooltip) Destroy() {
	if t.panel != 0 {
		t.label.Destroy()
		t.panel.Destroy()
	}
	t.trigger.Destroy()
}
