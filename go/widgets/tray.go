package widgets

import "github.com/natyv-io/sdks/go/widgets/internal"

// System tray. Not a natyv widget in its own right -- no rect, no layout,
// no rendering -- for the same reason filedialog.go isn't: it is an OS
// surface natyv drives on the app's behalf, not something that lives in a
// window's widget tree. It has no `.ntx` tag either; a tray is built and
// mutated over the app's lifetime rather than laid out once.
//
// Platforms actively discourage using a tray unless persistence is a real
// feature of the app (SDL says so in its own docs, and macOS and Windows
// both treat the menu bar / notification area as scarce shared space). A
// tray that only duplicates what a window already shows is clutter.
//
// Entry ids come from the same space widget ids do, so an entry's click
// arrives through the ordinary dispatch path and OnClick works exactly as
// it does on a Button.
//
// Note on what SDL does not offer: there is no callback for clicking the
// tray *icon* itself, only for menu entries. An app that wants "click the
// icon to show my window" gives its menu an entry that does that.

// Tray is a single system tray icon plus the menu behind it.
type Tray struct {
	id uint32
}

// TrayMenu is somewhere entries can be added: either a tray's own menu or
// a submenu entry's. There is no separate menu id on the wire -- a tray's
// menu is 1:1 with the tray and a submenu is 1:1 with the entry that owns
// it, so both are addressed by the id of the thing that owns them.
type TrayMenu struct {
	parentID uint32
}

// TrayEntry is one item in a tray menu.
type TrayEntry struct {
	id   uint32
	kind string
}

// CreateTray adds an icon to the system tray. The icon itself comes from
// conf.natyv.json's `icon` -- the same PNG `natyv build` already packages
// as the app icon -- so there is nothing to pass here. An app with no icon
// configured still gets a working menu; it just has nothing to draw, which
// on macOS means an empty slot in the menu bar.
//
// tooltip may be empty. Not every platform shows one.
func CreateTray(tooltip string) (*Tray, error) {
	id, err := internal.TrayCreate(tooltip)
	if err != nil {
		return nil, err
	}
	return &Tray{id: id}, nil
}

// ID is this tray's natyv id.
func (t *Tray) ID() uint32 { return t.id }

// Menu is the tray's own top-level menu.
func (t *Tray) Menu() *TrayMenu { return &TrayMenu{parentID: t.id} }

// SetTooltip replaces the hover text.
func (t *Tray) SetTooltip(tooltip string) error {
	return internal.TraySetTooltip(t.id, tooltip)
}

// Destroy removes the tray icon and every entry under it. The entries do
// not need removing first -- destroying a tray takes its whole menu tree.
func (t *Tray) Destroy() error { return internal.TrayDestroy(t.id) }

// AddButton appends a plain clickable entry. Register OnClick on the
// returned entry to do something when it is chosen.
func (m *TrayMenu) AddButton(label string) (*TrayEntry, error) {
	return m.insert(-1, "button", label, false, true)
}

// AddCheckbox appends an entry with a checkmark. The user clicking it
// toggles the mark themselves -- read Checked() inside OnClick to find out
// what it became, rather than tracking it guest-side, since the host is
// authoritative here the same way it is for a Slider's value.
func (m *TrayMenu) AddCheckbox(label string, checked bool) (*TrayEntry, error) {
	return m.insert(-1, "checkbox", label, checked, true)
}

// AddSeparator appends a divider line. Not clickable, and its label is
// ignored -- a separator is SDL's own convention of an entry with no label
// rather than a distinct kind.
func (m *TrayMenu) AddSeparator() (*TrayEntry, error) {
	return m.insert(-1, "separator", "", false, true)
}

// AddSubmenu appends an entry that opens a nested menu, and returns both
// the entry and the menu to fill. The entry itself never fires OnClick --
// choosing it opens the submenu.
func (m *TrayMenu) AddSubmenu(label string) (*TrayEntry, *TrayMenu, error) {
	entry, err := m.insert(-1, "submenu", label, false, true)
	if err != nil {
		return nil, nil, err
	}
	return entry, &TrayMenu{parentID: entry.id}, nil
}

// InsertButtonAt is AddButton with an explicit position. pos is an index
// into this menu's existing entries; -1 appends, which is what every Add*
// above uses.
func (m *TrayMenu) InsertButtonAt(pos int32, label string) (*TrayEntry, error) {
	return m.insert(pos, "button", label, false, true)
}

func (m *TrayMenu) insert(pos int32, kind, label string, checked, enabled bool) (*TrayEntry, error) {
	id, err := internal.TrayInsertEntry(m.parentID, pos, kind, label, checked, enabled)
	if err != nil {
		return nil, err
	}
	return &TrayEntry{id: id, kind: kind}, nil
}

// ID is this entry's natyv id -- the same id its click events carry.
func (e *TrayEntry) ID() uint32 { return e.id }

// OnClick registers what to run when this entry is chosen. Separators and
// submenu entries never fire, so registering on one is a no-op.
func (e *TrayEntry) OnClick(handler func() error) *TrayEntry {
	internal.RegisterClick(e.id, handler)
	return e
}

// Submenu is the menu behind a submenu entry, for adding to it later. Empty
// and inert for any other kind.
func (e *TrayEntry) Submenu() *TrayMenu { return &TrayMenu{parentID: e.id} }

// SetLabel changes the entry's text.
func (e *TrayEntry) SetLabel(label string) error {
	return internal.TraySetEntryLabel(e.id, label)
}

// SetChecked sets a checkbox entry's mark. Only meaningful for a checkbox;
// natyv-core ignores it for any other kind rather than letting SDL see a
// call it documents as checkbox-only.
func (e *TrayEntry) SetChecked(checked bool) error {
	return internal.TraySetEntryChecked(e.id, checked)
}

// Checked reports a checkbox entry's current mark, including a change the
// user made by clicking it.
func (e *TrayEntry) Checked() (bool, error) {
	return internal.TrayEntryChecked(e.id)
}

// SetEnabled greys the entry out when false.
func (e *TrayEntry) SetEnabled(enabled bool) error {
	return internal.TraySetEntryEnabled(e.id, enabled)
}

// Remove deletes this entry, and its submenu if it has one.
func (e *TrayEntry) Remove() error { return internal.TrayRemoveEntry(e.id) }
