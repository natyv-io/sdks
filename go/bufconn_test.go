package natyv

import "testing"

// Real bug regression test: a raw TCP read split precisely between a
// line's trailing '\r' and its '\n' used to make ReadLine silently absorb
// an entire subsequent response line as if it were part of the current
// one's content. Reproduces the exact byte split captured live from
// natyv-io/mail-natyv's own dropped-message bug (a real Gmail IMAP FETCH
// response cut mid-CRLF across two separate TcpRead calls) -- see
// ReadLine's own doc comment for the full story. Message 5's own literal
// content ("0123456789", declared as {10}) is deliberately not real
// header text -- ReadN doesn't care about content, only byte count, and
// keeping it simple isolates the CRLF-split bug from unrelated literal-
// parsing concerns.
func TestReadLine_CRLFSplitAcrossReads(t *testing.T) {
	chunks := [][]byte{
		[]byte("* 5 FETCH (FLAGS (\\Seen) BODY[HEADER.FIELDS (FROM SUBJECT DATE)] {10}\r\n0123456789)\r"),
		[]byte("\n* 6 FETCH (FLAGS (\\Seen) BODY[HEADER.FIELDS (FROM SUBJECT DATE)] {10}\r\n9876543210)\r\n"),
	}
	call := 0
	bc := &BufferedConn{
		readMore: func(maxLen int) ([]byte, error) {
			if call >= len(chunks) {
				t.Fatalf("readMore called more times than the test fixture has chunks for")
			}
			c := chunks[call]
			call++
			return c, nil
		},
	}

	line1, err := bc.ReadLine()
	if err != nil {
		t.Fatalf("ReadLine (1): %v", err)
	}
	want1 := "* 5 FETCH (FLAGS (\\Seen) BODY[HEADER.FIELDS (FROM SUBJECT DATE)] {10}"
	if line1 != want1 {
		t.Fatalf("ReadLine (1) = %q, want %q", line1, want1)
	}

	lit1, err := bc.ReadN(10)
	if err != nil {
		t.Fatalf("ReadN (1): %v", err)
	}
	if string(lit1) != "0123456789" {
		t.Fatalf("ReadN (1) = %q, want %q", lit1, "0123456789")
	}

	// This is exactly where the real bug lived: the buffer here is just
	// ")\r" -- a bare trailing '\r' with no '\n' yet, because that's
	// precisely where the live TCP read split. Without the fix, `rest`
	// comes back as ")\r\n* 6 FETCH (FLAGS (\\Seen) BODY[HEADER.FIELDS
	// (FROM SUBJECT DATE)] {10}" -- the entire next message's opening
	// FETCH line silently swallowed as this line's own content.
	rest, err := bc.ReadLine()
	if err != nil {
		t.Fatalf("ReadLine (rest): %v", err)
	}
	if rest != ")" {
		t.Fatalf("ReadLine (rest) = %q, want %q -- the CRLF split across reads was not handled correctly", rest, ")")
	}

	line2, err := bc.ReadLine()
	if err != nil {
		t.Fatalf("ReadLine (2): %v", err)
	}
	want2 := "* 6 FETCH (FLAGS (\\Seen) BODY[HEADER.FIELDS (FROM SUBJECT DATE)] {10}"
	if line2 != want2 {
		t.Fatalf("ReadLine (2) = %q, want %q -- message 6's own FETCH response was never recognized", line2, want2)
	}

	lit2, err := bc.ReadN(10)
	if err != nil {
		t.Fatalf("ReadN (2): %v", err)
	}
	if string(lit2) != "9876543210" {
		t.Fatalf("ReadN (2) = %q, want %q", lit2, "9876543210")
	}

	closeLine, err := bc.ReadLine()
	if err != nil {
		t.Fatalf("ReadLine (close): %v", err)
	}
	if closeLine != ")" {
		t.Fatalf("ReadLine (close) = %q, want %q", closeLine, ")")
	}
}
