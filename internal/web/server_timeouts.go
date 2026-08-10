package web

import (
	"net/http"
	"time"

	"github.com/GoMudEngine/GoMud/internal/configs"
)

// applyServerTimeouts sets bounded timeouts on an http.Server.
//
// Without them a slow client holds a connection open indefinitely at almost no
// cost to itself, which is the whole of the Slowloris family (review finding
// 20a). Go's zero value for each of these fields means "no limit", so an
// http.Server built with only an Addr has every one of them disabled.
//
// Websockets are unaffected. The /ws upgrade shares this server's mux, and a
// naive WriteTimeout would normally sever every player's web client mid-
// session. gorilla/websocket avoids that by clearing the deadlines the HTTP
// server set as soon as it hijacks the connection ("Clear deadlines set by
// HTTP server", server.go, verified against v1.5.3). If gorilla is upgraded,
// re-verify that line still exists before trusting this.
//
// Timeouts come from Network config and each has a non-zero default, so this
// cannot silently apply nothing.
func applyServerTimeouts(srv *http.Server, cfg configs.Network) {
	if srv == nil {
		return
	}

	srv.ReadHeaderTimeout = time.Duration(cfg.HttpReadHeaderTimeoutSeconds) * time.Second
	srv.ReadTimeout = time.Duration(cfg.HttpReadTimeoutSeconds) * time.Second
	srv.WriteTimeout = time.Duration(cfg.HttpWriteTimeoutSeconds) * time.Second
	srv.IdleTimeout = time.Duration(cfg.HttpIdleTimeoutSeconds) * time.Second
}
