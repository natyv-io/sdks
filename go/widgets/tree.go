package widgets

import (
	"strings"

	"github.com/natyv-io/sdks/go/widgets/internal"
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

// visibleRowCount is a fixed row pool's size: how many rows the viewport
// can show at once, plus 2 rows of overscan so a partially-visible row at
// the viewport's own bottom edge is never left blank. Shared by every
// widget virtualized this way (Tree, Table).
func visibleRowCount(viewportHeight, rowHeight float32) int {
	return int(viewportHeight/rowHeight) + 2
}

// virtualizedWindow computes the [startIdx, endIdx) slice of a
// total-length list that scrollOffsetY should populate a fixed-size pool
// of poolSize rows with -- the core virtualization math every fixed-pool
// widget in this SDK shares (Tree's own permanent row pool, and Table's
// identical technique reusing this directly rather than re-deriving it --
// see tree.go's own doc comment for why a permanent pool matters beyond
// performance). Pulled out as its own function (not a `*Tree` method) so
// `onScroll` can check whether a real scroll tick actually crossed a row
// boundary before paying for a pool relabel pass, and so Table's own
// `onScroll` can do the same without duplicating this math.
func virtualizedWindow(scrollOffsetY, rowHeight float32, poolSize, total int) (startIdx, endIdx int) {
	// scrollOffsetY is <= 0 (Clay's own convention -- 0 at the top, more
	// negative the further down you've scrolled, see ClayLayout.zig's
	// scrollContainerData).
	startIdx = int(-scrollOffsetY / rowHeight)
	if startIdx < 0 {
		startIdx = 0
	}
	if startIdx > total {
		startIdx = total
	}
	endIdx = startIdx + poolSize
	if endIdx > total {
		endIdx = total
	}
	return startIdx, endIdx
}

func (t *Tree) window(scrollOffsetY float32, total int) (startIdx, endIdx int) {
	return virtualizedWindow(scrollOffsetY, t.rowHeight, len(t.rowPool), total)
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

// ID is the widget id `.ntx`'s own `styles={...}`/`ref={...}` codegen
// targets, since Tree (unlike Container/Button/...) isn't itself a
// uint32-based type -- see ntx/Codegen.zig's `isStructBackedWidgetKind`.
func (t *Tree) ID() uint32 { return uint32(t.viewport) }

// Destroy destroys the viewport -- every row, topSpacer, and bottomSpacer
// is a real Clay child of it, so this alone cascades through all of them.
func (t *Tree) Destroy() {
	t.viewport.Destroy()
}

// TreeSnapshot carries every real widget id a Tree owns beyond its own
// viewport -- see WrapTree's own doc comment for why these must be
// supplied explicitly. RowPool must be in the exact same order CreateTree
// itself built it in -- there's no host-side "list this container's
// children" primitive to rediscover it, same real constraint
// TableSnapshot's own RowPool has.
type TreeSnapshot struct {
	TopSpacer    uint32
	BottomSpacer uint32
	RowPool      []uint32
	// Real scalar state, widened in alongside the pool's own widget ids
	// (Part 2, codegen automation), same reasoning as TableSnapshot's own
	// widening -- Snapshot() reads the *live* current value, not whatever
	// CreateTree was originally given, since natyv's own window-resize
	// support means viewportHeight in particular isn't guaranteed to have
	// stayed at its original authored value by the time a recycle happens.
	ViewportHeight float32
	RowHeight      float32
	ScrollOffsetY  float32
}

// Snapshot reads this Tree's own current pool ids and scalar state back
// out, in exactly the shape WrapTree expects to receive them in -- the
// real producer half of TreeSnapshot, letting a caller reattach this
// exact Tree after a recycle without hand-tracking any of it itself
// (roots is the one exception -- see WrapTree's own doc comment for why
// that stays the caller's responsibility).
func (t *Tree) Snapshot() TreeSnapshot {
	rowPoolIDs := make([]uint32, len(t.rowPool))
	for i, b := range t.rowPool {
		rowPoolIDs[i] = uint32(b)
	}
	return TreeSnapshot{
		TopSpacer:      uint32(t.topSpacer),
		BottomSpacer:   uint32(t.bottomSpacer),
		RowPool:        rowPoolIDs,
		ViewportHeight: t.viewportHeight,
		RowHeight:      t.rowHeight,
		ScrollOffsetY:  t.lastScrollOffsetY,
	}
}

// WrapTree reconstructs a *Tree for a pre-existing viewport widget id and
// the rest of its real widget ids (see TreeSnapshot) -- same "permanent
// pool, not a lazily-created/destroyed panel" reasoning as WrapTable's
// own doc comment (see that function for the fuller explanation, shared
// by both since Tree's own doc comment says Table reuses this exact
// virtualization technique directly).
//
// roots must be the same value CreateTree was originally given -- it
// carries each node's own live Expanded state, so a caller that wants
// expand/collapse state to survive a recycle is responsible for
// persisting and restoring the whole roots tree itself (guest-side data,
// not host-side, same reasoning as Menu's own entries param).
// rowHeight/viewportHeight/scrollOffsetY all now live on TreeSnapshot
// itself -- Snapshot() produces a real one straight off a live *Tree, so
// a caller doesn't need to separately track and pass any of them. selected
// is deliberately not a param -- it resets to nil here (no node selected),
// the same way Table's own selected already resets on every sort in the
// original, non-recycling code: it's used only for the "*" cosmetic
// marker, and plumbing a "which node was selected" identifier through
// would be disproportionate to what's lost.
//
// Ends by calling render(snap.ScrollOffsetY) -- the same function
// CreateTree itself calls to populate the pool initially, and the only
// place any row's own OnClick gets registered (see render's own doc
// comment) -- so this one call is what actually reattaches every visible
// row's click handler.
func WrapTree(viewportID uint32, snap TreeSnapshot, roots []*TreeNode) (*Tree, error) {
	t := &Tree{
		viewport:       Container(viewportID),
		viewportHeight: snap.ViewportHeight,
		roots:          roots,
		rowHeight:      snap.RowHeight,
		topSpacer:      Container(snap.TopSpacer),
		bottomSpacer:   Container(snap.BottomSpacer),
	}
	t.rowPool = make([]Button, len(snap.RowPool))
	for k, id := range snap.RowPool {
		t.rowPool[k] = WrapButton(id)
	}

	internal.RegisterScroll(uint32(t.viewport), func(_, offsetY float32) error {
		return t.onScroll(offsetY)
	})
	if err := t.render(snap.ScrollOffsetY); err != nil {
		return nil, err
	}
	return t, nil
}
