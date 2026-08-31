// Package imap is a minimal IMAP4rev1 client built directly on
// natyv/sdk's raw TCP/TLS primitives -- Extism guests have no access to a
// real net.Conn, so libraries like go-imap can't be used as-is (their
// STARTTLS support references crypto/tls types TinyGo's wasip1 target
// doesn't implement, which fails to compile even if that code path is
// never exercised at runtime). This package only implements what a
// typical mail client actually needs: LOGIN, SELECT, a header-only FETCH
// for a message list, and a full-body FETCH for reading one message.
// Search, threading, and attachment/MIME parsing are explicitly out of
// scope -- add a dedicated parser on top of FetchBody's raw bytes if a
// real app needs those.
package imap

import (
	"errors"
	"strconv"
	"strings"

	"natyv/sdk"
)

// Client is one open IMAP session.
type Client struct {
	c   *natyv.BufferedConn
	tag int
}

// Dial connects to host:port and expects TLS to already be active by the
// time the server's greeting arrives -- use this when the endpoint's
// conf.natyv.json allowed_sockets entry specifies "tls": "implicit" (the
// standard IMAPS convention, port 993). For a "starttls" entry, use
// DialStartTLS instead.
func Dial(host string, port uint16) (*Client, error) {
	conn, err := natyv.TcpConnect(host, port)
	if err != nil {
		return nil, err
	}
	return newClient(conn)
}

// DialStartTLS connects to host:port over what starts as a plaintext
// socket, issues IMAP's own STARTTLS command, and upgrades the connection
// via natyv.TcpUpgradeTLS before the session is handed back -- use this
// when the endpoint's allowed_sockets entry specifies "tls": "starttls".
func DialStartTLS(host string, port uint16) (*Client, error) {
	conn, err := natyv.TcpConnect(host, port)
	if err != nil {
		return nil, err
	}
	bc := natyv.NewBufferedConn(conn)
	greeting, err := bc.ReadLine()
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(greeting, "* OK") {
		return nil, errors.New("imap: unexpected greeting: " + greeting)
	}
	cl := &Client{c: bc}
	if _, err := cl.command("STARTTLS"); err != nil {
		return nil, err
	}
	if err := natyv.TcpUpgradeTLS(conn); err != nil {
		return nil, err
	}
	return cl, nil
}

func newClient(conn natyv.TcpConn) (*Client, error) {
	bc := natyv.NewBufferedConn(conn)
	greeting, err := bc.ReadLine()
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(greeting, "* OK") {
		return nil, errors.New("imap: unexpected greeting: " + greeting)
	}
	return &Client{c: bc}, nil
}

func (cl *Client) nextTag() string {
	cl.tag++
	return "a" + strconv.Itoa(cl.tag)
}

// literalLen reports the byte count of a trailing IMAP literal marker
// ("... {n}"), if line ends in one.
func literalLen(line string) (int, bool) {
	if !strings.HasSuffix(line, "}") {
		return 0, false
	}
	open := strings.LastIndex(line, "{")
	if open < 0 {
		return 0, false
	}
	n, err := strconv.Atoi(line[open+1 : len(line)-1])
	if err != nil {
		return 0, false
	}
	return n, true
}

// command sends one tagged command and returns every untagged ("*") line
// up to the matching tagged completion. A line ending in IMAP's literal
// marker ({n}) has its n raw bytes read and re-attached before returning --
// fine for commands whose untagged responses have no literal at all
// (LOGIN, SELECT, LOGOUT); FETCH uses fetchCommand instead, which keeps a
// literal's bytes separate rather than splicing them back into a string.
func (cl *Client) command(cmd string) ([]string, error) {
	tag := cl.nextTag()
	if err := cl.c.WriteLine(tag + " " + cmd); err != nil {
		return nil, err
	}
	var untagged []string
	for {
		line, err := cl.c.ReadLine()
		if err != nil {
			return nil, err
		}
		if n, ok := literalLen(line); ok {
			data, err := cl.c.ReadN(n)
			if err != nil {
				return nil, err
			}
			rest, err := cl.c.ReadLine()
			if err != nil {
				return nil, err
			}
			line = line[:strings.LastIndex(line, "{")] + string(data) + rest
		}
		if strings.HasPrefix(line, tag+" ") {
			if !strings.HasPrefix(line, tag+" OK") {
				return untagged, errors.New("imap: " + line)
			}
			return untagged, nil
		}
		untagged = append(untagged, line)
	}
}

func quote(s string) string {
	return `"` + strings.ReplaceAll(strings.ReplaceAll(s, `\`, `\\`), `"`, `\"`) + `"`
}

// Login authenticates with a plain username/password -- the only mechanism
// this package supports (matches an App Password's own auth model; OAuth
// bearer tokens would need a different command, not currently implemented).
func (cl *Client) Login(user, pass string) error {
	_, err := cl.command("LOGIN " + quote(user) + " " + quote(pass))
	return err
}

// Select opens a mailbox (e.g. "INBOX", "Sent") for subsequent Fetch calls.
func (cl *Client) Select(mailbox string) error {
	_, err := cl.command("SELECT " + quote(mailbox))
	return err
}

// FetchItem is one untagged "* N FETCH (...)" response, split at the point
// IMAP's literal syntax ({n}\r\n<n bytes>) inserts raw content -- Attrs is
// everything else (e.g. the FLAGS list and the closing ")"), Literal is
// the exact literal bytes (a header block or a message body), nil if this
// response had no literal at all.
type FetchItem struct {
	Seq     int
	Attrs   string
	Literal []byte
}

// parseFetchSeq splits "* 3 FETCH (...)" into (3, "(...)"); ok is false for
// any other untagged line shape (e.g. "* 3 EXISTS", a SELECT response).
func parseFetchSeq(line string) (seq int, rest string, ok bool) {
	if !strings.HasPrefix(line, "* ") {
		return 0, "", false
	}
	after := line[2:]
	sp := strings.IndexByte(after, ' ')
	if sp < 0 {
		return 0, "", false
	}
	n, err := strconv.Atoi(after[:sp])
	if err != nil {
		return 0, "", false
	}
	after = after[sp+1:]
	if !strings.HasPrefix(after, "FETCH ") {
		return 0, "", false
	}
	return n, after[len("FETCH "):], true
}

// fetchCommand is like command, but preserves each response's literal
// bytes separately instead of splicing them back into one flattened
// string -- Message/body parsing needs the literal's real byte boundaries.
func (cl *Client) fetchCommand(cmd string) ([]FetchItem, error) {
	tag := cl.nextTag()
	if err := cl.c.WriteLine(tag + " " + cmd); err != nil {
		return nil, err
	}
	var items []FetchItem
	for {
		line, err := cl.c.ReadLine()
		if err != nil {
			return nil, err
		}
		if strings.HasPrefix(line, tag+" ") {
			if !strings.HasPrefix(line, tag+" OK") {
				return items, errors.New("imap: " + line)
			}
			return items, nil
		}
		seq, attrs, ok := parseFetchSeq(line)
		if !ok {
			continue // some other untagged line, not a FETCH result
		}
		item := FetchItem{Seq: seq, Attrs: attrs}
		if n, litOK := literalLen(line); litOK {
			data, err := cl.c.ReadN(n)
			if err != nil {
				return nil, err
			}
			rest, err := cl.c.ReadLine() // usually just ")"
			if err != nil {
				return nil, err
			}
			item.Literal = data
			item.Attrs += rest
		}
		items = append(items, item)
	}
}

// mailHeader is one unfolded RFC 822 header line.
type mailHeader struct {
	name  string
	value string
}

// parseHeaderBlock splits a raw RFC 822 header block into name/value pairs,
// unfolding continuation lines (a line starting with a space or tab is a
// continuation of the previous header, per RFC 822 section 3.1.1).
func parseHeaderBlock(raw []byte) []mailHeader {
	var out []mailHeader
	for _, line := range strings.Split(string(raw), "\r\n") {
		if line == "" {
			continue
		}
		if (strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t")) && len(out) > 0 {
			out[len(out)-1].value += " " + strings.TrimSpace(line)
			continue
		}
		colon := strings.IndexByte(line, ':')
		if colon < 0 {
			continue
		}
		out = append(out, mailHeader{
			name:  strings.TrimSpace(line[:colon]),
			value: strings.TrimSpace(line[colon+1:]),
		})
	}
	return out
}

// Message is one entry in a FetchHeaders result -- just enough to render an
// inbox list. Seq is the message's sequence number within the currently
// selected mailbox (not a stable UID -- use IMAP UIDs instead if a real
// app needs identity across SELECTs, not implemented here).
type Message struct {
	Seq     int
	From    string
	Subject string
	Date    string
	Seen    bool
}

// FetchHeaders fetches From/Subject/Date and the \Seen flag for message
// sequence numbers from..to (inclusive) in the currently selected mailbox.
func (cl *Client) FetchHeaders(from, to int) ([]Message, error) {
	items, err := cl.fetchCommand("FETCH " + strconv.Itoa(from) + ":" + strconv.Itoa(to) + " (FLAGS BODY[HEADER.FIELDS (FROM SUBJECT DATE)])")
	if err != nil {
		return nil, err
	}
	msgs := make([]Message, 0, len(items))
	for _, it := range items {
		m := Message{Seq: it.Seq, Seen: strings.Contains(it.Attrs, `\Seen`)}
		for _, h := range parseHeaderBlock(it.Literal) {
			switch strings.ToLower(h.name) {
			case "from":
				m.From = h.value
			case "subject":
				m.Subject = h.value
			case "date":
				m.Date = h.value
			}
		}
		msgs = append(msgs, m)
	}
	return msgs, nil
}

// FetchBody returns the plain-text body of message sequence number seq in
// the currently selected mailbox.
func (cl *Client) FetchBody(seq int) (string, error) {
	items, err := cl.fetchCommand("FETCH " + strconv.Itoa(seq) + " (BODY[TEXT])")
	if err != nil {
		return "", err
	}
	if len(items) == 0 {
		return "", errors.New("imap: no such message")
	}
	return string(items[0].Literal), nil
}

// Logout ends the session and closes the underlying connection.
func (cl *Client) Logout() error {
	_, err := cl.command("LOGOUT")
	cl.c.Close()
	return err
}
