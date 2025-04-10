package cclient

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"

	http "github.com/EmpowerZ/fhttp"

	"github.com/EmpowerZ/fhttp/http2"
	"golang.org/x/net/proxy"

	utls "github.com/refraction-networking/utls"
)

var errProtocolNegotiated = errors.New("protocol negotiated")

type roundTripper struct {
	sync.Mutex

	clientHelloId utls.ClientHelloID

	cachedConnections map[string]net.Conn
	cachedTransports  map[string]http.RoundTripper
	skipTLSCheck      bool
	forceHttp11       bool
	keyLogWriter      io.Writer

	dialer proxy.ContextDialer
}

func (rt *roundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	addr := rt.getDialTLSAddr(req)
	if _, ok := rt.cachedTransports[addr]; !ok {
		if err := rt.getTransport(req, addr); err != nil {
			return nil, err
		}
	}
	return rt.cachedTransports[addr].RoundTrip(req)
}

func (rt *roundTripper) getTransport(req *http.Request, addr string) error {
	switch strings.ToLower(req.URL.Scheme) {
	case "http":
		rt.cachedTransports[addr] = &http.Transport{DialContext: rt.dialer.DialContext}
		return nil
	case "https":
	default:
		return fmt.Errorf("invalid URL scheme: [%v]", req.URL.Scheme)
	}

	_, err := rt.dialTLS(req.Context(), "tcp", addr)
	switch err {
	case errProtocolNegotiated:
	case nil:
		// Should never happen.
		//panic("dialTLS returned no error when determining cachedTransports")
	default:
		return err
	}

	return nil
}

func (rt *roundTripper) dialTLS(ctx context.Context, network, addr string) (net.Conn, error) {
	unlocked := false
	unlockIfLocked := func() {
		if !unlocked {
			rt.Unlock()
			unlocked = true
		}
	}
	rt.Lock()
	defer unlockIfLocked()

	// If we have the connection from when we determined the HTTPS
	// cachedTransports to use, return that.
	if conn := rt.cachedConnections[addr]; conn != nil {
		delete(rt.cachedConnections, addr)
		return conn, nil
	}
	unlockIfLocked()

	rawConn, err := rt.dialer.DialContext(ctx, network, addr)
	if err != nil {
		return nil, err
	}

	var host string
	if host, _, err = net.SplitHostPort(addr); err != nil {
		host = addr
	}

	var nextProtos []string
	if rt.forceHttp11 {
		nextProtos = []string{"http/1.1"}
	}
	conn := utls.UClient(rawConn, &utls.Config{ServerName: host, NextProtos: nextProtos, InsecureSkipVerify: rt.skipTLSCheck, KeyLogWriter: rt.keyLogWriter}, rt.clientHelloId)
	if err = conn.Handshake(); err != nil {
		_ = conn.Close()
		return nil, err
	}

	unlocked = false
	rt.Lock()
	defer unlockIfLocked()
	if rt.cachedTransports[addr] != nil {
		return conn, nil
	}

	var tlsConfig *utls.Config
	if rt.keyLogWriter != nil {
		tlsConfig = &utls.Config{KeyLogWriter: rt.keyLogWriter}
	}
	// No http.Transport constructed yet, create one based on the results
	// of ALPN.
	switch conn.ConnectionState().NegotiatedProtocol {
	case http2.NextProtoTLS:
		t2 := http2.Transport{DialTLS: rt.dialTLSHTTP2}
		t2.Settings = []http2.Setting{
			{ID: http2.SettingMaxConcurrentStreams, Val: 1000},
			{ID: http2.SettingMaxFrameSize, Val: 16384},
			{ID: http2.SettingMaxHeaderListSize, Val: 262144},
		}
		t2.TLSClientConfig = tlsConfig
		t2.InitialWindowSize = 6291456
		t2.HeaderTableSize = 65536
		t2.PushHandler = &http2.DefaultPushHandler{}
		rt.cachedTransports[addr] = &t2
	default:
		// Assume the remote peer is speaking HTTP 1.x + TLS.
		rt.cachedTransports[addr] = &http.Transport{DialTLSContext: rt.dialTLS, TLSClientConfig: tlsConfig}
	}

	// Stash the connection just established for use servicing the
	// actual request (should be near-immediate).
	rt.cachedConnections[addr] = conn

	return nil, errProtocolNegotiated
}

func (rt *roundTripper) dialTLSHTTP2(network, addr string, _ *utls.Config) (net.Conn, error) {
	return rt.dialTLS(context.Background(), network, addr)
}

func (rt *roundTripper) getDialTLSAddr(req *http.Request) string {
	host, port, err := net.SplitHostPort(req.URL.Host)
	if err == nil {
		return net.JoinHostPort(host, port)
	}
	return net.JoinHostPort(req.URL.Host, "443") // we can assume port is 443 at this point
}

func newRoundTripper(clientHello utls.ClientHelloID, skipTLSCheck, forceHttp11 bool, keyLogWriter io.Writer, dialer ...proxy.ContextDialer) http.RoundTripper {
	var d proxy.ContextDialer = proxy.Direct
	if len(dialer) > 0 {
		d = dialer[0]
	}
	return &roundTripper{
		dialer:            d,
		clientHelloId:     clientHello,
		skipTLSCheck:      skipTLSCheck,
		forceHttp11:       forceHttp11,
		keyLogWriter:      keyLogWriter,
		cachedTransports:  make(map[string]http.RoundTripper),
		cachedConnections: make(map[string]net.Conn),
	}
}
