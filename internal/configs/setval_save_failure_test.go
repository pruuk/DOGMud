package configs

import (
	"fmt"
	"path/filepath"
	"strconv"
	"testing"
)

// Prod mounts /mud-config read-only, so the durable save in setVal fails. The
// in-memory config must still follow the overrides map, or a counter such as
// Server.NextRoomId stops advancing while overrides keeps moving, and the
// allocator hands out the same id forever.
func TestSetEngineVal_SaveFailureStillAdvancesMemory(t *testing.T) {
	SetConfigWithLookupsForTest(t, lockTestConfig())
	if err := SetEngineVal(`Server.NextRoomId`, `6403`); err != nil {
		t.Fatalf(`setup: %v`, err)
	}
	// A save path whose directory does not exist fails exactly like the
	// read-only mount does (the open of <path>.new fails).
	t.Setenv(`CONFIG_PATH`, filepath.Join(t.TempDir(), `missing-dir`, `config.yaml`))

	for _, want := range []int{6404, 6405} {
		err := SetEngineVal(`Server.NextRoomId`, strconv.Itoa(want))
		if err == nil {
			t.Fatalf(`SetEngineVal(%d) returned nil, want the save error`, want)
		}
		if got := int(GetServerConfig().NextRoomId); got != want {
			t.Fatalf(`after a failed save NextRoomId in memory = %d, want %d`, got, want)
		}
		if got := overrides[`Server`].(map[string]any)[`NextRoomId`]; fmt.Sprint(got) != strconv.Itoa(want) {
			t.Fatalf(`overrides NextRoomId = %v, want %d (memory and overrides diverged)`, got, want)
		}
	}
}
