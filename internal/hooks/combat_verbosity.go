package hooks

import (
	"fmt"
	"sort"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// File: combat_verbosity.go
//
// Light-verbosity round tally for combat narration (spec:
// docs/superpowers/specs/completed/2026-06-10-combat-verbosity-design.md).
// When a viewer's effective verbosity is Light, the per-swing combat
// lines are suppressed at the drain (dispatchCritAndMessaging) and the
// AttackResult's swing data is recorded here instead. flushCombatTallies
// (Task 5) emits one compact line per fight pair per viewer at the end
// of DoCombat. All state is touched only from the game-loop goroutine.

// fighterRef identifies one combatant for tally purposes. Key is a
// stable identity ("u:<userId>" / "m:<mobInstanceId>") so same-named
// mobs don't merge; Name/IsMob drive rendering.
type fighterRef struct {
	Key   string
	Name  string
	IsMob bool
}

// swingStat is the slice of SwingEvent the tally needs.
type swingStat struct {
	Hit    bool
	Damage int
}

// tallyDir accumulates one attack direction within a fight pair.
type tallyDir struct {
	Hits        int
	Misses      int
	WorstHit    int
	TargetMaxHP int
}

func (d *tallyDir) add(swings []swingStat, targetMaxHP int) {
	for _, s := range swings {
		if s.Hit {
			d.Hits++
			if s.Damage > d.WorstHit {
				d.WorstHit = s.Damage
			}
		} else {
			d.Misses++
		}
	}
	if targetMaxHP > 0 {
		d.TargetMaxHP = targetMaxHP
	}
}

// combatTally is one (viewer, fight-pair) accumulator. A/B orientation
// is fixed by whichever direction is recorded first.
type combatTally struct {
	A, B fighterRef
	AtoB tallyDir
	BtoA tallyDir
}

type tallyKey struct {
	viewerId int
	pairKey  string // canonical unordered pair: min(key)+"|"+max(key)
}

type combatTallies struct {
	m map[tallyKey]*combatTally
}

func newCombatTallies() *combatTallies {
	return &combatTallies{m: map[tallyKey]*combatTally{}}
}

func pairKeyFor(a, b string) string {
	if a < b {
		return a + "|" + b
	}
	return b + "|" + a
}

// record adds one AttackResult's swings (attacker → defender) to the
// viewer's tally for that fight pair.
func (ct *combatTallies) record(viewerId int, attacker, defender fighterRef, swings []swingStat, defenderMaxHP int) {
	k := tallyKey{viewerId: viewerId, pairKey: pairKeyFor(attacker.Key, defender.Key)}
	t, ok := ct.m[k]
	if !ok {
		t = &combatTally{A: attacker, B: defender}
		ct.m[k] = t
	}
	if attacker.Key == t.A.Key {
		t.AtoB.add(swings, defenderMaxHP)
	} else {
		t.BtoA.add(swings, defenderMaxHP)
	}
}

// countWord renders a hit count as prose. 1 → "" (the verb carries it),
// per the no-hard-numbers rule everything stays qualitative.
func countWord(n int) string {
	switch {
	case n <= 1:
		return ""
	case n == 2:
		return " twice"
	case n == 3:
		return " three times"
	default:
		return " again and again"
	}
}

// nameToken renders a fighter's name with the engine's standard color
// alias for their kind.
func nameToken(f fighterRef) string {
	if f.IsMob {
		return `<ansi fg="mobname">` + f.Name + `</ansi>`
	}
	return `<ansi fg="username">` + f.Name + `</ansi>`
}

// pronounFails is the subject stand-in for a fighter on second mention,
// with its agreeing verb form for "fail".
func pronounFails(f fighterRef) string {
	if f.IsMob {
		return "it fails"
	}
	return "they fail"
}

// renderTally builds the tally line for one fight pair from a viewer's
// perspective. viewerKey is the viewer's fighterRef.Key when they are a
// participant (their side renders as "You" and their incoming LANDED
// hits are omitted — full prose already showed them under the floor
// rule). A spectator's key simply matches neither fighter, so they
// render third-person; "" is the logged-off/cleanup path.
func renderTally(t *combatTally, viewerKey string) string {
	// Orient so X = viewer (participant) or t.A (spectator).
	x, y := t.A, t.B
	xOut, yOut := t.AtoB, t.BtoA
	if viewerKey != "" && t.B.Key == viewerKey {
		x, y = t.B, t.A
		xOut, yOut = t.BtoA, t.AtoB
	}
	isParticipant := viewerKey != "" && x.Key == viewerKey

	xSwings := xOut.Hits + xOut.Misses
	ySwings := yOut.Hits + yOut.Misses

	// Whiff round: swings happened, nothing landed either way.
	if xOut.Hits == 0 && yOut.Hits == 0 && (xSwings > 0 || ySwings > 0) {
		if isParticipant {
			return fmt.Sprintf("You trade swings with %s; neither side draws blood.", nameToken(y))
		}
		return fmt.Sprintf("%s and %s trade swings without drawing blood.", nameToken(x), nameToken(y))
	}

	segs := []string{}

	// X's outgoing segment.
	if xOut.Hits > 0 {
		tier := combat.GetDamageDescription(xOut.WorstHit, xOut.TargetMaxHP)
		if isParticipant {
			segs = append(segs, fmt.Sprintf("You strike %s%s (%s)", nameToken(y), countWord(xOut.Hits), tier))
		} else {
			segs = append(segs, fmt.Sprintf("%s strikes %s%s (%s)", nameToken(x), nameToken(y), countWord(xOut.Hits), tier))
		}
	} else if xSwings > 0 {
		if isParticipant {
			segs = append(segs, fmt.Sprintf("You fail to break %s's guard", nameToken(y)))
		} else {
			segs = append(segs, fmt.Sprintf("%s can't get past %s's guard", nameToken(x), nameToken(y)))
		}
	}

	// Y's segment. For participants, landed incoming hits already showed
	// in full prose (floor rule) — only whiffs are worth a mention.
	if yOut.Hits > 0 {
		if !isParticipant {
			tier := combat.GetDamageDescription(yOut.WorstHit, yOut.TargetMaxHP)
			segs = append(segs, fmt.Sprintf("%s lands %s%s (%s)",
				nameToken(y), hitNoun(yOut.Hits), countWord(yOut.Hits), tier))
		}
	} else if ySwings > 0 {
		if isParticipant {
			segs = append(segs, fmt.Sprintf("%s to land a blow", pronounFails(y)))
		} else {
			segs = append(segs, fmt.Sprintf("%s fails to land a blow", nameToken(y)))
		}
	}

	if len(segs) == 0 {
		return ""
	}
	return strings.Join(segs, "; ") + "."
}

// hitNoun: "a blow" vs "blows".
func hitNoun(n int) string {
	if n == 1 {
		return "a blow"
	}
	return "blows"
}

// flushForViewer renders and removes all of one viewer's tallies,
// sorted by pair key for deterministic output.
func (ct *combatTallies) flushForViewer(viewerId int, viewerKey string) []string {
	keys := []tallyKey{}
	for k := range ct.m {
		if k.viewerId == viewerId {
			keys = append(keys, k)
		}
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].pairKey < keys[j].pairKey })

	lines := []string{}
	for _, k := range keys {
		if line := renderTally(ct.m[k], viewerKey); line != "" {
			lines = append(lines, line)
		}
		delete(ct.m, k)
	}
	return lines
}

// viewerIds returns the distinct viewers with pending tallies.
func (ct *combatTallies) viewerIds() []int {
	seen := map[int]bool{}
	out := []int{}
	for k := range ct.m {
		if !seen[k.viewerId] {
			seen[k.viewerId] = true
			out = append(out, k.viewerId)
		}
	}
	sort.Ints(out)
	return out
}

// ── Drain-side glue ────────────────────────────────────────────────────

// roundTallies is the per-round accumulator. Game-loop goroutine only.
var roundTallies = newCombatTallies()

// fighterRefFor builds a tally identity for an Actor. For mobs the
// display name is stamped with the room duplicate-index suffix (e.g.
// "Skeleton #2") so same-named mobs produce distinct tally labels —
// matching the disambiguation already applied to per-swing combat lines.
func fighterRefFor(a actions.Actor) fighterRef {
	if a.IsPlayer() {
		return fighterRef{Key: fmt.Sprintf("u:%d", a.GetUserId()), Name: a.GetCharacter().Name, IsMob: false}
	}
	name := a.GetCharacter().Name
	if room := a.GetRoom(); room != nil {
		if dupIdx := room.GetMobDuplicateIndex(a.GetMobInstanceId()); dupIdx > 0 {
			name = fmt.Sprintf("%s #%d", name, dupIdx)
		}
	}
	return fighterRef{Key: fmt.Sprintf("m:%d", a.GetMobInstanceId()), Name: name, IsMob: true}
}

// swingStatsFor extracts tally stats from an AttackResult. Rounds with
// no per-swing analytics (defensive fallback) degrade to one synthetic
// swing from the top-level Hit/DamageToTarget.
func swingStatsFor(res *combat.AttackResult) []swingStat {
	if len(res.SwingEvents) > 0 {
		out := make([]swingStat, 0, len(res.SwingEvents))
		for _, s := range res.SwingEvents {
			out = append(out, swingStat{Hit: s.Hit, Damage: s.Damage})
		}
		return out
	}
	if res.DefenderWasAttacked || res.Hit {
		return []swingStat{{Hit: res.Hit, Damage: res.DamageToTarget}}
	}
	return nil
}

// recordTallyFor records one AttackResult into a viewer's round tally.
func recordTallyFor(viewerId int, atk, def actions.Actor, res *combat.AttackResult) {
	swings := swingStatsFor(res)
	if len(swings) == 0 {
		return
	}
	roundTallies.record(viewerId, fighterRefFor(atk), fighterRefFor(def), swings,
		def.GetCharacter().HealthMax.Value)
}

// drainParticipantLines sends a participant's combat lines subject to
// their verbosity. incoming=true marks lines describing swings AGAINST
// the viewer: landed hits there are floor-protected (always full prose,
// any level); only defense/miss lines are suppressible.
func drainParticipantLines(u *users.UserRecord, msgs []combat.TaggedMessage, lvl messaging.Verbosity, incoming bool) {
	for _, msg := range msgs {
		if incoming && isHitCategory(msg.Category) {
			u.SendText(msg.Category, msg.Text) // floor: damage to you always shows
			continue
		}
		if lvl.Suppresses(msg.Category) {
			continue
		}
		u.SendText(msg.Category, msg.Text)
	}
}

// isHitCategory reports whether a category is one of the CategoryHit*
// damage bands.
func isHitCategory(cat messaging.Category) bool {
	switch cat {
	case messaging.CategoryHitMelee, messaging.CategoryHitBlunt, messaging.CategoryHitNaturalSharp,
		messaging.CategoryHitRanged, messaging.CategoryHitCaster, messaging.CategoryHitUnarmed:
		return true
	}
	return false
}

// drainSpectatorLines delivers room combat lines per spectator at their
// effective (one-step-lower) verbosity, preserving the sight gate via
// SendTextVisualToUser. Excluded ids are the combatants (they got their
// participant lines already).
func drainSpectatorLines(room *rooms.Room, msgs []combat.TaggedMessage, excludeUserIds []int) {
	if room == nil || len(msgs) == 0 {
		return
	}
	for _, uid := range room.GetPlayers() {
		if isExcludedUser(uid, excludeUserIds) {
			continue
		}
		u := users.GetByUserId(uid)
		if u == nil {
			continue
		}
		lvl := u.GetCombatVerbosity().OneStepLower()
		for _, msg := range msgs {
			if lvl.Suppresses(msg.Category) {
				continue
			}
			room.SendTextVisualToUser(u, msg.Category, msg.Text)
		}
	}
}

// recordSpectatorTallies records this AttackResult for every spectator
// whose effective verbosity is Light. Called once per AttackResult
// (NOT per message batch).
func recordSpectatorTallies(atkRoom, defRoom *rooms.Room, atk, def actions.Actor, res *combat.AttackResult, excludeUserIds []int) {
	seen := map[int]bool{}
	for _, room := range []*rooms.Room{atkRoom, defRoom} {
		if room == nil {
			continue
		}
		for _, uid := range room.GetPlayers() {
			if seen[uid] || isExcludedUser(uid, excludeUserIds) {
				continue
			}
			seen[uid] = true
			u := users.GetByUserId(uid)
			if u == nil {
				continue
			}
			// Sight gate: a spectator in darkness receives the generic
			// sounds-of-fighting fallback but must not receive a named
			// tally summary, which would leak combatant identities they
			// cannot see. Shapes-only (infrared) viewers are treated the
			// same as blind here — the named tally requires clear sight.
			//
			// CanSeeClearly is deliberate here, so this ALSO excludes a
			// sleeping spectator (added 2026-08-31). Someone asleep should not
			// be reading a named summary of a fight happening around them, for
			// the same reason they should not be reading the room's dialogue.
			// Contrast the darkness-substitution site in
			// NewRound_DoCombat_unified.go, which must ignore sleep because it
			// is choosing between lit and dark phrasing rather than deciding
			// whether to speak at all.
			if !messaging.CanSeeClearly(u.Character, room) {
				continue
			}
			if u.GetCombatVerbosity().OneStepLower() == messaging.VerbosityLight {
				recordTallyFor(uid, atk, def, res)
			}
		}
	}
}

// flushCombatTallies emits every pending tally line and clears the
// accumulator. Called once at the end of DoCombat each round.
func flushCombatTallies() {
	for _, viewerId := range roundTallies.viewerIds() {
		u := users.GetByUserId(viewerId)
		if u == nil {
			// Viewer logged off mid-round; drop their tallies.
			roundTallies.flushForViewer(viewerId, "")
			continue
		}
		viewerKey := fmt.Sprintf("u:%d", viewerId)
		for _, line := range roundTallies.flushForViewer(viewerId, viewerKey) {
			u.SendText(messaging.CategoryCombatSummary, line)
		}
	}
}

// ── The per-round blind notice (M4d PR 2, Task 4) ──────────────────────

// blindCombatNoticeText is the once-per-round reminder sent to a player
// who fought this round while unable to see clearly and paying for it. It
// names the condition (why the fight looks strange) and the mechanical
// cost (messaging.SightMult below 1.0 weakens both attack and defense
// scores; see internal/combat/situational.go) without ever printing a
// number.
const blindCombatNoticeText = "You cannot see clearly, so your attacks and defense are weaker."

// roundBlindCombatants is the per-round set of player userIds who took
// part in combat (as attacker or defender) this round while their sight
// verdict was not SightFull. Membership itself answers "fought this
// round": a player is only ever added here from inside
// dispatchCritAndMessaging, which runs once per AttackResult against a
// real atk/def pair, so a player merely standing in a dark room who
// never swung or was swung at never appears. A bool set (not a counter)
// is deliberate -- a player hit by two different attackers, or a
// multi-swing attacker, still gets exactly one notice per round.
// Game-loop goroutine only, mirroring roundTallies.
var roundBlindCombatants = map[int]bool{}

// markBlindCombatant records a player-controlled combatant whose sight
// verdict was not SightFull this round. canSeeClearly is the caller's
// already-computed messaging.CanSeeSightImpairedOnly(...) result (true
// only for SightFull); passing its negation in is cheaper than asking
// ParticipantSight a second time and keeps this seam asking the exact
// same optics question the darkness penalty itself reads.
//
// No sleep gate: CanSeeSightImpairedOnly does not consult sleep, so a
// sleeping combatant is recorded the same as an awake one. A sleeper is
// mid-round an auto-crit victim about to wake up (see the sleep-gate
// comment at the srcCanSee/tgtCanSee computation site), so telling them
// they can't see is not wasted -- they are about to be reading combat
// text again very shortly. CanSeeClearly (which DOES sleep-gate) is the
// wrong predicate here for the same reason it is wrong at that site.
//
// Shapes-only viewers (SightShapes) are included, not excluded:
// Balance.DarknessCombatPenalty applies to anyone who is not SightFull,
// shapes included, so the mechanical cost the notice describes is real
// for them too. The copy says "cannot see clearly" rather than "cannot
// see" specifically so it stays true for a shapes viewer who is, in the
// very same round, reading "a figure lunges at you."
//
// ...but only when the cost is real (lighting plan 5c). Since plan 5b the
// darkness price is the sight ramp, messaging.SightMult, not a flat penalty
// on every verdict below SightFull, and a strong enough infravision reads a
// dark room at no cost at all (reach 50 at light 0 is exactly 1.0). The
// notice fired every round for such a caster and told them they were
// weaker when they were not. So the verdict opens the gate and the
// player's own SightMult in their room, the number combat actually
// multiplies by, has to be below 1.0 as well.
func markBlindCombatant(actor actions.Actor, canSeeClearly bool) {
	if !actor.IsPlayer() || canSeeClearly {
		return
	}
	// A nil *rooms.Room must not reach SightMult as a non-nil interface
	// (ParticipantSight's typed-nil trap); with no room the verdict stands.
	if room := actor.GetRoom(); room != nil && messaging.SightMult(actor.GetCharacter(), room) >= 1.0 {
		return
	}
	roundBlindCombatants[actor.GetUserId()] = true
}

// flushBlindCombatNotices sends the once-per-round blind notice to every
// player who fought this round while unable to see clearly, then clears
// the set. Called once at the end of DoCombat each round, beside
// flushCombatTallies.
//
// NOT floor-protected: CategoryCombatBlindWarning goes through the
// viewer's ordinary Verbosity.Suppresses gate like any other category,
// not the isHitCategory bypass drainParticipantLines uses for damage-to-
// you lines. Owner ruling (M4d PR 2 followup): suppressible at Light,
// NOT at Medium (messaging/verbosity.go's suppressibleAtLight). At
// Medium the player still reads per-swing combat prose, so the notice
// explains text they are actually seeing; it stays unsuppressed there.
// At Light they have asked for near-silence, and a per-round line they
// cannot turn off would override a preference they deliberately set --
// even though a blind Light-verbosity combatant's tally line is ITSELF
// gated on srcCanSee/tgtCanSee (see dispatchCritAndMessaging), so this
// notice is the only combat text such a player would otherwise get.
// Light means they chose that silence.
func flushBlindCombatNotices() {
	for userId := range roundBlindCombatants {
		delete(roundBlindCombatants, userId)
		u := users.GetByUserId(userId)
		if u == nil {
			// Logged off mid-round; nothing to deliver.
			continue
		}
		if u.GetCombatVerbosity().Suppresses(messaging.CategoryCombatBlindWarning) {
			continue
		}
		u.SendText(messaging.CategoryCombatBlindWarning, blindCombatNoticeText)
	}
}

// ── The once-per-fight glare notice (#319) ─────────────────────────────

// glareCombatNoticeText is the once-per-fight reminder sent to a player who
// fights in light too bright for their eyes. A dazzled fighter sees every
// face (SightFull), so the blind notice above never speaks for them, yet
// the glare lowers their messaging.SightMult exactly as darkness does. It
// names the cause and the cost without a number, as the blind notice does.
const glareCombatNoticeText = "The glare is too bright, so your attacks and defense are weaker."

// roundGlareCombatants is the per-round set of player userIds who took part
// in combat this round at SightFull while glare cost them. Membership
// answers "fought this round" exactly as roundBlindCombatants does, and for
// the same reason it is a bool set. Game-loop goroutine only.
var roundGlareCombatants = map[int]bool{}

// glareToldThisFight is the set of players already told in the fight they
// are in now. Unlike the blind notice, which speaks every round, the glare
// notice speaks once per fight: a fighter who stays in the same light
// learns nothing new from it the second time. flushGlareCombatNotices
// drops a player from the set once they are out of combat, which is where
// a fight ends, so a later fight tells them again. Game-loop goroutine only.
var glareToldThisFight = map[int]bool{}

// markGlareCombatant records a player-controlled combatant who sees
// clearly (canSeeClearly, the caller's CanSeeSightImpairedOnly verdict, as
// markBlindCombatant takes it) while glare costs them: the bright fraction
// of messaging.ComfortDistance is above 0 and their SightMult in the room
// is below 1.0. Both are asked because the line makes two claims: the
// bright fraction says the light is too bright (SightMult alone would also
// fall for a dark ramp), and SightMult says it costs (a Balance.DazzleCap
// of 1.0 makes glare free, and then the line would be false).
//
// The blind and glare notices never mark the same player in one swing:
// this one needs canSeeClearly and markBlindCombatant needs its negation.
func markGlareCombatant(actor actions.Actor, canSeeClearly bool) {
	if !actor.IsPlayer() || !canSeeClearly {
		return
	}
	// The same typed-nil guard as markBlindCombatant: with no room there is
	// no light to be dazzled by.
	room := actor.GetRoom()
	if room == nil {
		return
	}
	char := actor.GetCharacter()
	if _, bright := messaging.ComfortDistance(char, room); bright <= 0 {
		return
	}
	if messaging.SightMult(char, room) >= 1.0 {
		return
	}
	roundGlareCombatants[actor.GetUserId()] = true
}

// flushGlareCombatNotices sends the glare notice to every player marked
// this round who has not yet been told in this fight, then clears the
// round set and forgets every player no longer in combat. Called once at
// the end of DoCombat each round, beside flushBlindCombatNotices.
//
// Same category and verbosity gate as the blind notice
// (CategoryCombatBlindWarning, suppressible at Light only; see
// flushBlindCombatNotices for the ruling). A suppressed notice is not
// counted as told, so a player who turns verbosity up mid-fight reads it.
//
// "A fight ends" is read from Character.IsInCombat at the end of the round
// (the combat phase is back to Idle). A player who leaves combat and enters
// a new one between two flushes is still in combat at the second flush, so
// that reads as the same fight and is not told again; the cost of that
// window is one notice withheld, never one repeated.
func flushGlareCombatNotices() {
	for userId := range roundGlareCombatants {
		delete(roundGlareCombatants, userId)
		if glareToldThisFight[userId] {
			continue
		}
		u := users.GetByUserId(userId)
		if u == nil {
			// Logged off mid-round; nothing to deliver.
			continue
		}
		if u.GetCombatVerbosity().Suppresses(messaging.CategoryCombatBlindWarning) {
			continue
		}
		u.SendText(messaging.CategoryCombatBlindWarning, glareCombatNoticeText)
		glareToldThisFight[userId] = true
	}
	for userId := range glareToldThisFight {
		if u := users.GetByUserId(userId); u == nil || !u.Character.IsInCombat() {
			delete(glareToldThisFight, userId)
		}
	}
}
