package widgets

import (
	"strings"

	"natyv/sdk/widgets/internal"
)

// indentPerDepth is how far each nesting level shifts a row's own label.
// Purely a text prefix, not real layout padding -- see rowLabel's own doc
// comment for why: Tree's rows are a fixed, permanently-alive pool
// (relabeled and shown/hidden in place, never destroyed/recreated -- see
// Tree's own doc comment), and there's no host primitive to change a
// widget's Padding after creation, only its Fixed height (natyv_set_size,
// see Container.SetHeight). Two spaces per depth, same plain-ASCII
// text-marker convention the chevron/selection marker below already use.
const indentPerDepth = "  "

// TreeNode is plain guest-side data, not a natyv widget -- Tree view is
// virtualized (see Tree's own doc comment), so a node's row only ever
// occupies a real (pool) Button while it's actually scrolled into view.
// Children are logical structure regardless of Expanded -- collapsing a
// node never discards its subtree, only removes it from the currently-
// flattened visible list.
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
// Accordion already established). Only whatever window of rows is
// currently scrolled into view is ever *populated*, but the underlying
// Button widgets themselves are a fixed-size pool (sized to the viewport's
// own row capacity) created once in CreateTree and never destroyed or
// recreated afterward -- every render (a click's expand/collapse/selection
// change, or a real `.scroll` event crossing a row boundary) only ever
// relabels a pool row, rewires its OnClick to whichever node now occupies
// that slot, and shows/hides it, plus resizing the top/bottom spacer
// Containers in place via SetHeight.
//
// This isn't just an optimization -- it's required for correctness. This
// renderer orders sibling widgets by registry slot index (whichever slot a
// widget lands in at creation time), not by creation time or any explicit
// z-order. An earlier version destroyed and recreated the whole row window
// on every render; that was safe internally (everything in one render was
// created back-to-back in a single ordered burst, guaranteeing correct
// relative slot order) but caused real, visible flash under fast real
// scrolling (natyv_dispatch runs on its own thread, concurrent with
// main.zig's draw loop, which could catch a mid-rebuild snapshot). A fixed
// permanent pool removes the churn (and the flash) entirely -- but if only
// the *spacers* were made permanent while rows kept being destroyed and
// recreated, the rows' freshly-assigned slot indices could land on either
// side of the spacers' old, fixed indices, visibly scrambling top-to-
// bottom order. Making the whole window (spacers + pool) permanent avoids
// that: nothing's relative order ever changes after CreateTree.
type Tree struct {
	viewport       Container
	viewportHeight float32
	roots          []*TreeNode
	rowHeight      float32
	selected       *TreeNode
	onSelect       func(*TreeNode) error

	lastScrollOffsetY float32
	lastStartIdx      int
	lastEndIdx        int
	topSpacer         Container
	bottomSpacer      Container
	rowPool           []Button
}

// CreateTree creates the scrollable viewport (Direction/ScrollVertical are
// forced regardless of what layout sets -- virtualization only works with a
// real top-to-bottom scroll container), the two spacer Containers, a fixed
// pool of row Buttons sized to the viewport's own row capacity, and renders
// the initial window. layout.Sizing.Height must be Fixed -- its own Max is
// the viewport's real pixel height, needed to size the pool and to know how
// many rows fit without an extra natyv_get_scroll_position round trip on
// every re-render.
func CreateTree(layout Layout, roots []*TreeNode, rowHeight float32) (*Tree, error) {
	viewportHeight := layout.Sizing.Height.Max
	layout.Direction = TopToBottom
	layout.ScrollVertical = true
	viewport, err := CreateContainer(layout, false, 0)
	if err != nil {
		return nil, err
	}
	t := &Tree{viewport: viewport, viewportHeight: viewportHeight, roots: roots, rowHeight: rowHeight}
	viewportID := uint32(viewport)

	// Created in this exact order -- topSpacer, then every pool row, then
	// bottomSpacer -- and never destroyed again, so their relative sibling
	// order (topSpacer above all rows, bottomSpacer below all of them) is
	// fixed for the Tree's whole lifetime, regardless of which node a given
	// pool row is relabeled to represent later.
	topSpacer, err := CreateContainer(Layout{
		ParentID: &viewportID,
		Sizing:   Sizing{Width: Grow(), Height: Fixed(0)},
	}, false, 0)
	if err != nil {
		t.Destroy()
		return nil, err
	}
	t.topSpacer = topSpacer

	poolSize := visibleRowCount(viewportHeight, rowHeight)
	t.rowPool = make([]Button, poolSize)
	for k := 0; k < poolSize; k++ {
		row, err := CreateButton(Layout{
			ParentID: &viewportID,
			Sizing:   Sizing{Width: Grow(), Height: Fixed(rowHeight)},
		}, "")
		if err != nil {
			t.Destroy()
			return nil, err
		}
		if err := row.SetVisible(false); err != nil {
			t.Destroy()
			return nil, err
		}
		t.rowPool[k] = row
	}

	bottomSpacer, err := CreateContainer(Layout{
		ParentID: &viewportID,
		Sizing:   Sizing{Width: Grow(), Height: Fixed(0)},
	}, false, 0)
	if err != nil {
		t.Destroy()
		return nil, err
	}
	t.bottomSpacer = bottomSpacer

	internal.RegisterScroll(viewportID, func(_, offsetY float32) error {
		return t.onScroll(offsetY)
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

// rowLabel: indentPerDepth-many indents for nesting, a chevron ("v "/"> ")
// only for a node with children (two blank spaces for a leaf, keeping
// labels visually aligned regardless of whether a given row is
// expandable), plus a "* " selection marker -- same plain-ASCII text-marker
// precedent Accordion's own chevron and Combobox's (W6) keyboard-highlight
// indicator already established, since Button has no settable background/
// fill color to highlight a selected row with instead.
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
	return prefix + strings.Repeat(indentPerDepth, n.depth) + chevron + n.Label
}

// visibleRowCount is the pool size: how many rows the viewport can show at
// once, plus 2 rows of overscan so a partially-visible row at the
// viewport's own bottom edge is never left blank.
func visibleRowCount(viewportHeight, rowHeight float32) int {
	return int(viewportHeight/rowHeight) + 2
}

// window computes the [startIdx, endIdx) slice of a total-length flattened
// list that scrollOffsetY should populate the pool with -- pulled out of
// render so onScroll can check whether a real scroll tick actually crossed
// a row boundary before paying for a pool relabel pass.
func (t *Tree) window(scrollOffsetY float32, total int) (startIdx, endIdx int) {
	// scrollOffsetY is <= 0 (Clay's own convention -- 0 at the top, more
	// negative the further down you've scrolled, see ClayLayout.zig's
	// scrollContainerData).
	startIdx = int(-scrollOffsetY / t.rowHeight)
	if startIdx < 0 {
		startIdx = 0
	}
	if startIdx > total {
		startIdx = total
	}
	endIdx = startIdx + len(t.rowPool)
	if endIdx > total {
		endIdx = total
	}
	return startIdx, endIdx
}

// onScroll is RegisterScroll's own handler. A real mouse-wheel gesture
// fires many `.scroll` events (one per frame the live offset moves), but
// the populated window only actually needs to change once scrolling
// crosses a row boundary -- most ticks land on the exact same [startIdx,
// endIdx) render already built, so this skips the (now cheap, but not
// free) relabel pass entirely when nothing would actually change.
func (t *Tree) onScroll(scrollOffsetY float32) error {
	t.lastScrollOffsetY = scrollOffsetY
	total := len(t.flatten())
	startIdx, endIdx := t.window(scrollOffsetY, total)
	if startIdx == t.lastStartIdx && endIdx == t.lastEndIdx {
		return nil
	}
	return t.render(scrollOffsetY)
}

// render repopulates the fixed pool for scrollOffsetY -- called on
// creation (offset 0), from onScroll once a scroll tick actually crosses a
// row boundary, and after any row's own click toggles expand/collapse or
// selection (using whatever offset was last known, so a toggle deep in the
// list doesn't reset scroll position). Never creates or destroys a widget
// -- see Tree's own doc comment for why that matters beyond performance.
func (t *Tree) render(scrollOffsetY float32) error {
	t.lastScrollOffsetY = scrollOffsetY
	flat := t.flatten()
	total := len(flat)
	startIdx, endIdx := t.window(scrollOffsetY, total)
	t.lastStartIdx = startIdx
	t.lastEndIdx = endIdx

	if err := t.topSpacer.SetHeight(float32(startIdx) * t.rowHeight); err != nil {
		return err
	}
	if err := t.bottomSpacer.SetHeight(float32(total-endIdx) * t.rowHeight); err != nil {
		return err
	}

	for k, row := range t.rowPool {
		i := startIdx + k
		if i >= endIdx {
			if err := row.SetVisible(false); err != nil {
				return err
			}
			continue
		}
		node := flat[i]
		if err := row.SetLabel(rowLabel(node, node == t.selected)); err != nil {
			return err
		}
		if err := row.SetVisible(true); err != nil {
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
	}

	return nil
}

// Destroy destroys every widget this Tree ever created -- natyv has no
// cascading delete, same explicit-per-child pattern every other
// multi-widget composition in this SDK already uses.
func (t *Tree) Destroy() {
	for _, row := range t.rowPool {
		row.Destroy()
	}
	t.topSpacer.Destroy()
	t.bottomSpacer.Destroy()
	t.viewport.Destroy()
}
