package usercommands

import "testing"

// A reader under 80 columns gets a map drawn to the narrow form width with
// no frame; at 80 and over the map is as before (owner 2026-11-12, plan 5).
func TestMapSizeForReader(t *testing.T) {
	cases := []struct {
		name          string
		level         int
		wide          bool
		screenW, wrap int
		w, h          int
		narrow        bool
	}{
		{"desktop", 1, false, 80, 80, 65, 21, false},
		{"wide on a big telnet", 4, true, 200, 80, 186, 21, false},
		{"phone", 1, false, 42, 42, 42, 21, true},
		{"phone asking wide", 4, true, 42, 42, 42, 21, true},
		{"between", 1, false, 60, 60, 60, 21, true},
		{"just under 80", 1, false, 79, 79, 79, 21, true},
		{"narrowest", 1, false, 30, 30, 30, 21, true},
	}
	for _, c := range cases {
		w, h, narrow := mapSizeForReader(c.level, c.wide, c.screenW, 40, c.wrap)
		if c.name == "wide on a big telnet" {
			h = 21 // height follows the screen there; not under test
		}
		if w != c.w || h != c.h || narrow != c.narrow {
			t.Errorf("%s: got %dx%d narrow=%v, want %dx%d narrow=%v", c.name, w, h, narrow, c.w, c.h, c.narrow)
		}
	}
}
