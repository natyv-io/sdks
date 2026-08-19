package widgets

import (
	"encoding/json"

	"github.com/extism/go-pdk"

	"natyv/sdk/widgets/internal"
)

//go:wasmimport extism:host/user natyv_clay_create_divider
func natyvClayCreateDividerHost(uint64) uint64

// Divider is a thin visual rule, purely decorative (no text, no focus, no
// events). Horizontal vs. vertical is entirely a function of the Layout
// sizing given here, not a separate field -- see Divider.zig's doc comment
// on the host side.
type Divider uint32

func CreateDivider(layout Layout) (Divider, error) {
	body, err := json.Marshal(struct {
		Layout Layout `json:"layout"`
	}{layout})
	if err != nil {
		return 0, err
	}
	resp, err := internal.DecodeWidgetResponse(natyvClayCreateDividerHost(pdk.ResultBytes(body)))
	if err != nil {
		return 0, err
	}
	return Divider(resp.WidgetID), nil
}

func (d Divider) Destroy() { internal.DestroyWidget(uint32(d)) }
