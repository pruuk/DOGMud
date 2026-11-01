package usercommands

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/behaviortree"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/menu"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/GoMudEngine/GoMud/internal/term"
	"github.com/GoMudEngine/GoMud/internal/users"
)

func Start(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {

	if user.Character.RoomId != -1 {
		return false, errors.New(`only allowed in the void`)
	}

	// Get if already exists, otherwise create new. (isNew is intentionally
	// unused: the former "You'll need to answer some questions." banner was
	// removed — it was an async line that flushed on top of the pending route
	// question and forced an extra prompt redraw, re-printing the whole
	// multi-line question. The question is self-explanatory on its own.)
	cmdPrompt, _ := user.StartPrompt(`start`, rest)

	if user.Character.SpeciesId == 0 {
		// All players are human in Delusions of Grandeur
		if humanSpecies, ok := species.FindSpecies("human"); ok {
			user.Character.SpeciesId = humanSpecies.Id()
			user.Character.Validate()

			user.SendBlankLine(messaging.CategorySystem)
			user.SendText(messaging.CategorySystem, fmt.Sprintf(`  <ansi fg="magenta">*** Your form takes shape on the world of Gaius ***</ansi>%s`, term.CRLFStr))
		}
	}

	// DOGMud has no separate account/character identity: the character name IS
	// the account username, set at signup (login.go -> SetCharacterName(username)).
	// We deliberately DROP upstream's "choose a character name different from your
	// username" prompt — asking a player to name themselves a second time (and
	// refusing the name they just chose) is the regression that surfaced once the
	// onboarding overhaul routed new players through the void/`start` flow.
	//
	// A normally-created character arrives here with Name == Username, which is
	// correct and needs no prompt. The only case we defend against is a character
	// that somehow reached `start` still carrying the TempName placeholder (legacy
	// save or an odd signup path): default its name to the username rather than
	// prompting. Never ask twice.
	if user.Character.Name == user.TempName() || len(user.Character.Name) == 0 || strings.ToLower(user.Character.Name) == `nameless` {
		// SetCharacterName only fails validation, and the username was already
		// validated at signup, so this cannot realistically error here.
		_ = user.SetCharacterName(user.Username)
		user.SendText(messaging.CategorySystem, fmt.Sprintf(`You will be known as <ansi fg="yellow-bold">%s</ansi>!%s`, user.Character.Name, term.CRLFStr))
	}

	question := cmdPrompt.AskMenu(menu.Menu{
		Heading: "How much of Gaius do you already know?",
		Entries: []menu.Entry{
			{Id: "1", Label: "New to text MUDs", Detail: "I'll teach you the basics first."},
			{Id: "2", Label: "New to DOGMud", Detail: "I know MUDs; show me what's different."},
			{Id: "3", Label: "Veteran", Detail: "Skip all tutorials; drop me into the city."},
		},
		// No Default (owner 2026-10-21): Enter asks again, so a newcomer who
		// only presses Enter never lands on a path that skips the tutorial.
	})
	if !question.Done {
		user.SendText(messaging.CategoryScreen, menu.Render(*question.Menu))
		return true, nil
	}

	// Announce the new character and log its creation ONLY once the route
	// question is answered (i.e. the player is actually entering the world).
	// Start() is re-invoked once per prompt answer, and the CharacterCreated
	// event drives the "X has entered the realm!" broadcast + a help hint. Firing
	// it before/while the question is pending flushed that async output on top of
	// the pending question — and because the question IS the command prompt, each
	// flush forced a redraw that re-rendered the whole multi-line question (the
	// 2026-07-17 "route question shows up twice" report). Deferring it past the
	// answer both fixes the double-render and is semantically correct.
	//
	// Guard so this fires exactly ONCE: the veteran route asks a second (confirm)
	// question, which re-enters Start() with the route question still Done — so
	// without the guard, veterans got a double "entered the realm" + double
	// welcome message.
	if _, announced := cmdPrompt.Recall(`entered`); !announced {
		cmdPrompt.Store(`entered`, true)
		user.EventLog.Add(`char`, fmt.Sprintf(`Created a new character: <ansi fg="username">%s</ansi>`, user.Character.Name))
		events.AddToQueue(events.CharacterCreated{UserId: user.UserId, CharacterName: user.Character.Name})

		// New eyes catch what ours miss, and Gaius is still being built — so ask
		// every new player, once on the way in, to send bugs and ideas (Malia's
		// 2026-07-18 feedback). Route-agnostic: runs for all three onboarding paths.
		// CategorySystem is never wrapped by the pipeline, so the line wraps
		// itself to the reader's width (as justice/arrest.go does, #430).
		user.SendText(messaging.CategorySystem, term.CRLFStr+messaging.WrapAnsi(
			`<ansi fg="magenta-bold">One quick thing before you go.</ansi> Gaius is still being built, and your `+
				`eyes are new, and that makes them valuable. If something breaks or reads `+
				`wrong, type <ansi fg="command">bug</ansi> and tell us what happened. If you `+
				`think of something that would make the world better, type `+
				`<ansi fg="command">suggest</ansi>. We read every one.`,
			user.GetLineWidth())+term.CRLFStr)
	}

	switch onboardingRoute(question.Response) {
	case routeVeteran:
		confirm := cmdPrompt.AskConfirm("Skip everything and start in the city, already Awakened?", "Yes, skip ahead", "No")
		if !confirm.Done {
			user.SendText(messaging.CategoryScreen, menu.Render(*confirm.Menu))
			return true, nil
		}
		if confirm.Response != "yes" {
			// Re-ask from the top. RejectResponse alone would re-show the
			// confirm (GetNextQuestion returns the last question), so clear
			// the prompt and re-enter Start — same pattern as the name decline.
			// The arrival already fired: carry the `entered` flag into the
			// fresh prompt so answering again does not announce it twice.
			user.ClearPrompt()
			fresh, _ := user.StartPrompt(`start`, rest)
			fresh.Store(`entered`, true)
			return Start(rest, user, room, flags)
		}
		user.ClearPrompt()
		autoAwaken(user)
		startVeteranInThornwall(user)
		return true, nil
	case routeNewbie:
		user.ClearPrompt()
		grantNewcomerMarker(user)
		if !startInAntechamber(user) {
			startInCoulee(user)
		}
		return true, nil
	default: // routeMudVet
		user.ClearPrompt()
		startInCoulee(user)
		// Option 2 is "I know MUDs; show me what's different." Point them at
		// the one doc that actually answers that, rather than leaving them to
		// discover it.
		user.SendText(messaging.CategorySystem, fmt.Sprintf(`%sNew to DOGMud but not to MUDs? Type <ansi fg="command">help dogmud</ansi> for what works differently here.%s`, term.CRLFStr, term.CRLFStr))
		return true, nil
	}
}

// ─── Onboarding route ────────────────────────────────────────────────────────

type onboardingRouteKind int

const (
	routeMudVet  onboardingRouteKind = iota // tier B (default): today's Coulee
	routeNewbie                             // tier A: tutorial antechamber
	routeVeteran                            // tier C: skip to Thornwall, Awakened
)

// onboardingRoute maps the poll answer to a route. Anything unrecognized
// (incl. the empty default) maps to the safe mud-vet path.
func onboardingRoute(answer string) onboardingRouteKind {
	switch strings.TrimSpace(answer) {
	case "1":
		return routeNewbie
	case "3":
		return routeVeteran
	default:
		return routeMudVet
	}
}

// ─── Routing helpers ─────────────────────────────────────────────────────────

// startInCoulee drops the character at the Awakening Pool (StartRoom 5200) —
// the standard new-player path (mud-vet, and the newbie fallback if the
// antechamber instance can't be created).
func startInCoulee(user *users.UserRecord) {
	user.SendText(messaging.CategorySystem, fmt.Sprintf(`<ansi fg="magenta">Suddenly, a vortex appears before you, drawing you in before you have any chance to react!</ansi>%s`, term.CRLFStr))
	if destRoom := rooms.LoadRoom(rooms.StartRoomIdAlias); destRoom != nil {
		rooms.MoveToRoom(user.UserId, destRoom.RoomId)
		Look(``, user, destRoom, events.CmdSecretly)
		// The vortex is a MoveToRoom, not a walked entry, so the room's mobs never
		// get the player_enter notification that greeters react to — a route-2
		// player arrived to silence and had to wait for Cleric Hadwen's low-chance
		// ambient backstop before learning to `ask hadwen begin` (2026-07-18
		// feedback). Fire player_enter here, mirroring go.go, so the pool's cleric
		// greets them and points at the rite right away.
		for _, mobInstId := range destRoom.GetMobs(rooms.FindAll) {
			mob := mobs.GetInstance(mobInstId)
			if mob == nil || mob.Character.IsCharmed() {
				continue
			}
			behaviortree.TryMobBehavior(mobInstId, behaviortree.EventContext{
				EventType: "player_enter",
				UserId:    user.UserId,
				RoomId:    destRoom.RoomId,
			})
		}
		// Nothing grants The Awakening by itself (#375 phase 7): Hadwen's
		// greeting above names his offer, and the pool's movement gate
		// (behaviors/rooms/pothole_coulee/5200.yaml) holds the player until
		// the rite is done and handed in.
	}
}

// startInAntechamber drops a new-to-MUDs player into a private, instanced copy
// of the tutorial antechamber (the TutorialRooms). Returns false if the
// instance could not be created (caller falls back to startInCoulee).
func startInAntechamber(user *users.UserRecord) bool {
	cfg := configs.GetSpecialRoomsConfig()
	ids := []int{}
	first := 0
	for i, s := range cfg.TutorialRooms {
		id, err := strconv.Atoi(s)
		if err != nil {
			continue
		}
		ids = append(ids, id)
		if i == 0 {
			first = id
		}
	}
	if len(ids) == 0 || first == 0 {
		return false
	}
	created, err := rooms.CreateEphemeralRoomIds(ids...)
	if err != nil {
		return false
	}
	// Start "New to text MUDs" players at medium combat verbosity (landed hits
	// only). In the tutorial's practice fight this hides the harmless miss/whiff
	// spam — the effigy deals no damage so it never lands a hit — while still
	// showing the player's landed auto-swings and their spike/trip, so Dewey's
	// teaching lines aren't buried in combat noise (2026-07-17 feedback). It's
	// also a gentler default for a first-timer, who can `set combatverbosity
	// full` whenever they want the firehose.
	user.CombatVerbosity = "medium"
	user.SendText(messaging.CategorySystem, fmt.Sprintf(`<ansi fg="magenta">The grey takes you, gentle as sleep...</ansi>%s`, term.CRLFStr))
	rooms.MoveToRoom(user.UserId, created[first])
	if r := rooms.LoadRoom(created[first]); r != nil {
		Look("", user, r, events.CmdSecretly)
	}
	// Dewey's greeting (ending on `talk dewey`) comes from quest 28's
	// room_enter trigger, raised by hooks.QuestRoomEnter for this move once
	// this command returns, so his lines land beneath the arrival look. The
	// quest itself starts at Dewey's offer (#375 phase 7).
	return true
}

// startVeteranInThornwall vortexes an already-Awakened veteran to Thornwall
// (468) and prints the back-door hint.
func startVeteranInThornwall(user *users.UserRecord) {
	user.SendText(messaging.CategorySystem, fmt.Sprintf(`<ansi fg="magenta">The pool is behind you before you ever saw it. You step out already changed, into a city that does not know your name.</ansi>%s`, term.CRLFStr))
	if destRoom := rooms.LoadRoom(468); destRoom != nil { // 468 = Thornwall Temple Interior (the coulee exit destination)
		rooms.MoveToRoom(user.UserId, destRoom.RoomId)
		Look(``, user, destRoom, events.CmdSecretly)
	}
	user.SendText(messaging.CategorySystem, `New to Gaius after all? Type <ansi fg="command">tutorial</ansi> to be taken to the newcomers' pool.`)
}

// grantNewcomerMarker flags a total-newbie character with the hidden
// Newcomer's Path token, which gates the newbie-only teaching beats (Cleric
// Hadwen's mutations/progression talk; Crier Toke's player-interaction talk).
func grantNewcomerMarker(user *users.UserRecord) {
	events.AddToQueue(events.Quest{
		UserId:     user.UserId,
		QuestToken: "21-start",
	})
}

// ─── Veteran auto-awaken ─────────────────────────────────────────────────────

// autoAwaken gives a veteran-tier character the Awakening Rite's world effect
// without making them perform it: the 30-end "Opened" token (clears the 5200
// movement block and Warden Esk's gate). No starter mutation is granted — the
// Awakening opens the capacity, but every mutation is earned through use-based
// drift (blank-slate Awakening; see the mutation-graph acquisition design).
func autoAwaken(user *users.UserRecord) {
	events.AddToQueue(events.Quest{
		UserId:     user.UserId,
		QuestToken: "30-end",
	})
}
