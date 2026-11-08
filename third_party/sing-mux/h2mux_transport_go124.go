//go:build go1.24

package mux

import (
	"golang.org/x/net/http2"
	"net/http"
)

const h2MuxWindowConfigSupported = true

func newH2MuxTransport(streamWindow int) (*http2.Transport, error) {
	if streamWindow == 0 {
		return &http2.Transport{}, nil
	}
	base := &http.Transport{HTTP2: &http.HTTP2Config{MaxReceiveBufferPerStream: streamWindow}}
	return http2.ConfigureTransports(base)
}
