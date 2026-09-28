// Package drawing is the Canvas widget's command builder, wire encoding and
// event payload decoding, split out from widgets for the same reason
// hid/internal/events is split from hid: widgets' host-function files use
// //go:wasmimport, which doesn't compile for a native target, and this
// logic deserves real native go test coverage.
//
// The wire format is natyv-core's (see core/src/CanvasStore.zig's header):
// each command is a single-key object, e.g. {"circle":{...}}. The host
// validates every command and rejects the whole drawing on the first bad
// one, so nothing here duplicates its limits -- a builder that checked a
// second copy of them could only ever drift out of step.
package drawing

import "encoding/json"

// Color matches widgets.Color field for field, so widgets converts between
// the two directly (Go ignores struct tags in conversions). It lives here
// too because this package can't import widgets.
type Color struct {
	R float32 `json:"r"`
	G float32 `json:"g"`
	B float32 `json:"b"`
	A float32 `json:"a"`
}

// Point is {x, y}, encoded as the host's [x, y] pair.
type Point [2]float32

// Style is a closed shape's paint. A nil Fill or Stroke is omitted; the
// host rejects a shape with neither. A zero StrokeWidth is omitted too, so
// the host's default of 1 applies.
type Style struct {
	Fill        *Color
	Stroke      *Color
	StrokeWidth float32
}

type line struct {
	X1    float32 `json:"x1"`
	Y1    float32 `json:"y1"`
	X2    float32 `json:"x2"`
	Y2    float32 `json:"y2"`
	Width float32 `json:"width"`
	Color Color   `json:"color"`
}

type polyline struct {
	Points []Point `json:"points"`
	Width  float32 `json:"width"`
	Color  Color   `json:"color"`
}

type rect struct {
	X           float32 `json:"x"`
	Y           float32 `json:"y"`
	W           float32 `json:"w"`
	H           float32 `json:"h"`
	Fill        *Color  `json:"fill,omitempty"`
	Stroke      *Color  `json:"stroke,omitempty"`
	StrokeWidth float32 `json:"stroke_width,omitempty"`
	Radius      float32 `json:"radius,omitempty"`
}

type circle struct {
	CX          float32 `json:"cx"`
	CY          float32 `json:"cy"`
	R           float32 `json:"r"`
	Fill        *Color  `json:"fill,omitempty"`
	Stroke      *Color  `json:"stroke,omitempty"`
	StrokeWidth float32 `json:"stroke_width,omitempty"`
}

type polygon struct {
	Points      []Point `json:"points"`
	Fill        *Color  `json:"fill,omitempty"`
	Stroke      *Color  `json:"stroke,omitempty"`
	StrokeWidth float32 `json:"stroke_width,omitempty"`
}

type arc struct {
	CX          float32 `json:"cx"`
	CY          float32 `json:"cy"`
	R           float32 `json:"r"`
	Start       float32 `json:"start"`
	End         float32 `json:"end"`
	Fill        *Color  `json:"fill,omitempty"`
	Stroke      *Color  `json:"stroke,omitempty"`
	StrokeWidth float32 `json:"stroke_width,omitempty"`
}

type text struct {
	X     float32 `json:"x"`
	Y     float32 `json:"y"`
	Text  string  `json:"text"`
	Color Color   `json:"color"`
	Align string  `json:"align,omitempty"`
}

// command is one wire command: exactly one field is set, so it encodes as
// a single-key object.
type command struct {
	Line     *line     `json:"line,omitempty"`
	Polyline *polyline `json:"polyline,omitempty"`
	Rect     *rect     `json:"rect,omitempty"`
	Circle   *circle   `json:"circle,omitempty"`
	Polygon  *polygon  `json:"polygon,omitempty"`
	Arc      *arc      `json:"arc,omitempty"`
	Text     *text     `json:"text,omitempty"`
}

// Drawing is an ordered list of commands; later ones draw on top.
type Drawing struct {
	commands []command
}

func (d *Drawing) Len() int { return len(d.commands) }

func (d *Drawing) Line(x1, y1, x2, y2, width float32, color Color) {
	d.commands = append(d.commands, command{Line: &line{x1, y1, x2, y2, width, color}})
}

// Polyline copies points, so the caller may reuse its slice.
func (d *Drawing) Polyline(points []Point, width float32, color Color) {
	d.commands = append(d.commands, command{Polyline: &polyline{copyPoints(points), width, color}})
}

func (d *Drawing) Rect(x, y, w, h, radius float32, style Style) {
	d.commands = append(d.commands, command{Rect: &rect{x, y, w, h, style.Fill, style.Stroke, style.StrokeWidth, radius}})
}

func (d *Drawing) Circle(cx, cy, r float32, style Style) {
	d.commands = append(d.commands, command{Circle: &circle{cx, cy, r, style.Fill, style.Stroke, style.StrokeWidth}})
}

// Polygon copies points, so the caller may reuse its slice.
func (d *Drawing) Polygon(points []Point, style Style) {
	d.commands = append(d.commands, command{Polygon: &polygon{copyPoints(points), style.Fill, style.Stroke, style.StrokeWidth}})
}

func (d *Drawing) Arc(cx, cy, r, start, end float32, style Style) {
	d.commands = append(d.commands, command{Arc: &arc{cx, cy, r, start, end, style.Fill, style.Stroke, style.StrokeWidth}})
}

// Text's align is "left", "center" or "right"; "" means left.
func (d *Drawing) Text(x, y float32, s string, color Color, align string) {
	d.commands = append(d.commands, command{Text: &text{x, y, s, color, align}})
}

// copyPoints never returns nil: a nil slice would encode as `null`, which
// the host rejects where it expects an array.
func copyPoints(points []Point) []Point {
	return append(make([]Point, 0, len(points)), points...)
}

// Encode returns the natyv_canvas_set request for canvas widgetID. A NaN
// or infinite coordinate fails here, since JSON can't represent it.
func (d *Drawing) Encode(widgetID uint32) ([]byte, error) {
	commands := d.commands
	if commands == nil {
		commands = []command{}
	}
	return json.Marshal(struct {
		WidgetID uint32    `json:"widget_id"`
		Commands []command `json:"commands"`
	}{widgetID, commands})
}

// ParseClick decodes a canvas `click` payload: `{"x":f,"y":f}`, relative
// to the canvas's top-left.
func ParseClick(payload string) (x, y float32, err error) {
	var p struct {
		X float32 `json:"x"`
		Y float32 `json:"y"`
	}
	err = json.Unmarshal([]byte(payload), &p)
	return p.X, p.Y, err
}

// ParseResize decodes a `canvas_resized` payload: `{"w":f,"h":f}`.
func ParseResize(payload string) (w, h float32, err error) {
	var p struct {
		W float32 `json:"w"`
		H float32 `json:"h"`
	}
	err = json.Unmarshal([]byte(payload), &p)
	return p.W, p.H, err
}
