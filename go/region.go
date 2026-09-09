// Rebuildable regions -- the memory-recycle ergonomics layer's real
// mechanism for widget/handler state (see the plan's own "Mechanism B"
// section). RegisterRebuildFunc/SetActiveRegion/SnapshotRegions/
// RestoreRegions below wrap one package-level region.Registry (see
// region/region.go for the real logic, its own doc comment on why it's a
// separate package, and Restore's own doc comment for the real
// make-before-break swap sequence), injecting the real widgets.Container
// primitives the swap needs. Package-level state persists across calls
// into the same plugin instance, same as widgets/internal's own
// event-handler tables -- registrations made during natyv_init are still
// there when a later natyv_checkpoint/natyv_resume call needs them.
package natyv

import (
	"encoding/json"

	"github.com/natyv-io/sdks/go/region"
	"github.com/natyv-io/sdks/go/widgets"
)

var regionRegistry = region.NewRegistry(
	func(parentID uint32, content region.ContentLayout) (uint32, error) {
		layout := widgets.ParentID(parentID)
		layout.Sizing = widgets.Sizing{Width: widgets.Grow(), Height: widgets.Grow()}
		layout.Direction = content.Direction
		layout.Padding = widgets.Padding{
			Left:   content.PaddingLeft,
			Right:  content.PaddingRight,
			Top:    content.PaddingTop,
			Bottom: content.PaddingBottom,
		}
		layout.ChildGap = content.ChildGap
		layout.ChildAlignment = widgets.Alignment{X: content.ChildAlignX, Y: content.ChildAlignY}
		layout.ScrollVertical = content.ScrollVertical
		layout.ScrollHorizontal = content.ScrollHorizontal
		child, err := widgets.CreateContainer(layout, false, 0)
		if err != nil {
			return 0, err
		}
		if err := child.SetVisible(false); err != nil {
			return 0, err
		}
		return uint32(child), nil
	},
	func(id uint32, visible bool) error {
		return widgets.Container(id).SetVisible(visible)
	},
	func(id uint32) {
		widgets.Container(id).Destroy()
	},
	func(parentID, exceptID uint32) error {
		return widgets.Container(parentID).DestroyChildrenExcept(widgets.Container(exceptID))
	},
)

// RegisterRebuildFunc names a function so a later natyv_resume can call it
// again by name via RestoreRegions. Call this once, e.g. during
// natyv_init -- registering the same name twice replaces the earlier
// registration. fn's own parent argument is whatever container it should
// actually build into -- during a resume-triggered rebuild this is a
// hidden staging container, not necessarily the region's own real stable
// parent (see RestoreRegions/region.Registry.Restore's own doc comment) --
// fn should simply build into whatever parent it's given, the same way it
// would for a normal in-app rebuild.
func RegisterRebuildFunc(name string, fn func(parent widgets.Container, args json.RawMessage) error) {
	regionRegistry.RegisterRebuildFunc(name, func(parentID uint32, args json.RawMessage) error {
		return fn(widgets.Container(parentID), args)
	})
}

// SetActiveRegion records the current recipe for reconstructing whatever
// is inside parent's subtree: on resume, RestoreRegions will call fn
// (already registered under funcName via RegisterRebuildFunc) with args to
// build a fresh replacement before parent's current children are touched
// at all. Call this every time the app rebuilds this region normally --
// exactly where it already does its own "I just rebuilt this" bookkeeping
// today (e.g. right where a real app updates its own view cache after a
// render). args must be JSON-marshalable; this only serializes it, it
// never inspects it.
//
// content should be the same Layout the caller just used to build its own
// region content (the one passed to widgets.ParentID(...) for the
// content's own top-level children) -- only the fields that actually
// determine child arrangement are kept (Direction/Padding/ChildGap/
// ChildAlignment/Scroll*); ParentID/Sizing/Floating/Modal/Toast are
// ignored, since Restore's own staging container always controls those
// itself. Passing a zero-value Layout{} reproduces the old, buggy
// left_to_right-only behavior -- see region.ContentLayout's own doc
// comment for the real bug this parameter exists to fix.
func SetActiveRegion(parent widgets.Container, funcName string, args any, content widgets.Layout) error {
	return regionRegistry.SetActiveRegion(uint32(parent), funcName, args, region.ContentLayout{
		Direction:        content.Direction,
		PaddingLeft:      content.Padding.Left,
		PaddingRight:     content.Padding.Right,
		PaddingTop:       content.Padding.Top,
		PaddingBottom:    content.Padding.Bottom,
		ChildGap:         content.ChildGap,
		ChildAlignX:      content.ChildAlignment.X,
		ChildAlignY:      content.ChildAlignment.Y,
		ScrollVertical:   content.ScrollVertical,
		ScrollHorizontal: content.ScrollHorizontal,
	})
}

// SnapshotRegions serializes every currently-active region's recipe --
// called from natyv_checkpoint (generated) alongside whatever plain data
// the app also persists.
func SnapshotRegions() (json.RawMessage, error) {
	return regionRegistry.Snapshot()
}

// RestoreRegions rebuilds every region SnapshotRegions captured. Called
// from natyv_resume. See region.Registry.Restore's own doc comment for
// the full behavior (the make-before-break swap sequence, why an
// unregistered function name is a hard error, why destroying every other
// current child rather than just the ones this registry tracks is
// deliberate).
func RestoreRegions(data json.RawMessage) error {
	return regionRegistry.Restore(data)
}
