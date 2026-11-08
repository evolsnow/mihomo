//go:build go1.24

package mux

import (
	"bytes"
	"fmt"
	"golang.org/x/net/http2"
	"io"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type windowRecordingConn struct {
	net.Conn
	mutex sync.Mutex
	sent  bytes.Buffer
}

func (c *windowRecordingConn) Write(p []byte) (int, error) {
	n, err := c.Conn.Write(p)
	c.mutex.Lock()
	_, _ = c.sent.Write(p[:n])
	c.mutex.Unlock()
	return n, err
}

func (c *windowRecordingConn) initialWindows() (uint32, uint32, error) {
	c.mutex.Lock()
	data := append([]byte(nil), c.sent.Bytes()...)
	c.mutex.Unlock()
	if !bytes.HasPrefix(data, []byte(http2.ClientPreface)) {
		return 0, 0, fmt.Errorf("missing client preface")
	}
	framer := http2.NewFramer(io.Discard, bytes.NewReader(data[len(http2.ClientPreface):]))
	var stream, connection uint32
	for {
		frame, err := framer.ReadFrame()
		if err != nil {
			return stream, connection, err
		}
		switch value := frame.(type) {
		case *http2.SettingsFrame:
			err = value.ForeachSetting(func(setting http2.Setting) error {
				if setting.ID == http2.SettingInitialWindowSize {
					stream = setting.Val
				}
				return nil
			})
			if err != nil {
				return 0, 0, err
			}
		case *http2.WindowUpdateFrame:
			if value.StreamID == 0 {
				connection = value.Increment
			}
		}
		if stream != 0 && connection != 0 {
			return stream, connection, nil
		}
	}
}

func TestH2MuxStreamWindowSettingsAndReuse(t *testing.T) {
	for _, item := range []struct {
		name       string
		configured int
		expected   uint32
	}{
		{"default", 0, 4 << 20}, {"16MiB", 16 << 20, 16 << 20}, {"32MiB", 32 << 20, 32 << 20},
	} {
		t.Run(item.name, func(t *testing.T) {
			left, right := net.Pipe()
			recorded := &windowRecordingConn{Conn: left}
			defer left.Close()
			defer right.Close()
			var requests atomic.Int32
			server := &http2.Server{IdleTimeout: idleTimeout}
			go server.ServeConn(right, &http2.ServeConnOpts{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				defer r.Body.Close()
				data := make([]byte, 1024)
				for {
					n, err := r.Body.Read(data)
					if n > 0 {
						if _, writeErr := w.Write(data[:n]); writeErr != nil {
							return
						}
						w.(http.Flusher).Flush()
					}
					if err != nil {
						return
					}
				}
			})})
			session, err := newH2MuxClient(recorded, item.configured)
			if err != nil {
				t.Fatal(err)
			}
			defer session.Close()
			stream, connection, err := recorded.initialWindows()
			if err != nil {
				t.Fatal(err)
			}
			if stream != item.expected {
				t.Fatalf("stream window=%d, want %d", stream, item.expected)
			}
			if connection != 1<<30 {
				t.Fatalf("connection window changed: %d", connection)
			}
			finished := make(chan error, 1)
			go func() {
				for i := 0; i < 2; i++ {
					conn, openErr := session.Open(5 * time.Second)
					if openErr != nil {
						finished <- openErr
						return
					}
					_, writeErr := conn.Write([]byte("ping"))
					if writeErr != nil {
						_ = conn.Close()
						finished <- writeErr
						return
					}
					data := make([]byte, 4)
					_, readErr := io.ReadFull(conn, data)
					_ = conn.Close()
					if readErr != nil {
						finished <- readErr
						return
					}
					if string(data) != "ping" {
						finished <- fmt.Errorf("unexpected echo %q", data)
						return
					}
				}
				finished <- nil
			}()
			select {
			case err = <-finished:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("requests did not complete on the reused connection")
			}
			if requests.Load() != 2 {
				t.Fatalf("requests=%d, want 2", requests.Load())
			}
		})
	}
}

func TestH2MuxStreamWindowValidation(t *testing.T) {
	cases := []Options{
		{Protocol: "h2mux", H2MuxStreamWindow: -1},
		{Protocol: "smux", H2MuxStreamWindow: 16 << 20},
		{Protocol: "yamux", H2MuxStreamWindow: 16 << 20},
	}
	aboveLimit := int64(math.MaxInt32)
	aboveLimit++
	if strconv.IntSize == 64 {
		cases = append(cases, Options{Protocol: "h2mux", H2MuxStreamWindow: int(aboveLimit)})
	}
	for _, options := range cases {
		if _, err := NewClient(options); err == nil || !strings.Contains(err.Error(), "h2mux-stream-window") {
			t.Fatalf("options=%+v error=%v", options, err)
		}
	}
	for _, options := range []Options{{Protocol: "h2mux"}, {Protocol: "smux"}, {Protocol: "yamux"}, {H2MuxStreamWindow: 16 << 20}} {
		if _, err := NewClient(options); err != nil {
			t.Fatalf("valid options=%+v: %v", options, err)
		}
	}
}
