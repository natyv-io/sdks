// Package hid is raw HID device access: talk to a macropad, a custom
// controller, or any device that speaks HID reports, without a driver.
//
// Every call requires conf.natyv.json to opt in, and only devices on its
// allowlist are ever visible:
//
//	"hid": {
//	  "enabled": true,
//	  "allowed_devices": [{ "vendor_id": "0x1234", "product_id": "0x5678" }]
//	}
//
// Ids are the ones the vendor publishes, as shown by System Information
// (macOS), lsusb (Linux) or Device Manager (Windows).
//
// There is no Read. The host reads every open device on its own thread and
// delivers each input report as an event, so register OnReport right after
// Open. Reports arriving faster than the app handles them are dropped past
// a small per-device backlog; the next report delivered says how many.
//
// Permissions are the OS's, not natyv's: on macOS the app needs Input
// Monitoring (System Settings > Privacy & Security), and on Linux the
// user needs a udev rule granting access to the device's hidraw node. Open
// reports "permission denied" with a hint when that's what failed.
package hid

import (
	"encoding/base64"
	"encoding/json"
	"errors"

	"github.com/extism/go-pdk"
	"github.com/natyv-io/sdks/go/hid/internal/events"
	"github.com/natyv-io/sdks/go/widgets"
)

//go:wasmimport extism:host/user hid_enumerate
func hidEnumerateHost(uint64) uint64

//go:wasmimport extism:host/user hid_open
func hidOpenHost(uint64) uint64

//go:wasmimport extism:host/user hid_write
func hidWriteHost(uint64) uint64

//go:wasmimport extism:host/user hid_close
func hidCloseHost(uint64) uint64

var handlers = events.NewTable()

// Only touches maps -- no host call can happen before natyv_init runs.
func init() {
	widgets.HandleEventType("hid_report", handlers.Report)
	widgets.HandleEventType("hid_disconnected", handlers.Disconnect)
}

// DeviceInfo describes one allowed, connected device interface. A single
// physical device often exposes several (a macropad might be a keyboard
// interface plus a vendor-defined one), each with its own Path;
// UsagePage/Usage tell them apart.
type DeviceInfo struct {
	Path               string `json:"path"`
	VendorID           uint16 `json:"vendor_id"`
	ProductID          uint16 `json:"product_id"`
	InterfaceNumber    int32  `json:"interface_number"`
	UsagePage          uint16 `json:"usage_page"`
	Usage              uint16 `json:"usage"`
	SerialNumber       string `json:"serial_number"`
	ProductString      string `json:"product_string"`
	ManufacturerString string `json:"manufacturer_string"`
}

// Enumerate lists the connected devices matching the allowlist. Paths are
// only valid while the device stays plugged in: unplugging and replugging
// can change them, so enumerate again rather than storing one.
func Enumerate() ([]DeviceInfo, error) {
	var resp struct {
		Devices []DeviceInfo `json:"devices"`
		Error   string       `json:"error,omitempty"`
	}
	if err := json.Unmarshal(pdk.ParamBytes(hidEnumerateHost(pdk.ResultBytes([]byte("{}")))), &resp); err != nil {
		return nil, err
	}
	if resp.Error != "" {
		return nil, errors.New(resp.Error)
	}
	return resp.Devices, nil
}

// Device is an open device handle.
type Device struct {
	id uint32
}

type openRequest struct {
	Path string `json:"path"`
}

// Open opens the device interface at path (from Enumerate). At most 8
// devices can be open at once.
//
// Opening a path that's already open returns the same handle rather than a
// second one. That's how devices survive a memory recycle: they stay open
// across it, and natyv_resume reclaims each one by calling Open with its
// path again -- then must call OnReport/OnDisconnect again too, since
// handlers live in guest memory and don't survive. A device resume doesn't
// reclaim is closed once resume returns.
func Open(path string) (*Device, error) {
	body, err := json.Marshal(openRequest{Path: path})
	if err != nil {
		return nil, err
	}
	var resp struct {
		Handle uint32 `json:"handle"`
		Error  string `json:"error,omitempty"`
	}
	if err := json.Unmarshal(pdk.ParamBytes(hidOpenHost(pdk.ResultBytes(body))), &resp); err != nil {
		return nil, err
	}
	if resp.Error != "" {
		return nil, errors.New(resp.Error)
	}
	return &Device{id: resp.Handle}, nil
}

// ID is the handle id reports are addressed to.
func (d *Device) ID() uint32 { return d.id }

// OnReport registers fn to receive every input report from this device.
// dropped is how many reports were discarded since the previous one
// delivered, because the app fell behind; usually 0.
func (d *Device) OnReport(fn func(data []byte, dropped uint32) error) {
	handlers.SetReport(d.id, fn)
}

// OnDisconnect registers fn to run once if the device is unplugged. The
// handle is dead afterwards (writes fail) but still needs Close.
func (d *Device) OnDisconnect(fn func() error) {
	handlers.SetDisconnect(d.id, fn)
}

type writeRequest struct {
	Handle uint32 `json:"handle"`
	Data   string `json:"data"`
}

// Write sends an output report. data[0] is the report id, or 0 for a
// device that doesn't number its reports -- so a 64-byte report is 65
// bytes here. Between 1 and 1025 bytes.
func (d *Device) Write(data []byte) error {
	body, err := json.Marshal(writeRequest{Handle: d.id, Data: base64.StdEncoding.EncodeToString(data)})
	if err != nil {
		return err
	}
	var resp struct {
		Ok    bool   `json:"ok"`
		Error string `json:"error,omitempty"`
	}
	if err := json.Unmarshal(pdk.ParamBytes(hidWriteHost(pdk.ResultBytes(body))), &resp); err != nil {
		return err
	}
	if resp.Error != "" {
		return errors.New(resp.Error)
	}
	return nil
}

type handleRequest struct {
	Handle uint32 `json:"handle"`
}

// Close closes the device and discards any reports still queued for it.
// Safe to call on an already-closed handle.
func (d *Device) Close() error {
	handlers.Forget(d.id)
	body, err := json.Marshal(handleRequest{Handle: d.id})
	if err != nil {
		return err
	}
	var resp struct {
		Error string `json:"error,omitempty"`
	}
	if err := json.Unmarshal(pdk.ParamBytes(hidCloseHost(pdk.ResultBytes(body))), &resp); err != nil {
		return err
	}
	if resp.Error != "" {
		return errors.New(resp.Error)
	}
	return nil
}
