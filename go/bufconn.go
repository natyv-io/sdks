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
}

// NewBufferedConn wraps an already-open TcpConn (from TcpConnect).
func NewBufferedConn(conn TcpConn) *BufferedConn {
	return &BufferedConn{Conn: conn}
}

func (b *BufferedConn) fill() error {
	if len(b.buf) > 0 {
		return nil
	}
	data, err := TcpRead(b.Conn, 4096)
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
func (b *BufferedConn) ReadLine() (string, error) {
	var line []byte
	for {
		if i := indexCRLF(b.buf); i >= 0 {
			line = append(line, b.buf[:i]...)
			b.buf = b.buf[i+2:]
			return string(line), nil
		}
		line = append(line, b.buf...)
		b.buf = nil
		if err := b.fill(); err != nil {
			return "", err
		}
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
