// Package rifts wires internal/rifts into the game: the exit router and
// entry guard, the event listeners, the per-round sweep and the daily
// rotation of portal sites (persisted so a reboot keeps the day's sites), and
// the `answer`, `touch` and admin `rift` commands. The rules live in
// internal/rifts; this module only connects them.
package rifts

import (
	"embed"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/apiframework"
	"github.com/GoMudEngine/GoMud/internal/baubles"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/plugins"
	rifts "github.com/GoMudEngine/GoMud/internal/rifts"
	"github.com/GoMudEngine/GoMud/internal/roomlife"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/usercommands"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

var (
	//go:embed files/*
	files embed.FS
)

// sitesStateId names the persisted portal sites (plugin storage).
const sitesStateId = `portal-sites`

// lostStateId names the persisted lost-items record (plugin storage): what
// players lost in rifts, which rubble may give back to someone else.
const lostStateId = `lost-items`

type module struct {
	plug   *plugins.Plugin
	writer *roomWriter // generated rooms (gen.go)
}

func init() {
	m := &module{plug: plugins.New(`rifts`, `1.0`)}
	if err := m.plug.AttachFileSystem(files); err != nil {
		panic(err)
	}

	m.plug.AddUserCommand(`answer`, m.answerCommand, false, false)
	m.plug.AddUserCommand(`touch`, m.touchCommand, false, false)
	m.plug.AddUserCommand(`rift`, m.riftAdminCommand, true, true)
	m.plug.Callbacks.SetOnLoad(m.onLoad)
	m.plug.Callbacks.SetOnSave(m.onSave)

	rooms.AddExitRouter(rifts.Router)
	rooms.AddEntryGuard(rifts.EntryGuard)
	actions.SetFeatureCacheHook(searchRubble)
	// Door names must not take over a player command (gen.go shadowsCommand).
	rifts.SetCommandNames(usercommands.GetAllUserCommands)
	// A lens place typed at a lens table (`1c`, `c1`, `1c 4e`) turns lenses.
	usercommands.AddFallbackHandler(func(cmd, rest string, user *users.UserRecord, room *rooms.Room) bool {
		return rifts.TurnLenses(user, room, cmd, rest)
	})
	roomlife.SetPlaceHook(riftPlace)

	events.RegisterListener(events.RoomChange{}, m.onRoomChange)
	events.RegisterListener(events.MobDeath{}, m.onMobDeath)
	events.RegisterListener(events.PlayerDeath{}, m.onPlayerDeath)
	events.RegisterListener(events.PlayerDespawn{}, m.onPlayerDespawn)
	events.RegisterListener(events.PlayerSpawn{}, m.onPlayerSpawn)
	events.RegisterListener(events.Looking{}, m.onLooking)
	events.RegisterListener(events.NewRound{}, m.onNewRound)
}

// ---- config and persistence ----

func (m *module) enabled() bool {
	if v, ok := m.plug.Config.Get(`Enabled`).(bool); ok {
		return v
	}
	return true
}

func (m *module) settings() rifts.SiteSettings {
	s := rifts.SiteSettings{}
	switch v := m.plug.Config.Get(`ExcludeZones`).(type) {
	case []string:
		s.ExcludeZones = v
	case []any:
		for _, z := range v {
			if zs, ok := z.(string); ok {
				s.ExcludeZones = append(s.ExcludeZones, zs)
			}
		}
	}
	return s
}

// onLoad restores today's portal sites, if they were saved today. Otherwise
// the first NewRound chooses new ones.
func (m *module) onLoad() {
	m.configureWriter()
	var lost rifts.LostState
	if err := m.plug.ReadIntoStruct(lostStateId, &lost); err != nil && !errors.Is(err, util.ErrStateAbsent) {
		// Never save over a record that could not be read: losses this
		// session are still taken, but not written until it is mended.
		mudlog.Error(`rifts`, `action`, `load lost items`, `error`, err, `saving`, `off this session`)
	} else {
		rifts.RestoreLost(lost)
		rifts.SetLostChanged(m.saveLost)
	}
	var st rifts.SiteState
	if err := m.plug.ReadIntoStruct(sitesStateId, &st); err != nil && !errors.Is(err, util.ErrStateAbsent) {
		mudlog.Error(`rifts`, `action`, `load sites`, `error`, err)
		return
	}
	if rifts.Restore(st) {
		mudlog.Info(`rifts`, `action`, `restored sites`, `day`, st.Day, `sites`, len(st.Sites))
	}
}

// configureWriter installs (or removes) the room writer from the Generate*
// settings.
func (m *module) configureWriter() {
	cfg := buildGenConfig(m.plug.Config.Get)
	if !cfg.Enabled {
		m.writer = nil
		rifts.SetGenerator(nil)
		mudlog.Info(`rifts`, `generated rooms`, `off`, `reason`, `Modules.rifts.GenerateEnabled is false`)
		return
	}
	m.writer = newRoomWriter(cfg)
	rifts.SetGenerator(m.writer)
	s := apiframework.RefreshServer()
	mudlog.Info(`rifts`, `generated rooms`, `on players' own keys`, `chance`, cfg.Chance,
		`moderated`, cfg.ModerateOutput && s.HasKey())
}

func (m *module) onSave() error {
	if m.writer != nil {
		apiframework.SaveBudget()
	}
	return nil
}

func (m *module) saveLost() {
	st := rifts.LostSnapshot()
	if err := m.plug.WriteStruct(lostStateId, &st); err != nil {
		mudlog.Error(`rifts`, `action`, `save lost items`, `error`, err)
	}
}

func (m *module) saveSites() {
	st := rifts.State()
	if err := m.plug.WriteStruct(sitesStateId, &st); err != nil {
		mudlog.Error(`rifts`, `action`, `save sites`, `error`, err)
	}
}

// riftPlace is the roomlife place hook: a rift room's generated ambient
// events come at the profile's own chance, are written for the rift's
// setting rather than the world's, and have no time of day.
func riftPlace(room *rooms.Room) (roomlife.Place, bool) {
	chance, setting, ok := rifts.AmbientPlace(room.RoomId)
	if !ok {
		return roomlife.Place{}, false
	}
	return roomlife.Place{Chance: chance, Setting: setting, Timeless: true}, true
}

// ---- listeners ----

// searchRubble is the search feature-cache hook: `search rubble` in a rift
// room claims the pile's one find, delivered as a bauble through the bauble
// system (named by the model when one is configured, else from the corpus).
func searchRubble(actor actions.Actor, room *rooms.Room, feature actions.SearchFeature) (bool, bool) {
	u := users.GetByUserId(actor.GetUserId())
	if u == nil {
		return false, false
	}
	tier, claimed, handled := rifts.ClaimRubble(u, room.RoomId, feature.Name)
	if claimed {
		actions.StartBaubleFind(u.UserId, room, baubles.ValueTier(tier), baubles.SourceSearch)
	}
	return claimed, handled
}

func (m *module) onRoomChange(e events.Event) events.ListenerReturn {
	if evt, ok := e.(events.RoomChange); ok {
		rifts.OnRoomChange(evt.UserId, evt.FromRoomId, evt.ToRoomId, evt.Unseen)
	}
	return events.Continue
}

func (m *module) onMobDeath(e events.Event) events.ListenerReturn {
	if evt, ok := e.(events.MobDeath); ok {
		rifts.OnMobDeath(evt.RoomId, evt.MobId, evt.InstanceId)
	}
	return events.Continue
}

func (m *module) onPlayerDeath(e events.Event) events.ListenerReturn {
	if evt, ok := e.(events.PlayerDeath); ok {
		rifts.OnPlayerDeath(evt.UserId, evt.RoomId)
	}
	return events.Continue
}

func (m *module) onPlayerDespawn(e events.Event) events.ListenerReturn {
	if evt, ok := e.(events.PlayerDespawn); ok {
		rifts.OnPlayerDespawn(evt.UserId, evt.RoomId)
	}
	return events.Continue
}

func (m *module) onPlayerSpawn(e events.Event) events.ListenerReturn {
	if evt, ok := e.(events.PlayerSpawn); ok {
		rifts.OnPlayerSpawn(evt.UserId)
	}
	return events.Continue
}

func (m *module) onLooking(e events.Event) events.ListenerReturn {
	if evt, ok := e.(events.Looking); ok {
		rifts.OnLook(evt.UserId, evt.RoomId, evt.Target)
	}
	return events.Continue
}

func (m *module) onNewRound(e events.Event) events.ListenerReturn {
	rifts.Sweep()
	if m.writer != nil {
		// A `server set` of a live Generate* setting reaches the next round.
		apiframework.RefreshServer()
		m.writer.setLive(m.plug.Config.Get)
	}
	if m.enabled() && rifts.DailyTick(m.settings()) {
		m.saveSites()
	}
	return events.Continue
}

// ---- player commands ----

func (m *module) answerCommand(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {
	if strings.TrimSpace(rest) == `` {
		user.SendText(messaging.CategorySystem, `Answer what? (<ansi fg="command">answer the moon</ansi>)`)
		return true, nil
	}
	if !rifts.Answer(user, room, rest) {
		user.SendText(messaging.CategorySystem, `Nothing here is waiting for an answer.`)
	}
	return true, nil
}

func (m *module) touchCommand(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {
	if strings.TrimSpace(rest) == `` {
		user.SendText(messaging.CategorySystem, `Touch what?`)
		return true, nil
	}
	// Touching a portal is going through it: the same path as `enter`.
	if exitName, ok := rifts.PortalNoun(room, rest); ok {
		user.Command(`go ` + exitName)
		return true, nil
	}
	if !rifts.Touch(user, room, rest) {
		user.SendText(messaging.CategorySystem, fmt.Sprintf(`You don't see any "%s" here.`, util.EscapeAnsiTags(strings.TrimSpace(rest))))
	}
	return true, nil
}

// ---- admin ----

const riftUsage = `Usage: <ansi fg="command">rift open [profile]</ansi> | <ansi fg="command">rift sites</ansi> | ` +
	`<ansi fg="command">rift rotate</ansi> | <ansi fg="command">rift unsite</ansi> | <ansi fg="command">rift list</ansi> | ` +
	`<ansi fg="command">rift info</ansi> | <ansi fg="command">rift key</ansi> | <ansi fg="command">rift hunt</ansi> | <ansi fg="command">rift lore [read|reset]</ansi> | ` +
	`<ansi fg="command">rift gen [pool]</ansi> | <ansi fg="command">rift lost</ansi> | <ansi fg="command">rift close [run id]</ansi>`

func (m *module) riftAdminCommand(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {
	args := strings.Fields(strings.ToLower(rest))
	if len(args) == 0 {
		user.SendText(messaging.CategorySystem, riftUsage)
		return true, nil
	}
	switch args[0] {
	case `open`:
		ids := rifts.ProfileIds()
		if len(ids) == 0 {
			user.SendText(messaging.CategorySystem, `No rift profiles are loaded.`)
			return true, nil
		}
		profile := ids[0]
		if len(args) > 1 {
			profile = args[1]
		}
		run, err := rifts.OpenPortal(profile, room.RoomId)
		if err != nil {
			user.SendText(messaging.CategorySystem, fmt.Sprintf(`Could not open a rift here: %v. Profiles: %s`, err, strings.Join(ids, `, `)))
			return true, nil
		}
		m.saveSites()
		user.SendText(messaging.CategorySystem, fmt.Sprintf(`Portal site placed here for today; rift run %d (%s) is joinable, entry room %d.`, run.Id, run.Profile.Id, run.EntryRoomId))

	case `sites`:
		all := rifts.Sites()
		if len(all) == 0 {
			user.SendText(messaging.CategorySystem, `No portal sites today.`)
			return true, nil
		}
		user.SendText(messaging.CategorySystem, fmt.Sprintf(`Portal sites for %s:`, rifts.Today()))
		for _, s := range all {
			user.SendText(messaging.CategorySystem, `  `+s.String())
		}

	case `rotate`:
		rifts.Rotate(m.settings())
		m.saveSites()
		user.SendText(messaging.CategorySystem, fmt.Sprintf(`Portal sites moved: %d placed.`, len(rifts.Sites())))

	case `unsite`:
		if !rifts.RemoveSite(room.RoomId) {
			user.SendText(messaging.CategorySystem, `There is no portal site in this room.`)
			return true, nil
		}
		m.saveSites()
		user.SendText(messaging.CategorySystem, `Portal site removed.`)

	case `lore`:
		if len(args) > 1 && args[1] == `read` {
			for _, id := range rifts.ProfileIds() {
				rifts.MakeReader(user, rifts.GetProfile(id))
			}
			user.SendText(messaging.CategorySystem, `You can read the rifts' writing now.`)
			return true, nil
		}
		if len(args) > 1 && args[1] == `reset` {
			for _, id := range rifts.ProfileIds() {
				rifts.ResetLore(user, rifts.GetProfile(id))
			}
			user.SendText(messaging.CategorySystem, `Your rift lore is forgotten.`)
			return true, nil
		}
		for _, id := range rifts.ProfileIds() {
			p := rifts.GetProfile(id)
			user.SendText(messaging.CategorySystem, fmt.Sprintf(`%s: lore studied %d/%d, reader %v, fragments read %v`,
				id, rifts.LoreStudied(user, p), p.Lore.Needed, rifts.IsReader(user, p), rifts.FragmentsRead(user, p)))
		}

	case `list`:
		all := rifts.Runs()
		if len(all) == 0 {
			user.SendText(messaging.CategorySystem, `No rifts are open.`)
			return true, nil
		}
		for _, run := range all {
			members := []string{}
			for uid := range run.Members {
				if u := users.GetByUserId(uid); u != nil {
					members = append(members, u.Character.Name)
				}
			}
			sort.Strings(members)
			user.SendText(messaging.CategorySystem, fmt.Sprintf(`run %d  %s  portal room %d  portal open: %v  rooms: %d  inside: %s`,
				run.Id, run.Profile.Id, run.OriginRoomId, run.PortalOpen, len(run.Rooms), strings.Join(members, `, `)))
		}

	case `info`:
		run, rr := rifts.RoomInfo(room.RoomId)
		if run == nil || rr == nil {
			user.SendText(messaging.CategorySystem, `This room is not part of a rift.`)
			return true, nil
		}
		user.SendText(messaging.CategorySystem, fmt.Sprintf(`run %d  room %d  pool %s  template %s  depth %d  solved %v`,
			run.Id, rr.RoomId, rr.Pool, rr.Template.Id, rr.Depth, rr.PuzzleSolved))
		for _, name := range rr.DoorOrder {
			d := rr.Doors[name]
			user.SendText(messaging.CategorySystem, fmt.Sprintf(`  %-10s -> pool %s  dest %d  locked %v  unlocked %v  sealed %v`,
				name, d.Pool, d.DestRoomId, d.Locked, d.Unlocked, d.Sealed))
		}
		if b := rr.Memory; b != nil {
			user.SendText(messaging.CategorySystem, fmt.Sprintf("Lens table (stage %d, mistakes left %d, dark %v):\n%s",
				b.Stage+1, b.FailsLeft, b.Dark, b.MemorySolution()))
		}

	case `key`:
		run, _ := rifts.RoomInfo(room.RoomId)
		if run == nil {
			user.SendText(messaging.CategorySystem, `Stand in a rift to take its key.`)
			return true, nil
		}
		rifts.GiveKeyTo(user, run)
		user.SendText(messaging.CategorySystem, `You conjure a key.`)

	case `hunt`:
		run, _ := rifts.RoomInfo(room.RoomId)
		if run == nil {
			user.SendText(messaging.CategorySystem, `Stand in a rift to be hunted in it.`)
			return true, nil
		}
		if !run.StartHunt(user) {
			user.SendText(messaging.CategorySystem, `Nothing hunts here, or something already hunts you.`)
		}

	case `lost`:
		// The lost-items record: how much rubble may give back.
		snap := rifts.LostSnapshot()
		user.SendText(messaging.CategorySystem, fmt.Sprintf(`Lost in rifts and not yet found: %d item(s).`, len(snap.Items)))
		for i := len(snap.Items) - 1; i >= 0 && i >= len(snap.Items)-10; i-- {
			li := snap.Items[i]
			user.SendText(messaging.CategorySystem, fmt.Sprintf(`  %s  %s (%s, %s)`, li.Lost, li.Item.DisplayName(), li.Owner, li.How))
		}

	case `gen`:
		// Write a new room for the bank now, on your own key.
		if len(args) < 2 {
			run, _ := rifts.RoomInfo(room.RoomId)
			if run == nil {
				user.SendText(messaging.CategorySystem, `Stand in a rift to see its bank.`)
				return true, nil
			}
			var sb strings.Builder
			sb.WriteString(fmt.Sprintf(`Room bank of %s (authored + generated):`, run.Profile.Id))
			for _, pool := range rifts.AllPools {
				gen := 0
				all := run.Profile.Templates(pool)
				for _, t := range all {
					if t.Source == `generated` {
						gen++
					}
				}
				sb.WriteString(fmt.Sprintf(` %s %d+%d`, pool, len(all)-gen, gen))
			}
			sb.WriteString(`. Usage: <ansi fg="command">rift gen [pool]</ansi> writes a new room for that pool (A, B, C, D or F) on your own key.`)
			user.SendText(messaging.CategorySystem, sb.String())
			return true, nil
		}
		pool := rifts.Pool(strings.ToUpper(args[1]))
		if err := rifts.ForceGenerate(user.UserId, pool); err != nil {
			user.SendText(messaging.CategorySystem, `No room written: `+err.Error()+`.`)
			return true, nil
		}
		user.SendText(messaging.CategorySystem, fmt.Sprintf(`New room for pool %s: being written on your key. The log says when it is saved; `+
			`<ansi fg="command">rift gen</ansi> shows the bank.`, pool))

	case `close`:
		var run *rifts.Run
		if len(args) > 1 {
			if id, err := strconv.Atoi(args[1]); err == nil {
				run = rifts.GetRun(id)
			}
		} else {
			run, _ = rifts.RoomInfo(room.RoomId)
		}
		if run == nil {
			user.SendText(messaging.CategorySystem, `No such rift. `+riftUsage)
			return true, nil
		}
		id := run.Id
		run.ForceEnd()
		user.SendText(messaging.CategorySystem, fmt.Sprintf(`Closed rift run %d.`, id))

	default:
		user.SendText(messaging.CategorySystem, riftUsage)
	}
	return true, nil
}
