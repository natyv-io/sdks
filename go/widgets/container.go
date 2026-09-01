package widgets

import (
	"encoding/json"

	"github.com/extism/go-pdk"

	"github.com/natyv-io/sdks/go/widgets/internal"
)

//go:wasmimport extism:host/user natyv_clay_create_container
func natyvClayCreateContainerHost(uint64) uint64

// Container is a layout-only Clay node -- position/size only, nothing
// drawn unless `background` is set -- used to group other widgets under a
// shared direction/padding/gap/alignment. See host-side widgets/
// Container.zig. Unlike every other widget kind, Container has no plain
// (non-Clay) equivalent -- layout-only grouping nodes only make sense in a
// Clay-managed tree.
type Container uint32

// background is a fixed host-drawn panel fill/border, not a guest-chosen
// color -- see WidgetHost.zig's ClayContainerRequest doc comment for why
// this isn't the start of a guest-controllable styling system. Pass false
// for a plain layout-only container. durationMs is 0 for a Container that
// never expires -- see WidgetHost.zig's Slot.expires_at_ms doc comment.
// Non-zero destroys this Container (and every descendant -- host-driven
// expiry cascades, unlike Destroy(), which stays explicit-per-child-only)
// automatically once that many milliseconds have passed, no guest polling/
// timer needed.
func CreateContainer(layout Layout, background bool, durationMs uint32) (Container, error) {
	body, err := json.Marshal(struct {
		Layout     Layout `json:"layout"`
		Background bool   `json:"background"`
		DurationMs uint32 `json:"duration_ms"`
	}{layout, background, durationMs})
	if err != nil {
		return 0, err
	}
	resp, err := internal.DecodeWidgetResponse(natyvClayCreateContainerHost(pdk.ResultBytes(body)))
	if err != nil {
		return 0, err
	}
	return Container(resp.WidgetID), nil
}

// OnDismiss registers the handler for a modal's "please close" request --
// fired on Escape or a backdrop click while this Container is the topmost
// open modal (Layout.Modal was true at CreateContainer time). The host
// never force-closes -- the handler decides whether to actually destroy the
// modal's subtree, same as any other guest-owned destroy/recreate widget.
// Only meaningful on a modal root -- registering it on a plain Container is
// harmless but never fires, since only a modal root's own widget id is
// ever the target of a `.dismiss` event.
func (c Container) OnDismiss(handler func() error) { internal.RegisterDismiss(uint32(c), handler) }

// SetHeight resizes this Container in place to a Fixed height, regardless
// of whatever Sizing it was created with -- see internal.SetHeight's own
// doc comment. Width is untouched.
func (c Container) SetHeight(height float32) error { return internal.SetHeight(uint32(c), height) }

func (c Container) Destroy() { internal.DestroyWidget(uint32(c)) }
