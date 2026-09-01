package widgets

// MenuItem is one entry in a Menu. Items == nil means a leaf -- selecting
// it fires OnSelect directly. Items != nil means this entry is itself a
// submenu trigger -- selecting it opens a nested one-level flyout of plain
// string leaves instead. Deliberately just `[]string`, not `[]MenuItem`:
// nesting stops at one level (a submenu leaf can't itself have a further
// submenu) -- see Menu's own doc comment for why.
type MenuItem struct {
	Label string
	Items []string
}

// Menu is a floating dropdown list of items opened from a trigger Button --
// productized from the fixture's own W9 demo (Quinn's call, 2026-08-19,
// scoping Menu bar/W24: Menu bar needs a real, reusable Menu to open per
// bar item, not another one-off composed directly in main.go). Dropdown and
// Popover are still fixture-only -- tracked for the same treatment later,
// not done in this same pass (see project memory).
//
// One level of cascading submenu is supported (any MenuItem with Items
// set) -- generalized from the fixture's old hard-coded single "More ▸"
// item to any number of submenu-triggering entries. Nesting deliberately
// stops at one level: a submenu's own leaves are plain strings, not
// MenuItem, so there's no further Items field to recurse into. Two levels
// is what the fixture's own demo already proved positions/hit-tests/
// focus-chains correctly; going further would mean generalizing blur's
// "did focus move to a legitimate descendant" check across arbitrary
// depth, real additional complexity not yet justified by any real need.
//
// Same "create the panel+items once per open, in-place relabel for
// highlight moves, destroy the whole panel/submenu on close" design the
// fixture's own updateMenuLabels doc comment root-caused: a naive
// destroy/recreate on every arrow-key highlight move visibly flickered
// under rapid real key repeat even though the widget registry itself
// stayed correct throughout.
type Menu struct {
	trigger     Button
	panel       Container
	items       []Button
	entries     []MenuItem
	highlighted int // -1 = none

	// subOpenIndex is which entries[] index's own submenu is currently
	// open, -1 = none. subPanel/subItems/subHighlighted are that submenu's
	// own state, all reset together whenever it opens/closes.
	subOpenIndex   int
	subPanel       Container
	subItems       []Button
	subHighlighted int

	onSelect func(itemIndex, subIndex int) error
}

// CreateMenu creates the trigger Button (from layout -- see Layout's own
// SizingAxis.Fit doc comment: a leaf widget like this trigger collapses
// toward zero width/height if sized Fit, so callers should use Fixed, same
// as every other Button in this SDK) and wires its OnClick/OnBlur/OnKeyNav
// to open/close/navigate a floating dropdown of items. Panels (both levels)
// are created on open and destroyed on close -- on-demand, not
// pre-allocated, same precedent every other floating widget in this SDK
// already follows.
func CreateMenu(layout Layout, triggerLabel string, items []MenuItem) (*Menu, error) {
	trigger, err := CreateButton(layout, triggerLabel)
	if err != nil {
		return nil, err
	}
	m := &Menu{trigger: trigger, entries: items, highlighted: -1, subOpenIndex: -1, subHighlighted: -1}
	trigger.OnClick(m.onTriggerClick)
	trigger.OnBlur(m.onTriggerBlur)
	trigger.OnKeyNav(m.onTriggerKeyNav)
	return m, nil
}

// OnSelect registers the handler for whichever leaf was picked: itemIndex
// is always entries[]'s own index. subIndex is -1 for a top-level leaf, or
// the picked leaf's own index into that entry's Items for a submenu pick.
// Register before the menu could plausibly be interacted with, same
// ordering every other OnSelect/OnClick registration in this SDK expects.
func (m *Menu) OnSelect(handler func(itemIndex, subIndex int) error) { m.onSelect = handler }

// onTriggerClick -- checked in this order: submenu open (select/close at
// that level) takes priority over top-level (once a submenu's open, every
// click/Enter landing on the trigger belongs to the submenu level, same as
// the fixture's own onMenuTriggerClick already established); then closed
// (open it); then top-level-highlighted-is-a-submenu-trigger (open its
// submenu); then top-level-highlighted-is-a-leaf (select it); else close.
func (m *Menu) onTriggerClick() error {
	switch {
	case m.panel == 0:
		return m.Open()
	case m.subPanel != 0:
		if m.subHighlighted >= 0 {
			return m.selectSubItem(m.subHighlighted)
		}
		return m.closeSubmenu()
	case m.highlighted >= 0 && len(m.entries[m.highlighted].Items) > 0:
		return m.openSubmenu(m.highlighted)
	case m.highlighted >= 0:
		return m.selectItem(m.highlighted)
	default:
		return m.Close()
	}
}

// onTriggerBlur closes the *entire* cascade unless focus moved to the
// currently-open submenu's own trigger item -- still part of this menu's
// own structure, not a real click-away, same "blur closes both levels at
// once" deliberate v1 simplification the fixture's own closeMenu doc
// comment already chose (native menus often close one level at a time;
// this doesn't).
func (m *Menu) onTriggerBlur(newFocusID uint32) error {
	if m.subOpenIndex >= 0 && newFocusID == uint32(m.items[m.subOpenIndex]) {
		return nil
	}
	return m.Close()
}

func (m *Menu) onTriggerKeyNav(key string) error {
	if m.subPanel != 0 {
		return m.cycleSubHighlight(key)
	}
	return m.cycleHighlight(key)
}

func (m *Menu) cycleHighlight(key string) error {
	switch key {
	case "down":
		if m.highlighted < len(m.entries)-1 {
			m.highlighted++
		} else if m.highlighted == -1 {
			m.highlighted = 0
		}
		return m.updateLabels()
	case "up":
		if m.highlighted > 0 {
			m.highlighted--
		} else if m.highlighted == -1 {
			m.highlighted = 0
		}
		return m.updateLabels()
	}
	return nil
}

func (m *Menu) cycleSubHighlight(key string) error {
	subLabels := m.entries[m.subOpenIndex].Items
	switch key {
	case "down":
		if m.subHighlighted < len(subLabels)-1 {
			m.subHighlighted++
		} else if m.subHighlighted == -1 {
			m.subHighlighted = 0
		}
		return m.updateSubLabels()
	case "up":
		if m.subHighlighted > 0 {
			m.subHighlighted--
		} else if m.subHighlighted == -1 {
			m.subHighlighted = 0
		}
		return m.updateSubLabels()
	}
	return nil
}

// itemLabel is entries[i]'s own display text -- a trailing " ▸" for a
// submenu trigger (fixed, independent of highlight), plus a leading "▸ "
// when i == m.highlighted. Matches the fixture's own already-proven
// "More ▸" / "▸ More ▸" text-marker convention exactly, just derived from
// MenuItem.Items instead of baked into the caller's own label string.
func (m *Menu) itemLabel(i int) string {
	label := m.entries[i].Label
	if len(m.entries[i].Items) > 0 {
		label += " ▸"
	}
	if i == m.highlighted {
		label = "▸ " + label
	}
	return label
}

// updateLabels relabels the already-created top-level item Buttons in
// place using the current highlighted index -- see the type doc comment
// for why this never destroys/recreates on a highlight move.
func (m *Menu) updateLabels() error {
	for i, item := range m.items {
		if err := item.SetLabel(m.itemLabel(i)); err != nil {
			return err
		}
	}
	return nil
}

func (m *Menu) updateSubLabels() error {
	subLabels := m.entries[m.subOpenIndex].Items
	for i, item := range m.subItems {
		label := subLabels[i]
		if i == m.subHighlighted {
			label = "▸ " + label
		}
		if err := item.SetLabel(label); err != nil {
			return err
		}
	}
	return nil
}

// Open creates the floating panel (parented to the trigger) and its item
// Buttons exactly once per open -- a highlight move afterward only ever
// relabels them (updateLabels), never recreates them. Exported so a caller
// can open a Menu programmatically (a keyboard shortcut, a test hook)
// without a real click -- a no-op if already open, matching Close's own
// "safe to call regardless of state" precedent.
func (m *Menu) Open() error {
	if m.panel != 0 {
		return nil
	}
	triggerID := uint32(m.trigger)
	panel, err := CreateContainer(Layout{
		ParentID:  &triggerID,
		Sizing:    Sizing{Width: Fixed(140), Height: Fit()},
		Direction: TopToBottom,
		Floating:  true,
	}, true, 0)
	if err != nil {
		return err
	}
	m.panel = panel
	m.highlighted = -1
	panelID := uint32(panel)
	m.items = make([]Button, len(m.entries))
	for i := range m.entries {
		item, err := CreateButton(Layout{
			ParentID: &panelID,
			Sizing:   Sizing{Width: Grow(), Height: Fixed(28)},
		}, m.itemLabel(i))
		if err != nil {
			return err
		}
		idx := i
		if len(m.entries[idx].Items) > 0 {
			item.OnClick(func() error { return m.openSubmenu(idx) })
			// A real mouse click (as opposed to Enter while highlighted,
			// which never moves focus off the trigger) moves host-side
			// focus onto this item itself once its submenu is open -- it
			// needs its own OnKeyNav/OnBlur wired for arrow keys and
			// click-away-to-close to keep working from that point on, not
			// just while focus is still on the trigger. Same handlers as
			// the trigger's own (onTriggerKeyNav already branches on
			// whether a submenu is open regardless of which widget the
			// event targets; onTriggerBlur's own exception check compares
			// against this item's own id, which never matches when blur is
			// firing *from* this same item, so it still closes correctly
			// when focus leaves it for anything else).
			item.OnKeyNav(m.onTriggerKeyNav)
			item.OnBlur(m.onTriggerBlur)
		} else {
			item.OnClick(func() error { return m.selectItem(idx) })
		}
		m.items[i] = item
	}
	return nil
}

// openSubmenu creates the nested floating panel -- parented to
// items[idx], not the trigger, so it stacks structurally under the
// top-level item that opened it. A no-op if idx's own submenu is already
// open (e.g. a stray double-click).
func (m *Menu) openSubmenu(idx int) error {
	if m.subOpenIndex == idx {
		return nil
	}
	itemID := uint32(m.items[idx])
	subLabels := m.entries[idx].Items
	panel, err := CreateContainer(Layout{
		ParentID:  &itemID,
		Sizing:    Sizing{Width: Fixed(120), Height: Fit()},
		Direction: TopToBottom,
		Floating:  true,
	}, true, 0)
	if err != nil {
		return err
	}
	m.subPanel = panel
	m.subOpenIndex = idx
	m.subHighlighted = -1
	panelID := uint32(panel)
	m.subItems = make([]Button, len(subLabels))
	for i, label := range subLabels {
		item, err := CreateButton(Layout{
			ParentID: &panelID,
			Sizing:   Sizing{Width: Grow(), Height: Fixed(28)},
		}, label)
		if err != nil {
			return err
		}
		sidx := i
		item.OnClick(func() error { return m.selectSubItem(sidx) })
		m.subItems[i] = item
	}
	return nil
}

// closeSubmenu destroys the submenu's panel (and, since every subItem is a
// real Clay child of it, every item along with it). Safe to call whether
// or not the submenu is actually open.
func (m *Menu) closeSubmenu() error {
	if m.subPanel == 0 {
		return nil
	}
	m.subItems = nil
	m.subPanel.Destroy()
	m.subPanel = 0
	m.subOpenIndex = -1
	m.subHighlighted = -1
	return nil
}

// selectItem closes the menu, then calls the registered OnSelect handler
// (if any) with (itemIndex, -1) -- never called for an entry with a
// submenu, which openSubmenu handles instead. Deliberately does not
// relabel the trigger itself with the chosen item's own text -- the
// fixture's own demo did that ("Menu: <item>"), but that's presentation
// behavior specific to a standalone Menu demo, not something every
// consumer wants (Menu bar's own "File"/"Edit"/"View" triggers must keep
// their own label regardless of which item was picked) -- callers that
// want it can do it themselves inside their own OnSelect handler.
func (m *Menu) selectItem(index int) error {
	if index < 0 || index >= len(m.entries) {
		return nil
	}
	handler := m.onSelect
	if err := m.Close(); err != nil {
		return err
	}
	if handler != nil {
		return handler(index, -1)
	}
	return nil
}

// selectSubItem closes the whole cascade, then calls OnSelect with
// (the submenu's own top-level index, subIndex).
func (m *Menu) selectSubItem(subIndex int) error {
	openIdx := m.subOpenIndex
	if openIdx < 0 || subIndex < 0 || subIndex >= len(m.entries[openIdx].Items) {
		return nil
	}
	handler := m.onSelect
	if err := m.Close(); err != nil {
		return err
	}
	if handler != nil {
		return handler(openIdx, subIndex)
	}
	return nil
}

// Close closes the dropdown (both levels, if open) -- panel.Destroy() alone
// would already cascade through items (and, transitively, a still-open
// submenu too), but closeSubmenu is still called first for its own
// state-reset side effects (subOpenIndex/subHighlighted). Safe to call
// whether or not it actually is, same as every other close-cascade in this
// SDK.
func (m *Menu) Close() error {
	if err := m.closeSubmenu(); err != nil {
		return err
	}
	if m.panel == 0 {
		return nil
	}
	m.items = nil
	m.panel.Destroy()
	m.panel = 0
	m.highlighted = -1
	return nil
}

// Destroy destroys the trigger -- panel, items, and a still-open submenu
// (if any) are all real Clay descendants of it, so this alone cascades
// through everything, no need to call Close() first.
func (m *Menu) Destroy() {
	m.trigger.Destroy()
}
