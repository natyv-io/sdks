package widgets

import (
	"encoding/json"

	"github.com/extism/go-pdk"

	"natyv/sdk/widgets/internal"
)

//go:wasmimport extism:host/user natyv_clay_create_progressbar
func natyvClayCreateProgressBarHost(uint64) uint64

type ProgressBar uint32

func CreateProgressBar(layout Layout, value float32) (ProgressBar, error) {
	body, err := json.Marshal(struct {
		Layout Layout  `json:"layout"`
		Value  float32 `json:"value"`
	}{layout, value})
	if err != nil {
		return 0, err
	}
	resp, err := internal.DecodeWidgetResponse(natyvClayCreateProgressBarHost(pdk.ResultBytes(body)))
	if err != nil {
		return 0, err
	}
	return ProgressBar(resp.WidgetID), nil
}

func (p ProgressBar) Value() (float32, error)      { return internal.GetValue(uint32(p)) }
func (p ProgressBar) SetValue(value float32) error { return internal.SetValue(uint32(p), value) }
func (p ProgressBar) Destroy()                     { internal.DestroyWidget(uint32(p)) }
