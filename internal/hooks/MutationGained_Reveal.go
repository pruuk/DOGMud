package hooks

import (
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/mutations"
	"github.com/GoMudEngine/GoMud/internal/splash"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// deepenFlourishText is the rank-up announcement (no splash ceremony —
// spec 2026-07-29: deepenings get the lighter reveal).
func deepenFlourishText(name string, rank int, maxLevel int) string {
	levelTag := fmt.Sprintf(`Level %d`, rank)
	if rank >= maxLevel {
		levelTag = `fully matured`
	}
	return fmt.Sprintf(
		`<ansi fg="magenta">The Chrysalis deepens its hold. Your <ansi fg="yellow">%s</ansi> grows stronger (%s).</ansi>`,
		name, levelTag)
}

// revealCaption is the plain-text form: the splash caption for
// screen-reader users, and part of the degraded no-splash path.
func revealCaption(name, description string) string {
	return fmt.Sprintf(`Something stirs beneath your skin. A mutation emerges: %s. %s`, name, description)
}

// flattenDescription collapses the YAML description's authored hard-wraps
// into one paragraph so the splash template's splitstring is the only
// wrapping applied.
func flattenDescription(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// displacedGearText is the line a player reads when a closed slot pushes a
// worn item off their body into the pack (#481), naming the real cause.
func displacedGearText(g characters.DisplacedGear) string {
	item := `<ansi fg="item">` + g.ItemName + `</ansi>`
	switch g.Reason {
	case characters.DisplacedByDisabledLegs:
		return `Your new tail leaves no room for ` + item + `, so you take it off.`
	case characters.DisplacedByLostTail:
		return `Your tail is gone, so you take off ` + item + `.`
	case characters.DisplacedByLostArm:
		return `Your extra arm is gone, so you take off ` + item + `.`
	case characters.DisplacedBySpecies:
		return `Your body has no place for ` + item + `, so you take it off.`
	case characters.DisplacedByTwoHandedGrip:
		return `Your weapon needs both hands again, so you let go of ` + item + `.`
	}
	return `Your body no longer has room for ` + item + `, so you take it off.`
}

// displacedGearNoticePriority orders the gear notice after every default
// (priority 0) event, including a splash.Splash that MutationGained_Reveal
// queues while the notice already waits, so the line follows the reveal on
// the splash path too. It stays ahead of the prompt redraw (100).
const displacedGearNoticePriority = 50

// displacedGearNotice asks DisplacedGear_Notify to tell one player about the
// gear a closed slot pushed into their pack.
type displacedGearNotice struct {
	UserId int
}

func (displacedGearNotice) Type() string { return `DisplacedGearNotice` }

// DisplacedGear_Notify drains the player's displacement record and sends one
// line per item. Draining here, when the notice is handled, means a record
// queued twice is still told once.
func DisplacedGear_Notify(e events.Event) events.ListenerReturn {
	evt, ok := e.(displacedGearNotice)
	if !ok {
		return events.Continue
	}
	user := users.GetByUserId(evt.UserId)
	if user == nil || user.Character == nil {
		return events.Continue
	}
	for _, g := range user.Character.TakeDisplacedGear() {
		user.SendText(messaging.CategoryMutation, displacedGearText(g))
	}
	return events.Continue
}

// endOfRoundValidate is the round tick's closing validate for one player,
// and the one drain point every mutation grant reaches: the round tick,
// pinnacle tick and quest grants queue mutations.Gained, but the Bloom Wafer
// and the Phial of Second Birth narrate their own grant and queue nothing.
// Whatever closed a slot since the last tick, the displacement is recorded by
// then, and the notice is queued behind any reveal already waiting.
func endOfRoundValidate(user *users.UserRecord) {
	if err := user.Character.Validate(); err != nil {
		mudlog.Warn("endOfRoundValidate", "userId", user.UserId, "error", err)
	}
	if user.Character.HasDisplacedGear() {
		events.AddToQueue(displacedGearNotice{UserId: user.UserId}, displacedGearNoticePriority)
	}
}

// MutationGained_Reveal renders the terminal side of a mutation reveal:
// a per-user chrysalis splash ceremony for new acquisitions, a short
// flourish for deepenings. The web card is pushed separately by the gmcp
// module listening to the same event.
func MutationGained_Reveal(e events.Event) events.ListenerReturn {
	evt, ok := e.(mutations.Gained)
	if !ok {
		return events.Continue
	}
	user := users.GetByUserId(evt.UserId)
	if user == nil {
		return events.Continue
	}
	if evt.Silent {
		return events.Continue
	}
	spec := mutations.GetMutation(evt.MutationId)
	if spec == nil {
		return events.Continue
	}

	if !evt.IsNew {
		user.SendText(messaging.CategoryMutation,
			deepenFlourishText(spec.Name, evt.Rank, int(configs.GetBalanceConfig().MutationMaxLevel)))
		return events.Continue
	}

	if !bool(configs.GetGamePlayConfig().SplashesEnabled) {
		// Splash_Deliver drops everything when splashes are disabled, so
		// degrade here to the classic two-line announcement.
		user.SendText(messaging.CategoryMutation, fmt.Sprintf(
			`<ansi fg="magenta">Something stirs beneath your skin. A mutation emerges: <ansi fg="yellow">%s</ansi>.</ansi>`,
			spec.Name))
		user.SendText(messaging.CategoryMutation, fmt.Sprintf(`<ansi fg="magenta">%s</ansi>`, spec.Description))
		return events.Continue
	}

	events.AddToQueue(splash.Splash{
		SceneId: `mutation_reveal`,
		Caption: revealCaption(spec.Name, flattenDescription(spec.Description)),
		Target:  splash.TargetUser,
		UserId:  evt.UserId,
		Data: map[string]any{
			`name`:        spec.Name,
			`description`: flattenDescription(spec.Description),
		},
	})
	return events.Continue
}
