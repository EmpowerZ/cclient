package cclient

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"

	"golang.org/x/sync/singleflight"

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
	enableHWTS        bool

	dialer            proxy.ContextDialer
	tlsHandshakeGroup singleflight.Group
	onNewConnection   func(network, address string)
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
		rt.cachedTransports[addr] = &http.Transport{DialContext: rt.dialer.DialContext, EnableHardwareRXTimestamping: rt.enableHWTS}
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

	// STEP 3. If we have the connection from when we determined the HTTPS
	// cachedTransports to use, return that to the Transport library.
	if conn := rt.cachedConnections[addr]; conn != nil {
		delete(rt.cachedConnections, addr)
		return conn, nil
	}
	unlockIfLocked()

	// When multiple handshakes were done at the same time, in old code, before using singleflight.Group,
	// connections created by non-first dialTLS were thrown away and transport for them was also never created.
	// So singleflight.Group avoids creating unnecessary connections.
	v, err, _ := rt.tlsHandshakeGroup.Do(addr, func() (interface{}, error) {
		if rt.onNewConnection != nil {
			rt.onNewConnection(network, addr)
		}
		// original raw dial + uTLS handshake
		rawConn, err := rt.dialer.DialContext(ctx, network, addr)
		if err != nil {
			return nil, err
		}
		wrappedConn := rawConn
		if rt.enableHWTS {
			if wrapped, _ := http.WrapForHardwareTimestamps(rawConn, true); wrapped != nil {
				wrappedConn = wrapped
			}
		}

		host, _, serr := net.SplitHostPort(addr)
		if serr != nil {
			host = addr
		}

		nextProtos := []string{}
		if rt.forceHttp11 {
			nextProtos = []string{"http/1.1"}
		}
		// STEP 1. Create this connection to do handshake. Later (step 2) stash it to cachedConnections to and later return it
		// to the library (step 3) to be used for actual requests.
		uconn := utls.UClient(wrappedConn, &utls.Config{
			ServerName:         host,
			NextProtos:         nextProtos,
			InsecureSkipVerify: rt.skipTLSCheck,
			KeyLogWriter:       rt.keyLogWriter,
		}, rt.clientHelloId)
		if err = uconn.Handshake(); err != nil {
			_ = uconn.Close()
			return nil, err
		}

		unlocked = false
		rt.Lock()
		defer unlockIfLocked()
		// I think this can happen, when connection is dropped by server and lib tries to get new connection.
		// And this time the call comes from the library and expects a connection, not `errProtocolNegotiated`.
		// But I am not 100% sure.
		if rt.cachedTransports[addr] != nil {
			return uconn, nil
		}

		var tlsConf *utls.Config
		if rt.keyLogWriter != nil {
			tlsConf = &utls.Config{KeyLogWriter: rt.keyLogWriter}
		}
		// No http.Transport constructed yet, create one based on the results
		// of ALPN.
		if uconn.ConnectionState().NegotiatedProtocol == http2.NextProtoTLS {
			t2 := http2.Transport{DialTLS: rt.dialTLSHTTP2, EnableHardwareRXTimestamping: rt.enableHWTS}
			t2.Settings = []http2.Setting{
				{ID: http2.SettingMaxConcurrentStreams, Val: 1000},
				{ID: http2.SettingMaxFrameSize, Val: 16384},
				{ID: http2.SettingMaxHeaderListSize, Val: 262144},
			}
			t2.TLSClientConfig = tlsConf
			t2.InitialWindowSize = 6291456
			t2.HeaderTableSize = 65536
			t2.PushHandler = &http2.DefaultPushHandler{}
			rt.cachedTransports[addr] = &t2
		} else {
			// Assume the remote peer is speaking HTTP 1.x + TLS.
			rt.cachedTransports[addr] = &http.Transport{
				DialTLSContext:               rt.dialTLS,
				TLSClientConfig:              tlsConf,
				EnableHardwareRXTimestamping: rt.enableHWTS,
			}
		}
		// STEP 2. Stash the connection just established for use servicing the
		// actual request (should be near-immediate).
		rt.cachedConnections[addr] = uconn
		return nil, errProtocolNegotiated
	})
	conn, _ := v.(net.Conn) // conn is nil if v==nil or not a net.Conn
	return conn, err
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

func newRoundTripper(clientHello utls.ClientHelloID, skipTLSCheck, forceHttp11 bool, keyLogWriter io.Writer, enableHWTS bool, onNewConnection func(network, address string), dialer ...proxy.ContextDialer) http.RoundTripper {
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
		onNewConnection:   onNewConnection,
		enableHWTS:        enableHWTS,
	}
}
