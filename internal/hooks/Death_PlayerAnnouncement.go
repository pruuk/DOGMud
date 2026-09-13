package hooks

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/life"
	"github.com/GoMudEngine/GoMud/internal/term"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/worldevents"
)

// wirePlayerDeathAnnouncement subscribes to player Life Alive→Dead
// transitions and handles all broadcast / text announcements,
// external event queue entries, and instance ejection.
//
// Migrated from internal/usercommands/suicide.go lines 54-145
// and 251-261 as part of chunk-2 Task 9.
func wirePlayerDeathAnnouncement(c *characters.Character) {
	c.Life.Inner().AfterTransition("player_death_announcement",
		func(from, to life.State, r state.TransitionReason) {
			if from != life.Alive || to != life.Dead {
				return
			}
			// Only fire for player characters.
			if c.GetUserId() == 0 {
				return
			}
			u := users.GetByUserId(c.GetUserId())
			if u == nil {
				return
			}

			d, _ := c.Life.DeadData()

			// chunk 4d: subdue / cripple submission outcomes route
			// through the Life cascade with NoDeprogression set so
			// the player wakes at the temple without losing training.
			// The fiction is "knocked out, not killed" — so skip the
			// global death announcements, "you feel weakened" flavour,
			// and PvE death event emission. Submission-specific
			// narration is handled by Position_Messaging (T11).
			// Instance ejection (step 7) still fires because being
			// subdued in an ephemeral zone should still kick you.
			if d.NoDeprogression {
				if rooms.IsEphemeralRoomId(c.RoomId) {
					if inst := rooms.GetInstanceRegistry().FindByRoomId(c.RoomId); inst != nil {
						if inst.DeathPolicy == "ejected" {
							inst.RevokeAccess(u.UserId)
							u.SendText(messaging.CategorySystem, `<ansi fg="red">You have been expelled from the instance. There is no return.</ansi>`)
						}
					}
				}
				return
			}

			// 1. Room broadcast "X has died."
			room := rooms.LoadRoom(c.RoomId)
			if room != nil {
				room.SendTextVisual(messaging.CategoryDeath,
					fmt.Sprintf(`<ansi fg="username">%s</ansi> has died.`, c.Name),
					u.UserId,
				)
			}

			// 2. Build killedBy string and collect killer user IDs.
			dmgCt := len(d.DamageMap)
			killedByUserIds := make([]int, 0, dmgCt)
			killedBy := ``
			i := 0
			for uid := range d.DamageMap {
				if killer := users.GetByUserId(uid); killer != nil {
					if i > 0 {
						if i < dmgCt-1 {
							killedBy += `, `
						} else {
							killedBy += ` and `
						}
					}
					killedBy += `<ansi fg="username">` + killer.Character.Name + `</ansi>`
					i++
				}
				killedByUserIds = append(killedByUserIds, uid)
			}

			// 3. Global "*** X has DIED! ***" broadcast.
			msg := fmt.Sprintf(
				`<ansi fg="magenta-bold">***</ansi> <ansi fg="username">%s</ansi> has <ansi fg="red-bold">DIED!</ansi> <ansi fg="magenta-bold">***</ansi>%s`,
				c.Name, term.CRLFStr,
			)
			if killedBy != `` {
				msg = fmt.Sprintf(
					`<ansi fg="magenta-bold">***</ansi> <ansi fg="username">%s</ansi> has <ansi fg="red-bold">DIED!</ansi> (killed by %s) <ansi fg="magenta-bold">***</ansi>%s`,
					c.Name, killedBy, term.CRLFStr,
				)
			}
			events.AddToQueue(events.Broadcast{Text: msg})

			// 4. External PlayerDeath event queue (Discord integration,
			//    etc.). Permanent hardcoded false — permadeath is sunset.
			events.AddToQueue(events.PlayerDeath{
				UserId:        u.UserId,
				RoomId:        c.RoomId,
				Username:      u.Username,
				CharacterName: c.Name,
				Permanent:     false,
				KilledByUsers: killedByUserIds,
			})

			// 5. WorldEvent PvE death emission (only when no player damage).
			if dmgCt == 0 {
				causeOfDeath := deathCauseFor(c)

				zone := c.Zone
				region := ""
				if zCfg := rooms.GetZoneConfig(zone); zCfg != nil {
					region = zCfg.Region
				}
				worldevents.EmitWorldEvent(worldevents.WorldEvent{
					Type:         worldevents.PlayerDiedPvE,
					Significance: worldevents.Global,
					ZoneName:     zone,
					RegionName:   region,
					Description:  causeOfDeath,
				})
			}

			// 6. "You feel weakened" text to the dying player.
			u.SendText(messaging.CategoryDeath, `<ansi fg="yellow">You feel weakened by the brush with death.</ansi>`)

			// 7. Instance ejection — if dead in an ephemeral zone with
			//    "ejected" death policy, revoke access before teleport.
			if rooms.IsEphemeralRoomId(c.RoomId) {
				if inst := rooms.GetInstanceRegistry().FindByRoomId(c.RoomId); inst != nil {
					if inst.DeathPolicy == "ejected" {
						inst.RevokeAccess(u.UserId)
						u.SendText(messaging.CategorySystem, `<ansi fg="red">You have been expelled from the instance. There is no return.</ansi>`)
					}
				}
			}

			// 8. "Darkness swallows you" flavour — fired on Dead-entry so
			//    the player gets closure text before the teleport fires.
			u.SendText(messaging.CategoryDeath, `<ansi fg="yellow">Darkness swallows you. When you open your eyes, you are somewhere safe.</ansi>`)
		})
}

// deathCauseFor derives the PvE death-event cause string for a character:
// the engaged mob's name if one is fighting them, else "poison" if the
// Poisoned record is held, else "bleeding out" if the Bleeding record is
// held, else whatever the killing tick stamped (LastTickCause), else the
// generic fallback. Order matters — a poisoned AND bleeding character reads
// "poison" because that check comes first.
//
// The held-record checks read by id (HasBuff), not by flag (HasBuffFlag):
// HasBuff only tests the id index and does not care whether the record
// already reads Expired, while HasBuffFlag skips expired records outright.
// Buffs.Trigger() decrements TriggersLeft before returning the triggered
// buff, so a tick that is the record's LAST trigger — every ordinary bleed,
// since buffs.TickTriggers commonly produces one trigger, and one poison
// tick in ten — arrives already Expired; the old flag read then reported
// "their own foolishness" for an outright poison or bleed-out kill. Reading
// by id survives that, but not a prune: PruneBuffs runs on every NewTurn and
// can remove the expired record before this announcement listener runs
// (death is a queued event — see Character.DeathQueued), which is what
// LastTickCause is for: the round tick that landed the fatal harm stamps it
// at the moment the tick fires, before either hole can open.
//
// Checking by id also means a poison-FLAGGED buff that is not the Poisoned
// record (Venom, Spore Toxin, Toxic Cloud, Nausea) no longer reports
// "poison" here — narrower than the flag read it replaces, but faithful to
// the enum this function replaced, which only ever knew the spell dot and
// the bleed record.
//
// Extracted from the dmgCt==0 branch of wirePlayerDeathAnnouncement so the
// derivation can be pinned directly.
func deathCauseFor(c *characters.Character) string {
	causeOfDeath := ""
	// Check if fighting a mob.
	if c.IsInCombat() && c.EngagedTarget().MobInstanceId > 0 {
		if mob := mobs.GetInstance(c.EngagedTarget().MobInstanceId); mob != nil {
			causeOfDeath = mob.Character.Name
		}
	}
	// Check for lethal conditions.
	if causeOfDeath == "" {
		if c.HasBuff(buffs.BuffIdPoisoned) {
			causeOfDeath = "poison"
		} else if c.HasBuff(buffs.BuffIdBleeding) {
			causeOfDeath = "bleeding out"
		} else if c.LastTickCause != "" {
			causeOfDeath = c.LastTickCause
		}
	}
	if causeOfDeath == "" {
		causeOfDeath = "their own foolishness"
	}
	return causeOfDeath
}

func init() {
	characters.OnCharacterCreated(wirePlayerDeathAnnouncement)
}
