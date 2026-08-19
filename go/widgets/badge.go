package widgets

import (
	"encoding/json"

	"github.com/extism/go-pdk"

	"natyv/sdk/widgets/internal"
)

//go:wasmimport extism:host/user natyv_clay_create_badge
func natyvClayCreateBadgeHost(uint64) uint64

// Badge is a small filled pill with centered text, purely decorative (no
// focus/click of its own). A dismissible tag is guest-composed from a
// Badge plus an adjacent Button (OnClick), same "guest composes it from
// primitives" precedent Breadcrumbs/Dialog already establish -- Badge
// itself has no dismiss button.
type Badge uint32

// BadgeTone is a small fixed set of semantic colors -- natyv has no way for
// a guest to pick an arbitrary widget color yet (every widget's palette is
// hardcoded host-side); see Badge.zig's own doc comment.
type BadgeTone string

const (
	BadgeTonePrimary BadgeTone = "primary"
	BadgeToneSuccess BadgeTone = "success"
	BadgeToneWarning BadgeTone = "warning"
	BadgeToneDanger  BadgeTone = "danger"
	BadgeToneNeutral BadgeTone = "neutral"
)

func CreateBadge(layout Layout, tone BadgeTone, label string) (Badge, error) {
	body, err := json.Marshal(struct {
		Layout Layout    `json:"layout"`
		Tone   BadgeTone `json:"tone"`
		Label  string    `json:"label"`
	}{layout, tone, label})
	if err != nil {
		return 0, err
	}
	resp, err := internal.DecodeWidgetResponse(natyvClayCreateBadgeHost(pdk.ResultBytes(body)))
	if err != nil {
		return 0, err
	}
	return Badge(resp.WidgetID), nil
}

func (bd Badge) SetLabel(label string) error { return internal.SetText(uint32(bd), label) }
func (bd Badge) Label() (string, error)      { return internal.GetText(uint32(bd)) }
func (bd Badge) Destroy()                    { internal.DestroyWidget(uint32(bd)) }
