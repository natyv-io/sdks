package widgets

import (
	"encoding/json"
	"errors"

	"github.com/extism/go-pdk"

	"github.com/natyv-io/sdks/go/widgets/internal"
	"github.com/natyv-io/sdks/go/widgets/internal/drawing"
)

//go:wasmimport extism:host/user natyv_clay_create_canvas
func natyvClayCreateCanvasHost(uint64) uint64

//go:wasmimport extism:host/user natyv_canvas_set
func natyvCanvasSetHost(uint64) uint64

// Canvas is a widget the guest draws into: lines, shapes, arcs and text,
// for charts and diagrams. The drawing is a retained list the host keeps
// and redraws from, so it costs nothing per frame and survives an instance
// recycle untouched -- Draw replaces the whole list, and there is no
// per-frame draw callback.
//
// Size it with Fixed, Grow or Percent: a canvas has no content to Fit to.
// Its laid-out size arrives through OnResize, first on its initial layout
// and again whenever it changes; redraw to fit there.
type Canvas uint32

func CreateCanvas(layout Layout) (Canvas, error) {
	body, err := json.Marshal(struct {
		Layout Layout `json:"layout"`
	}{layout})
	if err != nil {
		return 0, err
	}
	resp, err := internal.DecodeWidgetResponse(natyvClayCreateCanvasHost(pdk.ResultBytes(body)))
	if err != nil {
		return 0, err
	}
	return Canvas(resp.WidgetID), nil
}

// Point is an {x, y} pair in a canvas's logical pixels, from its top-left.
type Point = drawing.Point

// ShapeStyle paints a closed shape (Rect, Circle, Polygon, Arc). Set Fill,
// Stroke or both; the host rejects a shape with neither. A zero
// StrokeWidth means 1.
type ShapeStyle struct {
	Fill        *Color
	Stroke      *Color
	StrokeWidth float32
}

func (s ShapeStyle) toDrawing() drawing.Style {
	return drawing.Style{Fill: (*drawing.Color)(s.Fill), Stroke: (*drawing.Color)(s.Stroke), StrokeWidth: s.StrokeWidth}
}

// Drawing builds a Canvas's command list inside Draw. Commands draw in
// order, later on top. Coordinates are logical pixels from the canvas's
// top-left; angles are radians, clockwise from +x.
type Drawing struct {
	d drawing.Drawing
}

func (d *Drawing) Line(x1, y1, x2, y2, width float32, color Color) {
	d.d.Line(x1, y1, x2, y2, width, drawing.Color(color))
}

func (d *Drawing) Polyline(points []Point, width float32, color Color) {
	d.d.Polyline(points, width, drawing.Color(color))
}

func (d *Drawing) Rect(x, y, w, h float32, style ShapeStyle) {
	d.d.Rect(x, y, w, h, 0, style.toDrawing())
}

func (d *Drawing) RoundedRect(x, y, w, h, radius float32, style ShapeStyle) {
	d.d.Rect(x, y, w, h, radius, style.toDrawing())
}

func (d *Drawing) Circle(cx, cy, r float32, style ShapeStyle) {
	d.d.Circle(cx, cy, r, style.toDrawing())
}

func (d *Drawing) Polygon(points []Point, style ShapeStyle) {
	d.d.Polygon(points, style.toDrawing())
}

// Arc is a pie wedge when filled and an arc segment when stroked.
func (d *Drawing) Arc(cx, cy, r, start, end float32, style ShapeStyle) {
	d.d.Arc(cx, cy, r, start, end, style.toDrawing())
}

// Text draws s in the app's font with y at the top of the line box. align
// is AlignXLeft, AlignXCenter or AlignXRight, anchored at x; "" means left.
func (d *Drawing) Text(x, y float32, s string, color Color, align string) {
	d.d.Text(x, y, s, drawing.Color(color), align)
}

// Draw replaces the canvas's whole drawing with what fn builds, in one host
// call. If the host rejects any command, it keeps the previous drawing and
// the error names the offending command's index.
func (c Canvas) Draw(fn func(d *Drawing)) error {
	var d Drawing
	fn(&d)
	body, err := d.d.Encode(uint32(c))
	if err != nil {
		return err
	}
	var resp struct {
		Error string `json:"error,omitempty"`
	}
	if err := json.Unmarshal(pdk.ParamBytes(natyvCanvasSetHost(pdk.ResultBytes(body))), &resp); err != nil {
		return err
	}
	if resp.Error != "" {
		return errors.New(resp.Error)
	}
	return nil
}

// OnClick's x and y are where the click landed, relative to the canvas's
// top-left -- enough to tell which bar or node was hit.
func (c Canvas) OnClick(handler func(x, y float32) error) {
	internal.RegisterCanvasClick(uint32(c), handler)
}

func (c Canvas) OnResize(handler func(w, h float32) error) {
	internal.RegisterCanvasResize(uint32(c), handler)
}

func (c Canvas) Destroy() { internal.DestroyWidget(uint32(c)) }

// WrapCanvas returns a typed handle for a host-assigned widget id the
// guest didn't just create -- see widgets.WrapLabel's own doc comment for
// the real use case and caveats. The drawing itself survives a recycle
// host-side; only OnClick/OnResize need registering again.
func WrapCanvas(id uint32) Canvas {
	return Canvas(id)
}
