package time

import (
	"embed"
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/gametime"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/plugins"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

var (

	//////////////////////////////////////////////////////////////////////
	// NOTE: The below //go:embed directive is important!
	// It embeds the relative path into the var below it.
	//////////////////////////////////////////////////////////////////////

	//go:embed files/*
	files embed.FS
)

// ////////////////////////////////////////////////////////////////////
// NOTE: The init function in Go is a special function that is
// automatically executed before the main function within a package.
// It is used to initialize variables, set up configurations, or
// perform any other setup tasks that need to be done before the
// program starts running.
// ////////////////////////////////////////////////////////////////////
func init() {

	//
	// We can use all functions only, but this demonstrates
	//
	plug := plugins.New(`time`, `1.0`)

	//
	// Add the embedded filesystem
	//
	if err := plug.AttachFileSystem(files); err != nil {
		panic(err)
	}
	//
	// Register any user/mob commands
	//
	plug.AddUserCommand(`time`, TimeCommand, true, false)
}

//////////////////////////////////////////////////////////////////////
// NOTE: What follows is all custom code. For this module.
//////////////////////////////////////////////////////////////////////

func TimeCommand(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {

	gd := gametime.GetDate()

	if rest != `` { // testing code
		gd = gametime.GetDate(gametime.GetLastPeriod(rest, gd.RoundNumber))
	}

	// The lamps are read for the current round only; the testing argument
	// moves the date, not the sky, so it falls back to night alone.
	lampsLit := gd.Night
	if rest == `` {
		lampsLit = gametime.LampsLit()
	}

	user.SendText(messaging.CategoryTimeOfDay, timeLine(gd, gametime.DayPeriod(gd.Night, lampsLit, gd.Hour24)))

	return true, nil
}

// timeLine formats the `time` reply for a period from gametime.DayPeriod.
// Day and night keep their "-time" words and colours; dusk and dawn take the
// night colour, since the lamps are lit.
func timeLine(gd gametime.GameDate, period string) string {
	word, color := `daytime`, `day`
	switch period {
	case `night`:
		word, color = `nighttime`, `night`
	case `dusk`, `dawn`:
		word, color = period, `night`
	}
	return fmt.Sprintf(`It is now %s. It is <ansi fg="%s">%s</ansi> on <ansi fg="230">day %d</ansi> of <ansi fg="230">year %d</ansi>. The month is <ansi fg="230">%s</ansi>, and it is the year of the <ansi fg="230">%s</ansi>.`,
		gd.String(),
		color,
		word,
		gd.Day,
		gd.Year,
		gametime.MonthName(gd.Month),
		gametime.GetZodiac(gd.Year),
	)
}
