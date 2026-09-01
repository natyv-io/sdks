package widgets

import "github.com/natyv-io/sdks/go/widgets/internal"

// AccordionSection is guest-composed, not a new host WidgetKind -- unlike
// Tabs (which needs the host to coordinate mutual exclusivity across N
// panels atomically), each section's expand/collapse is fully independent,
// so there's nothing for the host to own. This is just an existing Button
// (the header) and an existing Container (the content) wired together with
// SetVisible/SetLabel in the click handler below -- see
// WidgetHostFunctions.zig's setVisibleHostFn doc comment for the host-side
// half of this.
type AccordionSection struct {
	header   Button
	Content  Container
	expanded bool
	title    string
}

// CreateAccordionSection creates the header Button (label = a chevron
// prefix + title) and the Content Container, and wires the header's click
// to toggle Content's visibility and flip the chevron. background/
// durationMs pass straight through to CreateContainer, same as any other
// Container caller -- durationMs is almost always 0 here (a section doesn't
// usually auto-expire).
func CreateAccordionSection(headerLayout, contentLayout Layout, title string, expanded bool, background bool) (*AccordionSection, error) {
	header, err := CreateButton(headerLayout, chevronLabel(title, expanded))
	if err != nil {
		return nil, err
	}
	content, err := CreateContainer(contentLayout, background, 0)
	if err != nil {
		header.Destroy()
		return nil, err
	}
	s := &AccordionSection{header: header, Content: content, expanded: expanded, title: title}
	if !expanded {
		if err := internal.SetVisible(uint32(content), false); err != nil {
			header.Destroy()
			content.Destroy()
			return nil, err
		}
	}
	header.OnClick(func() error {
		s.expanded = !s.expanded
		if err := internal.SetVisible(uint32(s.Content), s.expanded); err != nil {
			return err
		}
		if err := s.header.SetLabel(chevronLabel(s.title, s.expanded)); err != nil {
			return err
		}
		// Only on expand -- collapsing never needs to scroll anything into
		// view. Best-effort: a section with no scrollable ancestor (or
		// already fully visible) is a harmless no-op on the host side, not
		// worth failing the whole click over.
		if s.expanded {
			_ = internal.ScrollIntoView(uint32(s.Content))
		}
		return nil
	})
	return s, nil
}

// chevronLabel prefixes plain ASCII, not a real chevron/triangle glyph --
// no widget in this codebase renders a unicode arrow today, so glyph
// coverage in the bundled font is unverified. Swap for a real chevron
// character once a manual click-through confirms it renders.
func chevronLabel(title string, expanded bool) string {
	if expanded {
		return "v " + title
	}
	return "> " + title
}

func (s *AccordionSection) Expanded() bool { return s.expanded }

// Destroy destroys the header and content explicitly -- natyv has no
// cascading delete, same explicit-per-child pattern Tabs.Destroy/
// Dialog.close already use for their own multi-widget composition.
func (s *AccordionSection) Destroy() {
	s.header.Destroy()
	s.Content.Destroy()
}
