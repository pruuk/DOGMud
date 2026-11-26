package behaviortree

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

const addTempExitTreeYAML = `tree:
  type: action
  event: mob_idle
  do: add_temp_exit
%s  room_id: 2
`

// Owner ruling 18 (2026-11-26): an add_temp_exit node names an exit a player
// taps on a deck button, so the tree's load refuses a name longer than
// exit.MaxNameLetters, and a missing name, which could never open anything.
func TestAddTempExit_LoadRefusesALongOrMissingExitName(t *testing.T) {
	_, err := LoadTreeFromBytes(fmt.Appendf(nil, addTempExitTreeYAML, "  exit_name: rift\n"))
	require.NoError(t, err, "fixture check: a short name loads")

	_, err = LoadTreeFromBytes(fmt.Appendf(nil, addTempExitTreeYAML, "  exit_name: outlook\n"))
	require.ErrorContains(t, err, `exit "outlook" is 7 letters`)

	_, err = LoadTreeFromBytes(fmt.Appendf(nil, addTempExitTreeYAML, ""))
	require.ErrorContains(t, err, "add_temp_exit needs an exit_name param")
}
