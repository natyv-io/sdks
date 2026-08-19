package widgets

import "natyv/sdk/widgets/internal"

// indentPxPerDepth is how far each nesting level shifts a row's own label,
// via Layout.Padding.Left -- pure guest-side layout math, no host capability
// needed for indentation at all.
const indentPxPerDepth = 16

// TreeNode is plain guest-side data, not a natyv widget -- Tree view is
// virtualized (see Tree's own doc comment), so a node's row only ever
// becomes a real Button while it's actually scrolled into view. Children
// are logical structure regardless of Expanded -- collapsing a node never
// discards its subtree, only removes it from the currently-flattened
// visible list.
type TreeNode struct {
	Label    string
	Expanded bool
	Children []*TreeNode
	depth    int // set by AddChild, not caller-supplied
}

func NewTreeNode(label string) *TreeNode { return &TreeNode{Label: label} }

// AddChild also sets child's own depth from n's -- callers never compute or
// pass depth themselves.
func (n *TreeNode) AddChild(child *TreeNode) {
	child.depth = n.depth + 1
	n.Children = append(n.Children, child)
}

// Tree is a virtualized, single-select tree view -- guest-composed, no new
// host WidgetKind (same "no coordination the host needs to own" reasoning
// Accordion already established). Real widgets exist only for whatever
// window of rows is currently scrolled into view; roots (plus every
// expanded descendant) is otherwise pure Go data. Two spacer Containers
// (topSpacer/bottomSpacer, both plain no-child boxes) stand in for the
// off-screen rows above/below the window so Clay's own content-height (and
// therefore the scrollbar thumb/range) stays correct even though most rows
// aren't real widgets at any given moment -- the standard virtualized-list
// technique. The whole window (spacers + rows) is destroyed and rebuilt on
// every re-render rather than recycled in place, same "destroy old, create
// new" pattern every other on-demand panel in this SDK already uses
// (Dropdown/Popover), not true widget recycling.
type Tree struct {
	viewport       Container
	viewportHeight float32
	roots          []*TreeNode
	rowHeight      float32
	selected       *TreeNode
	onSelect       func(*TreeNode) error

	lastScrollOffsetY float32
	topSpacer         Container
	bottomSpacer      Container
	rows              []Button
}

// CreateTree creates the scrollable viewport (Direction/ScrollVertical are
// forced regardless of what layout sets -- virtualization only works with a
// real top-to-bottom scroll container) and renders the initial window.
// layout.Sizing.Height must be Fixed -- its own Max is the viewport's real
// pixel height, needed to know how many rows fit without an extra
// natyv_get_scroll_position round trip on every re-render.
func CreateTree(layout Layout, roots []*TreeNode, rowHeight float32) (*Tree, error) {
	viewportHeight := layout.Sizing.Height.Max
	layout.Direction = TopToBottom
	layout.ScrollVertical = true
	viewport, err := CreateContainer(layout, false, 0)
	if err != nil {
		return nil, err
	}
	t := &Tree{viewport: viewport, viewportHeight: viewportHeight, roots: roots, rowHeight: rowHeight}
	internal.RegisterScroll(uint32(viewport), func(_, offsetY float32) error {
		return t.render(offsetY)
	})
	if err := t.render(0); err != nil {
		t.Destroy()
		return nil, err
	}
	return t, nil
}

func (t *Tree) OnSelect(handler func(node *TreeNode) error) { t.onSelect = handler }

// flatten walks roots depth-first, skipping a node's children whenever it's
// not Expanded -- the ordered list of rows that would be visible with no
// scrolling at all.
func (t *Tree) flatten() []*TreeNode {
	var out []*TreeNode
	var walk func([]*TreeNode)
	walk = func(nodes []*TreeNode) {
		for _, n := range nodes {
			out = append(out, n)
			if n.Expanded && len(n.Children) > 0 {
				walk(n.Children)
			}
		}
	}
	walk(t.roots)
	return out
}

// rowLabel: chevron ("v "/"> ") only for a node with children, two blank
// spaces for a leaf (keeps labels visually aligned regardless of whether a
// given row is expandable), plus a "* " selection marker -- same plain-ASCII
// text-marker precedent Accordion's own chevron and Combobox's (W6)
// keyboard-highlight indicator already established, since Button has no
// settable background/fill color to highlight a selected row with instead.
func rowLabel(n *TreeNode, selected bool) string {
	prefix := ""
	if selected {
		prefix = "* "
	}
	chevron := "  "
	if len(n.Children) > 0 {
		if n.Expanded {
			chevron = "v "
		} else {
			chevron = "> "
		}
	}
	return prefix + chevron + n.Label
}

// render destroys the current window entirely and rebuilds it for
// scrollOffsetY -- called on creation (offset 0), on every real `.scroll`
// event (RegisterScroll above), and after any row's own click toggles
// expand/collapse (using whatever offset was last known, so a toggle deep
// in the list doesn't reset scroll position).
func (t *Tree) render(scrollOffsetY float32) error {
	t.lastScrollOffsetY = scrollOffsetY
	flat := t.flatten()
	total := len(flat)

	for _, row := range t.rows {
		row.Destroy()
	}
	t.rows = nil
	// Destroy is a best-effort no-op against widget id 0 (see
	// internal.DestroyWidget's own doc comment), so this is safe even on
	// the very first render, before either spacer has ever been created.
	t.topSpacer.Destroy()
	t.bottomSpacer.Destroy()

	// scrollOffsetY is <= 0 (Clay's own convention -- 0 at the top, more
	// negative the further down you've scrolled, see ClayLayout.zig's
	// scrollContainerData).
	startIdx := int(-scrollOffsetY / t.rowHeight)
	if startIdx < 0 {
		startIdx = 0
	}
	if startIdx > total {
		startIdx = total
	}
	// +2 rows of overscan on the end so a partially-visible row at the
	// viewport's own bottom edge is never left blank.
	visibleCount := int(t.viewportHeight/t.rowHeight) + 2
	endIdx := startIdx + visibleCount
	if endIdx > total {
		endIdx = total
	}

	viewportID := uint32(t.viewport)

	topSpacer, err := CreateContainer(Layout{
		ParentID: &viewportID,
		Sizing:   Sizing{Width: Grow(), Height: Fixed(float32(startIdx) * t.rowHeight)},
	}, false, 0)
	if err != nil {
		return err
	}
	t.topSpacer = topSpacer

	for i := startIdx; i < endIdx; i++ {
		node := flat[i]
		row, err := CreateButton(Layout{
			ParentID: &viewportID,
			Sizing:   Sizing{Width: Grow(), Height: Fixed(t.rowHeight)},
			Padding:  Padding{Left: uint16(node.depth * indentPxPerDepth)},
		}, rowLabel(node, node == t.selected))
		if err != nil {
			return err
		}
		n := node
		row.OnClick(func() error {
			if len(n.Children) > 0 {
				n.Expanded = !n.Expanded
			}
			t.selected = n
			if t.onSelect != nil {
				if err := t.onSelect(n); err != nil {
					return err
				}
			}
			return t.render(t.lastScrollOffsetY)
		})
		t.rows = append(t.rows, row)
	}

	bottomSpacer, err := CreateContainer(Layout{
		ParentID: &viewportID,
		Sizing:   Sizing{Width: Grow(), Height: Fixed(float32(total-endIdx) * t.rowHeight)},
	}, false, 0)
	if err != nil {
		return err
	}
	t.bottomSpacer = bottomSpacer

	return nil
}

// Destroy destroys every currently-materialized widget explicitly -- natyv
// has no cascading delete, same explicit-per-child pattern every other
// multi-widget composition in this SDK already uses.
func (t *Tree) Destroy() {
	for _, row := range t.rows {
		row.Destroy()
	}
	t.topSpacer.Destroy()
	t.bottomSpacer.Destroy()
	t.viewport.Destroy()
}
