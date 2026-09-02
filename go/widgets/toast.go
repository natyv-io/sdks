package widgets

// ToastStack is a persistent, corner-anchored container for transient,
// auto-dismissing messages -- individual toasts are plain (non-floating)
// children of it, each carrying its own durationMs (see CreateContainer's
// own doc comment), expiring on their own via the host's per-frame expiry
// sweep. Needs no primitive of its own: Container's Toast flag (anchoring)
// and its durationMs param (auto-expiry) already exist host-side.
//
// Productized from the fixture's own W7 demo.
type ToastStack struct {
	container Container
}

// CreateToastStack creates the persistent stack container (Toast: true) --
// create once, reuse for every toast shown afterward via Show. Unlike every
// on-demand floating panel elsewhere in this SDK, the stack itself is never
// destroyed/recreated -- only the individual toasts inside it come and go.
func CreateToastStack(childGap uint16) (*ToastStack, error) {
	c, err := CreateContainer(Layout{
		Sizing:    Sizing{Width: Fit(), Height: Fit()},
		ChildGap:  childGap,
		Direction: TopToBottom,
		Toast:     true,
	}, false, 0)
	if err != nil {
		return nil, err
	}
	return &ToastStack{container: c}, nil
}

// Show creates one new toast: a filled, durationMs Container parented to
// the stack, holding a message Label. Nothing here ever destroys it -- the
// host's own per-frame expiry sweep does that once durationMs elapses,
// cascading to the message Label too.
func (t *ToastStack) Show(width, height float32, durationMs uint32, message string) error {
	stackID := uint32(t.container)
	toast, err := CreateContainer(Layout{
		ParentID: &stackID,
		Sizing:   Sizing{Width: Fixed(width), Height: Fixed(height)},
		Padding:  Padding{Left: 8, Right: 8, Top: 6, Bottom: 6},
	}, true, durationMs)
	if err != nil {
		return err
	}
	toastID := uint32(toast)
	_, err = CreateLabel(Layout{
		ParentID: &toastID,
		Sizing:   Sizing{Width: Grow(), Height: Grow()},
	}, message)
	return err
}

// ID is the widget id `.ntx`'s own `styles={...}`/`ref={...}` codegen
// targets, since ToastStack (unlike Container/Button/...) isn't itself a
// uint32-based type -- see ntx/Codegen.zig's `isStructBackedWidgetKind`.
func (t *ToastStack) ID() uint32 { return uint32(t.container) }

// Destroy destroys the stack container -- any still-live (not yet expired)
// toasts, and their own message Labels, are real Clay descendants of it,
// so this cascades through them too, a clean teardown regardless of
// whether anything inside is still showing.
func (t *ToastStack) Destroy() { t.container.Destroy() }
