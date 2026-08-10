package gmcp

import (
	"errors"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Chunk 2.6 / finding 13 — zone creation must be transactional.
//
// buildZoneCreate ignored the errors from both SaveRoomTemplate and
// SaveZoneConfig, skipped its work silently when either lookup returned nil,
// published the new plane to the in-memory registry regardless, and returned
// Ok: true. The browser builder therefore reported a created zone that could be
// half-written on disk and would change or vanish at the next restart.
// ---------------------------------------------------------------------------

// zoneCreateSpy records what a run did, so the tests can assert on ordering
// (persist before publish) and rollback, not just on the returned result.
type zoneCreateSpy struct {
	deleted    []string
	markedPlan int
	marked     bool
	savedRoom  bool
	savedCfg   bool
}

// okDeps returns deps where every step succeeds, plus the spy watching them.
func okDeps() (zoneCreateDeps, *zoneCreateSpy) {
	spy := &zoneCreateSpy{markedPlan: -1}
	d := zoneCreateDeps{
		createZone:    func(name string) (int, error) { return 500, nil },
		nextFreePlane: func() int { return 7 },
		loadTemplate: func(id int) *rooms.Room {
			return &rooms.Room{RoomId: id, Zone: "Testzone"}
		},
		saveTemplate:   func(r rooms.Room) error { spy.savedRoom = true; return nil },
		getZoneConfig:  func(name string) *rooms.ZoneConfig { return &rooms.ZoneConfig{Name: name} },
		saveZoneConfig: func(cfg *rooms.ZoneConfig) error { spy.savedCfg = true; return nil },
		markPlane: func(plane int, nonEuclidean bool, label string) {
			spy.marked = true
			spy.markedPlan = plane
		},
		deleteZone: func(name string) error { spy.deleted = append(spy.deleted, name); return nil },
	}
	return d, spy
}

func req() zoneCreateReq {
	return zoneCreateReq{Name: "Testzone", Biome: "cave", Region: "test"}
}

// Control leg: a fully successful creation still works and publishes.
func TestBuildZoneCreate_HappyPathPersistsThenPublishes(t *testing.T) {
	d, spy := okDeps()

	res := buildZoneCreateWith(d, req())

	require.True(t, res.Ok, "a clean creation must succeed: %s", res.Error)
	assert.Equal(t, 500, res.RoomId)
	assert.True(t, spy.savedRoom)
	assert.True(t, spy.savedCfg)
	assert.True(t, spy.marked, "the plane must be published on success")
	assert.Equal(t, 7, spy.markedPlan)
	assert.Empty(t, spy.deleted, "nothing to roll back on success")
}

func TestBuildZoneCreate_RoomSaveFailureRollsBackAndReportsFailure(t *testing.T) {
	d, spy := okDeps()
	d.saveTemplate = func(r rooms.Room) error { return errors.New("disk full") }

	res := buildZoneCreateWith(d, req())

	assert.False(t, res.Ok, "a failed room save must not report success")
	assert.Contains(t, res.Error, "disk full")
	assert.Equal(t, []string{"Testzone"}, spy.deleted, "the half-created zone must be rolled back")
	assert.False(t, spy.marked,
		"the plane must NOT be published when the zone did not persist")
}

func TestBuildZoneCreate_ZoneConfigSaveFailureRollsBackAndReportsFailure(t *testing.T) {
	d, spy := okDeps()
	d.saveZoneConfig = func(cfg *rooms.ZoneConfig) error { return errors.New("permission denied") }

	res := buildZoneCreateWith(d, req())

	assert.False(t, res.Ok)
	assert.Contains(t, res.Error, "permission denied")
	assert.Equal(t, []string{"Testzone"}, spy.deleted)
	assert.False(t, spy.marked, "publishing must not outlive a failed persist")
}

// The silent-skip paths: a nil lookup used to fall straight through to
// Ok: true, leaving a zone with no entrance room or no usable config.
func TestBuildZoneCreate_MissingEntranceRoomRollsBackInsteadOfSucceeding(t *testing.T) {
	d, spy := okDeps()
	d.loadTemplate = func(id int) *rooms.Room { return nil }

	res := buildZoneCreateWith(d, req())

	assert.False(t, res.Ok, "a zone with no loadable entrance room is not a success")
	assert.Equal(t, []string{"Testzone"}, spy.deleted)
	assert.False(t, spy.marked)
}

func TestBuildZoneCreate_MissingZoneConfigRollsBackInsteadOfSucceeding(t *testing.T) {
	d, spy := okDeps()
	d.getZoneConfig = func(name string) *rooms.ZoneConfig { return nil }

	res := buildZoneCreateWith(d, req())

	assert.False(t, res.Ok)
	assert.Equal(t, []string{"Testzone"}, spy.deleted)
	assert.False(t, spy.marked)
}

// A rollback that itself fails must say so: the operator has a partial zone on
// disk and needs to know to clean it up by hand.
func TestBuildZoneCreate_FailedRollbackIsReportedNotSwallowed(t *testing.T) {
	d, _ := okDeps()
	d.saveZoneConfig = func(cfg *rooms.ZoneConfig) error { return errors.New("write failed") }
	d.deleteZone = func(name string) error { return errors.New("directory busy") }

	res := buildZoneCreateWith(d, req())

	assert.False(t, res.Ok)
	assert.Contains(t, res.Error, "write failed", "the original cause must survive")
	assert.Contains(t, res.Error, "rollback ALSO failed",
		"a failed rollback must be surfaced, not hidden behind the original error")
	assert.Contains(t, res.Error, "partially created")
}

// CreateZone failing is the one case with nothing to roll back.
func TestBuildZoneCreate_CreateFailureNeedsNoRollback(t *testing.T) {
	d, spy := okDeps()
	d.createZone = func(name string) (int, error) { return 0, errors.New("zone already exists") }

	res := buildZoneCreateWith(d, req())

	assert.False(t, res.Ok)
	assert.Contains(t, res.Error, "zone already exists")
	assert.Empty(t, spy.deleted, "nothing was created, so nothing to delete")
	assert.False(t, spy.marked)
}
