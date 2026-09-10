package widgets

import (
	"sort"

	"github.com/natyv-io/sdks/go/widgets/internal"
)

// tableHeaderHeight is the header row's own fixed height -- not
// configurable in v1, same "one sensible default" precedent Toast's
// bottom-right anchor already established.
const tableHeaderHeight float32 = 32

// Column is one Table column -- a fixed pixel width, shared between the
// header cell and every row's own cell in that position so columns stay
// vertically aligned (no column resizing in v1 -- see table.go's own
// design note, there's no host mechanism for it and it's not core to
// proving virtualization/sort/select work).
type Column struct {
	Header string
	Width  float32
}

// Table is a virtualized, single-select, sortable data grid -- guest-
// composed, no new host WidgetKind, reusing Tree's own permanent-row-pool
// virtualization technique directly (see tree.go's own doc comment for the
// full "why a churning pool risks scrambled sibling order, not just flash"
// story -- this design starts from that lesson rather than re-learning it).
//
// Each pool row is a **Button**, not a Container -- confirmed against
// main.zig's activateWidget/widgetContainsPoint that Container is never
// hit-tested for clicks at all, so a clickable, multi-cell row needs a
// Button as its own outer element. Its own label is left empty; N child
// Label cells (one per column, Fixed width matching the header) are laid
// out LEFT_TO_RIGHT inside it instead -- Layout.ParentID's own doc comment
// already anticipates "a leaf with children" as an unusual but supported
// shape, and Tree's own pool rows already proved an empty label renders
// cleanly. Header and body are two separate widgets under a non-scrolling
// outer wrapper, so the header stays fixed while only the body scrolls.
type Table struct {
	wrapper        Container
	header         Container
	headerBtns     []Button
	viewport       Container
	viewportHeight float32
	columns        []Column
	rows           [][]string
	rowHeight      float32
	sortCol        int
	sortAsc        bool
	// selected is a row index into `rows`, or -1 for none. Reset to -1 on
	// every sort -- sorting permutes `rows`, so an index-based selection
	// would otherwise silently end up marking a different row's data after
	// re-sorting. A deliberate v1 simplification, not a bug: many real data
	// grids clear selection on re-sort for exactly this reason.
	selected int
	onSelect func(rowIndex int) error

	lastScrollOffsetY float32
	lastStartIdx      int
	lastEndIdx        int
	topSpacer         Container
	bottomSpacer      Container
	rowPool           []Button
	// cellPool[k] is rowPool[k]'s own child Label cells, len(columns) each.
	cellPool [][]Label
}

// CreateTable creates the non-scrolling outer wrapper (from layout,
// Direction forced TopToBottom), the static header row, the scrolling body
// viewport (Fixed height = layout's own height minus the header's), its
// two spacers, and a fixed row pool sized to the body viewport's own row
// capacity -- then renders the initial window. layout.Sizing.Height must
// be Fixed -- its own Max is split between the header and the body.
func CreateTable(layout Layout, columns []Column, rows [][]string, rowHeight float32) (*Table, error) {
	outerHeight := layout.Sizing.Height.Max
	layout.Direction = TopToBottom
	wrapper, err := CreateContainer(layout, false, 0)
	if err != nil {
		return nil, err
	}
	t := &Table{wrapper: wrapper, columns: columns, rows: rows, rowHeight: rowHeight, selected: -1, sortCol: -1}
	wrapperID := uint32(wrapper)

	var totalWidth float32
	for _, col := range columns {
		totalWidth += col.Width
	}

	header, err := CreateContainer(Layout{
		ParentID:  &wrapperID,
		Sizing:    Sizing{Width: Fixed(totalWidth), Height: Fixed(tableHeaderHeight)},
		Direction: LeftToRight,
	}, false, 0)
	if err != nil {
		t.Destroy()
		return nil, err
	}
	t.header = header
	headerID := uint32(header)

	t.headerBtns = make([]Button, len(columns))
	for i, col := range columns {
		btn, err := CreateButton(Layout{
			ParentID: &headerID,
			Sizing:   Sizing{Width: Fixed(col.Width), Height: Fixed(tableHeaderHeight)},
		}, col.Header)
		if err != nil {
			t.Destroy()
			return nil, err
		}
		colIdx := i
		btn.OnClick(func() error { return t.sortBy(colIdx) })
		t.headerBtns[i] = btn
	}

	viewportHeight := outerHeight - tableHeaderHeight
	viewport, err := CreateContainer(Layout{
		ParentID:       &wrapperID,
		Sizing:         Sizing{Width: Fixed(totalWidth), Height: Fixed(viewportHeight)},
		Direction:      TopToBottom,
		ScrollVertical: true,
	}, false, 0)
	if err != nil {
		t.Destroy()
		return nil, err
	}
	t.viewport = viewport
	t.viewportHeight = viewportHeight
	viewportID := uint32(viewport)

	topSpacer, err := CreateContainer(Layout{
		ParentID: &viewportID,
		Sizing:   Sizing{Width: Fixed(totalWidth), Height: Fixed(0)},
	}, false, 0)
	if err != nil {
		t.Destroy()
		return nil, err
	}
	t.topSpacer = topSpacer

	poolSize := visibleRowCount(viewportHeight, rowHeight)
	t.rowPool = make([]Button, poolSize)
	t.cellPool = make([][]Label, poolSize)
	for k := 0; k < poolSize; k++ {
		row, err := CreateButton(Layout{
			ParentID:  &viewportID,
			Sizing:    Sizing{Width: Fixed(totalWidth), Height: Fixed(rowHeight)},
			Direction: LeftToRight,
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
		rowID := uint32(row)

		cells := make([]Label, len(columns))
		for i, col := range columns {
			cell, err := CreateLabel(Layout{
				ParentID: &rowID,
				Sizing:   Sizing{Width: Fixed(col.Width), Height: Fixed(rowHeight)},
			}, "")
			if err != nil {
				t.Destroy()
				return nil, err
			}
			cells[i] = cell
		}
		t.cellPool[k] = cells
	}

	bottomSpacer, err := CreateContainer(Layout{
		ParentID: &viewportID,
		Sizing:   Sizing{Width: Fixed(totalWidth), Height: Fixed(0)},
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

func (t *Table) OnSelect(handler func(rowIndex int) error) { t.onSelect = handler }

// onScroll is RegisterScroll's own handler -- see Tree's identical method
// for why skipping a no-op window is what keeps real scrolling flash-free.
func (t *Table) onScroll(scrollOffsetY float32) error {
	t.lastScrollOffsetY = scrollOffsetY
	startIdx, endIdx := virtualizedWindow(scrollOffsetY, t.rowHeight, len(t.rowPool), len(t.rows))
	if startIdx == t.lastStartIdx && endIdx == t.lastEndIdx {
		return nil
	}
	return t.render(scrollOffsetY)
}

// render repopulates the fixed pool for scrollOffsetY -- called on
// creation (offset 0), from onScroll once a scroll tick crosses a row
// boundary, after a row click changes selection, and after a header click
// re-sorts. Never creates or destroys a widget -- see Table's own doc
// comment for why that matters beyond performance.
func (t *Table) render(scrollOffsetY float32) error {
	t.lastScrollOffsetY = scrollOffsetY
	total := len(t.rows)
	startIdx, endIdx := virtualizedWindow(scrollOffsetY, t.rowHeight, len(t.rowPool), total)
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
		rowData := t.rows[i]
		for c, cell := range t.cellPool[k] {
			text := ""
			if c < len(rowData) {
				text = rowData[c]
			}
			// Same "* " selection-marker precedent Tree/Accordion/
			// Combobox/Menu already use, prefixed onto the first cell
			// only -- a Table row has no single string to prefix a whole
			// row into.
			if c == 0 && i == t.selected {
				text = "* " + text
			}
			if err := cell.SetText(text); err != nil {
				return err
			}
		}
		if err := row.SetVisible(true); err != nil {
			return err
		}
		rowIdx := i
		row.OnClick(func() error {
			t.selected = rowIdx
			if t.onSelect != nil {
				if err := t.onSelect(rowIdx); err != nil {
					return err
				}
			}
			return t.render(t.lastScrollOffsetY)
		})
	}

	return nil
}

// sortBy toggles ascending/descending if col is already the active sort
// column, else starts a fresh ascending sort on col. Lexicographic string
// comparison only in v1 -- callers with numeric columns need to
// pre-pad/format for a correct sort order, same "guest formats, host/SDK
// doesn't interpret" precedent every other plain-text cell in this SDK
// already follows.
func (t *Table) sortBy(col int) error {
	if t.sortCol == col {
		t.sortAsc = !t.sortAsc
	} else {
		t.sortCol = col
		t.sortAsc = true
	}
	asc := t.sortAsc
	sort.SliceStable(t.rows, func(a, b int) bool {
		va, vb := "", ""
		if col < len(t.rows[a]) {
			va = t.rows[a][col]
		}
		if col < len(t.rows[b]) {
			vb = t.rows[b][col]
		}
		if asc {
			return va < vb
		}
		return va > vb
	})
	t.selected = -1

	if err := t.updateHeaderLabels(); err != nil {
		return err
	}
	return t.render(t.lastScrollOffsetY)
}

// updateHeaderLabels relabels every header button from t.columns/t.sortCol/
// t.sortAsc -- split out of sortBy so WrapTable can reuse it to restore the
// sort-arrow marker on reattach, without duplicating the label-building
// logic.
func (t *Table) updateHeaderLabels() error {
	for i, btn := range t.headerBtns {
		label := t.columns[i].Header
		if i == t.sortCol {
			if t.sortAsc {
				label += " ^"
			} else {
				label += " v"
			}
		}
		if err := btn.SetLabel(label); err != nil {
			return err
		}
	}
	return nil
}

// ID is the widget id `.ntx`'s own `styles={...}`/`ref={...}` codegen
// targets, since Table (unlike Container/Button/...) isn't itself a
// uint32-based type -- see ntx/Codegen.zig's `isStructBackedWidgetKind`.
func (t *Table) ID() uint32 { return uint32(t.wrapper) }

// Destroy destroys the wrapper -- header, headerBtns, viewport, every row,
// every cell, and both spacers are all real Clay descendants of it, so
// this alone cascades through the entire table. Safe to call at any point
// during a failed CreateTable (Destroy on a zero-value handle is a no-op).
func (t *Table) Destroy() {
	t.wrapper.Destroy()
}

// TableSnapshot carries every real widget id a Table owns beyond its own
// wrapper -- see WrapTable's own doc comment for why these must be
// supplied explicitly. RowPool/CellPool must be in the exact same order
// CreateTable itself built them in (CellPool[k] is RowPool[k]'s own
// len(columns) cells, in column order) -- there's no host-side "list this
// container's children" primitive to rediscover them, same real
// constraint WrapMenuBar/WrapDialog/WrapBreadcrumbs already have.
type TableSnapshot struct {
	Header       uint32
	HeaderBtns   []uint32
	Viewport     uint32
	TopSpacer    uint32
	BottomSpacer uint32
	RowPool      []uint32
	CellPool     [][]uint32
}

// WrapTable reconstructs a *Table for a pre-existing wrapper widget id and
// the rest of its real widget ids (see TableSnapshot). Real, necessary
// difference from every other WrapX in this file: Table's entire pool
// (header/headerBtns/viewport/spacers/every row/every cell) is a
// permanent, never-destroyed set of real widgets for the table's whole
// lifetime (see this file's own type doc comment) -- unlike Menu/
// Combobox/DateTimePicker, where everything but the trigger is lazily
// created and destroyed, so there's no small "just the trigger" case
// here to lean on.
//
// columns/rows/rowHeight/viewportHeight must be the same values
// CreateTable was originally given (rows should reflect whatever order
// it was last sorted into, if the caller wants that to survive -- sortCol/
// sortAsc are for the header's own "^"/"v" label marker only, they don't
// re-sort rows themselves). scrollOffsetY lets a caller restore the prior
// scroll position; pass 0 to just reset to the top, matching CreateTable's
// own initial render(0) call.
//
// Ends by calling render(scrollOffsetY) -- the same function CreateTable
// itself calls to populate the pool initially, and the *only* place any
// row's own OnClick gets registered at all (see render's own doc
// comment) -- so this one call is what actually reattaches every visible
// row's click handler, not a separate mechanism.
func WrapTable(wrapperID uint32, snap TableSnapshot, columns []Column, rows [][]string, rowHeight, viewportHeight, scrollOffsetY float32, sortCol int, sortAsc bool) (*Table, error) {
	t := &Table{
		wrapper:        Container(wrapperID),
		header:         Container(snap.Header),
		viewport:       Container(snap.Viewport),
		viewportHeight: viewportHeight,
		columns:        columns,
		rows:           rows,
		rowHeight:      rowHeight,
		selected:       -1,
		sortCol:        sortCol,
		sortAsc:        sortAsc,
		topSpacer:      Container(snap.TopSpacer),
		bottomSpacer:   Container(snap.BottomSpacer),
	}

	t.headerBtns = make([]Button, len(snap.HeaderBtns))
	for i, id := range snap.HeaderBtns {
		btn := WrapButton(id)
		colIdx := i
		btn.OnClick(func() error { return t.sortBy(colIdx) })
		t.headerBtns[i] = btn
	}
	if err := t.updateHeaderLabels(); err != nil {
		return nil, err
	}

	t.rowPool = make([]Button, len(snap.RowPool))
	t.cellPool = make([][]Label, len(snap.RowPool))
	for k, id := range snap.RowPool {
		t.rowPool[k] = WrapButton(id)
		cells := make([]Label, len(snap.CellPool[k]))
		for c, cellID := range snap.CellPool[k] {
			cells[c] = WrapLabel(cellID)
		}
		t.cellPool[k] = cells
	}

	internal.RegisterScroll(uint32(t.viewport), func(_, offsetY float32) error {
		return t.onScroll(offsetY)
	})
	if err := t.render(scrollOffsetY); err != nil {
		return nil, err
	}
	return t, nil
}
