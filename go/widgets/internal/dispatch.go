package internal

import (
	"encoding/json"

	"github.com/extism/go-pdk"
)

// clickHandlers is the in-guest id->handler table the host-runtime event
// contract calls for: the host only ever knows "dispatch this event," never
// which widget maps to what app behavior -- OnClick is the guest-side
// concern that makes that lookup work. Package-level state persists across
// calls into the same plugin instance, so registrations made during
// natyv_init are still there when natyv_dispatch fires later.
var clickHandlers = map[uint32]func() error{}

// RegisterClick is exported (unlike this file's original, single-package
// home) since every widget kind now lives in its own file in the sibling
// `widgets` package, one import boundary away from this one.
func RegisterClick(id uint32, handler func() error) {
	clickHandlers[id] = handler
}

// changeHandlers is the OnChange counterpart to clickHandlers -- W3, the
// first host-authoritative event (a slider drag/nudge), so its handler
// signature carries the new value rather than being a bare notification.
var changeHandlers = map[uint32]func(value float32) error{}

func RegisterChange(id uint32, handler func(value float32) error) {
	changeHandlers[id] = handler
}

// dismissHandlers is the OnDismiss counterpart to clickHandlers -- W5,
// fired to a modal's own widget id on Escape or a backdrop click. The host
// never force-closes a modal; the guest decides here whether to actually
// destroy its subtree, same discretion every other destroy/recreate widget
// already has.
var dismissHandlers = map[uint32]func() error{}

func RegisterDismiss(id uint32, handler func() error) {
	dismissHandlers[id] = handler
}

// textChangeHandlers is the OnChange counterpart for TextField/TextArea --
// W6, fired after every keystroke (append or backspace).
var textChangeHandlers = map[uint32]func(text string) error{}

func RegisterTextChange(id uint32, handler func(text string) error) {
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

func RegisterBlur(id uint32, handler func(newFocusID uint32) error) {
	blurHandlers[id] = handler
}

// keyNavHandlers is OnKeyNav -- W6, fired to a focused TextField on
// Up/Down/Enter, so a Combobox can move its highlight / select the
// highlighted option. Never fired for any other widget kind.
var keyNavHandlers = map[uint32]func(key string) error{}

func RegisterKeyNav(id uint32, handler func(key string) error) {
	keyNavHandlers[id] = handler
}

// hoverHandlers is OnHover -- W15, fired when the host's hover-hold timer
// crosses its threshold (true) and again the moment that widget stops
// being hovered (false). Unlike OnClick, this reports a state transition,
// not a discrete action -- a handler typically creates a tooltip's
// Container+Label on true and destroys it on false, guarding the destroy
// on "did I actually create one" since the host coalesces hover events
// the same way it coalesces TextField's change events.
var hoverHandlers = map[uint32]func(hovering bool) error{}

func RegisterHover(id uint32, handler func(hovering bool) error) {
	hoverHandlers[id] = handler
}

// scrollHandlers is OnScroll -- Tree view, fired to a scroll container's own
// widget id whenever its live offset actually changes (host-side detection,
// see WidgetHostFunctions.zig/ClayLayout.zig's own doc comments). Coalesced
// like OnHover/OnChange: a container can move every frame while actively
// scrolling, only the latest offset matters to a guest re-windowing a
// virtualized list.
var scrollHandlers = map[uint32]func(offsetX, offsetY float32) error{}

func RegisterScroll(id uint32, handler func(offsetX, offsetY float32) error) {
	scrollHandlers[id] = handler
}

// fileSelectedHandlers is OnFileSelected -- W23, fired to the trigger
// widget id that originally called natyv_show_open_file_dialog/
// natyv_show_save_file_dialog, once the real SDL callback fires. Discrete
// like OnClick/OnDismiss, not coalesced (see EventQueue.EventType's own
// doc comment for why). paths is empty for a cancelled dialog or a real
// host-side error -- the wire contract doesn't distinguish the two, since
// a guest can't act differently either way.
var fileSelectedHandlers = map[uint32]func(paths []string) error{}

func RegisterFileSelected(id uint32, handler func(paths []string) error) {
	fileSelectedHandlers[id] = handler
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

type hoverPayload struct {
	Hovering bool `json:"hovering"`
}

type scrollPayload struct {
	ScrollOffsetX float32 `json:"scroll_offset_x"`
	ScrollOffsetY float32 `json:"scroll_offset_y"`
}

type fileSelectedPayload struct {
	Paths []string `json:"paths"`
}

// natyv_dispatch lives entirely in the SDK -- app code never needs its own
// //go:wasmexport natyv_dispatch or to parse an event payload itself, only
// to call OnClick when creating a widget. Living in `internal` doesn't
// change that -- a `//go:wasmexport` symbol is a link-level export, not a
// Go-level API surface, so TinyGo still emits it into the final binary as
// long as this package is transitively imported by main (which the
// `widgets` package guarantees just by importing this one).
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
	} else if event.EventType == "hover" {
		if handler, ok := hoverHandlers[event.WidgetID]; ok {
			var payload hoverPayload
			if err := json.Unmarshal([]byte(event.Payload), &payload); err != nil {
				pdk.SetErrorString(err.Error())
				return 1
			}
			if err := handler(payload.Hovering); err != nil {
				pdk.SetErrorString(err.Error())
				return 1
			}
		}
	} else if event.EventType == "scroll" {
		if handler, ok := scrollHandlers[event.WidgetID]; ok {
			var payload scrollPayload
			if err := json.Unmarshal([]byte(event.Payload), &payload); err != nil {
				pdk.SetErrorString(err.Error())
				return 1
			}
			if err := handler(payload.ScrollOffsetX, payload.ScrollOffsetY); err != nil {
				pdk.SetErrorString(err.Error())
				return 1
			}
		}
	} else if event.EventType == "file_selected" {
		if handler, ok := fileSelectedHandlers[event.WidgetID]; ok {
			var payload fileSelectedPayload
			if err := json.Unmarshal([]byte(event.Payload), &payload); err != nil {
				pdk.SetErrorString(err.Error())
				return 1
			}
			if err := handler(payload.Paths); err != nil {
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
