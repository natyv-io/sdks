package widgets

import (
	"encoding/json"

	"github.com/extism/go-pdk"

	"github.com/natyv-io/sdks/go/widgets/internal"
)

//go:wasmimport extism:host/user natyv_clay_create_label
func natyvClayCreateLabelHost(uint64) uint64

type Label uint32

func CreateLabel(layout Layout, text string) (Label, error) {
	body, err := json.Marshal(struct {
		Layout Layout `json:"layout"`
		Text   string `json:"text"`
	}{layout, text})
	if err != nil {
		return 0, err
	}
	resp, err := internal.DecodeWidgetResponse(natyvClayCreateLabelHost(pdk.ResultBytes(body)))
	if err != nil {
		return 0, err
	}
	return Label(resp.WidgetID), nil
}

func (l Label) SetText(text string) error { return internal.SetText(uint32(l), text) }
func (l Label) Text() (string, error)     { return internal.GetText(uint32(l)) }
func (l Label) Destroy()                  { internal.DestroyWidget(uint32(l)) }

// WrapLabel returns a typed handle for a host-assigned widget id the guest
// didn't just create -- needed by natyv_resume (natyv-core's memory-
// reclamation recycle mechanism), which reattaches to a pre-existing
// on-screen Label whose id came from a natyv_checkpoint blob, not a fresh
// CreateLabel call. Does no host-side validation itself, matching every
// other typed handle in this package (they're all just a widget_id
// underneath) -- behaves exactly like a real Label if id names one still
// alive host-side, and every method call fails with the host's own "no
// such widget" error otherwise.
func WrapLabel(id uint32) Label {
	return Label(id)
}
