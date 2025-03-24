package cclient

import (
	"io"
	"net/url"
	"os"
	"time"

	http "github.com/EmpowerZ/fhttp"
	"golang.org/x/net/proxy"

	utls "github.com/EmpowerZ/utls"
)

func NewClient(clientHello utls.ClientHelloID, proxyUrl *url.URL, allowRedirect bool, skipTLSCheck bool,
	forceHttp11 bool, timeout time.Duration, tlsKeyLogWriterFilepath string, directDialer ...proxy.ContextDialer) (http.Client, error) {
	var keyLogWriter io.Writer
	if tlsKeyLogWriterFilepath != "" {
		var err error
		keyLogWriter, err = os.OpenFile(tlsKeyLogWriterFilepath, os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			return http.Client{}, err
		}
	}

	if proxyUrl != nil {
		dialer, err := newConnectDialer(proxyUrl)
		if err != nil {
			if allowRedirect {
				return http.Client{
					Timeout: timeout,
				}, err
			}
			return http.Client{
				Timeout: timeout,
				CheckRedirect: func(req *http.Request, via []*http.Request) error {
					return http.ErrUseLastResponse
				},
			}, err
		}
		if allowRedirect {
			return http.Client{
				Transport: newRoundTripper(clientHello, skipTLSCheck, forceHttp11, keyLogWriter, dialer),
				Timeout:   timeout,
			}, nil
		}
		return http.Client{
			Transport: newRoundTripper(clientHello, skipTLSCheck, forceHttp11, keyLogWriter, dialer),
			Timeout:   timeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}, nil
	} else {
		var currDialer proxy.ContextDialer = proxy.Direct
		if len(directDialer) > 0 {
			currDialer = directDialer[0]
		}

		if allowRedirect {
			return http.Client{
				Transport: newRoundTripper(clientHello, skipTLSCheck, forceHttp11, keyLogWriter, currDialer),
				Timeout:   timeout,
			}, nil
		}
		return http.Client{
			Transport: newRoundTripper(clientHello, skipTLSCheck, forceHttp11, keyLogWriter, currDialer),
			Timeout:   timeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}, nil

	}
}
