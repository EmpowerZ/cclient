package cclient

import (
	"time"

	http "github.com/Carcraftz/fhttp"
	"golang.org/x/net/proxy"

	utls "github.com/Carcraftz/utls"
)

func NewClient(clientHello utls.ClientHelloID, proxyUrl string, allowRedirect bool, skipTLSCheck bool,
	forceHttp11 bool, timeout time.Duration, directDialer ...proxy.ContextDialer) (http.Client, error) {
	if len(proxyUrl) > 0 {
		dialer, err := newConnectDialer(proxyUrl)
		if err != nil {
			if allowRedirect {
				return http.Client{
					Timeout: time.Second * timeout,
				}, err
			}
			return http.Client{
				Timeout: time.Second * timeout,
				CheckRedirect: func(req *http.Request, via []*http.Request) error {
					return http.ErrUseLastResponse
				},
			}, err
		}
		if allowRedirect {
			return http.Client{
				Transport: newRoundTripper(clientHello, skipTLSCheck, forceHttp11, dialer),
				Timeout:   time.Second * timeout,
			}, nil
		}
		return http.Client{
			Transport: newRoundTripper(clientHello, skipTLSCheck, forceHttp11, dialer),
			Timeout:   time.Second * timeout,
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
				Transport: newRoundTripper(clientHello, skipTLSCheck, forceHttp11, currDialer),
				Timeout:   time.Second * timeout,
			}, nil
		}
		return http.Client{
			Transport: newRoundTripper(clientHello, skipTLSCheck, forceHttp11, currDialer),
			Timeout:   time.Second * timeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}, nil

	}
}

