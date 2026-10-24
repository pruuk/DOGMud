package hooks

import (
	"os"
	"strings"
	"testing"
)

// TestLightNoticeListenersAreRegistered guards the four event seams
// LightNotice_Triggers.go documents. The events package exposes no way to
// inspect what is registered at runtime (hooks_test.go's TestRegisterListeners
// says so directly: "We can't inspect listener count, but we can verify the
// function runs"), so this reads hooks.go's own source and asserts each
// registration line is present, the same source-level technique
// internal/guards/narration_render_callers_guard_test.go uses to guard a
// different registry.
//
// A missing registration is not a compile error and not a panic: the
// listener function simply never runs, and the corresponding trigger goes
// silent in production while every lightnotice-package test (which calls
// Check directly, bypassing hooks.go entirely) keeps passing. This is the
// guard for that gap.
func TestLightNoticeListenersAreRegistered(t *testing.T) {
	src, err := os.ReadFile("hooks.go")
	if err != nil {
		t.Fatalf("reading hooks.go: %v", err)
	}
	text := string(src)

	cases := []struct {
		name string
		line string
	}{
		{"LightNoticeOnMove (RoomChange)", "events.RegisterListener(events.RoomChange{}, LightNoticeOnMove)"},
		{"LightNoticeAttention (NewRound)", "events.RegisterListener(events.NewRound{}, LightNoticeAttention)"},
		{"LightNoticeOnSpawn (PlayerSpawn)", "events.RegisterListener(events.PlayerSpawn{}, LightNoticeOnSpawn)"},
		{"LightNoticeOnDespawn (PlayerDespawn)", "events.RegisterListener(events.PlayerDespawn{}, LightNoticeOnDespawn)"},
	}
	for _, c := range cases {
		if !strings.Contains(text, c.line) {
			t.Errorf("%s: hooks.go no longer registers %q", c.name, c.line)
		}
	}
}
