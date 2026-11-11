package users

import (
	"testing"
	"time"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/stretchr/testify/assert"
)

// The mail date splits into the day and the date-time, so a narrow screen
// can keep the date-time whole; a format without ", " stays one unit.
func TestMessageDateUnits(t *testing.T) {
	m := Message{DateSent: time.Date(2026, 11, 10, 12, 51, 27, 0, time.Local)}

	cfg := configs.GetConfig()
	cfg.TextFormats.Time = `Monday, 02-Jan-2006 3:04:05PM`
	configs.SetConfigForTest(t, cfg)
	assert.Equal(t, []string{"Tuesday,", "10-Nov-2026 12:51:27PM"}, m.DateUnits())

	cfg.TextFormats.Time = `2006-01-02 15:04`
	configs.SetConfigForTest(t, cfg)
	assert.Equal(t, []string{"2026-11-10 12:51"}, m.DateUnits())
}
