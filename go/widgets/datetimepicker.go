package widgets

import (
	"fmt"
	"time"
)

// DateTimePicker is a "trigger + floating panel" widget -- mechanically the
// same click-to-open/click-away-to-close composition Dropdown/Menu already
// establish, just with a bigger, more structured panel body (a month-nav
// header row, a weekday-label row, a day grid, an hour/minute time row)
// instead of a flat option list. Needs no primitive of its own.
//
// Calendar math lives entirely here in the guest, not the host -- natyv has
// no date/time capability and shouldn't grow one just for this, Go's own
// time package already computes day-of-week/month-length/leap-year
// correctly.
//
// Productized from the fixture's own W16 demo. renderGrid rebuilds only the
// day-grid portion (not the whole panel) on month nav -- narrower than a
// full-panel rebuild, which caused a visible flash a real click-through
// caught.
type DateTimePicker struct {
	trigger                        Button
	panel                          Container
	headerLabel                    Label
	prevBtn, nextBtn               Button
	headerRow, weekdayRow, timeRow Container
	weekdayLabels                  []Label
	gridRows                       []Container
	spacers                        []Container
	dayButtons                     []Button
	dayButtonDays                  []int
	hourStepper, minuteStepper     NumericStepper

	year, month, hour, minute int
	onSelect                  func(year, month, day, hour, minute int) error
}

// CreateDateTimePicker creates the trigger Button only -- the floating
// panel is created on demand by a real click. year/month/hour/minute seed
// the picker's own live (uncommitted) state; nothing is picked until
// OnSelect fires.
func CreateDateTimePicker(layout Layout, triggerLabel string, year, month, hour, minute int) (*DateTimePicker, error) {
	trigger, err := CreateButton(layout, triggerLabel)
	if err != nil {
		return nil, err
	}
	p := &DateTimePicker{trigger: trigger, year: year, month: month, hour: hour, minute: minute}
	trigger.OnClick(func() error {
		if p.panel == 0 {
			return p.open()
		}
		return nil
	})
	trigger.OnBlur(p.onBlur)
	return p, nil
}

// OnSelect registers the handler fired once a day is actually picked
// (commits year/month/day/hour/minute) -- the trigger's own label is
// already updated to the picked "YYYY-MM-DD HH:MM" value before this fires,
// same "trigger relabels itself" convention Dropdown already establishes.
func (p *DateTimePicker) OnSelect(handler func(year, month, day, hour, minute int) error) {
	p.onSelect = handler
}

// dtpDaysInMonth returns how many days the given month has -- pure calendar
// math (constructing day 0 of the *next* month rolls back to the last day
// of the target month, a standard Go idiom), not a lookup table, so leap
// years fall out for free.
func dtpDaysInMonth(year, month int) int {
	return time.Date(year, time.Month(month)+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

// dtpFirstWeekday returns which column (0=Sunday..6=Saturday) the 1st of
// the given month falls on -- how many leading blank spacer cells the grid
// needs before day 1.
func dtpFirstWeekday(year, month int) int {
	return int(time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC).Weekday())
}

// isOwnWidget reports whether id names a widget that currently belongs to
// the open picker -- the trigger, the (stable across re-renders) nav/
// stepper buttons, or one of the grid's current day buttons. Used by every
// picker-owned widget's own OnBlur to tell "focus moved to something else
// *inside* the still-open picker" (don't close) apart from a real
// click-away (close).
func (p *DateTimePicker) isOwnWidget(id uint32) bool {
	if id == uint32(p.trigger) || id == uint32(p.prevBtn) || id == uint32(p.nextBtn) ||
		id == uint32(p.hourStepper) || id == uint32(p.minuteStepper) {
		return true
	}
	for _, b := range p.dayButtons {
		if id == uint32(b) {
			return true
		}
	}
	return false
}

func (p *DateTimePicker) onBlur(newFocusID uint32) error {
	if p.isOwnWidget(newFocusID) {
		return nil
	}
	return p.close()
}

// destroyGrid tears down just the day-grid portion (rows, leading blank
// spacers, day buttons) -- not the panel, header, weekday labels, or time
// row, which never need touching once the panel's first opened. dayButtons
// and spacers are real Clay children of their own gridRow, so destroying
// each row alone cascades through both -- only gridRows itself needs an
// explicit loop, since the rows aren't descendants of one another. Safe to
// call with nothing built yet. Used by renderGrid (destroy before
// repopulating for a new month) -- close() no longer needs it, since
// destroying panel already cascades through the grid too.
func (p *DateTimePicker) destroyGrid() {
	for _, r := range p.gridRows {
		r.Destroy()
	}
	p.dayButtons = nil
	p.dayButtonDays = nil
	p.spacers = nil
	p.gridRows = nil
}

// close tears down the entire open panel -- the grid, every weekday header,
// the nav/time controls, and the panel itself. headerRow/weekdayRow/
// timeRow/gridRows are all real Clay children of panel (and everything
// each of them holds -- prevBtn/nextBtn/headerLabel/weekdayLabels/
// hourStepper/minuteStepper/dayButtons/spacers -- is in turn a descendant
// of one of those), so destroying panel alone cascades through the entire
// tree. Safe to call when nothing is open.
func (p *DateTimePicker) close() error {
	if p.panel == 0 {
		return nil
	}
	p.dayButtons = nil
	p.dayButtonDays = nil
	p.spacers = nil
	p.gridRows = nil
	p.weekdayLabels = nil
	p.panel.Destroy()
	p.panel = 0
	return nil
}

// shiftMonth moves the picker's own live month/year state by delta (+-1,
// wrapping across a year boundary) and rebuilds just the grid -- shared by
// the prev/next Buttons' OnClick and their own Left/Right OnKeyNav.
func (p *DateTimePicker) shiftMonth(delta int) error {
	p.month += delta
	if p.month < 1 {
		p.month = 12
		p.year--
	} else if p.month > 12 {
		p.month = 1
		p.year++
	}
	return p.renderGrid()
}

// renderGrid destroys and rebuilds just the day-grid portion against the
// current year/month, and updates the header label's text to match --
// called both by open (the initial population) and shiftMonth.
func (p *DateTimePicker) renderGrid() error {
	p.destroyGrid()
	if err := p.headerLabel.SetText(fmt.Sprintf("%s %d", time.Month(p.month).String(), p.year)); err != nil {
		return err
	}

	// Leading blank spacers for firstWeekday, then one real day button per
	// day, 7 cells per row. No trailing spacers -- a short last row is
	// normal and not misaligned.
	total := dtpDaysInMonth(p.year, p.month)
	leading := dtpFirstWeekday(p.year, p.month)
	panelID := uint32(p.panel)
	var row Container
	col := 0
	for cell := 0; cell < leading+total; cell++ {
		if col == 0 {
			r, err := CreateContainer(Layout{
				ParentID:  &panelID,
				Sizing:    Sizing{Width: Grow(), Height: Fixed(34)},
				Direction: LeftToRight,
				ChildGap:  2,
			}, false, 0)
			if err != nil {
				return err
			}
			row = r
			p.gridRows = append(p.gridRows, row)
		}
		rowID := uint32(row)
		if cell < leading {
			spacer, err := CreateContainer(Layout{
				ParentID: &rowID,
				Sizing:   Sizing{Width: Fixed(34), Height: Fixed(34)},
			}, false, 0)
			if err != nil {
				return err
			}
			p.spacers = append(p.spacers, spacer)
		} else {
			day := cell - leading + 1
			btn, err := CreateButton(Layout{
				ParentID: &rowID,
				Sizing:   Sizing{Width: Fixed(34), Height: Fixed(34)},
			}, fmt.Sprintf("%d", day))
			if err != nil {
				return err
			}
			p.dayButtons = append(p.dayButtons, btn)
			p.dayButtonDays = append(p.dayButtonDays, day)
			btn.OnBlur(p.onBlur)
			idx := len(p.dayButtons) - 1
			btn.OnClick(func() error { return p.selectDay(p.dayButtonDays[idx]) })
		}
		col++
		if col == 7 {
			col = 0
		}
	}
	return nil
}

// open creates the panel and everything in it -- header row, weekday
// labels, day grid, and time steppers -- called exactly once, when the
// panel first opens. Never called again while it's still open (see
// renderGrid's own doc comment).
func (p *DateTimePicker) open() error {
	triggerID := uint32(p.trigger)
	panel, err := CreateContainer(Layout{
		// 270 = 7*34 + 6*2 gaps + 16 padding, matching the 34px grid columns
		// below -- weekday abbreviations/day numbers measure wider than a
		// 30px column at Inter's 16pt default, so a narrower box forces
		// mid-word wraps.
		ParentID:  &triggerID,
		Sizing:    Sizing{Width: Fixed(270), Height: Fit()},
		Padding:   Padding{Left: 8, Right: 8, Top: 6, Bottom: 6},
		ChildGap:  4,
		Direction: TopToBottom,
		Floating:  true,
	}, true, 0)
	if err != nil {
		return err
	}
	p.panel = panel
	panelID := uint32(panel)

	// Header row: "‹" prev, month/year label, "›" next. Created once --
	// only the label's own text ever changes again (via renderGrid, on
	// month nav), never these three widgets themselves.
	headerRow, err := CreateContainer(Layout{
		ParentID:  &panelID,
		Sizing:    Sizing{Width: Grow(), Height: Fixed(24)},
		Direction: LeftToRight,
		ChildGap:  4,
	}, false, 0)
	if err != nil {
		return err
	}
	p.headerRow = headerRow
	headerRowID := uint32(headerRow)

	prevBtn, err := CreateButton(Layout{
		ParentID: &headerRowID,
		Sizing:   Sizing{Width: Fixed(24), Height: Fixed(24)},
	}, "‹")
	if err != nil {
		return err
	}
	p.prevBtn = prevBtn
	prevBtn.OnClick(func() error { return p.shiftMonth(-1) })
	prevBtn.OnBlur(p.onBlur)
	prevBtn.OnKeyNav(func(key string) error {
		if key == "left" {
			return p.shiftMonth(-1)
		}
		return nil
	})

	headerLabel, err := CreateLabel(Layout{
		ParentID: &headerRowID,
		Sizing:   Sizing{Width: Grow(), Height: Fixed(24)},
	}, fmt.Sprintf("%s %d", time.Month(p.month).String(), p.year))
	if err != nil {
		return err
	}
	p.headerLabel = headerLabel

	nextBtn, err := CreateButton(Layout{
		ParentID: &headerRowID,
		Sizing:   Sizing{Width: Fixed(24), Height: Fixed(24)},
	}, "›")
	if err != nil {
		return err
	}
	p.nextBtn = nextBtn
	nextBtn.OnClick(func() error { return p.shiftMonth(1) })
	nextBtn.OnBlur(p.onBlur)
	nextBtn.OnKeyNav(func(key string) error {
		if key == "right" {
			return p.shiftMonth(1)
		}
		return nil
	})

	// Weekday header row -- 34px columns, matching the day buttons/spacers
	// below so everything lines up vertically. Never touched again --
	// "Su Mo Tu..." doesn't change per month.
	weekdayRow, err := CreateContainer(Layout{
		ParentID:  &panelID,
		Sizing:    Sizing{Width: Grow(), Height: Fixed(18)},
		Direction: LeftToRight,
		ChildGap:  2,
	}, false, 0)
	if err != nil {
		return err
	}
	p.weekdayRow = weekdayRow
	weekdayRowID := uint32(weekdayRow)
	for _, name := range [7]string{"Su", "Mo", "Tu", "We", "Th", "Fr", "Sa"} {
		l, err := CreateLabel(Layout{
			ParentID: &weekdayRowID,
			Sizing:   Sizing{Width: Fixed(34), Height: Fixed(18)},
		}, name)
		if err != nil {
			return err
		}
		p.weekdayLabels = append(p.weekdayLabels, l)
	}

	if err := p.renderGrid(); err != nil {
		return err
	}

	// Time row: hour stepper, minute stepper -- two real NumericStepper
	// widgets. No separate ":" widget; the picked value's own "HH:MM"
	// formatting (selectDay) supplies that instead. Created once -- their
	// own displayed value updates via SetValue on a real change, the
	// widgets themselves never get destroyed/recreated on a time change.
	timeRow, err := CreateContainer(Layout{
		ParentID:  &panelID,
		Sizing:    Sizing{Width: Grow(), Height: Fixed(24)},
		Direction: LeftToRight,
		ChildGap:  4,
	}, false, 0)
	if err != nil {
		return err
	}
	p.timeRow = timeRow
	timeRowID := uint32(timeRow)

	hourStepper, err := CreateNumericStepper(Layout{
		ParentID: &timeRowID,
		Sizing:   Sizing{Width: Fixed(90), Height: Fixed(24)},
	}, int32(p.hour), 0, 23, 1, true)
	if err != nil {
		return err
	}
	p.hourStepper = hourStepper
	hourStepper.OnChange(func(value int32) error { p.hour = int(value); return nil })
	hourStepper.OnBlur(p.onBlur)

	// Minute's logical range is the full 0-59 minute-of-hour (not 0-55) --
	// wrapping is computed modulo max-min+1, so a max of 55 would wrap
	// modulo 56 instead of the real 60 a step-of-5 minute needs. The
	// stepper only ever lands on multiples of 5 in practice, the wider
	// logical range just makes the wrap math correct.
	minuteStepper, err := CreateNumericStepper(Layout{
		ParentID: &timeRowID,
		Sizing:   Sizing{Width: Fixed(90), Height: Fixed(24)},
	}, int32(p.minute), 0, 59, 5, true)
	if err != nil {
		return err
	}
	p.minuteStepper = minuteStepper
	minuteStepper.OnChange(func(value int32) error { p.minute = int(value); return nil })
	minuteStepper.OnBlur(p.onBlur)

	return nil
}

// selectDay commits {year, month, day, hour, minute} as the picked value,
// updates the trigger's own label, and closes the panel entirely -- one
// click both selects and closes, same precedent Dropdown's option click
// already establishes, no separate "Done" button needed.
func (p *DateTimePicker) selectDay(day int) error {
	if err := p.trigger.SetLabel(fmt.Sprintf("%04d-%02d-%02d %02d:%02d", p.year, p.month, day, p.hour, p.minute)); err != nil {
		return err
	}
	year, month, hour, minute := p.year, p.month, p.hour, p.minute
	if err := p.close(); err != nil {
		return err
	}
	if p.onSelect != nil {
		return p.onSelect(year, month, day, hour, minute)
	}
	return nil
}

// ID is the widget id `.ntx`'s own `styles={...}`/`ref={...}` codegen
// targets, since DateTimePicker (unlike Container/Button/...) isn't
// itself a uint32-based type -- see ntx/Codegen.zig's
// `isStructBackedWidgetKind`.
func (p *DateTimePicker) ID() uint32 { return uint32(p.trigger) }

// Destroy destroys the trigger -- panel (if open) and everything under it
// are real Clay descendants of it, so this alone cascades through
// everything, no need to call close() first.
func (p *DateTimePicker) Destroy() {
	p.trigger.Destroy()
}
