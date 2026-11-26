package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/stretchr/testify/require"
)

// #781, owner ruling 1: scan passes over a secret exit the scanner has not
// found, as look does, and scans through it once found (task 17 playtest:
// scan skipped every secret exit, found or not).
func TestScan_ASecretExitIsScannedOnceFound(t *testing.T) {
	actor := scanLockScene(t, 90, false, false, true)
	here := rooms.LoadRoom(scanLockHereId)
	north := here.Exits["north"]
	north.Secret = true
	here.Exits["north"] = north

	require.Empty(t, Scan(actor, ScanOptions{}).Sightings, "an unfound secret exit is not scanned")

	actor.char.LearnSecretExit(scanLockHereId, "north")
	result := Scan(actor, ScanOptions{})
	require.Len(t, result.Sightings, 1, "a found secret exit is scanned")
	require.Equal(t, "north", result.Sightings[0].ExitName)
}
