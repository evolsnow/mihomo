//go:build go1.24

package adapter

import (
	"strings"
	"testing"
)

func TestH2MuxStreamWindowConfig(t *testing.T) {
	cases := []struct {
		name      string
		protocol  string
		window    any
		wantError bool
	}{
		{"default", "h2mux", nil, false},
		{"16MiB", "h2mux", 16 << 20, false},
		{"32MiB", "h2mux", 32 << 20, false},
		{"negative", "h2mux", -1, true},
		{"overflow", "h2mux", int64(1) << 31, true},
		{"wrong_protocol", "smux", 16 << 20, true},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			options := map[string]any{"enabled": true, "protocol": item.protocol}
			if item.window != nil {
				options["h2mux-stream-window"] = item.window
			}
			proxy, err := ParseProxy(map[string]any{
				"name": "window-test", "type": "ss", "server": "127.0.0.1", "port": 12345,
				"cipher": "aes-128-gcm", "password": "test-password", "smux": options,
			})
			if item.wantError {
				if err == nil {
					if proxy != nil {
						_ = proxy.Adapter().Close()
					}
					t.Fatal("invalid window was silently accepted")
				}
				if !strings.Contains(err.Error(), "h2mux-stream-window") {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			_ = proxy.Adapter().Close()
		})
	}
}
