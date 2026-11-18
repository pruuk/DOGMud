package playtest

import (
	"embed"

	"github.com/GoMudEngine/GoMud/internal/connections"
	"github.com/GoMudEngine/GoMud/internal/plugins"
)

//go:embed files/*
var files embed.FS

// PlaytestModule wires AI-playtest policy that keys off the AI-port connection:
// per-round beacons, a structural safe mode, and admin flagging commands. It
// does NOT create or provision any account — the AI agent logs in (or creates a
// character via the normal new-player flow) exactly like a real player.
type PlaytestModule struct {
	plug     *plugins.Plugin
	cfg      Config
	sendGMCP func(int, string, any)
}

var module PlaytestModule

func init() {
	module = PlaytestModule{
		plug: plugins.New(`playtest`, `0.1.2`),
	}
	if err := module.plug.AttachFileSystem(files); err != nil {
		panic(err)
	}

	// This module registers no web pages. Module pages are served on the
	// public site with no login, so an admin page cannot be one of them.
	// Settings live in files/data-overlays/config.yaml (see README.md).

	module.plug.Callbacks.SetOnLoad(module.onLoad)
}

// onLoad runs after the world and users are loaded. It only registers behaviors;
// there is no account fabrication. Everything keys off whether a live session is
// on the AI port (see isAIConnection).
func (m *PlaytestModule) onLoad() {
	m.cfg = loadConfig(m.plug)
	if !m.cfg.Enabled {
		return
	}
	m.registerCommands()
	m.registerSafeMode()
	m.registerBeacons()
}

// isAIConnection reports whether a connection id is a live session on the AI
// port. This is the module's single notion of "a tester": anything connected on
// the AI port is an AI client, so no account flag or boot-time provisioning is
// needed.
func isAIConnection(connId uint64) bool {
	cd := connections.Get(connId)
	return cd != nil && cd.ConnType() == connections.ConnAI
}
