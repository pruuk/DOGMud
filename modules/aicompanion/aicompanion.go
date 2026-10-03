// Package aicompanion drives bonded AI companions: persistent companions
// (characters.CompanionBonded) that talk, remember and form opinions through
// a language model, while acting in the world only through ordinary mob
// commands. See context.md and docs/aicompanion/.
package aicompanion

import (
	"embed"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/GoMudEngine/GoMud/internal/apiframework"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/companionai"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobcommands"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/plugins"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/usercommands"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// helpFiles are the player-facing help pages. They are mounted with the
// commands they describe, so a server with the module off has no help for a
// feature it does not have.
//
//go:embed files/datafiles/*
var helpFiles embed.FS

//go:embed primer.txt
var primerText string

// maxPendingStimuli bounds how many unanswered things a companion holds on
// to while a call is in flight or it is rate limited. Older ones drop off.
const maxPendingStimuli = 6

// controller is the runtime half of one bonded companion: the link between
// its owner, its mob instance and its mind. Controllers exist only while the
// owner is online. Every field is read and written under the mud lock.
type controller struct {
	ownerUserId int
	profile     *Profile
	mind        *Mind

	instanceId int    // live mob instance, 0 while fallen or not yet spawned
	fellRound  uint64 // round the fall was noticed, 0 when standing
	// standRound is the last round sync found her standing beside an owner
	// she belongs to, not sneaking: the fight listener (onCombatRound) only
	// acts for a controller the previous sync passed.
	standRound   uint64
	greeted      bool // session greeting already queued this session
	farewellSaid bool // goodbye already queued for the owner's current quit

	sessionStartUnix    int64         // when this session began (for reflection)
	relaySeen           bool          // the owner's own key was live at some point this session
	lastSocialUnix      int64         // last time anyone spoke or acted socially near it
	lastInitiativeCheck int64         // last time a quiet-spell roll was made
	lastAttackBy        map[int]int64 // attacker user id -> last reaction time

	traces     []traceEntry  // recent decisions, for the admin trace view
	lastPrompt []chatMessage // last request sent, for the admin prompt view
	lastIntent string        // what it meant to do last time
	recentPath []string      // rooms it has just walked through, newest last

	worldRev     uint64        // bumped whenever what it can see changes materially
	cancelCall   func()        // cancels the model call in flight, if any
	convo        *conversation // talk in progress, gathered into one memory at its end
	lastGold     int           // purse as last seen, to spot coin it did not earn
	lastGoldSeen int           // 1 once the purse has been read at least once
	// Coin that turned up in her purse, matched against the GoldGiven events
	// that name who gave it: goldByEvent is given coin not yet seen in the
	// purse, goldUnexplained is purse growth no event has named yet (held
	// from round goldHeldRound), goldHers marks growth seen while she was
	// about her own business (a sale, a loot).
	goldByEvent     int
	goldUnexplained int
	goldHeldRound   uint64
	goldHers        bool
	lastSnapshot    uint64 // round its gear and gold were last copied to the owner's record
	snapshotDue     bool   // something changed its gear or gold; snapshot next round

	leaveAt      uint64 // round it walks away when nothing is left to stay for
	leaveAskedAt int64  // when it asked to part ways, waiting on the owner
	partAskedAt  int64  // when the owner last typed companion-part
	leaveWhy     string

	lastRoomId      int             // room the companion was in last round
	seenKeys        map[string]bool // notable scene things already noticed here
	lastNotice      int64           // last "noticed" stimulus
	lastAutonomy    int64           // last "idle" stimulus
	lastIdleEmote   int64           // last local idle emote
	lastPastime     int64           // last time it found something to do with itself
	lastGearUp      int64           // last time it sorted its gear out
	lastCraft       int64           // last time it set to work at its trade unprompted
	lastSearchFound []string        // what its last search turned up (onSearched)
	searchSaid      map[int]string  // per room, the finds last put to her (onSearched)
	lastSneakTry    uint64          // round it last tried to slip into the shadows
	followPending   bool            // a follow step is on its way and not yet taken
	followSince     uint64          // the round that step was issued
	calledAt        uint64          // the round its owner last called it back by name
	lastErrand      string          // what the last trip was for, so an idle wander is not lingered over

	askAuth         *askAuthority  // the owner's leave to put one question to one NPC
	ownerWasDown    bool           // the owner was on the ground when last looked at
	knownConditions map[int]bool   // what ailed them both when last looked at
	lastAilment     int64          // last time it remarked on one
	lastSavedSeen   int64          // when the mind was last written to disk
	budgetSpent     bool           // its last decision went unpaid for want of allowance
	pendingAct      *pendingAction // issued command awaiting its outcome
	travel          *travelPlan    // trip in progress
	apartSince      uint64         // round it was first found apart from its owner, 0 when together

	inFlightDirect  bool   // the call in flight answers someone speaking to it
	thinkShown      bool   // a thinking gesture was already made for this call
	partyKey        string // owner's party members last seen, sorted names
	partyKnown      bool   // partyKey has been read at least once this session
	ownerTalkedAway int64  // last time the owner spoke to someone else

	fight         *fightState       // fight in progress
	agenda        []int             // goal ids chosen for this session
	skillBase     map[string]string // favoured skill ranks when last checked
	lastGoalCheck uint64            // round goals were last checked

	seq      uint64 // bumps on every dispatch; stale results are dropped
	inFlight bool
	lastCall time.Time
	pending  []stimulus

	paused  bool
	dirty   bool
	lastErr string
}

func (c *controller) push(s stimulus) {
	if c.paused {
		return
	}
	c.pending = append(c.pending, s)
	if len(c.pending) > maxPendingStimuli {
		c.pending = append([]stimulus(nil), c.pending[len(c.pending)-maxPendingStimuli:]...)
	}
}

// AICompanionModule is the module singleton.
type AICompanionModule struct {
	// decisions counts decision calls still running off the mud lock.
	// Cancelling a call only tells it to stop; it still settles and applies
	// under the lock afterwards, so anything that rebuilds the world under
	// it (a test) waits on this first.
	decisions sync.WaitGroup

	plug     *plugins.Plugin
	cfg      Config
	profiles map[string]*Profile
	byMob    map[int]*Profile
	ctrls    map[int]*controller // keyed by owner user id
	minds    map[string]*Mind    // every mind loaded since boot, by mindIdentifier

	// countersDay is the day of the companion's own daily counts (calls,
	// errors, "you notice" moments), on the ledger's clock: the ledger's
	// day is the only day. The allowances and the server's token budget
	// are the ledger's own (apiframework).
	countersDay string
	callsToday  int
	errorsToday int
	lastErrLog  time.Time

	bonds         bondState             // who has met or turned away a companion
	consent       consentLedger         // who has agreed, as the model door reads it
	meetingPlace  map[int]string        // where each first meeting happened
	newcomers     map[int]bool          // new characters not yet told where the Hollow is
	roster        rosterState           // who travels with whom, and what each has learned (roster.go)
	waiting       map[string]*waiter    // companions waiting in the Hollow, by profile id (hollow.go)
	returning     map[string]bool       // companions on their way back to the Hollow, to be seen arriving
	models        modelChooser          // automatic model choice per tier
	stats         map[string]*tierStats // per model tier, since boot
	noticesToday  map[int]int           // "you notice" moments today per owner (NoticeCallsPerDay)
	lastBudgetLog time.Time

	// endpoint, when set, replaces the server's key and endpoint
	// (apiframework.Server) for this module only. Tests point it at a fake
	// provider; production leaves it nil.
	endpoint *apiframework.Endpoint

	// books is the server key's budget and breaker for this module when it
	// is not the shared set (apiframework.Shared): only in tests, where each
	// module under test gets its own (isolateBooks), so a call an earlier
	// test left in flight settles into that test's books, never the next
	// one's. Production leaves it empty: one server, one set of books.
	books atomic.Pointer[apiframework.Books]

	relays     *relayTable    // owners with a live relay for their own key (tier 2)
	relayCalls *pendingRelays // calls waiting on an owner's browser for a reply
	relaySend  relaySender    // how a request reaches the browser; nil is companionai.SendRelay

	// deferredReflect is a relay owner's end-of-session reflection, kept
	// until they are back online with their relay up (one per owner, the
	// newest). Read and written only under the mud lock.
	deferredReflect map[int]*deferredReflection
	// deferredSummaries are a relay owner's finished talks, kept the same
	// way until their relay is up (deferSummary). Under the mud lock.
	deferredSummaries map[int][]*deferredSummary

	tell func(userId int, text string) // how the owner is told things; nil sends a system line
}

var module AICompanionModule

func init() {
	module = AICompanionModule{
		plug:     plugins.New(`aicompanion`, `0.1.0`),
		profiles: map[string]*Profile{},
		byMob:    map[int]*Profile{},
		ctrls:    map[int]*controller{},
		minds:    map[string]*Mind{},

		meetingPlace: map[int]string{},
		newcomers:    map[int]bool{},
		waiting:      map[string]*waiter{},
		returning:    map[string]bool{},
		relays:       newRelayTable(),
		relayCalls:   newPendingRelays(),
	}
	// No data-overlays/config.yaml is shipped. A plugin overlay is pushed
	// into the live config AFTER _datafiles/config.yaml is read, and
	// overwrites it (only config-overrides.yaml is protected), so an
	// overlay would quietly ignore the file an operator edits. Every
	// default lives in buildConfig instead, and is documented in
	// docs/aicompanion/settings.md and in the config.yaml block.

	// Admin only, and registered whether or not the module is switched on,
	// so an operator can always ask it why nothing is happening.
	module.plug.AddUserCommand(`aicompanion`, module.cmdAICompanion, true, true)

	module.plug.Callbacks.SetOnLoad(module.onLoad)
	module.plug.Callbacks.SetOnSave(module.onSave)
}

func (m *AICompanionModule) onLoad() {
	m.cfg = loadConfig(m.plug)

	// The engine raises three events for this module alone, an emote, a
	// heal cast at a creature and gold given to a creature (give.go), and
	// nothing else listens to any of them. Left unheard, every one would be
	// counted and logged as an event nobody handled, so these listeners are
	// registered on or off. All three return at once while the module is
	// off.
	events.RegisterListener(events.Emote{}, m.onEmote)
	events.RegisterListener(events.Healed{}, m.onHealed)
	events.RegisterListener(events.GoldGiven{}, m.onGoldGiven)

	// Switched off, the module stops here: nothing is loaded, no other
	// listener is registered and no seam is installed, so the engine's nil
	// checks find nothing and the server behaves as though this module were
	// not built at all.
	if !m.cfg.Enabled {
		mudlog.Info(`aicompanion`, `enabled`, false,
			`message`, `switched off; set Modules.aicompanion.Enabled: true to use it`)
		return
	}

	profiles, errs := loadProfiles()
	for _, err := range errs {
		mudlog.Error(`aicompanion`, `action`, `loadProfiles`, `error`, err)
	}
	for id, p := range profiles {
		if mobs.GetMobSpec(mobs.MobId(p.MobId)) == nil {
			mudlog.Error(`aicompanion`, `action`, `loadProfiles`, `profile`, id,
				`error`, fmt.Sprintf(`mob template %d does not exist; profile disabled`, p.MobId))
			continue
		}
		if err := p.spellsValid(func(s string) bool { return spells.GetSpell(s) != nil }); err != nil {
			mudlog.Error(`aicompanion`, `action`, `loadProfiles`, `profile`, id, `error`, err)
			continue
		}
		if rooms.LoadRoom(p.Hollow.RoomId) == nil {
			mudlog.Error(`aicompanion`, `action`, `loadProfiles`, `profile`, id,
				`error`, fmt.Sprintf(`hollow room %d does not exist; profile disabled`, p.Hollow.RoomId))
			continue
		}
		m.profiles[id] = p
		m.byMob[p.MobId] = p
	}

	// The player commands, the companion's own mob commands and the help
	// files exist only while the module is on, so a server that leaves it
	// off has no dead commands in its registry, no help pages for a feature
	// that is not there, and nothing advertised to web clients.
	m.registerCommands()

	m.loadBonds()
	m.loadRoster()
	m.loadBudget()
	m.releaseLapsed(time.Now())

	// Her fight runs FIRST on the round, ahead of the combat hook, so a
	// reflex she chose (a bash, a ward, a spell) claims the shared special
	// move cooldown before her behaviour tree's own move can, and is not
	// lost to it every round. Everything else waits for onNewRound.
	events.RegisterListener(events.NewRound{}, m.onCombatRound, events.First)
	events.RegisterListener(events.NewRound{}, m.onNewRound)
	events.RegisterListener(events.CharacterCreated{}, m.onCharacterCreated)
	events.RegisterListener(events.PlayerDespawn{}, m.onPlayerDespawn)
	events.RegisterListener(events.Communication{}, m.onCommunication)
	events.RegisterListener(events.GiftAccepted{}, m.onGiftAccepted)
	events.RegisterListener(events.PlayerAttackedMob{}, m.onPlayerAttackedMob)
	events.RegisterListener(events.MobDeath{}, m.onMobDeath)
	events.RegisterListener(events.PlayerDeath{}, m.onPlayerDeath)
	companionai.SetAskHandler(m.handleAsk)
	companionai.SetShowHandler(m.handleShow)
	companionai.SetBaubleSearcher(m.baubleSearchFor)
	companionai.SetSearchedHandler(m.onSearched)
	companionai.SetBaubleFoundHandler(m.onBaubleFound)
	companionai.SetIdleHandler(m.handleIdle)
	companionai.SetHolder(m.holdFollow)
	companionai.SetBondedCheck(m.isBonded)
	companionai.SetDrivesCheck(m.drivesBonded)
	// Relay messages arrive on connection goroutines. onRelayInbound
	// touches only the relay tables, which have their own locks, and
	// ignores everything while player keys are not on offer.
	companionai.SetRelayInbound(m.onRelayInbound)
	// The key relay page, on its own origin, exists only while player
	// keys are offered.
	m.installRelayPage()

	// The companion is the one feature that can offer a player's own key,
	// so it lends its relay to the others (bauble naming), for what each
	// player allows (apiframework.PurposeFinds).
	apiframework.SetRelay(relayFor{m: m})

	s := apiframework.RefreshServer()
	if s.RejectedBaseURL != `` {
		mudlog.Error(`aicompanion`, `action`, `config`, `error`,
			`BaseURL `+s.RejectedBaseURL+` is not an OpenAI endpoint over https; set AllowCustomEndpoint to use another provider. Using the official endpoint.`)
	}
	if len(s.Legacy) > 0 {
		// Names only, never values: one of them may be the key.
		mudlog.Warn(`aicompanion`, `action`, `config`, `note`,
			`still read from Modules.aicompanion (it works as before): `+strings.Join(s.Legacy, `, `)+
				`. Move them to the APIFramework section, which every feature shares.`)
	}
	mudlog.Info(`aicompanion`, `enabled`, m.cfg.Enabled, `profiles`, len(m.profiles),
		`model`, m.cfg.Model, `apiKeyPresent`, m.apiKey() != ``)
	m.probeModels()
}

// onSave persists every changed mind. The plugin layer routes the write
// through the durable autosave queue.
func (m *AICompanionModule) onSave() error {
	if !m.cfg.Enabled {
		return nil
	}
	m.saveBudget()
	m.saveRoster()
	var firstErr error
	now := time.Now().Unix()
	for _, c := range m.ctrls {
		c.mind.LastSeenUnix = now
		// "When I last saw you" changes every minute and is what her
		// greeting is built on, so it is written even when nothing else
		// happened; otherwise a crash leaves her greeting the owner as
		// though no time had passed since the last thing she said.
		if !c.dirty && now-c.lastSavedSeen < 300 {
			continue
		}
		if err := saveMind(m.plug, c.mind); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue // try again next autosave rather than sit on the change
		}
		c.lastSavedSeen = now
		c.dirty = false
	}
	return firstErr
}

// getMind returns the one Mind for an owner and profile, loading it on first
// use. Minds stay cached for the life of the process, so a reflection that
// lands after its owner logged back in updates the same Mind the new
// session is using instead of a stale copy.
func (m *AICompanionModule) getMind(ownerUserId int, p *Profile) *Mind {
	key := mindIdentifier(ownerUserId, p.MobId)
	if mind, ok := m.minds[key]; ok {
		return mind
	}
	var mind *Mind
	if m.plug == nil {
		mind = newMind(ownerUserId, p) // built without storage, as the tests build her
	} else {
		mind = loadMind(m.plug, ownerUserId, p)
	}
	if m.minds == nil {
		m.minds = map[string]*Mind{}
	}
	m.minds[key] = mind
	return mind
}

// isolateBooks gives every module its own books on first use. Set only by
// this package's TestMain; production never sets it.
var isolateBooks bool

// fw is the server key's budget and breaker this module spends from.
func (m *AICompanionModule) fw() *apiframework.Books {
	if b := m.books.Load(); b != nil {
		return b
	}
	if !isolateBooks {
		return apiframework.Shared()
	}
	m.books.CompareAndSwap(nil, apiframework.NewBooksForTest())
	return m.books.Load()
}

// apiKey is the server's key (apiframework.Server: APIFramework.APIKeyEnv
// or APIKey, shared with every feature). It is never logged.
func (m *AICompanionModule) apiKey() string {
	if m.endpoint != nil {
		return m.endpoint.APIKey
	}
	return apiframework.Server().Endpoint.APIKey
}

// baseURL is the server key's endpoint (APIFramework.BaseURL).
func (m *AICompanionModule) baseURL() string {
	if m.endpoint != nil {
		return m.endpoint.BaseURL
	}
	return apiframework.Server().Endpoint.BaseURL
}

// rollCounters starts the companion's own daily counts afresh when the
// ledger's day has turned. Allowances roll with the ledger itself.
func (m *AICompanionModule) rollCounters() {
	if day := m.fw().Day(); day != m.countersDay {
		m.countersDay = day
		m.callsToday = 0
		m.errorsToday = 0
		m.noticesToday = map[int]int{}
	}
}

// countCall counts one model call started today.
func (m *AICompanionModule) countCall() {
	m.rollCounters()
	m.callsToday++
}

// modelReady reports whether a model call may be made right now for this
// owner's companion, on the owner's own account (modelReadyFor with no
// passer-by). ownerId 0 is no owner: never a relay, and no per-companion
// check.
func (m *AICompanionModule) modelReady(ownerId ...int) bool {
	owner := 0
	if len(ownerId) > 0 {
		owner = ownerId[0]
	}
	return m.modelReadyFor(owner, 0)
}

// modelReadyFor reports whether a call may be made right now, routed by the
// owner (route) whoever prompted it. On the owner's own key (tier 2) the
// server's budgets and breaker do not apply; the owner's breaker is part of
// the route. On the server's key (tier 3) the global breaker must be closed
// and the server's budget unspent, and a call on the owner's account
// (askerId 0) also needs the owner's companion allowance: a passer-by's is
// weighed when the call is reserved (reserveRoute), not here.
func (m *AICompanionModule) modelReadyFor(ownerId int, askerId int) bool {
	switch m.route(ownerId).kind {
	case routeRelay:
		return true
	case routeNone:
		return false
	}
	if m.breakerOpen(time.Now()) {
		return false
	}
	if !m.fw().HasRoom() {
		return false
	}
	if askerId <= 0 && ownerId > 0 && !m.ownerBudgetLeft(ownerId) {
		return false
	}
	return true
}

// bondedCompanionOf returns the owner's bonded companion record that has a
// loaded profile, or nil.
func (m *AICompanionModule) bondedCompanionOf(u *users.UserRecord) (*characters.CompanionInfo, *Profile) {
	if u == nil || u.Character == nil {
		return nil, nil
	}
	for i := range u.Character.Companions {
		c := &u.Character.Companions[i]
		if c.SourceType != characters.CompanionBonded {
			continue
		}
		if p, ok := m.byMob[c.MobId]; ok {
			return c, p
		}
	}
	return nil, nil
}

// controllerForInstance finds the controller driving a mob instance.
func (m *AICompanionModule) controllerForInstance(instanceId int) *controller {
	if instanceId <= 0 {
		return nil
	}
	for _, c := range m.ctrls {
		if c.instanceId == instanceId {
			return c
		}
	}
	return nil
}

// sortedOwnerIds gives a stable iteration order for status output.
func (m *AICompanionModule) sortedOwnerIds() []int {
	ids := make([]int, 0, len(m.ctrls))
	for id := range m.ctrls {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	return ids
}

func (m *AICompanionModule) logModelError(err error) {
	if errors.Is(err, errServerResting) {
		return // held back for a breaker's probe: no call, no error
	}
	m.errorsToday++
	// One line per ten seconds at most; an outage must not flood the log.
	if time.Since(m.lastErrLog) < 10*time.Second {
		return
	}
	m.lastErrLog = time.Now()
	mudlog.Warn(`aicompanion`, `action`, `modelCall`, `error`, err.Error(), `errorsToday`, m.errorsToday)
}

// bumpWorld marks that what the companion can see has changed materially:
// it moved, a fight started or ended, it fell, or it was respawned. A model
// reply built on the old picture is dropped rather than acted on.
func (c *controller) bumpWorld() {
	c.worldRev++
}

// cancelInFlight stops a model call whose answer can no longer be used, so
// it does not run on and spend tokens. The call's token reservation is
// settled by the goroutine itself, whichever way it ends.
func (c *controller) cancelInFlight() {
	if c.cancelCall != nil {
		c.cancelCall()
		c.cancelCall = nil
	}
	c.inFlight = false
}

// strangersOff reports whether passers-by may prompt no model calls for the
// owner's companion right now, on whichever key would pay for them now.
func (m *AICompanionModule) strangersOff(ownerId int) bool {
	return m.strangersOffOn(ownerId, m.route(ownerId))
}

// strangersOffOn is strangersOff for a call on route r. The owner's own
// word (companion-ai strangers on|off) decides; without it, passers-by are
// off on the owner's own key, since it is the owner who pays and they
// never agreed to pay for strangers, and on for the server's key, as
// before player keys existed.
func (m *AICompanionModule) strangersOffOn(ownerId int, r route) bool {
	if rec := m.bonds.Users[ownerId]; rec != nil {
		if rec.StrangersOff {
			return true
		}
		if rec.StrangersOn {
			return false
		}
	}
	return r.kind == routeRelay
}

// strangerMayPrompt reports whether a call prompted by this passer-by may
// be made for the owner's companion at all. askerId 0 is the owner's own
// call. With strangers off she still hears a passer-by and answers them
// with her set lines, but nothing they say starts a call on anyone's key.
func (m *AICompanionModule) strangerMayPrompt(ownerId int, askerId int) bool {
	if askerId > 0 && m.cfg.RequirePlayerKey && !m.hasOwnKey(askerId) {
		return false // she speaks only to players with a key of their own
	}
	return askerId <= 0 || !m.strangersOff(ownerId)
}

// isBonded reports whether a mob instance is a bonded companion this module
// drives. Used by the engine to leave its own per-template opinion score
// alone for companions, which keep their feelings in their own mind.
func (m *AICompanionModule) isBonded(mobInstanceId int) bool {
	if !m.cfg.Enabled {
		return false
	}
	return m.controllerForInstance(mobInstanceId) != nil
}

// drivesBonded reports whether this module drives the bonded companions of
// a mob template: it is on and has their profile, so sync takes each one
// up for its owner (bondedCompanionOf). The engine's dismiss asks it, per
// companion, before refusing; a bonded companion with no profile here
// would otherwise be one its owner could neither dismiss nor talk to.
func (m *AICompanionModule) drivesBonded(mobId int) bool {
	if !m.cfg.Enabled {
		return false
	}
	_, ok := m.byMob[mobId]
	return ok
}

// registerCommands puts the player and companion commands into the live
// registries. Called from onLoad, after the config is read, so nothing is
// registered on a server that never switches the module on.
// usercommands.RegisterCommand is used directly rather than the plugin
// helper because only it can say a command is allowed in combat.
func (m *AICompanionModule) registerCommands() {
	usercommands.RegisterCommand(`companion-unstick`, m.cmdUnstick, false, true, false)
	usercommands.RegisterCommand(`companion-part`, m.cmdPart, false, true, false)
	usercommands.RegisterCommand(`companion-court`, m.cmdCourt, false, false, false)
	usercommands.RegisterCommand(`companion-boundary`, m.cmdBoundary, false, true, false)
	usercommands.RegisterCommand(`companion-ask`, m.cmdAskFor, false, false, false)
	usercommands.RegisterCommand(`companion-stay`, m.cmdStay, false, true, false)
	usercommands.RegisterCommand(`companion-ai`, m.cmdAI, false, true, false)

	// mobcommands.RegisterCommand, not plug.AddMobCommand: the plugin
	// helper only fills a map that plugins.Load copies into the registry
	// before onLoad runs, so a command added here would never arrive and
	// the companion would be told it does not know how to do it.
	mobcommands.RegisterCommand(cmdCompanionLoot, mobCompanionLoot, false)
	mobcommands.RegisterCommand(cmdCompanionTakeout, mobCompanionTakeout, false)
	mobcommands.RegisterCommand(cmdCompanionUnlock, mobCompanionUnlock, false)
	mobcommands.RegisterCommand(cmdCompanionBuy, mobCompanionBuy, false)
	mobcommands.RegisterCommand(cmdCompanionFollow, mobCompanionFollow, false)

	// The help pages are mounted with the commands they describe.
	if err := m.plug.AttachFileSystem(helpFiles); err != nil {
		mudlog.Error(`aicompanion`, `action`, `attachHelp`, `error`, err)
	}
}
