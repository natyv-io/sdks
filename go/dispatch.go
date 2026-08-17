package natyv

import (
	"encoding/json"

	"github.com/extism/go-pdk"
)

// clickHandlers is the in-guest id->handler table the host-runtime event
// contract calls for: the host only ever knows "dispatch this event," never
// which widget maps to what app behavior -- OnClick is the guest-side
// concern that makes that lookup work. Package-level state persists across
// calls into the same plugin instance (the same invariant the bookstore
// example relied on before this existed), so registrations made during
// natyv_init are still there when natyv_dispatch fires later.
var clickHandlers = map[uint32]func() error{}

func registerClick(id uint32, handler func() error) {
	clickHandlers[id] = handler
}

// changeHandlers is the OnChange counterpart to clickHandlers -- W3, the
// first host-authoritative event (a slider drag/nudge), so its handler
// signature carries the new value rather than being a bare notification.
var changeHandlers = map[uint32]func(value float32) error{}

func registerChange(id uint32, handler func(value float32) error) {
	changeHandlers[id] = handler
}

// dismissHandlers is the OnDismiss counterpart to clickHandlers -- W5,
// fired to a modal's own widget id on Escape or a backdrop click. The host
// never force-closes a modal; the guest decides here whether to actually
// destroy its subtree, same discretion every other destroy/recreate widget
// already has. Exported (unlike clickHandlers/registerClick, which never
// needed to cross a package boundary) since Container -- the type a modal
// root actually is -- lives in the sibling `clay` package, not here.
var dismissHandlers = map[uint32]func() error{}

// RegisterDismiss is exported for clay.Container.OnDismiss to call into --
// see dismissHandlers' doc comment for why this one needs to be exported
// where registerClick/registerChange don't.
func RegisterDismiss(id uint32, handler func() error) {
	dismissHandlers[id] = handler
}

// textChangeHandlers is the OnChange counterpart for TextField -- W6, fired
// after every keystroke (append or backspace). TextField lives in this same
// package (widgets.go), so this stays unexported like registerClick/
// registerChange, unlike RegisterDismiss above.
var textChangeHandlers = map[uint32]func(text string) error{}

func registerTextChange(id uint32, handler func(text string) error) {
	textChangeHandlers[id] = handler
}

// blurHandlers is OnBlur -- W6, fired to whatever widget just lost focus
// (any kind, not just TextField -- the host fires it generically). W9:
// the handler receives newFocusID (0 = none) -- which widget focus moved
// *to* -- a real gap found via Menu's submenu (clicking a submenu trigger
// moves focus onto it, firing a blur on the level above in the very same
// frame; without knowing where focus went, a handler can't tell "still
// part of my own widget tree" apart from a real click-away).
var blurHandlers = map[uint32]func(newFocusID uint32) error{}

func registerBlur(id uint32, handler func(newFocusID uint32) error) {
	blurHandlers[id] = handler
}

// keyNavHandlers is OnKeyNav -- W6, fired to a focused TextField on
// Up/Down/Enter, so a Combobox can move its highlight / select the
// highlighted option. Never fired for any other widget kind.
var keyNavHandlers = map[uint32]func(key string) error{}

func registerKeyNav(id uint32, handler func(key string) error) {
	keyNavHandlers[id] = handler
}

type dispatchEvent struct {
	WidgetID  uint32 `json:"widget_id"`
	EventType string `json:"event_type"`
	// W5: which modal (if any) WidgetID is nested under, or 0 (root/main
	// surface) -- present on every event, not just modal-related ones, see
	// FloatingOrder.surfaceIdFor's doc comment on the host side.
	SurfaceID uint32 `json:"surface_id"`
	// W3: absent/empty for "click" (never carried a payload before this),
	// populated for "change" -- always a JSON-encoded string regardless of
	// what's inside it, per Dispatch.zig's buildDispatchPayload on the host
	// side, so this needs a second Unmarshal to reach the actual value.
	Payload string `json:"payload"`
}

type changePayload struct {
	Value float32 `json:"value"`
}

type textChangedPayload struct {
	Text string `json:"text"`
}

type keyNavPayload struct {
	Key string `json:"key"`
}

type blurPayload struct {
	NewFocusID uint32 `json:"new_focus_id"`
}

// natyv_dispatch lives entirely in the SDK -- app code never needs its own
// //go:wasmexport natyv_dispatch or to parse an event payload itself, only
// to call OnClick when creating a widget.
//
//go:wasmexport natyv_dispatch
func natyvDispatchExport() int32 {
	var event dispatchEvent
	if err := pdk.InputJSON(&event); err != nil {
		pdk.SetErrorString(err.Error())
		return 1
	}

	if event.EventType == "click" {
		if handler, ok := clickHandlers[event.WidgetID]; ok {
			if err := handler(); err != nil {
				pdk.SetErrorString(err.Error())
				return 1
			}
		}
	} else if event.EventType == "change" {
		if handler, ok := changeHandlers[event.WidgetID]; ok {
			var payload changePayload
			if err := json.Unmarshal([]byte(event.Payload), &payload); err != nil {
				pdk.SetErrorString(err.Error())
				return 1
			}
			if err := handler(payload.Value); err != nil {
				pdk.SetErrorString(err.Error())
				return 1
			}
		}
	} else if event.EventType == "dismiss" {
		if handler, ok := dismissHandlers[event.WidgetID]; ok {
			if err := handler(); err != nil {
				pdk.SetErrorString(err.Error())
				return 1
			}
		}
	} else if event.EventType == "text_changed" {
		if handler, ok := textChangeHandlers[event.WidgetID]; ok {
			var payload textChangedPayload
			if err := json.Unmarshal([]byte(event.Payload), &payload); err != nil {
				pdk.SetErrorString(err.Error())
				return 1
			}
			if err := handler(payload.Text); err != nil {
				pdk.SetErrorString(err.Error())
				return 1
			}
		}
	} else if event.EventType == "blur" {
		if handler, ok := blurHandlers[event.WidgetID]; ok {
			var payload blurPayload
			if err := json.Unmarshal([]byte(event.Payload), &payload); err != nil {
				pdk.SetErrorString(err.Error())
				return 1
			}
			if err := handler(payload.NewFocusID); err != nil {
				pdk.SetErrorString(err.Error())
				return 1
			}
		}
	} else if event.EventType == "key_nav" {
		if handler, ok := keyNavHandlers[event.WidgetID]; ok {
			var payload keyNavPayload
			if err := json.Unmarshal([]byte(event.Payload), &payload); err != nil {
				pdk.SetErrorString(err.Error())
				return 1
			}
			if err := handler(payload.Key); err != nil {
				pdk.SetErrorString(err.Error())
				return 1
			}
		}
	}

	if err := pdk.OutputJSON(map[string]string{}); err != nil {
		pdk.SetErrorString(err.Error())
		return 1
	}
	return 0
}
