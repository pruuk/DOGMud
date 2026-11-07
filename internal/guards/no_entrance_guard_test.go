package guards

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/stretchr/testify/require"
)

// TestEveryRoomWithoutAnEntranceIsReachedByDesign loads the shipped world and
// asks which rooms the boot's "No Entrance" check still warns about. Owner
// ruling 2026-11-07: a room no exit leads into is either listed in
// rooms.noEntranceByDesign with the way players reach it (the tutorial, an
// instance template, a ferry), or it is reported. So a new room someone
// forgot to link fails here by its id.
func TestEveryRoomWithoutAnEntranceIsReachedByDesign(t *testing.T) {
	mudlog.SetupLogger(nil, `LOW`, ``, false)
	configs.SetConfigForTest(t, configs.GetConfig())
	require.NoError(t, configs.ReloadConfig())
	rooms.LoadBiomeDataFiles()
	rooms.LoadDataFiles()
	require.Empty(t, rooms.RoomsWithoutEntrance(),
		"a room with no exit leading in: link it, or add it to noEntranceByDesign with how players reach it")
}
