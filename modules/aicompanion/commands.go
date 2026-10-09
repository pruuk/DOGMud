package aicompanion

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/apiframework"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/targeting"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

const aiCompanionUsage = `Usage:
  aicompanion status
  aicompanion profiles
  aicompanion grant <character> <profile>
  aicompanion revoke <character>
  aicompanion pause <character>
  aicompanion resume <character>
  aicompanion mind <character>
  aicompanion trace <character>
  aicompanion prompt <character>
  aicompanion models
  aicompanion breaker reset`

// cmdAICompanion is the admin command. Registered admin-only, so ordinary
// players never see it and companions never reach it.
func (m *AICompanionModule) cmdAICompanion(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {
	if !m.cfg.Enabled {
		return false, nil // switched off: the server has never heard of this command
	}
	args := strings.Fields(rest)
	if len(args) == 0 {
		args = []string{`status`}
	}

	switch strings.ToLower(args[0]) {
	case `status`:
		m.cmdStatus(user)
	case `profiles`:
		m.cmdProfiles(user)
	case `grant`:
		if len(args) < 3 {
			user.SendText(messaging.CategorySystem, aiCompanionUsage)
			return true, nil
		}
		user.SendText(messaging.CategorySystem, m.grant(args[1], args[2]))
	case `revoke`:
		if len(args) < 2 {
			user.SendText(messaging.CategorySystem, aiCompanionUsage)
			return true, nil
		}
		user.SendText(messaging.CategorySystem, m.revoke(args[1]))
	case `mind`:
		if len(args) < 2 {
			user.SendText(messaging.CategorySystem, aiCompanionUsage)
			return true, nil
		}
		user.SendText(messaging.CategorySystem, m.describeMind(args[1]))
	case `trace`, `prompt`:
		if len(args) < 2 {
			user.SendText(messaging.CategorySystem, aiCompanionUsage)
			return true, nil
		}
		user.SendText(messaging.CategorySystem, m.describeDebug(args[1], strings.ToLower(args[0])))
	case `models`:
		m.cmdModels(user)
	case `breaker`:
		// The server key's breaker is shared with every feature that uses it.
		m.fw().ResetBreaker()
		user.SendText(messaging.CategorySystem, `Circuit breaker reset; model calls resume.`)
	case `pause`, `resume`:
		if len(args) < 2 {
			user.SendText(messaging.CategorySystem, aiCompanionUsage)
			return true, nil
		}
		user.SendText(messaging.CategorySystem, m.setPaused(args[1], strings.ToLower(args[0]) == `pause`))
	default:
		user.SendText(messaging.CategorySystem, aiCompanionUsage)
	}
	return true, nil
}

func (m *AICompanionModule) cmdStatus(user *users.UserRecord) {
	m.rollCounters()
	var b strings.Builder
	fmt.Fprintf(&b, "AI companions: enabled=%v model=%q apiKey=%v profiles=%d\n",
		m.cfg.Enabled, m.cfg.Model, m.apiKey() != ``, len(m.profiles))
	fmt.Fprintf(&b, "Player keys: offered=%v relayOrigin=%q waitingReflections=%d\n",
		m.playerKeysOffered(), m.cfg.RelayOrigin, len(m.deferredReflect))
	// The server's key has one daily budget, shared with every feature
	// (APIFramework.DailyTokenBudget). An admin command runs on the game
	// loop, so the settings are read fresh.
	apiframework.RefreshServer()
	u := m.fw().Today()
	companionTokens := 0
	for _, c := range u.ByConsumer {
		if c.Consumer == apiframework.ConsumerCompanion {
			companionTokens = c.Tokens
		}
	}
	limit := `unlimited`
	if u.Limit > 0 {
		limit = fmt.Sprintf(`%d`, u.Limit)
	}
	fmt.Fprintf(&b, "Today (UTC): calls=%d errors=%d; server key tokens=%d/%s (companions %d, all features together)\n",
		m.callsToday, m.errorsToday, u.Tokens, limit, companionTokens)
	if m.cfg.DailyTokensPerCompanion > 0 {
		fmt.Fprintf(&b, "Per companion today: cap=%d tokens (0 = only the server budget applies)\n",
			m.cfg.DailyTokensPerCompanion)
	}
	if now := time.Now(); m.breakerOpen(now) {
		which := `the companion's own (calls failing; check her model settings)`
		if m.fw().BreakerOpen(now) {
			which = `the provider's, shared by every feature (the provider or key is unwell)`
		}
		if until := m.fw().BreakerUntil(apiframework.ConsumerCompanion); now.Before(until) {
			fmt.Fprintf(&b, "Circuit breaker OPEN until %s: %s (fallback lines only).\n", until.Format(`15:04:05`), which)
		} else {
			fmt.Fprintf(&b, "Circuit breaker half-open, a probe call is out: %s.\n", which)
		}
	}

	if len(m.ctrls) == 0 {
		b.WriteString("No active companions.")
	}
	for _, id := range m.sortedOwnerIds() {
		c := m.ctrls[id]
		ownerName := fmt.Sprintf(`user %d`, id)
		if u := users.GetByUserId(id); u != nil && u.Character != nil {
			ownerName = u.Character.Name
		}
		state := `standing`
		switch {
		case c.paused:
			state = `paused`
		case c.instanceId == 0 && c.fellRound > 0:
			state = `fallen`
		case c.instanceId == 0:
			state = `not spawned`
		case c.inFlight:
			state = `thinking`
		case c.travel != nil:
			state = `travelling to ` + c.travel.DestName
		}
		fmt.Fprintf(&b, "  %s -> %s [%s] mood=%s sessions=%d memories=%d facts=%d lines=%d pending=%d",
			ownerName, c.profile.Name, state, c.mind.Mood, c.mind.SessionCount,
			len(c.mind.Memories), len(c.mind.Facts), len(c.mind.RecentLines), len(c.pending))
		fmt.Fprintf(&b, " spentToday=%d tier=%s", m.fw().Allowance(apiframework.DimCompanionOwner, c.ownerUserId), tierName(m.route(c.ownerUserId).kind))
		if m.strangersOff(c.ownerUserId) {
			b.WriteString(` strangers=off`)
		}
		if !m.consented(c.ownerUserId) {
			b.WriteString(` NOT CONSENTED (they have not said "i agree", so she answers with set lines only)`)
		}
		if c.budgetSpent {
			b.WriteString(` BUDGET SPENT (set lines only until the day turns, or raise DailyTokensPerCompanion)`)
		}
		if c.lastErr != `` {
			fmt.Fprintf(&b, " lastError=%q", c.lastErr)
		}
		b.WriteString("\n")
	}
	user.SendText(messaging.CategorySystem, strings.TrimRight(b.String(), "\n"))
}

func (m *AICompanionModule) cmdProfiles(user *users.UserRecord) {
	if len(m.profiles) == 0 {
		user.SendText(messaging.CategorySystem, `No companion profiles are loaded.`)
		return
	}
	ids := make([]string, 0, len(m.profiles))
	for id := range m.profiles {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var b strings.Builder
	for _, id := range ids {
		p := m.profiles[id]
		fmt.Fprintf(&b, "  %s: %s (mob %d)\n", id, p.Name, p.MobId)
	}
	user.SendText(messaging.CategorySystem, strings.TrimRight(b.String(), "\n"))
}

// grant bonds a companion to an online player (admin). See bondTo.
func (m *AICompanionModule) grant(charName string, profileId string) string {
	p, ok := m.profiles[strings.ToLower(profileId)]
	if !ok {
		return fmt.Sprintf(`No profile %q. Try: aicompanion profiles`, profileId)
	}
	owner := users.GetByCharacterName(charName)
	if owner == nil || owner.Character == nil {
		return fmt.Sprintf(`%s is not online.`, charName)
	}
	if err := m.bondTo(owner, p, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> falls into step beside <ansi fg="username">%s</ansi>.`,
		p.Name, owner.Character.Name)); err != nil {
		return err.Error()
	}
	return fmt.Sprintf(`Bonded %s to %s.`, p.Name, owner.Character.Name)
}

// bondTo spawns a profile's companion in the owner's room and bonds it: the
// same spawn a summon uses, but as CompanionBonded (no Conviction reserve,
// no components). arrival is the line the room sees.
func (m *AICompanionModule) bondTo(owner *users.UserRecord, p *Profile, arrival string) error {
	for i := range owner.Character.Companions {
		if owner.Character.Companions[i].SourceType == characters.CompanionBonded {
			return fmt.Errorf(`%s already has a bonded companion`, owner.Character.Name)
		}
	}
	room := rooms.LoadRoom(owner.Character.RoomId)
	if room == nil {
		return fmt.Errorf(`the player's room could not be loaded`)
	}

	mob := mobs.NewMobByIdFresh(mobs.MobId(p.MobId), room.RoomId)
	if mob == nil {
		return fmt.Errorf(`mob template %d could not be spawned`, p.MobId)
	}
	mob.Character.Name = p.Name
	room.AddMob(mob.InstanceId)

	mob.Character.Charm(owner.UserId, -1, ``)
	targeting.Release(&mob.Character, targeting.ReasonDisengage)
	owner.Character.TrackCharmed(mob.InstanceId, true)

	info := characters.CompanionInfo{
		MobId:      p.MobId,
		InstanceId: mob.InstanceId,
		SourceType: characters.CompanionBonded,
		Name:       p.Name,
		BaseName:   p.Name,
		AutoAssist: true,
	}
	if !owner.Character.AddCompanion(info) {
		owner.Character.TrackCharmed(mob.InstanceId, false)
		mob.Character.RemoveCharm()
		room.RemoveMob(mob.InstanceId)
		mobs.DestroyInstance(mob.InstanceId)
		return fmt.Errorf(`%s cannot keep any more companions`, owner.Character.Name)
	}
	owner.Character.RecalculateStats()
	events.AddToQueue(events.CharacterVitalsChanged{UserId: owner.UserId})

	if err := users.SaveUser(owner); err != nil {
		mudlog.Error(`aicompanion`, `action`, `bond`, `owner`, owner.UserId, `error`, err)
	}
	if arrival != `` {
		room.SendTextVisual(messaging.CategoryMobEmote, arrival)
	}

	// Her own few things, the first time she takes up with anyone: a
	// companion template carries nothing, so this is where a starting kit
	// belongs. A returning companion keeps what it already had.
	if rec := m.bonds.Users[owner.UserId]; rec == nil || !rec.Met {
		equipStartingKit(mob, p)
	}
	m.markBond(owner.UserId, p.Id, false)
	mudlog.Info(`aicompanion`, `action`, `bond`, `owner`, owner.UserId, `profile`, p.Id, `instance`, mob.InstanceId)
	return nil
}

// revoke removes a bonded companion from an online player (admin). The
// mind file is kept on disk, so bonding the same profile again restores
// their memories.
func (m *AICompanionModule) revoke(charName string) string {
	owner := users.GetByCharacterName(charName)
	if owner == nil || owner.Character == nil {
		return fmt.Sprintf(`%s is not online.`, charName)
	}
	name, err := m.unbond(owner, `turns away and takes a different road.`)
	if err != nil {
		return err.Error()
	}
	return fmt.Sprintf(`Removed %s from %s. Their memories are kept on disk.`, name, owner.Character.Name)
}

// unbond ends a bond: the session is closed (and reflected on), the
// companion leaves the room with a departure emote, and the record is
// removed from the owner. Its mind stays on disk. Returns its name.
func (m *AICompanionModule) unbond(owner *users.UserRecord, departure string) (string, error) {
	idx := -1
	for i := range owner.Character.Companions {
		if owner.Character.Companions[i].SourceType == characters.CompanionBonded {
			idx = i
			break
		}
	}
	if idx < 0 {
		return ``, fmt.Errorf(`%s has no bonded companion`, owner.Character.Name)
	}
	comp := owner.Character.Companions[idx]

	if c, ok := m.ctrls[owner.UserId]; ok {
		m.detach(c, owner.Character.Name)
	}

	if comp.InstanceId > 0 {
		if mob := mobs.GetInstance(comp.InstanceId); mob != nil {
			mob.Character.RemoveCharm()
			if r := rooms.LoadRoom(mob.Character.RoomId); r != nil {
				r.RemoveMob(mob.InstanceId)
				if departure != `` {
					r.SendTextVisual(messaging.CategoryMobEmote, fmt.Sprintf(
						`<ansi fg="mobname">%s</ansi> %s`, comp.Name, util.EscapeAnsiTags(departure)))
				}
			}
			mobs.DestroyInstance(mob.InstanceId)
		}
		owner.Character.TrackCharmed(comp.InstanceId, false)
	}

	owner.Character.Companions = append(owner.Character.Companions[:idx], owner.Character.Companions[idx+1:]...)
	owner.Character.RecalculateStats()
	events.AddToQueue(events.CharacterVitalsChanged{UserId: owner.UserId})
	if err := users.SaveUser(owner); err != nil {
		mudlog.Error(`aicompanion`, `action`, `unbond`, `owner`, owner.UserId, `error`, err)
	}
	mudlog.Info(`aicompanion`, `action`, `unbond`, `owner`, owner.UserId, `companion`, comp.Name)
	return comp.Name, nil
}

func (m *AICompanionModule) setPaused(charName string, pause bool) string {
	owner := users.GetByCharacterName(charName)
	if owner == nil {
		return fmt.Sprintf(`%s is not online.`, charName)
	}
	c, ok := m.ctrls[owner.UserId]
	if !ok {
		return fmt.Sprintf(`%s has no active companion.`, owner.Character.Name)
	}
	c.paused = pause
	if pause {
		c.pending = nil
		c.seq++
		c.cancelInFlight()
		return fmt.Sprintf(`%s is paused.`, c.profile.Name)
	}
	return fmt.Sprintf(`%s is resumed.`, c.profile.Name)
}

// describeMind is the admin view of a companion's inner state (F19.2):
// opinion scores (which players never see), mood, recent memories, facts,
// open promises and recent opinion changes. It is for debugging and tuning.
func (m *AICompanionModule) describeMind(charName string) string {
	owner := users.GetByCharacterName(charName)
	if owner == nil || owner.Character == nil {
		return fmt.Sprintf(`%s is not online.`, charName)
	}
	c, ok := m.ctrls[owner.UserId]
	if !ok {
		return fmt.Sprintf(`%s has no active companion.`, owner.Character.Name)
	}
	mind := c.mind
	now := time.Now().Unix()

	var b strings.Builder
	fmt.Fprintf(&b, "%s's mind (owner %s)\n", c.profile.Name, owner.Character.Name)
	fmt.Fprintf(&b, "Opinion: trust %d, respect %d, affection %d. Mood: %s. Sessions: %d. Deaths: %d.\n",
		mind.Opinion.Trust, mind.Opinion.Respect, mind.Opinion.Affection,
		mind.Mood, mind.SessionCount, mind.DeathCount)
	fmt.Fprintf(&b, "Memories: %d. Facts: %d. Summaries: %d. Open promises: %d. Known rooms: %d. Hearsay: %d. Loot rule: %s.\n",
		len(mind.Memories), len(mind.Facts), len(mind.Summaries), len(mind.openPromises()),
		len(mind.Map), len(mind.Hearsay), mind.LootRule)
	if c.travel != nil {
		fmt.Fprintf(&b, "Travelling: %s to %s, step %d of %d.\n", c.travel.Purpose, c.travel.DestName, c.travel.Next, len(c.travel.Steps))
	}

	b.WriteString("Latest memories:\n")
	start := len(mind.Memories) - 6
	if start < 0 {
		start = 0
	}
	for _, mem := range mind.Memories[start:] {
		fmt.Fprintf(&b, "  [%s, %d, %s ago] %s\n", mem.Kind, mem.Importance, humanizeElapsed(now-mem.Unix), mem.Text)
	}
	if len(mind.Facts) > 0 {
		b.WriteString("Facts:\n")
		for _, f := range mind.Facts {
			fmt.Fprintf(&b, "  (%s) %s\n", f.Source, f.Text)
		}
	}
	for _, p := range mind.openPromises() {
		fmt.Fprintf(&b, "Promise [%d] by %s: %s\n", p.Id, p.By, p.Text)
	}
	b.WriteString("Recent opinion changes:\n")
	start = len(mind.OpinionLog) - 5
	if start < 0 {
		start = 0
	}
	for _, ch := range mind.OpinionLog[start:] {
		fmt.Fprintf(&b, "  %s ago %s/%s %+d/%+d/%+d %s\n", humanizeElapsed(now-ch.Unix), ch.Trigger, ch.Source,
			ch.Delta.Trust, ch.Delta.Respect, ch.Delta.Affection, ch.Reason)
	}
	return strings.TrimRight(b.String(), "\n")
}

// cmdModels shows the model routing and per-tier metrics.
func (m *AICompanionModule) cmdModels(user *users.UserRecord) {
	var b strings.Builder
	for _, tier := range []string{tierFast, tierMain, tierDeep} {
		ts := m.settingsFor(tier, false)
		effort := ts.Effort
		if effort == `` {
			effort = `(not sent)`
		}
		if ts.Model == `` {
			fmt.Fprintf(&b, "%s: NO USABLE MODEL (every candidate refused by the API; set a model in the config)\n", tier)
			continue
		}
		fmt.Fprintf(&b, "%s: model=%q maxTokens=%d timeout=%s effort=%s\n", tier, ts.Model, ts.MaxTokens, ts.Timeout, effort)
	}
	b.WriteString("Used for: fast = fights, noticing, quiet moments, follow-ups; main = conversation and relationship; deep = reflection after a session.\n")
	srv := apiframework.RefreshServer() // an admin command: on the game loop
	fmt.Fprintf(&b, "Moderation: %v (%s). Retry transient: %v. Breaker: server key %d errors -> %ds; a player's own key %d errors -> %ds.\n",
		m.cfg.ModerateOutput, m.cfg.ModerationModel, m.cfg.RetryTransient, srv.BreakerErrors, srv.BreakerSeconds, m.cfg.BreakerErrors, m.cfg.BreakerSeconds)
	if lines := m.statsLines(); len(lines) > 0 {
		b.WriteString("Since boot:\n")
		b.WriteString(strings.Join(lines, "\n"))
	}
	user.SendText(messaging.CategorySystem, strings.TrimRight(b.String(), "\n"))
}

// describeDebug shows a companion's recent decisions, or the full last
// request sent to the model, so an admin can see exactly what the model was
// told. Admin only: the prompt contains what players said.
func (m *AICompanionModule) describeDebug(charName string, what string) string {
	owner := users.GetByCharacterName(charName)
	if owner == nil {
		return fmt.Sprintf(`%s is not online.`, charName)
	}
	c, ok := m.ctrls[owner.UserId]
	if !ok {
		return fmt.Sprintf(`%s has no active companion.`, owner.Character.Name)
	}
	if what == `trace` {
		if len(c.traces) == 0 {
			return `No decisions yet this session.`
		}
		lines := make([]string, 0, len(c.traces))
		for _, t := range c.traces {
			lines = append(lines, t.String())
		}
		return strings.Join(lines, "\n")
	}
	if len(c.lastPrompt) == 0 {
		return `No request sent yet this session.`
	}
	var b strings.Builder
	for _, msg := range c.lastPrompt {
		fmt.Fprintf(&b, "===== %s (%d characters) =====\n%s\n", strings.ToUpper(msg.Role), len(msg.Content), msg.Content)
	}
	return util.EscapeAnsiTags(strings.TrimRight(b.String(), "\n"))
}

// unstickSeconds is how long a companion-unstick waits before the next.
const (
	unstickCooldownTag = `aicompanion-unstick`
	unstickSeconds     = 60
)

// cmdUnstick is the owner's out-of-character fallback (F18.2): it clears a
// companion that seems stuck (a trip, a pending action, queued moments, a
// hung call) without touching its mind. The companion never mentions it.
func (m *AICompanionModule) cmdUnstick(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {
	if !m.cfg.Enabled {
		return false, nil // switched off: the server has never heard of this command
	}
	c, ok := m.ctrls[user.UserId]
	if !ok {
		user.SendText(messaging.CategorySystem, `You have no companion to reset.`)
		return true, nil
	}
	// Each reset abandons a call that may already have been paid for, and
	// frees her to start the next at once, so it is not to be used as a
	// way to make her think again and again.
	if user.Character != nil && !user.Character.TryCooldown(unstickCooldownTag, fmt.Sprintf(`%d real seconds`, unstickSeconds)) {
		user.SendText(messaging.CategorySystem, `You reset your companion only a moment ago. Give it a minute.`)
		return true, nil
	}
	c.seq++
	c.cancelInFlight()
	c.pending = nil
	c.pendingAct = nil
	c.travel = nil
	if c.fight != nil {
		m.setHoldBack(c, user, false)
		c.fight = nil
	}
	c.paused = false
	user.SendText(messaging.CategorySystem, `Your companion has been reset. Their memories are untouched.`)
	return true, nil
}

// cmdPart is the owner's side of parting ways. It never happens on the
// model's word alone: the companion can ask to go, but the bond ends only
// here, and only when the owner means it (a second use within a minute, or
// a confirmation of the companion's own request).
func (m *AICompanionModule) cmdPart(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {
	if !m.cfg.Enabled {
		return false, nil // switched off: the server has never heard of this command
	}
	c, ok := m.ctrls[user.UserId]
	if !ok {
		// A bond with no mind behind it (the profile was removed, or the
		// module never took it up) can still be ended, or the player is
		// stuck with a companion nobody is driving.
		if name, err := m.unbond(user, `turns away and takes a different road.`); err == nil {
			user.SendText(messaging.CategorySystem, name+` has gone their own way.`)
			return true, nil
		}
		user.SendText(messaging.CategorySystem, `You have no companion travelling with you.`)
		return true, nil
	}
	if msg := m.confirmPart(c, user); msg != `` {
		user.SendText(messaging.CategorySystem, msg)
	}
	return true, nil
}

// cmdCourt is the owner saying yes: nothing about a romance moves without
// it. It also lifts a line they drew earlier.
func (m *AICompanionModule) cmdCourt(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {
	if !m.cfg.Enabled {
		return false, nil // switched off: the server has never heard of this command
	}
	c, ok := m.ctrls[user.UserId]
	if !ok {
		user.SendText(messaging.CategorySystem, `You have no companion travelling with you.`)
		return true, nil
	}
	if msg := m.courtStep(c, user); msg != `` {
		// The system category is never wrapped for the reader.
		user.SendText(messaging.CategorySystem, messaging.WrapAnsi(msg, 80))
	}
	return true, nil
}

// cmdBoundary is the owner drawing a line, and it is kept.
func (m *AICompanionModule) cmdBoundary(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {
	if !m.cfg.Enabled {
		return false, nil // switched off: the server has never heard of this command
	}
	c, ok := m.ctrls[user.UserId]
	if !ok {
		user.SendText(messaging.CategorySystem, `You have no companion travelling with you.`)
		return true, nil
	}
	user.SendText(messaging.CategorySystem, messaging.WrapAnsi(m.setBoundary(c, strings.ToLower(strings.TrimSpace(rest))), 80))
	return true, nil
}

// askAuthority is the owner's leave for the companion to put one question
// to one NPC, on one topic, within a short window. Without it a companion's
// words to an NPC are just words: the game's dialogue, quest and
// behaviour-tree machinery is never run on the owner's behalf, because the
// model must not be able to move a player's quests, items or gold.
type askAuthority struct {
	MobInstanceId int
	Topic         string
	Expires       int64
}

// valid reports whether this leave covers a question to a particular NPC
// now. The topic must still be recognisable in what she actually says.
func (a *askAuthority) valid(mobInstanceId int, text string, nowUnix int64) bool {
	if a == nil || a.MobInstanceId != mobInstanceId || nowUnix > a.Expires {
		return false
	}
	return topicPresent(a.Topic, text)
}

// topicPresent reports whether the words the companion chose still carry
// the topic the owner authorised, so "ask the smith about the ore" cannot
// turn into a question about something else entirely.
func topicPresent(topic string, text string) bool {
	text = strings.ToLower(text)
	hits, words := 0, 0
	for _, w := range strings.Fields(strings.ToLower(topic)) {
		if len(w) < 3 {
			continue
		}
		words++
		if strings.Contains(text, w) {
			hits++
		}
	}
	if words == 0 {
		// A topic of nothing but tiny words is no topic: it would give
		// unconstrained authority to ask that NPC anything at all.
		return false
	}
	return hits*2 >= words
}

// cmdAskFor is the owner sending their companion to ask an NPC something:
// "companion-ask smith about the ore". It is the only thing that lets her
// words reach an NPC's dialogue, and it covers that NPC and that topic for
// a minute.
func (m *AICompanionModule) cmdAskFor(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {
	if !m.cfg.Enabled {
		return false, nil // switched off: the server has never heard of this command
	}
	c, ok := m.ctrls[user.UserId]
	if !ok {
		user.SendText(messaging.CategorySystem, `You have no companion travelling with you.`)
		return true, nil
	}
	who, topic, _ := strings.Cut(strings.TrimSpace(rest), ` `)
	topic = strings.TrimPrefix(strings.TrimSpace(topic), `about `)
	if who == `` || topic == `` {
		user.SendText(messaging.CategorySystem, `Usage: companion-ask <who> about <what>`)
		return true, nil
	}
	if room == nil {
		return true, nil
	}
	// Seen by the owner: they cannot send their companion to question
	// somebody they themselves cannot see. The typed name goes through the
	// shared sight rule first (#454), so in the dark a name that is there
	// and one that is not read the same refusal.
	who, refusal := actions.AimBySight(user.Character, user.UserId, room, who, `companion-ask`)
	if refusal != `` {
		user.SendText(messaging.CategorySystem, refusal)
		return true, nil
	}
	_, mobInstanceId := room.FindByNameSeenBy(user.Character, who)
	target := mobs.GetInstance(mobInstanceId)
	if target == nil || target.Character.IsCharmed() {
		user.SendText(messaging.CategorySystem, `There is nobody like that here to ask.`)
		return true, nil
	}
	c.askAuth = &askAuthority{MobInstanceId: mobInstanceId, Topic: topic, Expires: time.Now().Unix() + 60}
	// The topic is the owner's own words, so it is written into her mind
	// only once they have agreed that her mind may be sent.
	if m.consented(user.UserId) {
		c.mind.addLine(Line{Speaker: user.Character.Name, Kind: `asked`, ToMe: true,
			Text: `Ask ` + target.Character.Name + ` about ` + topic + `.`}, m.cfg.WorkingMemoryLines)
		c.dirty = true
	}
	c.push(stimulus{Kind: `errand_ask`, Speaker: user.Character.Name,
		Text: `put a question to ` + target.Character.Name + ` about ` + topic, FromOwner: true})
	return true, nil
}

// cmdStay is how an owner pins a companion without a conversation:
// "companion-stay" on its own says where things stand, and "close",
// "normal" or "free" sets how far they may roam. Saying it in words still
// works; this is for when the model is not to be relied on, or not
// available at all.
func (m *AICompanionModule) cmdStay(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {
	if !m.cfg.Enabled {
		return false, nil
	}
	c, ok := m.ctrls[user.UserId]
	if !ok {
		user.SendText(messaging.CategorySystem, `You have no companion travelling with you.`)
		return true, nil
	}
	want := strings.ToLower(strings.TrimSpace(rest))
	switch want {
	case autonomyClose, autonomyNormal, autonomyFree:
		c.mind.Autonomy = want
		c.travel = nil
		c.dirty = true
		user.SendText(messaging.CategorySystem, fmt.Sprintf(
			`%s will %s`, c.profile.Name, roamWords(want)))
	case ``:
		user.SendText(messaging.CategorySystem, fmt.Sprintf(
			`%s will %s (companion-stay close|normal|free)`, c.profile.Name, roamWords(c.mind.Autonomy)))
	default:
		user.SendText(messaging.CategorySystem, `Usage: companion-stay close|normal|free`)
	}
	return true, nil
}

// roamWords says what a roaming level means, in plain terms.
func roamWords(level string) string {
	switch level {
	case autonomyClose:
		return `stay at your side and go nowhere without you asking`
	case autonomyFree:
		return `go about their own business when nothing needs them`
	}
	return `stay with you unless you ask them to go somewhere`
}

// cmdAI is how a player changes their mind about the model after the first
// meeting: "companion-ai" says where things stand, "on" agrees, "off"
// stops anything they say from leaving the server. The spoken "i agree" is
// only read while the question is actually open, so an ordinary
// conversation cannot flip this by accident; this command is the durable
// way to set it.
func (m *AICompanionModule) cmdAI(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {
	if !m.cfg.Enabled {
		return false, nil
	}
	rec := m.bonds.Users[user.UserId]
	if rec == nil {
		user.SendText(messaging.CategorySystem, `You have no companion travelling with you.`)
		return true, nil
	}
	name := `Your companion`
	if c, ok := m.ctrls[user.UserId]; ok {
		name = c.profile.Name
	}
	// The system category is never wrapped for the reader, so every line
	// here is wrapped at 80 columns before it is sent.
	tell := func(format string, args ...any) {
		user.SendText(messaging.CategorySystem, messaging.WrapAnsi(fmt.Sprintf(format, args...), 80))
	}
	where := `OpenAI`
	if m.playerKeysOffered() {
		where = `OpenAI, or to your own key's provider when you use one,`
	}
	switch arg := strings.ToLower(strings.Join(strings.Fields(rest), ` `)); arg {
	case `on`, `yes`, `enable`:
		rec.Consented, rec.Refused = true, false
		m.saveBonds()
		if c, ok := m.ctrls[user.UserId]; ok {
			// Anything still queued was said before they agreed, and the next
			// call would carry it.
			c.pending = nil
			// A first meeting before they agreed was kept without their
			// name and without her introduction: both happen now.
			m.keepFirstMeeting(c, user)
		}
		tell(`(Agreed. What you say to %s, and what happens around you both, is sent to %s to decide what they say, and is kept on this server. "companion-ai off" stops it.)`, name, where)
	case `off`, `no`, `disable`:
		rec.Consented, rec.Refused = false, true
		m.saveBonds()
		tell(`(Stopped. Nothing you say leaves this server. %s stays with you and answers with a few set lines. "companion-ai on" starts it again.)`, name)
	case `strangers on`:
		rec.StrangersOff, rec.StrangersOn = false, true
		m.saveBonds()
		if m.playerKeysOffered() {
			tell(`(Passers-by can talk to %s now. While %s thinks on your own key, what they say is paid for from your key too.)`, name, name)
		} else {
			tell(`(Passers-by can talk to %s now.)`, name)
		}
	case `strangers off`:
		rec.StrangersOff, rec.StrangersOn = true, false
		m.saveBonds()
		tell(`(%s will hear passers-by but answer them only with a few set lines.)`, name)
	case `strangers`:
		switch {
		case m.strangersOff(user.UserId) && !rec.StrangersOff:
			tell(`(%s answers passers-by only with a few set lines while thinking on your own key, so that they spend none of it. "companion-ai strangers on" lets them talk to %s on your key.)`, name, name)
		case m.strangersOff(user.UserId):
			tell(`(%s answers passers-by only with a few set lines. "companion-ai strangers on" changes that.)`, name)
		default:
			tell(`(Passers-by can talk to %s. "companion-ai strangers off" stops them costing anything.)`, name)
		}
	case ``:
		if !rec.Consented {
			tell(`(%s is a plain companion, answering with a few set lines: nothing you say leaves this server. "companion-ai on" changes that. See "help aicompanion".)`, name)
			break
		}
		tell(`(%s is %s, and what is said is kept on this server. "companion-ai off" stops it. See "help aicompanion".)`,
			name, tierWords(m.route(user.UserId).kind))
	default:
		tell(`Usage: companion-ai on|off, or companion-ai strangers on|off`)
	}
	return true, nil
}

// tierWords says, for the owner, which tier is answering for her now.
func tierWords(k routeKind) string {
	switch k {
	case routeRelay:
		return `answering through your own key`
	case routeServer:
		return `answering through this server's key`
	}
	return `answering with a few set lines`
}

// tierName is the tier as the admin status shows it.
func tierName(k routeKind) string {
	switch k {
	case routeRelay:
		return `relay`
	case routeServer:
		return `server`
	}
	return `none`
}
