package plugins

import (
	"os"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/savequeue"
)

// mudlog.Error dereferences a package-level logger that is nil until it is set
// up, so any test touching a failure path panics instead of failing. This is
// the first test file in the package, so it owns TestMain.
func TestMain(m *testing.M) {
	mudlog.SetupLogger(nil, "", "", false)
	os.Exit(m.Run())
}

func TestCollector_CollectsInOrderAndReports(t *testing.T) {
	c := newPendingCollector()

	p1 := &Plugin{name: "alpha", version: "1.0"}
	p2 := &Plugin{name: "beta", version: "1.0"}

	c.add(p1, savequeue.PendingWrite{Kind: "plugin", Path: "/tmp/a.dat", Data: []byte("a")})
	c.add(p2, savequeue.PendingWrite{Kind: "plugin", Path: "/tmp/b.dat", Data: []byte("b")})

	got := c.writes()
	if len(got) != 2 {
		t.Fatalf("collected %d writes, want 2", len(got))
	}
	if got[0].Path != "/tmp/a.dat" || got[1].Path != "/tmp/b.dat" {
		t.Errorf("writes out of order: %q then %q", got[0].Path, got[1].Path)
	}
}

// A callback that fails may have collected a PARTIAL snapshot. Persisting half
// of a plugin's state is worse than leaving the previous complete file alone
// for another cycle, so its writes are dropped.
func TestCollector_DiscardDropsOnlyThatPluginsWrites(t *testing.T) {
	c := newPendingCollector()

	good := &Plugin{name: "good", version: "1.0"}
	bad := &Plugin{name: "bad", version: "1.0"}

	c.add(good, savequeue.PendingWrite{Path: "/tmp/good.dat", Data: []byte("g")})
	c.add(bad, savequeue.PendingWrite{Path: "/tmp/bad1.dat", Data: []byte("b1")})
	c.add(bad, savequeue.PendingWrite{Path: "/tmp/bad2.dat", Data: []byte("b2")})

	c.discard(bad)

	got := c.writes()
	if len(got) != 1 {
		t.Fatalf("after discard got %d writes, want 1", len(got))
	}
	if got[0].Path != "/tmp/good.dat" {
		t.Errorf("kept the wrong write: %q", got[0].Path)
	}
}

func TestCollector_DiscardOfAnUnknownPluginIsSafe(t *testing.T) {
	c := newPendingCollector()
	c.add(&Plugin{name: "a", version: "1.0"}, savequeue.PendingWrite{Path: "/tmp/a.dat"})

	c.discard(&Plugin{name: "never-added", version: "1.0"})

	if len(c.writes()) != 1 {
		t.Errorf("discarding an unknown plugin changed the collected set")
	}
}
