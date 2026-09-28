package events

import (
	"bytes"
	"errors"
	"testing"
)

func TestReportDecodesDataAndDropped(t *testing.T) {
	table := NewTable()
	var gotData []byte
	var gotDropped uint32
	table.SetReport(7, func(data []byte, dropped uint32) error {
		gotData, gotDropped = data, dropped
		return nil
	})

	if err := table.Report(7, `{"data":"AQID","dropped":2}`); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(gotData, []byte{1, 2, 3}) || gotDropped != 2 {
		t.Fatalf("got %v dropped=%d", gotData, gotDropped)
	}

	// No "dropped" field means nothing was lost since the last report.
	if err := table.Report(7, `{"data":"BA=="}`); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(gotData, []byte{4}) || gotDropped != 0 {
		t.Fatalf("got %v dropped=%d", gotData, gotDropped)
	}
}

func TestEventsForUnknownHandlesAreIgnored(t *testing.T) {
	table := NewTable()
	if err := table.Report(9, `not json`); err != nil {
		t.Fatalf("unhandled report should be a no-op, got %v", err)
	}
	if err := table.Disconnect(9, `{}`); err != nil {
		t.Fatal(err)
	}
}

func TestBadPayloadSurfacesAnError(t *testing.T) {
	table := NewTable()
	table.SetReport(1, func([]byte, uint32) error { return nil })
	if err := table.Report(1, `{"data":"%%%"}`); err == nil {
		t.Fatal("invalid base64 should error")
	}
}

func TestHandlerErrorPropagates(t *testing.T) {
	table := NewTable()
	want := errors.New("boom")
	table.SetDisconnect(3, func() error { return want })
	if err := table.Disconnect(3, `{}`); !errors.Is(err, want) {
		t.Fatalf("got %v", err)
	}
}

func TestForgetDropsBothHandlers(t *testing.T) {
	table := NewTable()
	called := false
	table.SetReport(5, func([]byte, uint32) error { called = true; return nil })
	table.SetDisconnect(5, func() error { called = true; return nil })
	table.Forget(5)

	_ = table.Report(5, `{"data":"AQ=="}`)
	_ = table.Disconnect(5, `{}`)
	if called {
		t.Fatal("handler ran after Forget")
	}
}
