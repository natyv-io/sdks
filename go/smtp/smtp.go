// Package smtp is a minimal SMTP submission client built directly on
// natyv/sdk's raw TCP/TLS primitives -- Extism guests have no access to a
// real net.Conn, so the standard library's own net/smtp can't be used as
// wrapped (it references crypto/tls types TinyGo's wasip1 target doesn't
// implement, which fails to compile even if StartTLS is never called).
// This package only implements AUTH LOGIN + plain-text message
// composition/sending -- the common case for an app-password-authenticated
// account. Other AUTH mechanisms and MIME/attachment support are
// explicitly out of scope.
package smtp

import (
	"encoding/base64"
	"errors"
	"strings"

	"natyv/sdk"
)

// Client is one open SMTP session, past the initial greeting and EHLO.
type Client struct {
	c *natyv.BufferedConn
}

// readReply reads a possibly multi-line SMTP reply ("250-..." continuation
// lines terminated by "250 ...") and errors unless the final code is want.
func readReply(c *natyv.BufferedConn, want string) ([]string, error) {
	var lines []string
	for {
		line, err := c.ReadLine()
		if err != nil {
			return nil, err
		}
		lines = append(lines, line)
		if len(line) >= 4 && line[3] == ' ' {
			if line[:3] != want {
				return lines, errors.New("smtp: " + line)
			}
			return lines, nil
		}
		// line[3] == '-' means more lines follow.
	}
}

// Dial connects to host:port and expects TLS to already be active by the
// time the server's greeting arrives -- use this when the endpoint's
// conf.natyv.json allowed_sockets entry specifies "tls": "implicit" (SMTP
// submission over port 465). For a "starttls" entry (the traditional
// port-587 convention), use DialStartTLS instead. heloName is the value
// sent in the EHLO command -- any identifying string is accepted by real
// servers, it doesn't need to resolve to anything.
func Dial(host string, port uint16, heloName string) (*Client, error) {
	conn, err := natyv.TcpConnect(host, port)
	if err != nil {
		return nil, err
	}
	return newClient(conn, heloName)
}

// DialStartTLS connects to host:port over what starts as a plaintext
// socket, issues SMTP's own STARTTLS command, upgrades the connection via
// natyv.TcpUpgradeTLS, then re-sends EHLO as RFC 3207 requires (a server
// must not carry over the pre-TLS EHLO's capabilities).
func DialStartTLS(host string, port uint16, heloName string) (*Client, error) {
	conn, err := natyv.TcpConnect(host, port)
	if err != nil {
		return nil, err
	}
	bc := natyv.NewBufferedConn(conn)
	if _, err := readReply(bc, "220"); err != nil {
		return nil, err
	}
	if err := bc.WriteLine("EHLO " + heloName); err != nil {
		return nil, err
	}
	if _, err := readReply(bc, "250"); err != nil {
		return nil, err
	}
	if err := bc.WriteLine("STARTTLS"); err != nil {
		return nil, err
	}
	if _, err := readReply(bc, "220"); err != nil {
		return nil, err
	}
	if err := natyv.TcpUpgradeTLS(conn); err != nil {
		return nil, err
	}
	if err := bc.WriteLine("EHLO " + heloName); err != nil {
		return nil, err
	}
	if _, err := readReply(bc, "250"); err != nil {
		return nil, err
	}
	return &Client{c: bc}, nil
}

func newClient(conn natyv.TcpConn, heloName string) (*Client, error) {
	bc := natyv.NewBufferedConn(conn)
	if _, err := readReply(bc, "220"); err != nil {
		return nil, err
	}
	if err := bc.WriteLine("EHLO " + heloName); err != nil {
		return nil, err
	}
	if _, err := readReply(bc, "250"); err != nil {
		return nil, err
	}
	return &Client{c: bc}, nil
}

// AuthLogin authenticates via the AUTH LOGIN mechanism (base64-encoded
// username then password) -- the mechanism every major provider's App
// Password support expects.
func (cl *Client) AuthLogin(user, pass string) error {
	if err := cl.c.WriteLine("AUTH LOGIN"); err != nil {
		return err
	}
	if _, err := readReply(cl.c, "334"); err != nil {
		return err
	}
	if err := cl.c.WriteLine(base64.StdEncoding.EncodeToString([]byte(user))); err != nil {
		return err
	}
	if _, err := readReply(cl.c, "334"); err != nil {
		return err
	}
	if err := cl.c.WriteLine(base64.StdEncoding.EncodeToString([]byte(pass))); err != nil {
		return err
	}
	_, err := readReply(cl.c, "235")
	return err
}

// dotStuff escapes a leading "." on any line per RFC 5321 section 4.5.2 --
// required so a line of message content that happens to start with "."
// isn't mistaken for the DATA terminator.
func dotStuff(body string) string {
	lines := strings.Split(body, "\r\n")
	for i, l := range lines {
		if strings.HasPrefix(l, ".") {
			lines[i] = "." + l
		}
	}
	return strings.Join(lines, "\r\n")
}

// Send sends one plain-text message. from/to are bare addresses (no
// display-name angle-bracket wrapping needed here -- that's added
// internally for the envelope commands and the From/To headers).
func (cl *Client) Send(from, to, subject, body string) error {
	if err := cl.c.WriteLine("MAIL FROM:<" + from + ">"); err != nil {
		return err
	}
	if _, err := readReply(cl.c, "250"); err != nil {
		return err
	}
	if err := cl.c.WriteLine("RCPT TO:<" + to + ">"); err != nil {
		return err
	}
	if _, err := readReply(cl.c, "250"); err != nil {
		return err
	}
	if err := cl.c.WriteLine("DATA"); err != nil {
		return err
	}
	if _, err := readReply(cl.c, "354"); err != nil {
		return err
	}
	msg := "From: " + from + "\r\nTo: " + to + "\r\nSubject: " + subject + "\r\n\r\n" + dotStuff(body)
	if err := cl.c.WriteLine(msg + "\r\n."); err != nil {
		return err
	}
	_, err := readReply(cl.c, "250")
	return err
}

// Quit sends QUIT and closes the underlying connection.
func (cl *Client) Quit() error {
	cl.c.WriteLine("QUIT")
	return cl.c.Close()
}
