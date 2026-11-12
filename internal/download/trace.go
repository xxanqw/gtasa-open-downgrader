package download

import (
	"context"
	"crypto/tls"
	"net/http/httptrace"
	"time"

	"github.com/xxanqw/gtasa-open-downgrader/internal/applog"
)

func traceRequest(ctx context.Context) context.Context {
	started := time.Now()
	return httptrace.WithClientTrace(ctx, &httptrace.ClientTrace{
		DNSDone: func(info httptrace.DNSDoneInfo) {
			applog.Event(ctx, "DEBUG", "http.dns", "addresses", info.Addrs, "coalesced", info.Coalesced, "error", info.Err)
		},
		ConnectDone: func(network, address string, err error) {
			applog.Event(ctx, "DEBUG", "http.connect", "network", network, "address", address, "error", err)
		},
		TLSHandshakeDone: func(state tls.ConnectionState, err error) {
			applog.Event(ctx, "DEBUG", "http.tls", "server", state.ServerName, "version", tls.VersionName(state.Version), "cipher", tls.CipherSuiteName(state.CipherSuite), "error", err)
		},
		GotConn: func(info httptrace.GotConnInfo) {
			applog.Event(ctx, "DEBUG", "http.connection", "reused", info.Reused, "was_idle", info.WasIdle, "idle_ms", info.IdleTime.Milliseconds())
		},
		GotFirstResponseByte: func() {
			applog.Event(ctx, "DEBUG", "http.first_byte", "elapsed_ms", float64(time.Since(started).Microseconds())/1000)
		},
	})
}
