package widgets

import "github.com/natyv-io/sdks/go/widgets/internal"

// File picker (W23): not a natyv widget in its own right -- no visual
// representation of any kind, unlike every other file in this package.
// Triggers a real native OS file dialog off an existing widget id (usually
// a Button's own OnClick), and the eventual result (chosen path(s), or none
// if the user cancelled) arrives later as a real event through
// OnFileSelected, not a return value from ShowOpenFileDialog/
// ShowSaveFileDialog themselves -- those only ever confirm the request was
// queued, same "host-authoritative, guest finds out via a real event"
// pattern every other async natyv interaction already follows (Slider's
// drag, a real click, a real scroll).
//
// "Upload" is not a separate primitive here -- once the guest has a real
// path, reading its bytes is the guest's own language's file I/O, and
// actually transmitting them reuses the already-existing Extism HTTP
// capability. File picker's only job is getting a real path into guest
// hands.

// ShowOpenFileDialog requests a native "open file" dialog, modal for the
// app's own window. allowMany lets the user select more than one entry
// (not all platforms support this -- SDL falls back to single-select where
// it doesn't). Register OnFileSelected(triggerID, ...) before calling this
// -- the dialog may stay open indefinitely, and the result only ever
// arrives as that later event, never synchronously from this call.
func ShowOpenFileDialog(triggerID uint32, allowMany bool) error {
	return internal.ShowOpenFileDialog(triggerID, allowMany)
}

// ShowSaveFileDialog requests a native "choose where to save" dialog --
// same OnFileSelected delivery as ShowOpenFileDialog, but the chosen path
// may not exist yet (that's the whole point of a save dialog), and at most
// one path is ever returned.
func ShowSaveFileDialog(triggerID uint32) error {
	return internal.ShowSaveFileDialog(triggerID)
}

// OnFileSelected registers the handler for triggerID's own pending dialog
// result. paths is empty if the user cancelled or a real host-side error
// occurred -- natyv doesn't distinguish the two on the wire, since a guest
// can't act differently either way. Works for both ShowOpenFileDialog (0,
// 1, or many paths) and ShowSaveFileDialog (0 or 1).
func OnFileSelected(triggerID uint32, handler func(paths []string) error) {
	internal.RegisterFileSelected(triggerID, handler)
}
