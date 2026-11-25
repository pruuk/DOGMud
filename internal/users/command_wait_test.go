package users

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/require"
)

// A wait of zero or less means now. gearup and wimpy's flee pass -1; it used
// to wrap to a turn about 2^64 away on a fresh boot, so the command never ran.
func TestCommandReadyTurn_NegativeWaitIsNow(t *testing.T) {
	now := util.GetTurnCount()
	require.Equal(t, now, commandReadyTurn(nil))
	require.Equal(t, now, commandReadyTurn([]float64{0}))
	require.Equal(t, now, commandReadyTurn([]float64{-1}))
	require.Equal(t, now, commandReadyTurn([]float64{-0.5}))

	perSecond := uint64(configs.GetTimingConfig().SecondsToTurns(1))
	require.Equal(t, now+2*perSecond, commandReadyTurn([]float64{2}), "a positive wait is unchanged")
}
