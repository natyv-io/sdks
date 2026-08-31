// Raw TCP/TLS socket access -- see sql.go for the package-level doc comment.
package natyv

import (
	"encoding/base64"
	"encoding/json"
	"errors"

	"github.com/extism/go-pdk"
)

//go:wasmimport extism:host/user tcp_connect
func tcpConnectHost(uint64) uint64

//go:wasmimport extism:host/user tcp_upgrade_tls
func tcpUpgradeTlsHost(uint64) uint64

//go:wasmimport extism:host/user tcp_read
func tcpReadHost(uint64) uint64

//go:wasmimport extism:host/user tcp_write
func tcpWriteHost(uint64) uint64

//go:wasmimport extism:host/user tcp_close
func tcpCloseHost(uint64) uint64

// TcpConn is an opaque handle to an open TCP connection, returned by
// TcpConnect. Mirrors natyv-core's own connection-registry id -- never
// construct one by hand.
type TcpConn uint32

// ErrTcpEOF is returned by TcpRead once the peer has closed its end of the
// connection cleanly.
var ErrTcpEOF = errors.New("tcp: connection closed by peer")

type tcpConnectRequest struct {
	Host string `json:"host"`
	Port uint16 `json:"port"`
}

// TcpConnect opens a TCP connection to host:port, which must appear in this
// app's own conf.natyv.json network.tcp.allowed_sockets -- any other
// host:port is rejected by the host before a socket is ever opened. If the
// matched allowed_sockets entry specifies "tls": "implicit", the TLS
// handshake happens here, before this call returns, verified against the
// endpoint's own configured CA (the bundled Mozilla store by default, or a
// dev-supplied ca_cert_path) -- a successful TcpConnect against an
// implicit-TLS endpoint is already a live, verified TLS session. A
// "starttls" endpoint stays plaintext until a later TcpUpgradeTLS call.
func TcpConnect(host string, port uint16) (TcpConn, error) {
	body, err := json.Marshal(tcpConnectRequest{Host: host, Port: port})
	if err != nil {
		return 0, err
	}
	var resp struct {
		Handle uint32 `json:"handle"`
		Error  string `json:"error,omitempty"`
	}
	if err := json.Unmarshal(pdk.ParamBytes(tcpConnectHost(pdk.ResultBytes(body))), &resp); err != nil {
		return 0, err
	}
	if resp.Error != "" {
		return 0, errors.New(resp.Error)
	}
	return TcpConn(resp.Handle), nil
}

type tcpHandleRequest struct {
	Handle uint32 `json:"handle"`
}

// TcpUpgradeTLS performs a TLS handshake in place over an already-connected,
// plaintext socket -- the STARTTLS pattern (SMTP, some IMAP deployments):
// connect plaintext, send the protocol's own STARTTLS-equivalent command,
// read the server's plaintext acknowledgement via TcpRead, then call this.
// Only valid on a connection whose matched allowed_sockets entry specifies
// "tls": "starttls"; returns an error if called on a connection not
// configured for it, or twice on the same connection.
func TcpUpgradeTLS(conn TcpConn) error {
	body, err := json.Marshal(tcpHandleRequest{Handle: uint32(conn)})
	if err != nil {
		return err
	}
	var resp struct {
		Ok    bool   `json:"ok"`
		Error string `json:"error,omitempty"`
	}
	if err := json.Unmarshal(pdk.ParamBytes(tcpUpgradeTlsHost(pdk.ResultBytes(body))), &resp); err != nil {
		return err
	}
	if resp.Error != "" {
		return errors.New(resp.Error)
	}
	return nil
}

type tcpReadRequest struct {
	Handle uint32 `json:"handle"`
	MaxLen int    `json:"max_len"`
}

// TcpRead performs one raw read, returning whatever bytes are available
// right now -- up to maxLen, possibly fewer, never blocking to fill the
// full amount requested. natyv-core does no message framing of its own, so
// a caller that needs an exact byte count (a fixed-length protocol field,
// a line) must loop and buffer itself, exactly like a plain net.Conn.
// Returns ErrTcpEOF once the peer has closed its end.
func TcpRead(conn TcpConn, maxLen int) ([]byte, error) {
	body, err := json.Marshal(tcpReadRequest{Handle: uint32(conn), MaxLen: maxLen})
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data  string `json:"data,omitempty"`
		Eof   bool   `json:"eof,omitempty"`
		Error string `json:"error,omitempty"`
	}
	if err := json.Unmarshal(pdk.ParamBytes(tcpReadHost(pdk.ResultBytes(body))), &resp); err != nil {
		return nil, err
	}
	if resp.Error != "" {
		return nil, errors.New(resp.Error)
	}
	if resp.Eof {
		return nil, ErrTcpEOF
	}
	decoded, err := base64.StdEncoding.DecodeString(resp.Data)
	if err != nil {
		return nil, err
	}
	return decoded, nil
}

type tcpWriteRequest struct {
	Handle uint32 `json:"handle"`
	Data   string `json:"data"`
}

// TcpWrite writes data in full -- the host loops internally until every
// byte is sent or an error occurs, so callers never need to handle a
// partial write themselves.
func TcpWrite(conn TcpConn, data []byte) error {
	body, err := json.Marshal(tcpWriteRequest{Handle: uint32(conn), Data: base64.StdEncoding.EncodeToString(data)})
	if err != nil {
		return err
	}
	var resp struct {
		Ok    bool   `json:"ok"`
		Error string `json:"error,omitempty"`
	}
	if err := json.Unmarshal(pdk.ParamBytes(tcpWriteHost(pdk.ResultBytes(body))), &resp); err != nil {
		return err
	}
	if resp.Error != "" {
		return errors.New(resp.Error)
	}
	return nil
}

// TcpClose closes an open connection, tearing down any active TLS session
// first. Safe to call on an already-closed handle.
func TcpClose(conn TcpConn) error {
	body, err := json.Marshal(tcpHandleRequest{Handle: uint32(conn)})
	if err != nil {
		return err
	}
	_ = pdk.ParamBytes(tcpCloseHost(pdk.ResultBytes(body)))
	return nil
}
