package drawing

import (
	"math"
	"testing"
)

func encode(t *testing.T, d *Drawing) string {
	t.Helper()
	body, err := d.Encode(7)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func TestEveryCommandEncodesAsTheHostsSingleKeyObject(t *testing.T) {
	red := Color{1, 0, 0, 1}
	blue := Color{0, 0, 1, 0.5}
	var d Drawing
	d.Line(0, 1, 2, 3, 1.5, red)
	d.Polyline([]Point{{0, 0}, {5, 3}}, 2, red)
	d.Rect(0, 0, 10, 5, 2, Style{Fill: &red, Stroke: &blue, StrokeWidth: 3})
	d.Circle(5, 5, 3, Style{Fill: &red})
	d.Polygon([]Point{{0, 0}, {9, 0}, {4, 7}}, Style{Stroke: &blue})
	d.Arc(5, 5, 4, 0, 1.5, Style{Fill: &red})
	d.Text(1, 2, "Q3", blue, "center")

	want := `{"widget_id":7,"commands":[` +
		`{"line":{"x1":0,"y1":1,"x2":2,"y2":3,"width":1.5,"color":{"r":1,"g":0,"b":0,"a":1}}},` +
		`{"polyline":{"points":[[0,0],[5,3]],"width":2,"color":{"r":1,"g":0,"b":0,"a":1}}},` +
		`{"rect":{"x":0,"y":0,"w":10,"h":5,"fill":{"r":1,"g":0,"b":0,"a":1},"stroke":{"r":0,"g":0,"b":1,"a":0.5},"stroke_width":3,"radius":2}},` +
		`{"circle":{"cx":5,"cy":5,"r":3,"fill":{"r":1,"g":0,"b":0,"a":1}}},` +
		`{"polygon":{"points":[[0,0],[9,0],[4,7]],"stroke":{"r":0,"g":0,"b":1,"a":0.5}}},` +
		`{"arc":{"cx":5,"cy":5,"r":4,"start":0,"end":1.5,"fill":{"r":1,"g":0,"b":0,"a":1}}},` +
		`{"text":{"x":1,"y":2,"text":"Q3","color":{"r":0,"g":0,"b":1,"a":0.5},"align":"center"}}` +
		`]}`
	if got := encode(t, &d); got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
	if d.Len() != 7 {
		t.Fatalf("Len() = %d", d.Len())
	}
}

// The host parses `commands` and `points` as arrays; `null` would fail the
// whole drawing, so an empty drawing (clearing a canvas) and an empty point
// list must encode as [].
func TestEmptyDrawingAndEmptyPointsEncodeAsArraysNotNull(t *testing.T) {
	var d Drawing
	if got := encode(t, &d); got != `{"widget_id":7,"commands":[]}` {
		t.Fatalf("got %s", got)
	}
	d.Polyline(nil, 1, Color{})
	if got := encode(t, &d); got != `{"widget_id":7,"commands":[{"polyline":{"points":[],"width":1,"color":{"r":0,"g":0,"b":0,"a":0}}}]}` {
		t.Fatalf("got %s", got)
	}
}

// A zero StrokeWidth and "" align are the Go zero values, so they mean "the
// host's default" and are left out rather than sent as values the host
// would reject (width 0) or not recognise ("").
func TestZeroValuesAreOmittedSoHostDefaultsApply(t *testing.T) {
	var d Drawing
	c := Color{0, 0, 0, 1}
	d.Rect(0, 0, 1, 1, 0, Style{Fill: &c})
	d.Text(0, 0, "a", c, "")
	want := `{"widget_id":7,"commands":[` +
		`{"rect":{"x":0,"y":0,"w":1,"h":1,"fill":{"r":0,"g":0,"b":0,"a":1}}},` +
		`{"text":{"x":0,"y":0,"text":"a","color":{"r":0,"g":0,"b":0,"a":1}}}]}`
	if got := encode(t, &d); got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
}

func TestPointsAreCopiedSoTheCallerCanReuseItsSlice(t *testing.T) {
	var d Drawing
	pts := []Point{{1, 1}, {2, 2}}
	d.Polyline(pts, 1, Color{})
	d.Polygon(pts, Style{})
	pts[0] = Point{9, 9}
	want := `{"widget_id":7,"commands":[` +
		`{"polyline":{"points":[[1,1],[2,2]],"width":1,"color":{"r":0,"g":0,"b":0,"a":0}}},` +
		`{"polygon":{"points":[[1,1],[2,2]]}}]}`
	if got := encode(t, &d); got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
}

func TestNonFiniteValuesFailToEncode(t *testing.T) {
	for _, v := range []float32{float32(math.NaN()), float32(math.Inf(1))} {
		var d Drawing
		d.Circle(v, 0, 1, Style{})
		if _, err := d.Encode(1); err == nil {
			t.Fatalf("%v encoded without error", v)
		}
	}
}

func TestParseClickAndResize(t *testing.T) {
	x, y, err := ParseClick(`{"x":12.5,"y":3}`)
	if err != nil || x != 12.5 || y != 3 {
		t.Fatalf("ParseClick = %v, %v, %v", x, y, err)
	}
	w, h, err := ParseResize(`{"w":640,"h":480.5}`)
	if err != nil || w != 640 || h != 480.5 {
		t.Fatalf("ParseResize = %v, %v, %v", w, h, err)
	}
	if _, _, err := ParseClick(`not json`); err == nil {
		t.Fatal("bad click payload parsed")
	}
	if _, _, err := ParseResize(``); err == nil {
		t.Fatal("empty resize payload parsed")
	}
}
