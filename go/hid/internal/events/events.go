// Package events is the hid package's id->handler tables and payload
// decoding, split out from hid itself for the same reason persist is split
// from the root package: hid's host-function file uses //go:wasmimport,
// which doesn't compile for a native target, and this logic deserves real
// native go test coverage.
package events

import (
	"encoding/base64"
	"encoding/json"
)

// Table holds one device's worth of handlers per handle id. A missing
// handler is a silent no-op, the same as every other event the SDK routes
// -- which is why a resumed guest must re-register (see hid.Open).
type Table struct {
	report     map[uint32]func(data []byte, dropped uint32) error
	disconnect map[uint32]func() error
}

func NewTable() *Table {
	return &Table{
		report:     map[uint32]func(data []byte, dropped uint32) error{},
		disconnect: map[uint32]func() error{},
	}
}

func (t *Table) SetReport(id uint32, fn func(data []byte, dropped uint32) error) {
	t.report[id] = fn
}

func (t *Table) SetDisconnect(id uint32, fn func() error) {
	t.disconnect[id] = fn
}

// Forget drops both handlers for id, once its handle is closed. Handle ids
// share the widget id space and are never reused, so this is about not
// holding closures forever, not about misrouting.
func (t *Table) Forget(id uint32) {
	delete(t.report, id)
	delete(t.disconnect, id)
}

type reportPayload struct {
	Data    string `json:"data"`
	Dropped uint32 `json:"dropped"`
}

// Report handles one `hid_report` event: payload is
// `{"data":"<base64>"[,"dropped":N]}`.
func (t *Table) Report(id uint32, payload string) error {
	fn, ok := t.report[id]
	if !ok {
		return nil
	}
	var p reportPayload
	if err := json.Unmarshal([]byte(payload), &p); err != nil {
		return err
	}
	data, err := base64.StdEncoding.DecodeString(p.Data)
	if err != nil {
		return err
	}
	return fn(data, p.Dropped)
}

// Disconnect handles the single `hid_disconnected` event a handle gets when
// its device is unplugged. The handle stays registered host-side until
// closed, so the handlers are kept too.
func (t *Table) Disconnect(id uint32, _ string) error {
	fn, ok := t.disconnect[id]
	if !ok {
		return nil
	}
	return fn()
}
