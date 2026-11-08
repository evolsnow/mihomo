//go:build !go1.24

package mux

import (
	"fmt"
	"golang.org/x/net/http2"
)

const h2MuxWindowConfigSupported = false

func newH2MuxTransport(streamWindow int) (*http2.Transport, error) {
	if streamWindow != 0 {
		return nil, fmt.Errorf("h2mux-stream-window requires Go 1.24 or newer")
	}
	return &http2.Transport{}, nil
}
