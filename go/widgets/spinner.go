package widgets

import (
	"encoding/json"

	"github.com/extism/go-pdk"

	"github.com/natyv-io/sdks/go/widgets/internal"
)

//go:wasmimport extism:host/user natyv_clay_create_spinner
func natyvClayCreateSpinnerHost(uint64) uint64

// Spinner is a purely decorative loading indicator -- no interactivity, no
// guest-readable state, animates purely from wall-clock time on the host
// side (see Spinner.zig's own doc comment). Not focusable and not
// hit-testable -- there's genuinely nothing here for a guest to read or
// react to beyond creating and eventually destroying it, same minimal
// Create-and-Destroy-only shape Divider already has.
type Spinner uint32

func CreateSpinner(layout Layout) (Spinner, error) {
	body, err := json.Marshal(struct {
		Layout Layout `json:"layout"`
	}{layout})
	if err != nil {
		return 0, err
	}
	resp, err := internal.DecodeWidgetResponse(natyvClayCreateSpinnerHost(pdk.ResultBytes(body)))
	if err != nil {
		return 0, err
	}
	return Spinner(resp.WidgetID), nil
}

func (s Spinner) Destroy() { internal.DestroyWidget(uint32(s)) }
