package natyv

import "github.com/extism/go-pdk"

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

type dispatchEvent struct {
	WidgetID  uint32 `json:"widget_id"`
	EventType string `json:"event_type"`
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
	}

	if err := pdk.OutputJSON(map[string]string{}); err != nil {
		pdk.SetErrorString(err.Error())
		return 1
	}
	return 0
}
