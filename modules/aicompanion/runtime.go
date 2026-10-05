package aicompanion

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"sort"
	"strings"
	"time"

	"github.com/GoMudEngine/GoMud/internal/apiframework"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/companionai"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/parties"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// Locking model, the one rule this file lives by:
//
//   - Event listeners, the ask handler and admin commands already run under
//     the mud lock. They must never take it again (it is not reentrant).
//   - dispatch and startReflection start goroutines for the HTTP call. Those
//     goroutines hold no game pointers, and take util.LockMud() only around
//     applyResult / applyReflection.
//   - A controller's seq is bumped on every dispatch, detach, fall and
//     pause, so a reply that lands after its owner logged out, or after a
//     newer call started, is dropped instead of acted on.

// onNewRound keeps controllers in step with who is online and whose
// companion is standing, handles recovery from death, and dispatches any
// pending decisions.
func (m *AICompanionModule) onNewRound(e events.Event) events.ListenerReturn {
	evt, ok := e.(events.NewRound)
	if !ok || !m.cfg.Enabled {
		return events.Continue
	}
	// The server key's settings, re-read here on the game loop, where the
	// config is written (apiframework.RefreshServer).
	apiframework.RefreshServer()
	m.sync(evt.RoundNumber)
	if evt.RoundNumber%600 == 0 {
		m.releaseLapsed(time.Now())
	}
	m.tendHollow(evt.RoundNumber)
	m.dispatchAll()
	return events.Continue
}

// onCombatRound runs each standing companion's fight. It is registered
// First on the round, ahead of the engine's combat hook: her behaviour tree
// runs inside that hook and claims the shared special move cooldown, so a
// reflex chosen here after it would always find the cooldown spent and be
// dropped. Only a controller the previous round's sync passed (owner online,
// she is theirs, standing, not sneaking) is run.
func (m *AICompanionModule) onCombatRound(e events.Event) events.ListenerReturn {
	evt, ok := e.(events.NewRound)
	if !ok || !m.cfg.Enabled {
		return events.Continue
	}
	for userId, c := range m.ctrls {
		if c.instanceId == 0 || c.standRound == 0 || c.standRound+1 < evt.RoundNumber {
			continue
		}
		u := users.GetByUserId(userId)
		if u == nil || u.Character == nil || m.sneaking(c, u) {
			continue
		}
		m.combatTick(c, u, evt.RoundNumber)
	}
	return events.Continue
}

func (m *AICompanionModule) onPlayerDespawn(e events.Event) events.ListenerReturn {
	evt, ok := e.(events.PlayerDespawn)
	if !ok {
		return events.Continue
	}
	if c, found := m.ctrls[evt.UserId]; found {
		m.detach(c, evt.CharacterName)
	}
	// Their browser is closing: no call may wait on it or be routed to it.
	m.relayGone(evt.UserId)
	return events.Continue
}

// sync attaches a controller to every online owner of a bonded companion
// and detaches controllers whose owner has gone.
func (m *AICompanionModule) sync(round uint64) {
	now := time.Now()
	seen := map[int]bool{}

	for _, u := range users.GetAllActiveUsers() {
		m.hintHollow(u)
		comp, p := m.bondedCompanionOf(u)
		if comp == nil {
			// The roster says this character has her, but it carries no
			// record of her (dismissed while the module was off, or parted
			// before her mind took her up): she would never come back to
			// the Hollow, and they could never win anyone else. Free her.
			if held := m.heldBy(u.UserId); held != nil && m.holdsHer(held, u) {
				m.unclaim(held, partSentAway)
				mudlog.Info(`aicompanion`, `action`, `release`, `profile`, held.Id, `owner`, u.UserId, `reason`, `no record of her`)
			}
			continue
		}
		// There is one of each companion, and the roster says who has her.
		// A record that still carries one the roster gives to nobody else
		// (an owner away too long, an admin release, a companion fielded
		// before there was one of each, another character on the same
		// account) loses her here.
		if !m.holdsHer(p, u) {
			m.reclaimFrom(u, p)
			continue
		}
		m.seeOwner(p, now.Unix())
		seen[u.UserId] = true

		c := m.ctrls[u.UserId]
		if c == nil || c.profile.MobId != p.MobId {
			if c != nil {
				m.detach(c, u.Character.Name)
			}
			c = &controller{
				ownerUserId:      u.UserId,
				profile:          p,
				mind:             m.getMind(u.UserId, p),
				sessionStartUnix: now.Unix(),
				lastSocialUnix:   now.Unix(),
				lastAttackBy:     map[int]int64{},
			}
			m.ctrls[u.UserId] = c
			restoreAssist(c.mind, comp)
			// Still not right after being killed: coming back through the
			// door does not mend it.
			if mob := mobs.GetInstance(comp.InstanceId); mob != nil && now.Unix() < c.mind.FallenUntilUnix {
				if half := mob.Character.EffectivePoolMax(characters.PoolHealth) / 2; half > 0 && mob.Character.Health > half {
					mob.Character.ApplyCost(characters.PoolHealth, mob.Character.Health-half)
				}
				c.mind.addLine(Line{Kind: `event`, Text: `You are still not right after being killed.`}, m.cfg.WorkingMemoryLines)
				c.dirty = true
			}
		}

		// An owner on their own key this session reflects on it later,
		// through their relay, not at logout (detachReflection). A
		// reflection kept from an earlier session starts here, on the
		// round, once they are back with their relay up.
		if m.route(u.UserId).kind == routeRelay {
			c.relaySeen = true
			m.startDueReflection(u.UserId)
			m.startDueSummaries(u.UserId)
		}

		if comp.InstanceId == 0 {
			m.handleFallen(c, u, p, round, now)
			continue
		}

		c.instanceId = comp.InstanceId
		c.fellRound = 0

		if !c.greeted {
			c.greeted = true
			c.mind.seedAmbitions(p, now.Unix())
			if mob := mobs.GetInstance(c.instanceId); mob != nil {
				seedRecipes(mob, p)
				c.mind.ensureRestockGoals(mob, p, now.Unix())
				c.skillBase = skillSnapshot(mob, &p.Archetype)
			}
			c.agenda = c.mind.pickAgenda()
			c.mind.SessionCount++
			if c.mind.FirstMetUnix == 0 {
				m.firstMet(c, u, now.Unix())
			} else if m.cfg.GreetOnLogin {
				elapsed := int64(0)
				if c.mind.LastSeenUnix > 0 {
					elapsed = now.Unix() - c.mind.LastSeenUnix
				}
				c.push(stimulus{Kind: `session_start`, ElapsedSecs: elapsed, FromOwner: true})
			}
			c.dirty = true
		}

		// Too far gone: trust and affection both at the bottom. She does
		// not ask to go, she goes, a couple of rounds from now.
		if c.leaveAt == 0 && m.abandons(c.mind.Opinion) {
			c.leaveAt = round + 2
			if c.leaveWhy == `` {
				c.leaveWhy = partCantBear
			}
		}
		if c.leaveAt > 0 && round >= c.leaveAt {
			m.leave(c, u, c.leaveWhy)
			continue
		}
		// A leave the owner never confirmed lapses quietly: she stays.
		if c.leaveAskedAt > 0 && now.Unix()-c.leaveAskedAt > int64(m.cfg.LeaveConfirmSeconds) {
			c.leaveAskedAt, c.partAskedAt, c.leaveWhy = 0, 0, ``
			c.mind.Autonomy = autonomyNormal
			c.dirty = true
		}

		if m.sneaking(c, u) {
			// Nothing at all while they are moving in secret.
			continue
		}

		m.maybeThink(c, now)
		m.watchParty(c, u)
		m.tendPurpose(c, round, now)
		c.standRound = round // her fight is run by onCombatRound
		m.snapshotIfDue(c, round)

		// Goodbye. `quit` starts condition 0 (a short meditation) and the
		// owner leaves when it expires, taking the companion with them. That
		// window is the only time a farewell can be heard, so it jumps the
		// queue. If the quit is cancelled the flag resets for next time.
		if u.Character.HasCondition(0) {
			if !c.farewellSaid {
				c.farewellSaid = true
				c.pending = nil
				c.push(stimulus{Kind: `farewell`, FromOwner: true})
				if !c.inFlight {
					m.dispatch(c)
				}
			}
		} else {
			c.farewellSaid = false
			m.perceive(c, u, round, now)
			m.maybeInitiative(c, u, now)
		}

		m.decayMood(c, now)
	}

	for id, c := range m.ctrls {
		if !seen[id] {
			name := ``
			if u := users.GetByUserId(id); u != nil && u.Character != nil {
				name = u.Character.Name
			}
			m.detach(c, name)
		}
	}
}

// firstMet is the session in which she first travels with her owner: the
// date is kept whatever they have agreed to, the memory (their name) only
// once they have agreed, and she introduces herself in her own words only
// then; agreeing later writes it and queues that introduction
// (keepFirstMeeting, from companion-ai on).
func (m *AICompanionModule) firstMet(c *controller, u *users.UserRecord, now int64) {
	c.mind.FirstMetUnix = now
	m.keepFirstMeeting(c, u)
}

// keepFirstMeeting writes the first meeting into her mind and queues her
// introduction, once: only after the meeting has happened (FirstMetUnix)
// and only once her owner has agreed. Agreeing again later, after turning
// it off, is not a second first meeting.
func (m *AICompanionModule) keepFirstMeeting(c *controller, u *users.UserRecord) {
	if c.mind.FirstMetUnix == 0 || c.mind.FirstMetKept || !m.mayRemember(c) || u == nil || u.Character == nil {
		return
	}
	c.mind.FirstMetKept = true
	c.dirty = true
	// A mind from before FirstMetKept existed has the memory already.
	for _, mem := range c.mind.Memories {
		if strings.HasPrefix(mem.Text, `I started travelling with `) {
			return
		}
	}
	c.mind.addMemory(Memory{
		Unix: c.mind.FirstMetUnix,
		Kind: `event`, Text: `I started travelling with ` + u.Character.Name + `.`,
		Importance: 7, Emotion: `curiosity`, People: []string{u.Character.Name},
	}, m.cfg.MaxMemories)
	c.push(stimulus{Kind: `first_meeting`, Text: m.meetingPlace[u.UserId], FromOwner: true})
	delete(m.meetingPlace, u.UserId)
}

// handleFallen notices a fall and brings the companion back once it has
// recovered.
func (m *AICompanionModule) handleFallen(c *controller, u *users.UserRecord, p *Profile, round uint64, now time.Time) {
	// Only a controller that had a live instance treats a missing one as a
	// fall; at login the instance may simply not be spawned yet.
	if c.instanceId > 0 && c.fellRound == 0 {
		// A fight ends with the fall; put back the owner's assist setting
		// if the companion was holding back (the instance is already gone,
		// so it is restored from the mind).
		c.fight = nil
		c.bumpWorld()
		c.cancelInFlight()
		if comp, _ := m.bondedCompanionOf(u); comp != nil {
			restoreAssist(c.mind, comp)
		}
		c.mind.markDanger(c.lastRoomId, 3, now.Unix(), false)
		c.travel = nil
		c.pendingAct = nil
		c.fellRound = round
		c.instanceId = 0
		c.pending = nil
		c.seq++
		c.inFlight = false
		c.mind.DeathCount++
		// Kept in the mind, not in the session, so logging out and back in
		// cannot shorten it.
		c.mind.FallenUntilUnix = now.Unix() + int64(m.cfg.RecoveryRounds)*4
		c.mind.LastDeathUnix = now.Unix()
		c.mind.addLine(Line{Kind: `event`, Text: `You were beaten unconscious in a fight.`}, m.cfg.WorkingMemoryLines)
		if m.mayRemember(c) {
			c.mind.addMemory(Memory{
				Kind: `death`, Text: `I was beaten unconscious in a fight while travelling with ` + u.Character.Name + `.`,
				Importance: 8, Emotion: `fear`, People: []string{u.Character.Name},
			}, m.cfg.MaxMemories)
		}
		c.dirty = true
	}
	if c.fellRound > 0 && round-c.fellRound >= uint64(m.cfg.RecoveryRounds) {
		if id := companionai.RespawnBonded(u.UserId, p.MobId); id > 0 {
			c.instanceId = id
			c.fellRound = 0
			c.bumpWorld()
			if mob := mobs.GetInstance(id); mob != nil {
				seedRecipes(mob, p)
			}
			c.mind.addLine(Line{Kind: `event`, Text: `You recovered and rejoined your companion.`}, m.cfg.WorkingMemoryLines)
			c.push(stimulus{Kind: `recovered`, FromOwner: true})
			c.dirty = true
		}
	}
}

// maybeInitiative lets the companion start a conversation after a quiet
// spell (F2.8). It only considers it when the owner is with it, nobody is
// fighting, and nothing is already waiting; whether it does is a roll
// against the profile's talkativeness, and the model may still choose
// silence.
func (m *AICompanionModule) maybeInitiative(c *controller, u *users.UserRecord, now time.Time) {
	if m.cfg.InitiativeMinutes <= 0 || c.paused || c.inFlight || len(c.pending) > 0 || c.instanceId == 0 {
		return
	}
	if now.Unix()-c.lastSocialUnix < int64(m.cfg.InitiativeMinutes)*60 {
		return
	}
	if now.Unix()-c.lastInitiativeCheck < 60 {
		return
	}
	c.lastInitiativeCheck = now.Unix()

	mob := mobs.GetInstance(c.instanceId)
	if mob == nil || mob.Character.RoomId != u.Character.RoomId {
		return
	}
	if mob.Character.IsInCombat() || u.Character.IsInCombat() {
		return
	}
	// The warmer it feels toward its owner, the likelier it is to break a
	// silence; a companion that has gone cold keeps its own counsel.
	chance := c.profile.talkativeness() * (1.0 + float64(c.mind.Opinion.Affection)/150.0)
	if chance > 0.95 {
		chance = 0.95
	}
	if float64(util.Rand(1000))/1000.0 >= chance {
		return
	}
	c.lastSocialUnix = now.Unix()
	c.push(stimulus{Kind: `quiet`, FromOwner: true})
}

// decayMood lets a mood the model set drift back to calm when nothing has
// renewed it for MoodDecayMinutes.
func (m *AICompanionModule) decayMood(c *controller, now time.Time) {
	if m.cfg.MoodDecayMinutes <= 0 || c.mind.Mood == `calm` || c.mind.MoodSetUnix == 0 {
		return
	}
	if now.Unix()-c.mind.MoodSetUnix >= int64(m.cfg.MoodDecayMinutes)*60 {
		c.mind.Mood = `calm`
		c.mind.MoodSetUnix = 0
		c.dirty = true
	}
}

// detach ends a session: time together counts toward the relationship, the
// mind is saved, and a private reflection on the session is started. Any
// reply still in flight is orphaned by the seq bump and dropped when it
// lands.
func (m *AICompanionModule) detach(c *controller, ownerName string) {
	c.seq++
	c.cancelInFlight()
	m.closeConversation(c, `they logged out`)
	if c.fight != nil {
		m.setHoldBack(c, users.GetByUserId(c.ownerUserId), false)
		c.fight = nil
	}
	now := time.Now().Unix()
	c.mind.LastSeenUnix = now
	m.keepFrom(c)
	if c.profile != nil {
		m.seeOwner(c.profile, now)
	}
	m.saveRoster()

	// Time spent together (F5.2): a session of ten minutes or more nudges
	// affection and trust up a little, only while they are still modest.
	if now-c.sessionStartUnix >= 600 {
		d := Opinion{}
		if c.mind.Opinion.Affection < 60 {
			d.Affection = 1
		}
		if c.mind.Opinion.Trust < 40 {
			d.Trust = 1
		}
		c.mind.applyOpinion(d, `time_together`, `rule`, `a session spent together`, false)
	}

	if m.plug == nil {
		// built without storage, as the tests build her
	} else if err := saveMind(m.plug, c.mind); err != nil {
		mudlog.Error(`aicompanion`, `action`, `saveMind`, `owner`, c.ownerUserId, `error`, err)
	}
	if n := m.cfg.BackupEverySessions; m.plug != nil && n > 0 && c.mind.SessionCount%n == 0 {
		if err := saveMindBackup(m.plug, c.mind); err != nil {
			mudlog.Error(`aicompanion`, `action`, `saveMindBackup`, `owner`, c.ownerUserId, `error`, err)
		}
	}
	delete(m.ctrls, c.ownerUserId)

	if ownerName == `` {
		ownerName = `your companion`
	}
	m.detachReflection(c, ownerName)
}

func (m *AICompanionModule) dispatchAll() {
	minGap := time.Duration(m.cfg.MinSecondsBetweenCalls) * time.Second
	for _, c := range m.ctrls {
		if c.paused || c.inFlight || len(c.pending) == 0 || c.instanceId == 0 {
			continue
		}
		if time.Since(c.lastCall) < minGap {
			continue
		}
		m.dispatch(c)
	}
}

// recallContextFor gathers what the moment is about, for memory retrieval.
func recallContextFor(mob *mobs.Mob, owner *users.UserRecord, stims []stimulus, now time.Time) recallContext {
	ctx := recallContext{
		NowUnix:  now.Unix(),
		People:   map[string]bool{},
		Keywords: map[string]bool{},
	}
	if owner != nil && owner.Character != nil {
		ctx.People[strings.ToLower(owner.Character.Name)] = true
	}
	for _, s := range stims {
		if s.Speaker != `` {
			ctx.People[strings.ToLower(s.Speaker)] = true
		}
		for w := range keywordsOf(s.Text) {
			ctx.Keywords[w] = true
		}
	}
	if mob != nil {
		ctx.PlaceId = mob.Character.RoomId
		if room := rooms.LoadRoom(mob.Character.RoomId); room != nil {
			for w := range keywordsOf(room.Title) {
				ctx.Keywords[w] = true
			}
		}
	}
	return ctx
}

// dispatch turns a controller's pending stimuli into one decision: a model
// call when one is available, otherwise an immediate fallback.
func (m *AICompanionModule) dispatch(c *controller) {
	if len(c.pending) == 0 {
		return
	}
	// One prompter per decision: her owner's words never share a call with
	// a passer-by's, which wait for the next one (see nextBatch).
	stims, rest := nextBatch(c.pending, c.ownerUserId)
	c.pending = rest
	// Whoever prompted this pays for it: a passer-by from their own daily
	// allowance, never the owner's.
	asker := strangerBehind(stims, c.ownerUserId)

	mob := mobs.GetInstance(c.instanceId)
	owner := users.GetByUserId(c.ownerUserId)
	if mob == nil || owner == nil {
		return
	}

	if !m.consented(c.ownerUserId) {
		// They have not agreed that their words may be sent anywhere. She
		// is an ordinary companion until they do.
		m.fallback(c, mob, stims)
		return
	}
	// A stranger's question is not refused because her owner's allowance is
	// spent (it is not theirs to spend); the server's budget and the
	// breaker still apply on the server's key, and the stranger's own
	// allowance is weighed when the call is reserved. It is routed by her
	// owner either way: on the owner's own key, the owner's key pays.
	// Her owner has asked that passers-by start no calls: she answers
	// them with her set lines.
	if !m.strangerMayPrompt(c.ownerUserId, asker) || !m.modelReadyFor(c.ownerUserId, asker) {
		m.fallback(c, mob, stims)
		return
	}

	now := time.Now()
	// A fight, a thing noticed or a quiet moment needs a quick answer, not
	// a considered one, so those calls carry a shorter prompt: fewer
	// memories, fewer lines, and none of the sections that only matter in
	// conversation.
	brief := tierFor(stims) == tierFast
	memoryCount, lineCount, optionCount := m.cfg.PromptMemories, m.cfg.PromptMemoryLines, m.cfg.OptionsInPrompt
	if brief {
		memoryCount, lineCount, optionCount = memoryCount/2, lineCount/2, optionCount/2
		if memoryCount < 2 {
			memoryCount = 2
		}
		if lineCount < 6 {
			lineCount = 6
		}
		if optionCount < 4 {
			optionCount = 4
		}
	}
	memories := selectMemories(c.mind.Memories, recallContextFor(mob, owner, stims, now), memoryCount)
	c.mind.markRecalled(memories, now.Unix())

	summaries := c.mind.Summaries
	if len(summaries) > 2 {
		summaries = summaries[len(summaries)-2:]
	}

	// The scene the model chooses from. It travels with the call so the
	// reply is validated against exactly what the model was shown.
	sc := buildScene(mob, owner, c.profile, c.mind, now.Unix())

	in := promptInput{
		Profile:      c.profile,
		Primer:       primerText,
		OwnerName:    owner.Character.Name,
		Mood:         c.mind.Mood,
		SessionCount: c.mind.SessionCount,
		FirstMetUnix: c.mind.FirstMetUnix,
		Now:          now,
		Opinion:      c.mind.Opinion,
		Memories:     memories,
		Reflections:  c.mind.reflections(3),
		Summaries:    summaries,
		Facts:        c.mind.Facts,
		OpenPromises: c.mind.openPromises(),
		Lines:        c.mind.lastLines(lineCount),
		Brief:        brief,
		Situation:    describeSituation(mob, owner),
		Stimuli:      stims,
		Options:      sc.describeOptions(optionCount),
		LootRule:     c.mind.LootRule,
		Impressions:  append(impressionLines(sc, c.mind), peopleLines(sc, c.mind)...),
		Places:       nearbyPlaces(c.mind.Map, mob.Character.RoomId, m.cfg.NearbyPlacesInPrompt, now.Unix()),
		Unexplored:   unexploredExits(c.mind.Map[mob.Character.RoomId]),
		Hearsay:      hearsayLines(c.mind, now.Unix()),
		Trip:         tripWords(c.travel),
		Frontier:     frontierPlaces(c.mind.Map, mob.Character.RoomId, 3),
		OwnPhrases:   c.mind.OwnPhrases,
		Craft:        craftLines(c.profile, skillWords(mob, &c.profile.Archetype)),
		Recipes:      craftLinesFor(craftableHere(mob, c.profile, rooms.LoadRoom(mob.Character.RoomId))),
		RecipesAway:  craftableElsewhere(mob, c.profile, rooms.LoadRoom(mob.Character.RoomId)),
		Spells:       spellLines(spellsReady(mob)),
		Pack:         packLines(mob, c.profile, c.mind),
		Prices:       priceLines(mob, c.profile, c.mind, now.Unix()),
		Goals:        goalLines(c.mind, c.agenda),
		Autonomy:     c.mind.Autonomy,
		Core:         coreLines(c.mind, now.Unix()),
		Conditions:   conditionLines(mob, owner, owner.Character != nil && !cannotSeeOwner(mob, owner)),
		Factions:     factionLines(rooms.LoadRoom(mob.Character.RoomId), mob, owner),
		Quests:       questLines(ownerIfPresent(mob, owner), 4),
		Talk:         talkLines(rooms.LoadRoom(mob.Character.RoomId), mob, m.cfg.RoadTalkLines),
		Romance:      romanceLines(c.mind, c.profile, owner.Character.Name),
		Intimacy:     intimacyGuidance(c.mind, c.profile),
		Capabilities: capabilityWordsFor(m.cfg, c.profile),
		LastIntent:   c.lastIntent,
		Doing:        doingLines(c),
	}
	if c.fight != nil {
		if room := rooms.LoadRoom(mob.Character.RoomId); room != nil {
			in.Fight = fightLines(c.fight, room, mob, owner, c.profile.ammoWord())
		}
	}

	tier := tierFor(stims)
	toolRounds := 0
	if tier == tierMain && asker == 0 {
		// A passer-by gets her answer without her stopping to consult the
		// game first: those rounds are what make a call's worst case
		// several times its prompt, and held against StrangerDailyTokens
		// they could leave no room for a single question.
		toolRounds = m.cfg.ToolRounds
	}
	ts := m.settingsFor(tier, toolRounds > 0)

	call := modelCall{
		BaseURL:     m.baseURL(),
		APIKey:      m.apiKey(),
		Model:       ts.Model,
		Timeout:     ts.Timeout,
		MaxTokens:   ts.MaxTokens,
		Temperature: m.cfg.Temperature,
		SchemaName:  `companion_decision`,
		Schema:      decisionSchema(),
		Effort:      ts.Effort,
		Retry:       m.cfg.RetryTransient && tier != tierFast,
		OwnerUserId: c.ownerUserId,
	}
	m.applyRoute(&call)
	rt := call.Route
	// Through her owner's own browser the owner can read the whole prompt,
	// so it carries nothing another player did not show or say to them.
	if rt.kind == routeRelay {
		in.Lines = relaySafeLines(in.Lines, in.OwnerName, c.profile.Name)
		in.Stimuli = relaySafeStimuli(in.Stimuli)
	}
	messages := buildMessages(in)
	call.Messages = messages
	if rt.kind == routeNone || (asker > 0 && m.strangersOffOn(c.ownerUserId, rt)) {
		// Her owner's relay went away since modelReadyFor, and there is no
		// server key to cover: nothing to call. Or the relay came up since
		// strangerMayPrompt, and on the owner's own key a passer-by prompts
		// nothing unless the owner said so: the route the call really
		// carries decides.
		m.fallback(c, mob, stims)
		return
	}
	if call.Model == `` {
		// Every model this tier knows has been refused by the API. Rather
		// than hammer one that will not answer, she falls back to her own
		// lines until an operator sets a model or the key is fixed.
		m.logModelError(fmt.Errorf(`no usable model for the %s tier; set Model in the config`, tier))
		m.fallback(c, mob, stims)
		return
	}
	c.lastPrompt = messages
	// Moderation is the server's check on what the server's key bought. A
	// reply from a player's own key is forgeable by that player anyway, and
	// their provider may have no moderation endpoint; the owner answers for
	// it instead.
	moderation := m.cfg.ModerateOutput && rt.kind == routeServer
	moderationModel := m.cfg.ModerationModel
	// Words a passer-by prompted are held to the stricter rule: if the
	// check cannot be made, they are not said at all.
	strictModeration := asker > 0

	c.seq++
	seq := c.seq
	rev := c.worldRev
	roomAtCall := mob.Character.RoomId
	ownerId := c.ownerUserId
	// Hold this call's worst case against both budgets in one step, and
	// give it up if it does not fit: checking and charging apart is what
	// let two calls slip past a nearly spent budget together.
	// Whoever prompted this pays for it in their own daily allowance: a
	// passer-by cannot spend an owner's companion into silence (asker,
	// above). The settlement below goes back to the same payer, by the
	// same route: on her owner's own key only a passer-by's allowance is
	// held (reserveRoute).
	reserved := worstCaseTokens(estimateTokens(messages)+requestOverhead(call), ts.MaxTokens, toolRounds, call.Retry)
	held, ok := m.reserveRoute(rt, ownerId, asker, reserved)
	if !ok {
		// Out of allowance, not out of sorts: without this line a spent
		// budget looks exactly like a broken companion, because she carries
		// on answering with her authored lines and nothing is logged. A
		// passer-by's spent allowance is theirs, not her owner's, so it does
		// not mark her as spent.
		if asker == 0 {
			c.budgetSpent = true
		}
		m.logBudgetRefusal(ownerId, asker, reserved, held.refusal)
		c.seq++ // the decision is abandoned, not merely delayed
		m.fallback(c, mob, stims)
		return
	}
	if asker == 0 {
		c.budgetSpent = false
	}
	ctx, cancel := context.WithCancel(context.Background())
	c.cancelCall = cancel
	call.Ctx = ctx
	c.inFlight = true
	c.lastCall = now
	c.thinkShown = false
	c.inFlightDirect = false
	for _, s := range stims {
		if s.Kind == `heard` || s.Kind == `asked` {
			c.inFlightDirect = true
		}
	}
	m.countCall()

	m.decisions.Add(1)
	go func() {
		defer m.decisions.Done()
		// The breakers' leave is always handed back last: a no-op once its
		// outcome was recorded, and what frees a half-open breaker's probe
		// if a panic kept the outcome from ever being recorded.
		var tk apiframework.Ticket
		call.ticketOut = &tk
		defer func() { m.fw().Release(apiframework.ConsumerCompanion, tk) }()
		settled := false
		used := 0 // what the call spent, once it is known
		defer func() {
			if r := recover(); r != nil {
				mudlog.Error(`aicompanion`, `action`, `modelCall`, `panic`, r, `stack`, string(debug.Stack()))
			}
			// The call never reached the settlement below (it panicked
			// before the lock, or the goroutine was torn down): give the
			// tokens back, or the day's budget drains on calls that never
			// happened, and keep what one that did happen spent.
			if !settled {
				util.LockMud()
				defer util.UnlockMud()
				m.settleRoute(held, used)
				if c := m.ctrls[ownerId]; c != nil && c.seq == seq {
					c.inFlight = false
					c.cancelCall = nil
				}
			}
		}()

		res := m.callWithTools(call, ownerId, seq, rev, sc, toolRounds, &used)
		used = res.Tokens

		// Parse and moderate here, off the game loop.
		if res.Err == nil {
			d, err := parseDecision(res.Content)
			if err != nil {
				res.ParseErr = err
			} else {
				if moderation {
					res.Moderated = m.moderateDecision(ownerId, &d, call.BaseURL, call.APIKey, moderationModel, 5*time.Second, strictModeration)
				}
				res.Parsed = &d
			}
		}

		// The lock is taken and given back inside a closure with a deferred
		// unlock, and the reservation is settled in a defer of its own, so
		// a panic while applying the result still releases the game loop,
		// still settles exactly once, and still frees the companion to
		// think again. settled is set first: a panic inside settleRoute
		// itself must not let the outer cleanup settle a second time.
		func() {
			util.LockMud()
			defer util.UnlockMud()
			defer func() {
				settled = true
				m.settleRoute(held, res.Tokens)
				if c := m.ctrls[ownerId]; c != nil && c.seq == seq {
					c.inFlight = false
					c.cancelCall = nil
				}
			}()
			m.applyResult(ownerId, seq, rev, roomAtCall, reserved, stims, sc, tier, call.Model, rt, res)
		}()
	}()
}

// applyResult acts on a model reply. Runs under the mud lock.
// applyResult applies one model reply. Its caller owns the mud lock, the
// token settlement and the in-flight flags (see dispatch), so a panic in
// here cannot leave the budget or the companion stuck.
func (m *AICompanionModule) applyResult(ownerId int, seq uint64, rev uint64, roomAtCall int, reserved int, stims []stimulus, sc *scene, tier string, model string, rt route, res modelResult) {
	m.rollCounters()
	m.recordCall(tier, res)
	failure := res.Err
	if failure == nil && res.ParseErr != nil {
		failure = fmt.Errorf(`parse decision: %w`, res.ParseErr)
	}
	// A call the module itself gave up on (logout, pause, a move that made
	// the answer useless) is not the provider failing, and must not count
	// towards the circuit breaker: ordinary play would otherwise switch the
	// AI off for everyone. Nor is it a success: reporting it as one would
	// reset the count of real failures, so a breaker could never open for
	// an owner who keeps resetting her. It reaches no breaker at all.
	if res.Canceled {
		failure = nil
		if rt.kind != routeRelay {
			m.fw().Release(apiframework.ConsumerCompanion, res.Ticket)
		}
	} else {
		m.routeResult(rt, ownerId, res.Ticket, failure, time.Now())
	}
	// A model the player's provider refused says nothing about the server's
	// choice of models.
	if rt.kind == routeServer && modelRefused(res) {
		m.models.refuse(model)
		mudlog.Warn(`aicompanion`, `action`, `modelRefused`, `tier`, tier, `model`, model, `next`, m.models.pick(tier))
	}

	c := m.ctrls[ownerId]
	if c == nil || c.seq != seq {
		return
	}
	c.mind.TokensLifetime += int64(res.Tokens)

	mob := mobs.GetInstance(c.instanceId)
	if mob == nil {
		return
	}

	// The world moved while the model was thinking: the reply is about a
	// place, a fight or a body that is no longer there. None of it is used,
	// neither the words nor the change of mind behind them. The room is
	// checked directly as well as through the revision, because a move can
	// land between one round's bookkeeping and the next.
	if c.worldRev != rev || mob.Character.RoomId != roomAtCall {
		c.addTrace(traceEntry{Unix: time.Now().Unix(), Tier: tier, Model: model, Triggers: stimulusKinds(stims),
			Tokens: res.Tokens, Latency: res.Latency, Err: `dropped: the moment had passed`})
		return
	}

	trace := traceEntry{Unix: time.Now().Unix(), Tier: tier, Model: model, Triggers: stimulusKinds(stims),
		Tokens: res.Tokens, Latency: res.Latency, Tools: res.ToolsUsed}
	if failure != nil || res.Parsed == nil {
		if failure == nil {
			failure = fmt.Errorf(`no decision`)
		}
		if errors.Is(failure, errServerResting) {
			// Held back while another call probes a half-open breaker: no
			// call was made, so no error to show or count; set lines.
			m.fallback(c, mob, stims)
			return
		}
		c.lastErr = failure.Error()
		trace.Err = c.lastErr
		c.addTrace(trace)
		m.logModelError(failure)
		m.fallback(c, mob, stims)
		return
	}
	c.lastErr = ``
	raw := *res.Parsed
	if len(raw.Speech) == 0 && res.Moderated > 0 {
		// Everything it meant to say was flagged: an authored reply instead.
		m.fallback(c, mob, stims)
	}

	d := sanitizeDecision(raw, mob.Character.Name, c.mind.Mood)
	m.speak(c, mob, d.Speech, rt)

	now := time.Now().Unix()
	owner := users.GetByUserId(c.ownerUserId)

	// One action, after the words. The command waits until the speech has
	// been said, so "Here, take this" comes before the hand-over.
	actDelay := 0.5
	for _, l := range d.Speech {
		actDelay += 1.0 + float64(len(l.Text))/60.0
	}
	outcome := actionOutcome{}
	if c.pendingAct == nil || d.Action.Verb == `look_at` || d.Action.Verb == `consider` || d.Action.Verb == `find_place` || d.Action.Verb == `sayto` || d.Action.Verb == `browse` {
		outcome = m.performAction(c, mob, owner, sc, d.Action, stims, actDelay, util.GetRoundCount())
		if d.Action.Verb == `sayto` && outcome.Issued {
			// Words to someone, spoken aloud: logged like any other line.
			logSpeech(rt, c.ownerUserId, c.profile.Name, `sayto`, d.Action.Query)
		}
	} else if d.Action.Verb != `none` {
		outcome.Refused = `still busy with the last thing`
	}
	if outcome.Pending != nil {
		c.pendingAct = outcome.Pending
	}
	// A refusal is something the companion learns, not just a log line: it
	// went to do a thing and found it could not. Without this it would try
	// the same thing again next moment.
	if outcome.Refused != `` && d.Action.Verb != `` && d.Action.Verb != `none` {
		c.mind.addLine(Line{Kind: `event`, Text: fmt.Sprintf(`You went to %s, but you could not: %s.`,
			strings.ReplaceAll(d.Action.Verb, `_`, ` `), outcome.Refused)}, m.cfg.WorkingMemoryLines)
		c.dirty = true
	}
	// A look or a size-up gets one follow-up, so the companion can react to
	// what it learned. Never a second, so it cannot chain looks forever.
	if outcome.Perceived != `` && !isFollowUp(stims) {
		c.push(lookedFollowUp(stims, c.ownerUserId, outcome))
	}

	m.applyImpression(c, sc, d.Impression, now)
	// A standing arrangement changes at the owner's word, not a passer-by's.
	if d.LootRule != `unchanged` && d.LootRule != `` && d.LootRule != c.mind.LootRule && ownerAskedNow(stims) {
		c.mind.LootRule = d.LootRule
		if owner != nil && owner.Character != nil {
			c.mind.addMemory(Memory{Unix: now, Kind: `event`, Text: lootRuleWords(d.LootRule, owner.Character.Name),
				Importance: 5, Emotion: `neutral`, People: []string{owner.Character.Name}}, m.cfg.MaxMemories)
		}
	}
	if d.Mood != `` && d.Mood != c.mind.Mood {
		c.mind.Mood = d.Mood
		c.mind.MoodSetUnix = now
	}

	ownerName := ``
	if owner != nil && owner.Character != nil {
		ownerName = owner.Character.Name
	}

	if d.PlaceTip != `` {
		from := ownerName
		for _, s := range stims {
			if (s.Kind == `heard` || s.Kind == `asked`) && s.Speaker != `` {
				from = s.Speaker
				break
			}
		}
		c.mind.addHearsay(from, d.PlaceTip, now)
	}

	if c.fight != nil {
		m.applyCombatProposal(c, d.Combat, owner)
	}

	// Parting ways is a request, not a deed: the companion says its piece
	// and hangs back. Only the owner confirming with `companion-part` ends
	// the bond, unless the relationship has collapsed entirely, in which
	// case it goes of its own accord a couple of rounds later.
	if d.Leave && c.leaveAskedAt == 0 && leaveRequestAllowed(c.mind.Opinion, stims) && owner != nil {
		if collapsed(c.mind.Opinion) {
			c.leaveAt = util.GetRoundCount() + 2
			c.leaveWhy = d.Intent
		} else {
			m.requestLeave(c, owner, d.Intent)
		}
	}

	if d.Goal.Action != `none` && d.Goal.Action != `` {
		goalResult := c.mind.applyGoalProposal(d.Goal, ownerAskedNow(stims), now)
		if d.Goal.Action == `add` && strings.HasPrefix(goalResult, `added`) {
			c.agenda = c.mind.pickAgenda()
		}
	}
	if d.Autonomy != `unchanged` && d.Autonomy != `` && d.Autonomy != c.mind.Autonomy && ownerAskedNow(stims) {
		c.mind.Autonomy = d.Autonomy
		c.mind.addLine(Line{Kind: `event`, Text: `You agreed how far to roam: ` + d.Autonomy + `.`}, m.cfg.WorkingMemoryLines)
	}

	remembered := false
	if d.Memory.Text != `` {
		room := rooms.LoadRoom(mob.Character.RoomId)
		place := ``
		if room != nil {
			place = room.Title
		}
		people := []string{}
		if ownerName != `` {
			people = append(people, ownerName)
		}
		for _, s := range stims {
			if s.Speaker != `` && !strings.EqualFold(s.Speaker, ownerName) {
				people = append(people, s.Speaker)
			}
		}
		mem := Memory{
			Unix: now, Kind: `conversation`, Text: d.Memory.Text,
			Importance: d.Memory.Importance, Emotion: d.Memory.Emotion,
			People: people, Place: place, PlaceId: mob.Character.RoomId,
		}
		// Mid-conversation, this is held until the talk is over and summed
		// up as one memory (conversation.go). Anything weighty is written
		// at once, in case the talk is cut short.
		if m.holdMemory(c, mem) {
			remembered = true
		} else {
			remembered = c.mind.addMemory(mem, m.cfg.MaxMemories)
		}
	}

	// A fact is only "told" when the owner actually said something in this
	// moment, and it carries their words with it. Anything else is the
	// companion's own reading, kept at lower confidence.
	source, confidence, quote := `observed`, `low`, ``
	for _, s := range stims {
		if s.FromOwner && (s.Kind == `heard` || s.Kind == `asked`) && strings.TrimSpace(s.Text) != `` {
			source, confidence, quote = `told`, `medium`, cleanText(s.Text, 200)
		}
	}
	for _, f := range d.Facts {
		c.mind.addFact(Fact{Unix: now, Text: f, Source: source, Confidence: confidence, Quote: quote}, m.cfg.MaxFacts)
	}

	m.applyPromise(c, d.Promise, ownerName)

	// Only her own companion's deeds move how she feels about them. A
	// passer-by can talk to her all day and it changes nothing.
	applied := Opinion{}
	if env, ok := envelopeFor(stims); ok {
		proposed := Opinion{Trust: d.Opinion.Trust, Respect: d.Opinion.Respect, Affection: d.Opinion.Affection}
		fromWords := wordsOnly(stims)
		recentWarmWords := 0
		if fromWords {
			recentWarmWords = c.mind.positiveWordChangesSince(now - 3600)
		}
		bounded := boundDelta(proposed, env, c.profile.sensitivity(), recentWarmWords)
		applied = c.mind.applyOpinion(bounded, stimulusKinds(stims), `model`, d.Opinion.Reason, fromWords)
	}

	c.dirty = true
	c.lastIntent = d.Intent
	trace.Intent, trace.Lines, trace.Action = d.Intent, len(d.Speech), strings.TrimSpace(d.Action.Verb+` `+d.Action.Ref)
	c.addTrace(trace)
	m.traceDecision(c, stims, d, remembered, applied, outcome, res)
}

// isFollowUp reports whether a decision was itself a follow-up.
func isFollowUp(stims []stimulus) bool {
	for _, s := range stims {
		if s.Chain > 0 {
			return true
		}
	}
	return false
}

// applyPromise records a promise, or settles an open one. Promises move
// trust mainly when they are settled (F5.5): an owner keeping their word
// builds trust, breaking it costs more.
func (m *AICompanionModule) applyPromise(c *controller, p PromiseProposal, ownerName string) {
	switch p.Kind {
	case `made_by_me`:
		c.mind.addPromise(`me`, p.Text)
	case `made_by_them`:
		c.mind.addPromise(`them`, p.Text)
	case `kept`, `broken`:
		pr, ok := c.mind.resolvePromise(p.Ref, p.Kind)
		if !ok {
			return
		}
		who := `I`
		if pr.By == `them` {
			who = ownerName
		}
		verb := `kept`
		emotion := `gratitude`
		importance := 6
		if p.Kind == `broken` {
			verb = `broke`
			emotion = `hurt`
			importance = 7
		}
		c.mind.addMemory(Memory{
			Kind: `promise`, Text: fmt.Sprintf(`%s %s a promise: %s`, who, verb, pr.Text),
			Importance: importance, Emotion: emotion, People: []string{ownerName},
		}, m.cfg.MaxMemories)
		// The model's judgement that a promise was kept or broken is not
		// something the game saw happen, so it moves trust gently. Deeds
		// the game did see (gifts, attacks, healing) move it more.
		if pr.By == `them` && p.Kind == `kept` {
			m.noteMilestone(c, `promise_kept`)
		}
		if pr.By == `them` {
			d := Opinion{Trust: 2}
			if p.Kind == `broken` {
				d = Opinion{Trust: -3, Affection: -1}
			}
			c.mind.applyOpinion(d, `promise_`+p.Kind, `model`, pr.Text, false)
		}
	}
}

func stimulusKinds(stims []stimulus) string {
	kinds := make([]string, 0, len(stims))
	for _, s := range stims {
		kinds = append(kinds, s.Kind)
	}
	return strings.Join(kinds, `,`)
}

// traceDecision writes one log line per decision so "why did she do that?"
// can be answered. It records the trigger kinds, the model's private intent
// and what changed, never what players said and never the API key.
func (m *AICompanionModule) traceDecision(c *controller, stims []stimulus, d Decision, remembered bool, applied Opinion, act actionOutcome, res modelResult) {
	if !m.cfg.LogDecisions {
		return
	}
	mudlog.Info(`aicompanion`, `action`, `decision`,
		`owner`, c.ownerUserId,
		`companion`, c.profile.Id,
		`triggers`, stimulusKinds(stims),
		`intent`, d.Intent,
		`lines`, len(d.Speech),
		`mood`, c.mind.Mood,
		`remembered`, remembered,
		`facts`, len(d.Facts),
		`promise`, d.Promise.Kind,
		`action`, d.Action.Verb+` `+d.Action.Ref,
		`actionIssued`, act.Issued,
		`actionRefused`, act.Refused,
		`goal`, d.Goal.Action,
		`combat`, d.Combat.Stance+` `+d.Combat.Target,
		`opinion`, fmt.Sprintf(`%+d/%+d/%+d`, applied.Trust, applied.Respect, applied.Affection),
		`tokens`, res.Tokens,
		`latencyMs`, res.Latency.Milliseconds(),
		`moderated`, res.Moderated,
		`toolsUsed`, res.ToolsUsed,
	)
}

// spokenLines is what she may say of lines: all of them, or nothing when
// her owner is muted (or cannot be found). What she says is her owner's to
// answer for, on every tier: a say and an emote are both free text, so the
// owner's mute silences both.
func spokenLines(owner *users.UserRecord, lines []SpeechLine) []SpeechLine {
	if owner == nil || owner.Muted {
		return nil
	}
	return lines
}

// ownerSilenced reports whether her authored words (a battle line, an idle
// gesture, a thinking gesture) are to go unsaid: her owner is muted, or
// cannot be found. It is spokenLines' rule for what no model wrote.
func ownerSilenced(ownerUserId int) bool {
	owner := users.GetByUserId(ownerUserId)
	return owner == nil || owner.Muted
}

// speechLogLine is the log record for one line she says through her
// owner's own key, or nil when the line is not logged. Nothing moderates
// the owner's key (their provider may have no moderation, and the reply
// could be forged anyway), so each line is logged against the owner who
// answers for it. The server's key passed moderation, and set lines are
// authored, so neither is logged.
func speechLogLine(rt route, ownerId int, name string, kind string, text string) []any {
	if rt.kind != routeRelay {
		return nil
	}
	return []any{`action`, `speech`, `owner`, ownerId, `companion`, name, `kind`, kind, `text`, text}
}

// logSpeech logs one line she said through her owner's own key.
func logSpeech(rt route, ownerId int, name string, kind string, text string) {
	if attrs := speechLogLine(rt, ownerId, name, kind, text); attrs != nil {
		mudlog.Info(`aicompanion`, attrs...)
	}
}

// speak issues what she says as ordinary commands, spaced so a reply reads
// like someone talking rather than a block of text. A long line is broken
// at sentence ends into pieces a screen can hold, each said in turn with a
// pause, so a story arrives the way someone telling one would deliver it.
// rt is the route the words came by: routeNone for her set lines.
func (m *AICompanionModule) speak(c *controller, mob *mobs.Mob, lines []SpeechLine, rt route) {
	lines = spokenLines(users.GetByUserId(c.ownerUserId), lines)
	if len(lines) == 0 {
		return
	}
	spoken := 0
	for i, l := range lines {
		delay := 0.5
		if i > 0 {
			delay = 1.0 + float64(len(lines[i-1].Text))/60.0
		}

		pieces := []string{l.Text}
		if l.Kind == `say` {
			pieces = speakInChunks(l.Text, maxSayChunkRunes)
		}
		for _, piece := range pieces {
			if spoken >= maxSayChunks {
				break
			}
			spoken++
			mob.Command(l.Kind+` `+util.EscapeAnsiTags(piece), delay)
			logSpeech(rt, c.ownerUserId, c.profile.Name, l.Kind, piece)
			// The next piece waits for this one to have been read: about a
			// second and a half, and longer for a longer piece.
			delay = 1.4 + float64(len(piece))/45.0
		}

		kind := `said`
		if l.Kind == `emote` {
			kind = `emoted`
		}
		line := Line{Speaker: c.profile.Name, Kind: kind, Text: l.Text, Unix: time.Now().Unix()}
		c.mind.addLine(line, m.cfg.WorkingMemoryLines)
		c.mind.addOwnPhrase(l.Text, 12)
		if mob != nil {
			m.noteConversation(c, mob.Character.RoomId, ``, 0, line)
		}
	}
	if len(lines) > 0 {
		c.lastSocialUnix = time.Now().Unix()
		c.dirty = true
	}
}

// fallback answers without a model: a short authored emote from the
// profile, and only when something called for a response.
func (m *AICompanionModule) fallback(c *controller, mob *mobs.Mob, stims []stimulus) {
	// She speaks only to players who bring their own key: to anyone else,
	// her owner included, she gives no answer at all, not even set lines.
	if m.cfg.RequirePlayerKey {
		who := strangerBehind(stims, c.ownerUserId)
		if who == 0 {
			who = c.ownerUserId
		}
		if !m.hasOwnKey(who) {
			return
		}
	}
	var pool []string
	for _, s := range stims {
		switch s.Kind {
		case `first_meeting`, `session_start`:
			pool = c.profile.Fallback.Greetings
		case `farewell`:
			if len(c.profile.Fallback.Farewells) > 0 {
				pool = c.profile.Fallback.Farewells
			} else {
				pool = c.profile.Fallback.Replies
			}
		case `recovered`:
			if len(c.profile.Fallback.Recovered) > 0 {
				pool = c.profile.Fallback.Recovered
			} else {
				pool = c.profile.Fallback.Replies
			}
		case `heard`, `asked`, `emote`, `gift`, `healed`:
			if pool == nil {
				pool = c.profile.Fallback.Replies
			}
		}
	}
	if len(pool) == 0 {
		return
	}
	line := pool[util.Rand(len(pool))]
	m.speak(c, mob, []SpeechLine{{Kind: `emote`, Text: cleanText(line, maxEmoteRunes)}}, route{kind: routeNone})
}

// maybeThink makes a small authored "thinking" gesture when a reply to
// someone speaking to the companion is slow (F2.11), so a pause reads as a
// pause and not as being ignored. Once per call, no model involved.
func (m *AICompanionModule) maybeThink(c *controller, now time.Time) {
	if !c.inFlight || !c.inFlightDirect || c.thinkShown || m.cfg.ThinkingSeconds <= 0 || ownerSilenced(c.ownerUserId) {
		return
	}
	if now.Sub(c.lastCall) < time.Duration(m.cfg.ThinkingSeconds)*time.Second {
		return
	}
	c.thinkShown = true
	lines := c.profile.ThinkingEmotes
	if len(lines) == 0 {
		lines = []string{`considers that for a moment.`}
	}
	mob := mobs.GetInstance(c.instanceId)
	if mob == nil {
		return
	}
	line := cleanText(lines[util.Rand(len(lines))], maxEmoteRunes)
	if line != `` {
		mob.Command(`emote ` + util.EscapeAnsiTags(line))
	}
}

// watchParty notices the owner joining, leaving or changing a party, and
// gives the companion a moment to react. Polled each round rather than
// driven by PartyUpdated, because that event is not sent for every way a
// party can change.
func (m *AICompanionModule) watchParty(c *controller, u *users.UserRecord) {
	key, names := partyOf(u)
	if !c.partyKnown {
		c.partyKnown = true
		c.partyKey = key
		return
	}
	if key == c.partyKey {
		return
	}
	c.partyKey = key
	text := u.Character.Name + ` is no longer travelling in a group.`
	if len(names) > 0 {
		text = u.Character.Name + ` is now travelling in a group with ` + strings.Join(names, `, `) + `.`
	}
	if m.mayRemember(c) {
		c.mind.addLine(Line{Kind: `event`, Text: text}, m.cfg.WorkingMemoryLines)
		c.dirty = true
	}
	c.push(stimulus{Kind: `party`, Text: text, FromOwner: true})
}

// partyOf returns a stable key for the owner's party and the names of the
// other members, sorted.
func partyOf(u *users.UserRecord) (string, []string) {
	p := parties.Get(u.UserId)
	if p == nil {
		return ``, nil
	}
	var names []string
	for _, id := range p.GetMembers() {
		if id == u.UserId {
			continue
		}
		if other := users.GetByUserId(id); other != nil && other.Character != nil {
			names = append(names, other.Character.Name)
		}
	}
	sort.Strings(names)
	return strings.Join(names, `,`), names
}

// tendPurpose runs every ten rounds for a standing companion: it checks
// goals against the game, turns supply shortfalls into restock goals,
// forgets protection for items it no longer has, and notices skills that
// have grown (phase 5). None of it calls the model; what matters is queued
// as a stimulus.
func (m *AICompanionModule) tendPurpose(c *controller, round uint64, now time.Time) {
	if round < c.lastGoalCheck+10 {
		return
	}
	c.lastGoalCheck = round
	if c.convo != nil && time.Now().Unix()-c.convo.LastUnix > int64(m.cfg.ConversationGapSeconds) {
		m.closeConversation(c, `it went quiet`)
	}
	mob := mobs.GetInstance(c.instanceId)
	if mob == nil {
		return
	}
	nowUnix := now.Unix()

	done := c.mind.checkGoals(mob, nowUnix)
	if len(done) > 0 {
		c.agenda = c.mind.pickAgenda()
	}
	for _, g := range done {
		c.mind.addMemory(goalDoneMemory(g), m.cfg.MaxMemories)
		c.mind.addLine(Line{Kind: `event`, Text: `You finished something you set out to do: ` + g.Text + `.`}, m.cfg.WorkingMemoryLines)
		c.push(stimulus{Kind: `goal_done`, Text: g.Text})
		c.dirty = true
	}
	if added := c.mind.ensureRestockGoals(mob, c.profile, nowUnix); len(added) > 0 {
		c.agenda = c.mind.pickAgenda()
		c.dirty = true
	}
	reconcileProtected(c.mind, mob)
	if len(c.mind.Protected) > 0 && c.mind.SessionCount >= 4 {
		m.noteMilestone(c, `keepsake`)
	}
	if u := users.GetByUserId(c.ownerUserId); u != nil {
		m.tendRomance(c, u, nowUnix)
	}

	after := skillSnapshot(mob, &c.profile.Archetype)
	if c.skillBase != nil {
		if gains := skillGains(c.skillBase, after); len(gains) > 0 {
			text := strings.Join(gains, `; `)
			c.mind.addMemory(Memory{Unix: nowUnix, Kind: `event`, Text: `I noticed I have got better: ` + text + `.`,
				Importance: 6, Emotion: `pride`}, m.cfg.MaxMemories)
			c.push(stimulus{Kind: `grew`, Text: text})
			c.dirty = true
		}
	}
	c.skillBase = after
}

// snapshotIfDue copies the companion's gear, gold and progression into the
// owner's record every SnapshotRounds, and straight after anything changed
// them, so the engine's autosave and shutdown save carry them.
func (m *AICompanionModule) snapshotIfDue(c *controller, round uint64) {
	if !c.snapshotDue && round < c.lastSnapshot+uint64(m.cfg.SnapshotRounds) {
		return
	}
	if companionai.Snapshot(c.ownerUserId) {
		c.lastSnapshot = round
		c.snapshotDue = false
		m.keepFrom(c)
	}
}

// keepFrom copies what a companion has learned into the roster, so it goes
// with her if she leaves this owner while they are away (roster.go).
func (m *AICompanionModule) keepFrom(c *controller) {
	if c == nil || c.profile == nil || m.holderOf(c.profile.Id) != c.ownerUserId {
		return
	}
	if mob := mobs.GetInstance(c.instanceId); c.instanceId > 0 && mob != nil {
		m.keepProgress(c.profile, progressOfMob(mob))
		return
	}
	if u := users.GetByUserId(c.ownerUserId); u != nil {
		if comp, p := m.bondedCompanionOf(u); comp != nil && p.Id == c.profile.Id {
			m.keepProgress(c.profile, progressOfInfo(comp))
		}
	}
}

// callWithTools runs one decision, letting the model ask the game up to
// toolRounds rounds of read-only questions first (look closer, size up,
// wares, recall, find a place). Runs on the model goroutine; the answers
// are read under the mud lock, which is released before the next call.
// spent, when not nil, is kept up to date with what the rounds so far
// cost, so a panic while answering still settles what was billed.
func (m *AICompanionModule) callWithTools(call modelCall, ownerId int, seq uint64, rev uint64, sc *scene, toolRounds int, spent *int) modelResult {
	tokens, used := 0, 0
	latency := time.Duration(0)
	for round := 0; ; round++ {
		if toolRounds > 0 {
			call.Tools = toolSpecs()
			call.ToolChoice = `auto`
			if round >= toolRounds {
				call.ToolChoice = `none` // time to answer
			}
		}
		res := m.callModel(call)
		call.ticket, call.admitted = res.Ticket, res.Admitted // one leave for every round
		tokens += res.Tokens
		if spent != nil {
			*spent = tokens
		}
		latency += res.Latency
		res.Tokens, res.Latency, res.ToolsUsed = tokens, latency, used
		if res.Err != nil || len(res.ToolCalls) == 0 || round >= toolRounds {
			if len(res.ToolCalls) > 0 && res.Err == nil {
				res.Err = fmt.Errorf(`model kept asking questions instead of answering`)
			}
			return res
		}

		var answers []string
		var ok bool
		func() {
			util.LockMud()
			defer util.UnlockMud()
			answers, ok = m.answerTools(ownerId, seq, rev, sc, res.ToolCalls, call.Route.kind == routeRelay)
		}()
		if !ok {
			res.Err = fmt.Errorf(`companion changed while the model was asking`)
			return res
		}
		used += len(res.ToolCalls)
		call.Messages = append(call.Messages, chatMessage{Role: `assistant`, ToolCalls: res.ToolCalls})
		for i, tc := range res.ToolCalls {
			call.Messages = append(call.Messages, chatMessage{Role: `tool`, ToolCallId: tc.Id, Content: answers[i]})
		}
	}
}

// speakInChunks breaks a long line into pieces of at most max runes,
// preferring to end on a sentence and then on a word, so nothing is cut
// mid-thought. A short line comes back whole.
func speakInChunks(text string, max int) []string {
	text = strings.TrimSpace(text)
	if text == `` {
		return nil
	}
	runes := []rune(text)
	if len(runes) <= max {
		return []string{text}
	}
	// Everything here counts in runes. Mixing byte offsets from strings
	// searching with rune offsets from slicing is how this used to cut a
	// Cyrillic story in half and panic.
	enders := [][]rune{[]rune(`. `), []rune(`! `), []rune(`? `), []rune(`." `), []rune(`!" `), []rune(`?" `)}
	var out []string
	for len(runes) > 0 {
		if len(runes) <= max {
			out = append(out, strings.TrimSpace(string(runes)))
			break
		}
		window := runes[:max]
		cut := -1
		for _, end := range enders {
			if i := lastRuneIndex(window, end); i >= 0 && i+len(end) > max/3 && i+len(end) > cut {
				cut = i + len(end)
			}
		}
		if cut < 0 {
			if i := lastRuneIndex(window, []rune{' '}); i > max/3 {
				cut = i + 1
			} else {
				cut = max
			}
		}
		piece := strings.TrimSpace(string(runes[:cut]))
		if piece != `` {
			out = append(out, piece)
		}
		runes = runes[cut:]
		for len(runes) > 0 && runes[0] == ' ' {
			runes = runes[1:]
		}
	}
	return out
}

// lastRuneIndex is strings.LastIndex in rune space: the index of the last
// occurrence of needle in haystack, counted in runes, or -1.
func lastRuneIndex(haystack []rune, needle []rune) int {
	if len(needle) == 0 || len(needle) > len(haystack) {
		return -1
	}
	for i := len(haystack) - len(needle); i >= 0; i-- {
		match := true
		for j := range needle {
			if haystack[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

// lookedFollowUp is the one follow-up a look or a size-up earns, carrying
// the payer of the decision that looked: a passer-by's as their asking
// (AskerUserId, so it is theirs and opens no owner-only verb), her
// owner's as PaidBy, so it is decided with the owner's and never lands
// on a passer-by's allowance; her own look carries neither.
func lookedFollowUp(stims []stimulus, ownerUserId int, outcome actionOutcome) stimulus {
	s := stimulus{Kind: `looked`, Text: outcome.Perceived, Plain: outcome.Plain, Chain: 1}
	if asker := strangerBehind(stims, ownerUserId); asker > 0 {
		s.AskerUserId = asker
		return s
	}
	for _, st := range stims {
		if promptedBy(st, ownerUserId) == ownerUserId {
			s.PaidBy = ownerUserId
			break
		}
	}
	return s
}

// strangerBehind is the passer-by who pays for this decision: whose words
// prompted it, or the payer a follow-up carries (PaidBy) when that is not
// her owner; 0 when it was the owner's doing or her own.
func strangerBehind(stims []stimulus, ownerUserId int) int {
	for _, s := range stims {
		if s.PaidBy > 0 && s.PaidBy != ownerUserId {
			return s.PaidBy
		}
		if s.FromOwner || s.AskerUserId == 0 || s.AskerUserId == ownerUserId {
			continue
		}
		return s.AskerUserId
	}
	return 0
}

// ownerDeeds are the kinds of stimulus that, carried FromOwner, are
// something her owner did or asked: their words, an emote, a gift,
// healing, an attack, a question they sent her to ask (companion-ask),
// the greeting and the farewell of a session, the first meeting, an
// answer to her about the two of them, and a deed of theirs she saw. The
// rest of what is marked FromOwner (a quiet moment, a memory, a trouble
// noticed) is her own business and goes with anyone's decision.
var ownerDeeds = map[string]bool{
	`heard`: true, `asked`: true, `emote`: true, `gift`: true, `healed`: true,
	`attacked`: true, `errand_ask`: true, `session_start`: true, `farewell`: true,
	`first_meeting`: true, `romance_yes`: true, `romance_no`: true, `witnessed`: true,
}

// promptedBy is who a stimulus puts a question to her for: her owner for
// anything they did or asked (ownerDeeds) and for a fight, a passer-by
// (their user id) for anything they did that was aimed at her, the payer
// a follow-up carries (PaidBy), or 0 for the world and her own business,
// which can go with anyone's.
func promptedBy(s stimulus, ownerUserId int) int {
	// Arriving on an errand her owner asked for is the owner's say-so
	// carried to the place (ownerAskedNow), so it goes with the owner.
	if s.Kind == `arrived` && s.Authorized {
		return ownerUserId
	}
	// A follow-up carries the payer of the decision it follows.
	if s.PaidBy > 0 {
		return s.PaidBy
	}
	// A fight is her owner's to plan and pay for, whoever started it:
	// defending her owner is the owner's concern.
	if s.Kind == `fight` || s.Kind == `fight_over` {
		return ownerUserId
	}
	if s.FromOwner {
		if ownerDeeds[s.Kind] {
			return ownerUserId
		}
		return 0
	}
	if s.AskerUserId > 0 && s.AskerUserId != ownerUserId {
		return s.AskerUserId
	}
	return 0
}

// nextBatch takes the next decision's stimuli from the queue: everything
// that belongs to the first person who put something to her, with the
// world's stimuli, and leaves anyone else's for the decision after. So her
// owner's words and a passer-by's never share a call. Sharing one let the
// stranger ride on the owner's say-so into the owner-only verbs, and put
// the whole call on whichever of them strangerBehind happened to find.
// Nobody's words are dropped, only put back in the order they came.
func nextBatch(pending []stimulus, ownerUserId int) (batch []stimulus, rest []stimulus) {
	first := 0
	for _, s := range pending {
		if first = promptedBy(s, ownerUserId); first != 0 {
			break
		}
	}
	if first == 0 {
		return pending, nil
	}
	for _, s := range pending {
		if p := promptedBy(s, ownerUserId); p == 0 || p == first {
			batch = append(batch, s)
		} else {
			rest = append(rest, s)
		}
	}
	return batch, rest
}

// logBudgetRefusal notes a decision the budgets would not pay for, at most
// once a minute per server, so a spent allowance is visible in the log
// rather than silently turning a companion into a set of stock phrases.
// refusedBy names the counter that said no (apiframework.RefusedBy: the
// day's budget "global", the companion's "share", or one allowance's
// dimension), so a spent share is not mistaken for a spent allowance.
func (m *AICompanionModule) logBudgetRefusal(ownerId int, askerId int, wanted int, why error) {
	now := time.Now()
	if now.Sub(m.lastBudgetLog) < time.Minute {
		return
	}
	m.lastBudgetLog = now
	server := m.fw().Today() // the one budget every feature shares
	refusedBy := apiframework.RefusedBy(why)
	if askerId > 0 {
		mudlog.Warn(`aicompanion`, `action`, `budgetRefused`, `refusedBy`, refusedBy, `owner`, ownerId, `asker`, askerId, `wanted`, wanted,
			`askerSpentToday`, m.fw().Allowance(apiframework.DimCompanionStranger, askerId), `askerCap`, m.cfg.StrangerDailyTokens,
			`serverSpentToday`, server.Tokens, `serverCap`, server.Limit)
		return
	}
	mudlog.Warn(`aicompanion`, `action`, `budgetRefused`, `refusedBy`, refusedBy, `owner`, ownerId, `wanted`, wanted,
		`ownerSpentToday`, m.fw().Allowance(apiframework.DimCompanionOwner, ownerId), `ownerCap`, m.cfg.DailyTokensPerCompanion,
		`serverSpentToday`, server.Tokens, `serverCap`, server.Limit)
}
