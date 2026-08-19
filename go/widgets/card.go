package widgets

// Card is Panel plus a semantic title convention -- an optional title
// Label and a Divider above a content area, all inside the same
// background-styled Container Panel already establishes. Needs no
// primitive of its own beyond what Panel/Label/Divider/Container already
// provide, same "semantic convention over an existing primitive, not a new
// one" precedent Dialog (over Modal) and Dropdown (over Menu) already set.
//
// Unlike Panel, Card imposes one real opinion: `Direction` is always
// TopToBottom (title stacked above content), overriding whatever the
// caller's own layout set -- everything else (Padding, ChildGap, Sizing,
// ParentID, ...) is passed straight through, same as CreatePanel. No
// hidden default padding/gap beyond that -- pass your own, same as every
// other CreateContainer call in this SDK already expects to.
type Card struct {
	panel   Container
	title   Label
	divider Divider
	content Container
}

// CreateCard creates the card's outer panel and, if title is non-empty, a
// title row (Label + Divider) above the content area -- pass "" for a
// title-less card. Parent your own content under ContentID(), not
// CreateCard's own `layout.ParentID` -- that was already consumed for the
// outer panel itself.
func CreateCard(layout Layout, title string) (*Card, error) {
	layout.Direction = TopToBottom
	panel, err := CreatePanel(layout)
	if err != nil {
		return nil, err
	}
	card := &Card{panel: panel}
	panelID := uint32(panel)

	if title != "" {
		titleLabel, err := CreateLabel(Layout{
			ParentID: &panelID,
			Sizing:   Sizing{Width: Grow(), Height: Fixed(24)},
		}, title)
		if err != nil {
			return nil, err
		}
		card.title = titleLabel

		divider, err := CreateDivider(Layout{
			ParentID: &panelID,
			Sizing:   Sizing{Width: Grow(), Height: Fixed(2)},
		})
		if err != nil {
			return nil, err
		}
		card.divider = divider
	}

	content, err := CreateContainer(Layout{
		ParentID:  &panelID,
		Sizing:    Sizing{Width: Grow(), Height: Fit()},
		ChildGap:  6,
		Direction: TopToBottom,
	}, false, 0)
	if err != nil {
		return nil, err
	}
	card.content = content
	return card, nil
}

// ContentID is the widget id to parent your own content under -- pass
// widgets.ParentID(card.ContentID()) (or a plain &id) to any CreateXxx
// call, the same way any other Container's own id is used as a parent.
func (c *Card) ContentID() uint32 { return uint32(c.content) }

// SetTitle updates the title text -- a no-op if CreateCard was given "" (no
// title row exists to update).
func (c *Card) SetTitle(title string) error {
	if c.title == 0 {
		return nil
	}
	return c.title.SetText(title)
}

// Destroy destroys every widget CreateCard created -- no cascading delete
// exists in natyv (see Dialog's own doc comment for the same reasoning).
// Does not destroy whatever the caller parented under ContentID() --
// that's the caller's own content, the caller's own responsibility, same
// as Popover's own Close() only ever destroying what build itself created.
func (c *Card) Destroy() {
	if c.title != 0 {
		c.title.Destroy()
		c.divider.Destroy()
	}
	c.content.Destroy()
	c.panel.Destroy()
}
