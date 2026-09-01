package widgets

// MenuBar is a fixed-height, non-scrolling row of Menu items -- a File/
// Edit/View-style application menu bar. Needs no host mechanism beyond
// what Menu itself already uses (Floating panels + global focus/blur): a
// top-level, non-scrolling Container is inherently "sticky" by
// construction in natyv's layout model -- there's no page-level scroll
// wrapping top-level content, scrolling is opt-in per-Container (see
// Layout.ScrollVertical/ScrollHorizontal) -- so a plain bar Container
// simply isn't nested inside anything that could scroll it away.
//
// "Only one bar menu open at a time" needs no bar-level coordination code:
// composing N real Menu instances is enough on its own. Clicking a
// different bar item's trigger fires the host's single global
// focused_widget_id's `.blur` on whichever trigger was previously focused
// (carrying the new trigger's own id as newFocusID) before that item's own
// `.click` handler runs -- and Menu.onTriggerBlur already closes
// unconditionally on any blur, submenu-exception-free by design (see
// menu.go's own doc comment). So switching bar items closes the old
// dropdown and opens the new one for free, the same way it would if Menu
// were used standalone and a click landed on some unrelated focusable
// widget elsewhere on screen.
type MenuBar struct {
	bar   Container
	menus []*Menu
}

// MenuBarEntry is one top-level bar item -- Header is the trigger's own
// label ("File"), Items is its dropdown's own item list (see MenuItem --
// any entry can itself be a one-level submenu trigger, e.g. an "Export ▸"
// item under "File").
type MenuBarEntry struct {
	Header string
	Items  []MenuItem
}

// CreateMenuBar creates the bar Container (from layout, Direction forced
// LeftToRight) and one Menu per entries[i], each trigger Fixed-sized to
// itemWidth/itemHeight -- Fixed, not Fit, since a leaf Button sized Fit
// collapses toward zero (see Layout.Floating's sibling SizingAxis.Fit doc
// comment) -- a single shared size for every top-level item, same
// "explicit, not auto-measured" precedent Table's own Column{Width} already
// set, appropriate for short single-word bar labels.
func CreateMenuBar(layout Layout, entries []MenuBarEntry, itemWidth, itemHeight float32) (*MenuBar, error) {
	layout.Direction = LeftToRight
	bar, err := CreateContainer(layout, true, 0)
	if err != nil {
		return nil, err
	}
	barID := uint32(bar)
	mb := &MenuBar{bar: bar}
	for _, entry := range entries {
		menu, err := CreateMenu(Layout{
			ParentID: &barID,
			Sizing:   Sizing{Width: Fixed(itemWidth), Height: Fixed(itemHeight)},
		}, entry.Header, entry.Items)
		if err != nil {
			mb.Destroy()
			return nil, err
		}
		mb.menus = append(mb.menus, menu)
	}
	return mb, nil
}

// OpenBarItem opens entries[barIndex]'s own dropdown programmatically --
// same use case as Menu.Open's own doc comment (a keyboard shortcut, a test
// hook), just addressed by bar position instead of holding a *Menu
// directly.
func (mb *MenuBar) OpenBarItem(barIndex int) error {
	if barIndex < 0 || barIndex >= len(mb.menus) {
		return nil
	}
	return mb.menus[barIndex].Open()
}

// OnSelect registers one handler for the whole bar, called with which
// top-level entry (barIndex, into the entries CreateMenuBar was given) and
// which of its own items (itemIndex) was picked -- subIndex follows
// Menu.OnSelect's own convention: -1 for a top-level leaf, or the picked
// submenu leaf's own index.
func (mb *MenuBar) OnSelect(handler func(barIndex, itemIndex, subIndex int) error) {
	for i, menu := range mb.menus {
		bi := i
		menu.OnSelect(func(itemIndex, subIndex int) error { return handler(bi, itemIndex, subIndex) })
	}
}

// Destroy destroys the bar Container -- every Menu's own trigger is a real
// Clay child of it (and everything each Menu itself created is, in turn, a
// descendant of that trigger), so this alone cascades through the entire
// bar.
func (mb *MenuBar) Destroy() {
	mb.bar.Destroy()
}
