package widgets

import (
	"encoding/json"

	"github.com/extism/go-pdk"

	"natyv/sdk/widgets/internal"
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
