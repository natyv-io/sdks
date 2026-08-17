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

type dispatchEvent struct {
	WidgetID  uint32 `json:"widget_id"`
	EventType string `json:"event_type"`
	// W3: absent/empty for "click" (never carried a payload before this),
	// populated for "change" -- always a JSON-encoded string regardless of
	// what's inside it, per Dispatch.zig's buildDispatchPayload on the host
	// side, so this needs a second Unmarshal to reach the actual value.
	Payload string `json:"payload"`
}

type changePayload struct {
	Value float32 `json:"value"`
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
	}

	if err := pdk.OutputJSON(map[string]string{}); err != nil {
		pdk.SetErrorString(err.Error())
		return 1
	}
	return 0
}
