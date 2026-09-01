package widgets

import "github.com/natyv-io/sdks/go/widgets/internal"

// popoverPadding/popoverChildGap are the panel's own fixed layout
// constants -- not caller-configurable, same "one sensible default"
// precedent Table's own tableHeaderHeight and Dialog's own hardcoded
// Padding/ChildGap already set. panelWidth (CreatePopover's own parameter)
// is the one dimension that genuinely varies per use case; padding/gap
// don't need to.
const popoverPadding uint16 = 10
const popoverChildGap uint16 = 8

// Popover is a click-triggered floating panel holding arbitrary
// caller-built content -- unlike Menu/Dropdown's fixed item-list shape,
// there's no host-level concept of what's inside (a description Label, a
// Checkbox, a Close Button -- whatever the caller's own build callback
// creates). Needs no primitive of its own beyond Floating + the global
// focus/blur mechanism natyv already has -- the same class of pure-
// composition widget Dialog already is over Modal. Productized from the
// fixture's own W18 demo.
type Popover struct {
	trigger Button
	panel   Container
	// ownIDs is every widget id build's own last call created -- both for
	// Close to destroy them explicitly (natyv has no cascading delete, see
	// Dialog's own doc comment for the same reasoning) and so their own
	// blur can be wired to keep the popover open while focus is still
	// somewhere inside it. Includes non-focusable ids too (a plain Label)
	// -- wiring blur on something that can never actually receive focus is
	// harmless, and simpler than asking build to separate "interactive"
	// from "decorative" children itself.
	ownIDs []uint32
	build  func(panelID uint32) ([]uint32, error)
	width  float32
}

// CreatePopover creates the trigger Button (from layout) wired to toggle
// the popover open/closed. build is called fresh every time the popover
// opens, given the floating panel's own widget id to parent content
// under -- it creates whatever it wants (Labels, Checkboxes, Buttons, ...)
// and returns every id it created, so Popover can track them for Close and
// wire each one's own blur. A caller that wants its own "Close" button
// inside the popover closes over the *Popover CreatePopover returns and
// calls its own Close() from that button's OnClick, same as any other
// closure in this SDK capturing an outer variable. panelWidth is Fixed;
// panel height is always Fit to whatever build creates.
func CreatePopover(layout Layout, triggerLabel string, panelWidth float32, build func(panelID uint32) ([]uint32, error)) (*Popover, error) {
	trigger, err := CreateButton(layout, triggerLabel)
	if err != nil {
		return nil, err
	}
	p := &Popover{trigger: trigger, build: build, width: panelWidth}
	trigger.OnClick(func() error {
		if p.panel == 0 {
			return p.Open()
		}
		return p.Close()
	})
	trigger.OnBlur(p.onBlur)
	return p, nil
}

// Open creates the floating panel and calls build to populate it -- a
// no-op if already open, matching Close's own "safe to call regardless of
// state" precedent. Exported so a caller can open a Popover programmatically
// (a keyboard shortcut, a test hook) without a real click, same reasoning
// Menu.Open's own doc comment gives.
func (p *Popover) Open() error {
	if p.panel != 0 {
		return nil
	}
	triggerID := uint32(p.trigger)
	panel, err := CreateContainer(Layout{
		ParentID:  &triggerID,
		Sizing:    Sizing{Width: Fixed(p.width), Height: Fit()},
		Padding:   Padding{Left: popoverPadding, Right: popoverPadding, Top: popoverPadding, Bottom: popoverPadding},
		ChildGap:  popoverChildGap,
		Direction: TopToBottom,
		Floating:  true,
	}, true, 0)
	if err != nil {
		return err
	}
	p.panel = panel
	childIDs, err := p.build(uint32(panel))
	if err != nil {
		p.panel.Destroy()
		p.panel = 0
		return err
	}
	p.ownIDs = childIDs
	for _, id := range p.ownIDs {
		internal.RegisterBlur(id, p.onBlur)
	}
	return nil
}

// onBlur closes the popover unless focus moved to one of its own tracked
// children (or stayed on the trigger itself) -- same "blur closes unless
// focus is still inside my own structure" shape Menu.onTriggerBlur already
// uses, generalized here to an arbitrary, caller-reported id set instead of
// Menu's own known fixed structure.
func (p *Popover) onBlur(newFocusID uint32) error {
	if newFocusID == uint32(p.trigger) {
		return nil
	}
	for _, id := range p.ownIDs {
		if newFocusID == id {
			return nil
		}
	}
	return p.Close()
}

// Close destroys every widget build created (see ownIDs' own doc comment
// for why that's every returned id, not just the "interactive" ones) and
// the panel itself -- safe to call whether or not the popover is actually
// open, same as every other close-cascade in this SDK.
func (p *Popover) Close() error {
	if p.panel == 0 {
		return nil
	}
	for _, id := range p.ownIDs {
		internal.DestroyWidget(id)
	}
	p.ownIDs = nil
	p.panel.Destroy()
	p.panel = 0
	return nil
}

// Destroy closes the popover (if open) and destroys the trigger itself.
func (p *Popover) Destroy() {
	_ = p.Close()
	p.trigger.Destroy()
}
