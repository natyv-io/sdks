package widgets

// CreatePanel creates a Container with its background+border fill on --
// the same visual every floating panel in this SDK (Dropdown, Modal,
// Popover, Tooltip, the Combobox/Date-picker panels, ...) already uses via
// CreateContainer's own `background` param, just declared as a normal
// (non-floating) layout element instead of an overlay. Not a new type --
// Container already has everything a panel needs (OnDismiss, SetHeight,
// Destroy), so this is purely a naming convenience over
// CreateContainer(layout, true, 0), same "zero new host mechanism, pure
// guest-side convenience" shape this SDK's own pre-implementation notes on
// Card/Panel called for.
func CreatePanel(layout Layout) (Container, error) {
	return CreateContainer(layout, true, 0)
}
