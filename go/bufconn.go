// Buffered line/exact-byte reading over a TcpConn -- shared by any
// text-based protocol client built on top of raw TCP/TLS (see
// github.com/natyv-io/sdks/go/imap, github.com/natyv-io/sdks/go/smtp). See tcp.go for the package-level doc
// comment.
package natyv

// BufferedConn adds line- and exact-byte-count reads on top of a TcpConn.
// TcpRead returns whatever's available right now with no message framing
// of its own, so most real text-based wire protocols (IMAP, SMTP, and
// others) need a buffering layer like this one on the guest side -- it
// exists here, in the base package, so every protocol client can share one
// implementation instead of each hand-rolling its own. Not safe for
// concurrent use from multiple goroutines against the same connection;
// natyv-core only ever has one guest call in flight at a time regardless,
// so this is never a real constraint in practice.
type BufferedConn struct {
	Conn TcpConn
	buf  []byte
	// readMore stands in for a real TcpRead call in tests -- nil in every
	// real (production) BufferedConn, which always goes through TcpRead
	// itself; a wasmimport host function call can't be exercised by a
	// plain `go test` run at all, so this is the only seam that lets
	// ReadLine's own CRLF-split-across-reads handling get a real,
	// deterministic unit test rather than relying on live verification
	// alone (see ReadLine's own doc comment for the bug this covers).
	readMore func(maxLen int) ([]byte, error)
}

// NewBufferedConn wraps an already-open TcpConn (from TcpConnect).
func NewBufferedConn(conn TcpConn) *BufferedConn {
	return &BufferedConn{Conn: conn}
}

func (b *BufferedConn) readChunk(maxLen int) ([]byte, error) {
	if b.readMore != nil {
		return b.readMore(maxLen)
	}
	return TcpRead(b.Conn, maxLen)
}

func (b *BufferedConn) fill() error {
	if len(b.buf) > 0 {
		return nil
	}
	data, err := b.readChunk(4096)
	if err != nil {
		return err
	}
	b.buf = data
	return nil
}

func indexCRLF(data []byte) int {
	for i := 0; i+1 < len(data); i++ {
		if data[i] == '\r' && data[i+1] == '\n' {
			return i
		}
	}
	return -1
}

// ReadLine returns one CRLF-terminated line, without the trailing CRLF.
//
// Real bug, found live (natyv-io/mail-natyv's own Inbox silently dropping
// exactly one message on some refreshes, traced down to a raw TCP read
// split precisely between a line's trailing '\r' and '\n'): the naive
// "no CRLF found yet, flush the whole buffer into line and fetch more"
// loop below used to flush a trailing lone '\r' into `line` too. Once that
// happened, the matching '\n' arrived as the *next* read's own leading
// byte with no preceding '\r' left anywhere in `b.buf` for indexCRLF to
// pair it with -- the scan would sail past it and match some *later* real
// CRLF instead, silently absorbing an entire intervening response line
// (framing, literal, and all) as if it were this line's own content. A
// trailing lone '\r' is held back in `b.buf` instead of being flushed, so
// the next read's leading byte can still complete the pair correctly.
func (b *BufferedConn) ReadLine() (string, error) {
	var line []byte
	for {
		if i := indexCRLF(b.buf); i >= 0 {
			line = append(line, b.buf[:i]...)
			b.buf = b.buf[i+2:]
			return string(line), nil
		}
		flush := len(b.buf)
		if flush > 0 && b.buf[flush-1] == '\r' {
			flush--
		}
		line = append(line, b.buf[:flush]...)
		pending := b.buf[flush:]
		// Bypasses fill()'s own "b.buf already has data, skip the read"
		// short-circuit -- a held-back pending '\r' would otherwise
		// prevent it from ever fetching more.
		data, err := b.readChunk(4096)
		if err != nil {
			return "", err
		}
		b.buf = append(append([]byte{}, pending...), data...)
	}
}

// ReadN returns exactly n raw bytes -- for wire formats with an explicit
// length prefix (e.g. IMAP's literal syntax, "{n}\r\n" followed by exactly
// n bytes that may themselves contain \r\n) where line-based reading can't
// be used.
func (b *BufferedConn) ReadN(n int) ([]byte, error) {
	out := make([]byte, 0, n)
	for len(out) < n {
		if len(b.buf) == 0 {
			if err := b.fill(); err != nil {
				return nil, err
			}
		}
		take := n - len(out)
		if take > len(b.buf) {
			take = len(b.buf)
		}
		out = append(out, b.buf[:take]...)
		b.buf = b.buf[take:]
	}
	return out, nil
}

// WriteLine writes s followed by a CRLF.
func (b *BufferedConn) WriteLine(s string) error {
	return TcpWrite(b.Conn, []byte(s+"\r\n"))
}

// Close closes the underlying connection.
func (b *BufferedConn) Close() error {
	return TcpClose(b.Conn)
}
