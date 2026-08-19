package widgets

// Dropdown is a single-select list -- Menu plus two semantic conventions:
// flat items only (no submenus) and the trigger relabels itself with the
// chosen option on select, matching a native OS dropdown/select control.
// Needs no primitive of its own -- entirely composed from Menu, the same
// "semantic convention over an existing primitive, not a new one" precedent
// Dialog already set over Modal.
//
// Productized from the fixture's own W4 demo. That demo deliberately
// scoped Dropdown to click-only, no arrow-key navigation -- true at the
// time (W4 predates W9's own Menu, which is where key_nav highlighting was
// first built), not a requirement worth preserving now. Building Dropdown
// on top of the since-productized Menu means it inherits Menu's own real,
// tested keyboard nav for free rather than deliberately omitting it.
type Dropdown struct {
	menu     *Menu
	options  []string
	onSelect func(index int) error
}

// CreateDropdown creates the trigger Button (from layout -- see Menu's own
// CreateMenu doc comment for the leaf-Fit-collapse gotcha this inherits)
// and a flat Menu of options underneath it.
func CreateDropdown(layout Layout, triggerLabel string, options []string) (*Dropdown, error) {
	items := make([]MenuItem, len(options))
	for i, o := range options {
		items[i] = MenuItem{Label: o}
	}
	menu, err := CreateMenu(layout, triggerLabel, items)
	if err != nil {
		return nil, err
	}
	d := &Dropdown{menu: menu, options: options}
	menu.OnSelect(func(itemIndex, _ int) error {
		// subIndex is always -1 here -- Dropdown never builds a MenuItem
		// with Items set, so Menu never opens a submenu for it.
		if err := d.menu.trigger.SetLabel(d.options[itemIndex]); err != nil {
			return err
		}
		if d.onSelect != nil {
			return d.onSelect(itemIndex)
		}
		return nil
	})
	return d, nil
}

// OnSelect registers the handler for whichever option was picked (its
// index into the options CreateDropdown was given) -- called after the
// trigger's own label has already been updated. Register before the
// dropdown could plausibly be interacted with, same ordering every other
// OnSelect/OnClick registration in this SDK expects.
func (d *Dropdown) OnSelect(handler func(index int) error) { d.onSelect = handler }

// Destroy destroys the underlying Menu (trigger and, if open, the panel).
func (d *Dropdown) Destroy() { d.menu.Destroy() }
