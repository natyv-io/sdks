package widgets

import "strings"

// Combobox is a typable TextField combined with a filtered floating options
// list -- mechanically Dropdown's own trigger+panel shape plus TextField's
// already-solved live-editing, not a new interaction model. As the field's
// text changes it re-filters its own in-memory option list and
// destroys/recreates the visible filtered option Buttons in the floating
// panel, same destroy/recreate composition Dropdown's own select-and-close
// already establishes, just re-run on every keystroke instead of once on
// selection. Needs no primitive of its own beyond what TextField + Floating
// already provide.
//
// Productized from the fixture's own W6 demo.
type Combobox struct {
	field         TextField
	options       []string
	panel         Container
	visible       []Button
	visibleLabels []string
	highlighted   int
	query         string
	onSelect      func(index int) error
}

// CreateCombobox creates the typable TextField trigger. The filtered options
// panel is only created on demand, once the field's text actually changes.
func CreateCombobox(layout Layout, placeholder string, options []string) (*Combobox, error) {
	field, err := CreateTextField(layout, placeholder)
	if err != nil {
		return nil, err
	}
	c := &Combobox{field: field, options: options, highlighted: -1}
	field.OnChange(func(text string) error {
		c.highlighted = -1
		return c.render(text)
	})
	field.OnBlur(func(uint32) error { return c.close() })
	field.OnKeyNav(c.onKeyNav)
	return c, nil
}

func (c *Combobox) onKeyNav(key string) error {
	switch key {
	case "down":
		if len(c.visible) > 0 {
			if c.highlighted < len(c.visible)-1 {
				c.highlighted++
			} else if c.highlighted == -1 {
				c.highlighted = 0
			}
			return c.render(c.query)
		}
	case "up":
		if len(c.visible) > 0 {
			if c.highlighted > 0 {
				c.highlighted--
			} else if c.highlighted == -1 {
				c.highlighted = 0
			}
			return c.render(c.query)
		}
	case "enter":
		return c.selectOption(c.highlighted)
	}
	return nil
}

// destroyWidgets destroys the panel (if open) -- visible is a real Clay
// child of it, so this alone cascades through every option button too.
// Doesn't touch highlighted -- render's low-level half, split out so it
// can rebuild without wiping the highlight it's about to reapply.
func (c *Combobox) destroyWidgets() {
	if c.panel == 0 {
		return
	}
	c.panel.Destroy()
	c.panel = 0
	c.visible = nil
	c.visibleLabels = nil
}

// close fully closes the combobox -- destroys any open panel/options and
// resets the highlight. Called on blur or after a selection; not used
// internally by render (see destroyWidgets's own doc comment).
func (c *Combobox) close() error {
	c.destroyWidgets()
	c.highlighted = -1
	return nil
}

// render destroys any existing panel/options and rebuilds them from scratch
// against options filtered by query (case-insensitive substring match) --
// same "destroy old widgets, create new ones" pattern Dropdown/Menu already
// establish, not a diff/patch. The highlighted row (if any) gets a "▸ "
// label prefix -- there's no guest-controllable fill color for a Button, so
// a text marker is the only way to indicate highlight without new host
// mechanism.
func (c *Combobox) render(query string) error {
	c.destroyWidgets()
	c.query = query

	var filtered []string
	lowerQuery := strings.ToLower(query)
	for _, opt := range c.options {
		if strings.Contains(strings.ToLower(opt), lowerQuery) {
			filtered = append(filtered, opt)
		}
	}
	if len(filtered) == 0 {
		c.highlighted = -1
		return nil
	}
	if c.highlighted >= len(filtered) {
		c.highlighted = len(filtered) - 1
	}

	fieldID := uint32(c.field)
	panel, err := CreateContainer(Layout{
		ParentID:  &fieldID,
		Sizing:    Sizing{Width: Fixed(180), Height: Fit()},
		Direction: TopToBottom,
		Floating:  true,
	}, true, 0)
	if err != nil {
		return err
	}
	c.panel = panel
	panelID := uint32(panel)

	for i, opt := range filtered {
		label := opt
		if i == c.highlighted {
			label = "▸ " + opt
		}
		btn, err := CreateButton(Layout{
			ParentID: &panelID,
			Sizing:   Sizing{Width: Grow(), Height: Fixed(28)},
		}, label)
		if err != nil {
			return err
		}
		c.visible = append(c.visible, btn)
		c.visibleLabels = append(c.visibleLabels, opt)
		idx := i
		btn.OnClick(func() error { return c.selectOption(idx) })
	}
	return nil
}

// selectOption sets the TextField's text to the chosen option's label and
// closes the panel -- called both from a direct click on an option and from
// Enter-on-the-highlighted-one (OnKeyNav's "enter" case).
func (c *Combobox) selectOption(index int) error {
	if index < 0 || index >= len(c.visible) {
		return nil
	}
	label := c.visibleLabels[index]
	if err := c.field.SetText(label); err != nil {
		return err
	}
	if err := c.close(); err != nil {
		return err
	}
	if c.onSelect != nil {
		return c.onSelect(index)
	}
	return nil
}

// OnSelect registers the handler for whichever option was picked (its index
// into the options CreateCombobox was given) -- called after the field's
// own text has already been updated.
func (c *Combobox) OnSelect(handler func(index int) error) { c.onSelect = handler }

// Text/SetText pass straight through to the underlying TextField, for
// reading/setting the combobox's current value without going through a
// real selection.
func (c *Combobox) Text() (string, error)     { return c.field.Text() }
func (c *Combobox) SetText(text string) error { return c.field.SetText(text) }

// ID is the widget id `.ntx`'s own `styles={...}`/`ref={...}` codegen
// targets, since Combobox (unlike Container/Button/...) isn't itself a
// uint32-based type -- see ntx/Codegen.zig's `isStructBackedWidgetKind`.
func (c *Combobox) ID() uint32 { return uint32(c.field) }

// Destroy destroys the field -- panel and every option button (if open)
// are real Clay descendants of it, so this alone cascades through
// everything, no need to call destroyWidgets first.
func (c *Combobox) Destroy() {
	c.field.Destroy()
}

// WrapCombobox reconstructs a *Combobox for a pre-existing TextField
// widget id -- mirrors CreateCombobox's own wiring exactly (field
// OnChange/OnBlur/OnKeyNav), just via WrapTextField instead of
// CreateTextField. See WrapMenu's own doc comment for the general shape
// and caveats this inherits: options must be the same list
// CreateCombobox was originally given, and a panel that happened to be
// open at recycle time is left orphaned (a later keystroke still
// rebuilds it correctly, since render() always destroys-then-rebuilds
// from scratch regardless of prior state).
func WrapCombobox(fieldID uint32, options []string) *Combobox {
	field := WrapTextField(fieldID)
	c := &Combobox{field: field, options: options, highlighted: -1}
	field.OnChange(func(text string) error {
		c.highlighted = -1
		return c.render(text)
	})
	field.OnBlur(func(uint32) error { return c.close() })
	field.OnKeyNav(c.onKeyNav)
	return c
}
