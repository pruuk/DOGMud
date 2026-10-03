package aicompanion

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// Combat (F13.1 to F13.9). The engine's own mob combat AI keeps doing the
// fighting at game speed: attacks, special moves, auto-assisting the owner.
// This file adds a thin layer over it:
//
//   - When a fight starts or changes, the model picks a stance, a target, a
//     style and when to run. That is one call, in the background; the fight
//     never waits for it.
//   - Every round, local reflexes carry the stance out with ordinary mob
//     commands (attack #id, fire #id, taunt, flee, drink, aid @id), at most
//     one per round and never on the very round something happened.
//   - Short battle lines come from the profile, locally, with no model call.
//   - When the fight ends it is summed up, remembered, and handed to the
//     model so the companion can check on its owner and talk about it.
//
// Health is read as the same bands a player sees (healthWords); nothing is
// told to the model as a number.

// CombatProfile is the authored fighting temperament of a companion.
type CombatProfile struct {
	FleeAt  string      `yaml:"flee_at"` // never, badly_hurt, about_to_die
	Bravery float64     `yaml:"bravery"` // 0..1: willingness to take risks for the owner
	Style   string      `yaml:"style"`   // melee, ranged
	Lines   CombatLines `yaml:"lines"`   // said or done at game speed
	Refuse  []string    `yaml:"refuse"`  // words in names it will not fight unprovoked (merchant, child)

	// How this particular companion fights. The engine's scripted combat AI
	// (the mob template's behavior_archetype) does the blow by blow; these
	// are what makes one companion's fight unlike another's on top of it.

	// Stance is how they go into a fight before the model has said
	// anything: fight, protect (between their companion and harm) or
	// hold_back (only defending themselves). Empty means fight.
	Stance string `yaml:"stance"`
	// Moves are the special moves that suit them, in the order they reach
	// for them. Only these are offered to the model; empty offers them all.
	Moves []string `yaml:"moves"`
	// MendBelow mends their companion mid-fight with a healing spell they
	// know once the companion's health falls below this percentage. Zero
	// never does.
	MendBelow int `yaml:"mend_below"`
	// WardOwner casts a warding spell they know on their companion once,
	// as a fight opens.
	WardOwner bool `yaml:"ward_owner"`
	// Opener "surprise": from hiding, their first blow is a surprise attack
	// at whatever is going for their companion. It needs them hidden
	// (sneak) when the fight starts.
	Opener string `yaml:"opener"`
	// Approach is how they fight, in a sentence, for the model.
	Approach string `yaml:"approach"`
}

// combatStanceDefaults are the stances a profile may go into a fight with.
var combatStanceDefaults = []string{``, `fight`, `protect`, `hold_back`}

// movesFor is the special moves offered to this companion: the profile's
// own, or all of them when it says nothing (an explicit empty list offers
// none: some people have no tricks of the body at all).
func movesFor(p *Profile) []string {
	if p == nil || p.Combat.Moves == nil {
		return realMoves()
	}
	return p.Combat.Moves
}

// realMoves is combatMoves without unchanged and none, which are not moves.
func realMoves() []string {
	return combatMoves[2:]
}

// CombatLines are authored battle lines; each is emote text unless it
// starts with a quote, in which case it is said.
type CombatLines struct {
	Start     []string `yaml:"start"`
	Hurt      []string `yaml:"hurt"`
	OwnerHurt []string `yaml:"owner_hurt"`
	EnemyDown []string `yaml:"enemy_down"`
	Victory   []string `yaml:"victory"`
	Flee      []string `yaml:"flee"`
}

// fightState is one fight in progress, from the companion's side.
type fightState struct {
	StartRound   uint64
	Enemies      map[int]string // mob instance id -> name, everyone who has fought them
	EnemyUsers   map[int]string // player attackers
	Refs         map[string]int // e1.. -> mob instance id (negative: -userId)
	Stance       string         // fight, protect, hold_back, flee
	TargetId     int            // preferred mob instance id, 0 none
	FleeAt       string
	Style        string
	WorstSelf    int // lowest health percent seen
	WorstOwner   int
	SelfBand     string
	OwnerBand    string
	Killed       []string
	Fled         bool
	LastReflex   uint64
	LastReplan   uint64
	LastLine     uint64
	AmmoAtStart  int    // arrows in hand when it began, for gathering them afterwards
	AmmoItemId   int    // which bundle they came out of, in case it emptied
	Move         string // a special move the model called for, used once
	Spell        *fightSpell
	Warded       bool // ward_owner has been tried this fight
	SaidHurt     bool
	SaidOwnerHrt bool
	SaidNoAmmo   bool
	AssistWas    bool // the owner's AutoAssist setting before hold_back, to restore
	AssistSet    bool
}

// fightSpell is a spell the model called for in a fight, used once, and
// who it is for: her owner, herself, or one enemy (a mob instance id).
type fightSpell struct {
	Id      string
	Name    string
	Harm    bool
	Area    bool
	SelfOK  bool
	AtOwner bool
	AtSelf  bool
	AtMob   int
}

const (
	replanEveryRounds = 3
	lineEveryRounds   = 2
)

// healthPct is a character's health against the ceiling it can actually
// reach. Gear that reserves part of a pool lowers that ceiling, so reading
// HealthMax directly makes a fully fit companion look permanently wounded,
// and a companion with a flee threshold runs from every fight.
func healthPct(ch *characters.Character) int {
	max := ch.EffectivePoolMax(characters.PoolHealth)
	if max <= 0 {
		return 100
	}
	return ch.Health * 100 / max
}

// fleeThreshold turns a flee setting into a health percentage.
func fleeThreshold(fleeAt string) int {
	switch fleeAt {
	case `badly_hurt`:
		return 50
	case `about_to_die`:
		return 15
	}
	return 0 // never
}

// fighting reports whether the companion or its owner is in a fight here.
func fighting(mob *mobs.Mob, owner *users.UserRecord) bool {
	if mob.Character.IsInCombat() {
		return true
	}
	return owner != nil && owner.Character != nil && owner.Character.RoomId == mob.Character.RoomId && owner.Character.IsInCombat()
}

// enemiesIn finds who, in the room, is fighting the companion or its owner,
// or is being fought by them.
func enemiesIn(room *rooms.Room, mob *mobs.Mob, owner *users.UserRecord) (map[int]string, map[int]string) {
	mobsOut, usersOut := map[int]string{}, map[int]string{}
	ownerId := 0
	if owner != nil {
		ownerId = owner.UserId
	}
	isUs := func(userId, mobId int) bool {
		return (userId != 0 && userId == ownerId) || (mobId != 0 && mobId == mob.InstanceId)
	}
	for _, id := range room.GetMobs() {
		if id == mob.InstanceId {
			continue
		}
		m := mobs.GetInstance(id)
		if m == nil || !m.Character.IsInCombat() {
			continue
		}
		t := m.Character.CurrentCombatTarget()
		if isUs(t.UserId, t.MobInstanceId) {
			mobsOut[id] = m.Character.Name
		}
	}
	for _, uid := range room.GetPlayers() {
		if uid == ownerId {
			continue
		}
		u := users.GetByUserId(uid)
		if u == nil || u.Character == nil || !u.Character.IsInCombat() {
			continue
		}
		t := u.Character.CurrentCombatTarget()
		if isUs(t.UserId, t.MobInstanceId) {
			usersOut[uid] = u.Character.Name
		}
	}
	// Whoever we are hitting counts too.
	if t := mob.Character.CurrentCombatTarget(); t.MobInstanceId != 0 {
		if m := mobs.GetInstance(t.MobInstanceId); m != nil && m.Character.RoomId == room.RoomId {
			mobsOut[t.MobInstanceId] = m.Character.Name
		}
	}
	if owner != nil && owner.Character != nil && owner.Character.IsInCombat() {
		if t := owner.Character.CurrentCombatTarget(); t.MobInstanceId != 0 {
			if m := mobs.GetInstance(t.MobInstanceId); m != nil && m.Character.RoomId == room.RoomId {
				mobsOut[t.MobInstanceId] = m.Character.Name
			}
		}
	}
	return mobsOut, usersOut
}

// fightLines describes the fight for the prompt, and assigns e-refs.
func fightLines(f *fightState, room *rooms.Room, mob *mobs.Mob, owner *users.UserRecord, ammo string) []string {
	f.Refs = map[string]int{}
	var out []string
	self := actions.NewMobActorInRoom(mob, room)

	ids := make([]int, 0, len(f.Enemies))
	for id := range f.Enemies {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	n := 0
	for _, id := range ids {
		m := mobs.GetInstance(id)
		if m == nil || m.Character.RoomId != room.RoomId || m.Character.Health <= 0 {
			continue
		}
		n++
		ref := fmt.Sprintf(`e%d`, n)
		f.Refs[ref] = id
		who := `someone`
		t := m.Character.CurrentCombatTarget()
		switch {
		case t.MobInstanceId == mob.InstanceId:
			who = `you`
		case owner != nil && t.UserId == owner.UserId:
			who = owner.Character.Name
		case t.MobInstanceId != 0 || t.UserId != 0:
			who = `someone else`
		}
		odds := considerWords(actions.Consider(self, actions.NewMobActorInRoom(m, room)).Ratio)
		mark := ``
		if id == f.TargetId {
			mark = ` (your chosen target)`
		}
		out = append(out, fmt.Sprintf(`[%s] %s: %s, fighting %s; %s%s`, ref, m.Character.Name,
			healthWords(&m.Character), who, odds, mark))
	}
	// People who are fighting are named but given no ref: a companion does
	// not pick a person as a target. It defends itself and its owner, and
	// the engine's combat rules decide the rest.
	uids := make([]int, 0, len(f.EnemyUsers))
	for id := range f.EnemyUsers {
		uids = append(uids, id)
	}
	sort.Ints(uids)
	for _, id := range uids {
		u := users.GetByUserId(id)
		if u == nil || u.Character == nil || u.Character.RoomId != room.RoomId {
			continue
		}
		out = append(out, fmt.Sprintf(`%s (a person) is in this fight: %s. You cannot choose a person as your target.`,
			u.Character.Name, healthWords(u.Character)))
	}
	out = append(out, fmt.Sprintf(`You are %s. %s is %s.`, healthWords(&mob.Character),
		ownerNameOf(owner), ownerHealth(owner)))
	line := fmt.Sprintf(`Your stance: %s. You run when %s.`, stanceWords(f.Stance), fleeWords(f.FleeAt))
	if hasShootingWeapon(mob) {
		if hasAmmo(mob) {
			line += ` You still have ` + ammo + `.`
		} else {
			line += ` You are out of ` + ammo + `; only hand to hand now.`
		}
	}
	out = append(out, line)
	return out
}

func ownerNameOf(owner *users.UserRecord) string {
	if owner == nil || owner.Character == nil {
		return `Your companion`
	}
	return owner.Character.Name
}

func ownerHealth(owner *users.UserRecord) string {
	if owner == nil || owner.Character == nil {
		return `not here`
	}
	return healthWords(owner.Character)
}

func stanceWords(s string) string {
	switch s {
	case `protect`:
		return `protecting your companion first`
	case `hold_back`:
		return `holding back, only defending yourself`
	case `flee`:
		return `getting out`
	}
	return `fighting`
}

func fleeWords(f string) string {
	switch f {
	case `badly_hurt`:
		return `you are badly hurt`
	case `about_to_die`:
		return `you are about to die`
	}
	return `never`
}

// combatTick runs every round for a standing companion. It starts and ends
// fights, keeps the enemy list current, asks for a new plan when the fight
// changes, and carries the plan out with at most one reflex per round.
func (m *AICompanionModule) combatTick(c *controller, u *users.UserRecord, round uint64) {
	mob := mobs.GetInstance(c.instanceId)
	if mob == nil {
		return
	}
	room := rooms.LoadRoom(mob.Character.RoomId)
	if room == nil {
		return
	}

	if !fighting(mob, u) {
		if c.fight != nil {
			m.endFight(c, mob, u, round)
		}
		return
	}

	f := c.fight
	enemies, enemyUsers := enemiesIn(room, mob, u)
	if f == nil {
		f = &fightState{
			StartRound: round, Enemies: map[int]string{}, EnemyUsers: map[int]string{},
			Stance: c.profile.Combat.Stance, FleeAt: c.profile.Combat.FleeAt, Style: c.profile.Combat.Style,
			WorstSelf: 100, WorstOwner: 100, LastReflex: round,
		}
		if f.Stance == `` {
			f.Stance = `fight`
		}
		if f.FleeAt == `` {
			f.FleeAt = `about_to_die`
		}
		c.fight = f
		// Some go into a fight hanging back by nature (a healer behind the
		// line); the model can change its mind once it has looked.
		m.setHoldBack(c, u, f.Stance == `hold_back`)
		for id, name := range enemies {
			f.Enemies[id] = name
		}
		for id, name := range enemyUsers {
			f.EnemyUsers[id] = name
		}
		c.travel = nil
		c.bumpWorld()
		f.AmmoAtStart, f.AmmoItemId = ammoOnHand(mob)
		m.closeConversation(c, `a fight started`)
		// Sizing up what it is facing, as anyone would: hopeless odds make
		// it readier to run before the model has said anything.
		if odds := worstOdds(room, mob, f); odds > 0 && odds < 0.4 && f.FleeAt == `about_to_die` {
			f.FleeAt = `badly_hurt`
			c.mind.addLine(Line{Kind: `event`, Text: `You sized this up as a fight you could lose.`}, m.cfg.WorkingMemoryLines)
		}
		m.combatLine(c, mob, round, c.profile.Combat.Lines.Start)
		m.surpriseOpener(c, mob, u, room, round, enemies)
		text := enemyNames(enemies, enemyUsers)
		if name, unjust := unjustFight(c.profile, f); unjust {
			text += `; you would never willingly fight ` + name + `, and you did not start this`
		}
		m.requestPlan(c, round, `start`, text)
		return
	}

	// New enemies joining is a change worth re-planning for.
	var joined []string
	for id, name := range enemies {
		if _, ok := f.Enemies[id]; !ok {
			f.Enemies[id] = name
			joined = append(joined, name)
		}
	}
	for id, name := range enemyUsers {
		if _, ok := f.EnemyUsers[id]; !ok {
			f.EnemyUsers[id] = name
			joined = append(joined, name)
		}
	}

	selfPct := healthPct(&mob.Character)
	ownerPct := 100
	ownerHere := u != nil && u.Character != nil && u.Character.RoomId == mob.Character.RoomId
	if ownerHere {
		ownerPct = healthPct(u.Character)
	}
	if selfPct < f.WorstSelf {
		f.WorstSelf = selfPct
	}
	if ownerHere && ownerPct < f.WorstOwner {
		f.WorstOwner = ownerPct
	}
	selfBand := healthWords(&mob.Character)
	ownerBand := ``
	if ownerHere {
		ownerBand = healthWords(u.Character)
	}

	switch {
	case len(joined) > 0:
		m.requestPlan(c, round, `joined`, strings.Join(joined, `, `)+` joined the fight`)
	case selfBand != f.SelfBand && f.SelfBand != `` && selfPct < 50:
		m.requestPlan(c, round, `hurt`, `you are now `+selfBand)
	case ownerBand != f.OwnerBand && f.OwnerBand != `` && ownerPct < 50:
		m.requestPlan(c, round, `owner_hurt`, ownerNameOf(u)+` is now `+ownerBand)
	}
	f.SelfBand, f.OwnerBand = selfBand, ownerBand

	if selfPct < 50 && !f.SaidHurt {
		f.SaidHurt = m.combatLine(c, mob, round, c.profile.Combat.Lines.Hurt)
	}
	if ownerHere && ownerPct < 50 && !f.SaidOwnerHrt {
		f.SaidOwnerHrt = m.combatLine(c, mob, round, c.profile.Combat.Lines.OwnerHurt)
	}

	// Holding back has to be kept, not just set: the engine's retargeting
	// (a foe dies, the next one is picked) pulls her back into the fight
	// each time. Unless her foe is fighting her, she steps out again.
	if f.Stance == `hold_back` && mob.Character.IsInCombat() {
		cur := mob.Character.CurrentCombatTarget()
		if !foeFightingHer(u, room, mob, cur.MobInstanceId, cur.UserId) {
			mob.Character.EndAggro()
		}
	}

	m.reflex(c, mob, u, room, round, selfPct, ownerPct, ownerHere)
}

func enemyNames(a map[int]string, b map[int]string) string {
	var names []string
	for _, n := range a {
		names = append(names, n)
	}
	for _, n := range b {
		names = append(names, n)
	}
	sort.Strings(names)
	return strings.Join(names, `, `)
}

// requestPlan asks the model for a stance, rate limited, jumping the queue.
func (m *AICompanionModule) requestPlan(c *controller, round uint64, why string, text string) {
	f := c.fight
	if f == nil || (why != `start` && round < f.LastReplan+replanEveryRounds) {
		return
	}
	f.LastReplan = round
	// A fight pre-empts idle business; conversation waiting is kept.
	var keep []stimulus
	for _, s := range c.pending {
		if s.Kind == `heard` || s.Kind == `asked` || s.Kind == `attacked` {
			keep = append(keep, s)
		}
	}
	// The plan goes first, so it is decided on its own payer, her owner's
	// (promptedBy), and never waits behind a passer-by's words or rides
	// on their allowance: whoever started the fight, defending her owner
	// is the owner's concern.
	c.pending = keep
	c.push(stimulus{Kind: `fight`, Text: why + `: ` + text})
	if n := len(c.pending); n > 1 && c.pending[n-1].Kind == `fight` {
		c.pending = append([]stimulus{c.pending[n-1]}, c.pending[:n-1]...)
	}
	if !c.inFlight {
		m.dispatch(c)
	}
}

// reflex carries out the stance with at most one ordinary command a round,
// and never in the same round as the last one (F13.5).
func (m *AICompanionModule) reflex(c *controller, mob *mobs.Mob, u *users.UserRecord, room *rooms.Room,
	round uint64, selfPct int, ownerPct int, ownerHere bool) {

	f := c.fight
	if round < f.LastReflex+uint64(m.cfg.CombatReactionRounds) {
		return
	}
	delay := 0.5 + float64(util.Rand(10))/10.0
	act := func(command string) {
		mob.Command(command, delay)
		f.LastReflex = round
	}

	// 1. Run (F13.6). The point at which nerve goes is not exact.
	threshold := fleeThreshold(f.FleeAt)
	if threshold > 0 {
		threshold += int(m.jitter() * 15)
	}
	if f.Stance == `flee` || (threshold > 0 && selfPct < threshold) {
		// Only when a flee can actually begin: the flee command's own gate
		// (actions.FleeGate) refuses one out of combat, frenzied, rooted,
		// grappled or knocked down, and speaking the flee line and setting
		// Fled for a refused flee remembers a run that never happened. A
		// flee already under way is left to finish rather than overridden.
		switch actions.FleeGate(&mob.Character) {
		case actions.FleeRefuseAlready:
			return
		case actions.FleeOK:
			if !f.Fled {
				m.combatLine(c, mob, round, c.profile.Combat.Lines.Flee)
			}
			f.Fled = true
			act(`flee`)
			return
		}
	}

	// 2. Tend itself with a potion when badly hurt.
	if selfPct < 40 {
		if name := drinkablePotion(mob); name != `` {
			act(`drink ` + util.EscapeAnsiTags(strings.ToLower(name)))
			return
		}
	}

	// 2b. A healer's hands: mend her owner when they are hurt past her
	// profile's line, and ward them once as the fight opens. These are her
	// nature rather than a plan, so they hold in any stance but flight.
	if ownerHere {
		if cmd := m.tendOwner(c, mob, u, room, ownerPct); cmd != `` {
			act(cmd)
			return
		}
	}

	// 3. Protect the owner (F13.3): go for whatever is hurting them, then
	// draw its attention. Willingness rises with trust, affection and
	// bravery; a "protect" stance always does it.
	if ownerHere && f.Stance != `hold_back` && (f.Stance == `protect` || (ownerPct < 35 && m.willProtect(c))) {
		if threat := threatTo(room, u.UserId); threat != 0 && mayStrike(u, room, threat) {
			cur := mob.Character.CurrentCombatTarget()
			if cur.MobInstanceId != threat {
				act(m.strikeCommand(c, mob, threat))
				return
			}
			if ownerPct < 35 {
				// Standing between them and it is the kind of thing that
				// is remembered afterwards (romance.go).
				m.noteMilestone(c, `defended`)
				act(`taunt`)
				return
			}
		}
	}

	// 4. A special move the model called for, when the engine says it would
	// land. Skills do not unlock moves in DOGMud; they decide how well one
	// goes.
	if f.Stance != `hold_back` && f.Move != `` && moveReady(mob, room, f.Move) && mayStrikeCurrent(u, room, mob) {
		move := f.Move
		f.Move = `` // one use per plan; the model may call for it again
		act(move)
		return
	}

	// 4b. A spell the model called for, when it would start now and lands
	// only where it may (fightCastCommand).
	if f.Spell != nil {
		if cmd, keep := fightCastCommand(c, mob, u, room, f.Spell); cmd != `` {
			f.Spell = nil
			act(cmd)
			return
		} else if !keep {
			f.Spell = nil
		}
	}

	// 5. The chosen target.
	if f.Stance != `hold_back` && f.TargetId != 0 {
		if t := mobs.GetInstance(f.TargetId); t != nil && t.Character.RoomId == room.RoomId && t.Character.Health > 0 &&
			mayStrike(u, room, f.TargetId) {
			if mob.Character.CurrentCombatTarget().MobInstanceId != f.TargetId {
				act(m.strikeCommand(c, mob, f.TargetId))
				return
			}
		} else {
			f.TargetId = 0
		}
	}

	// 5b. An archer with an empty chamber: one shot at whoever she is
	// fighting. Firing chambers an unloaded bow itself and the next round
	// after it, from which her archetype's try_fire keeps shooting.
	if f.Style == `ranged` && f.Stance != `hold_back` && hasShootingWeapon(mob) && hasAmmo(mob) &&
		!mob.Character.Equipment.Weapon.Loaded && mayStrikeCurrent(u, room, mob) {
		if cur := mob.Character.CurrentCombatTarget(); cur.MobInstanceId > 0 && !mob.Character.IsActing() {
			act(fmt.Sprintf(`fire #%d`, cur.MobInstanceId))
			return
		}
	}

	// 6. Out of arrows, with a bow in hand: say so, and close in from now on.
	if f.Style == `ranged` && hasShootingWeapon(mob) && !hasAmmo(mob) && !f.SaidNoAmmo {
		f.SaidNoAmmo = true
		f.Style = `melee`
		c.mind.addLine(Line{Kind: `event`, Text: `You loosed the last of your ` + c.profile.ammoWord() + ` and had to close in.`}, m.cfg.WorkingMemoryLines)
		m.requestPlan(c, round, `no_ammo`, `you are out of `+c.profile.ammoWord()+` and fighting hand to hand now`)
	}

	// Bringing a downed owner round is not done here: first aid needs a
	// calm room, and this only runs while the fighting is on. The engine's
	// own idle handling does it once the room settles, and the milestone is
	// recorded then, when it has actually happened.
}

// willProtect is the companion's willingness to put itself between its
// owner and harm: opinion and bravery, plus a little of the day's mood, so
// the same companion in the same spot does not always do the same thing.
func (m *AICompanionModule) willProtect(c *controller) bool {
	o := c.mind.Opinion
	score := float64(o.Trust+o.Affection)/200.0 + c.profile.Combat.Bravery + m.jitter()
	return score >= 0.6
}

// jitter is the small unpredictability in a companion's nerve, scaled by
// CombatVariance: roughly plus or minus that much, centred on nothing.
func (m *AICompanionModule) jitter() float64 {
	v := m.cfg.CombatVariance
	if v <= 0 {
		return 0
	}
	return (float64(util.Rand(201))/100.0 - 1.0) * v
}

// threatTo is the mob instance fighting a user in the room, if any.
func threatTo(room *rooms.Room, userId int) int {
	for _, id := range room.GetMobs() {
		if m := mobs.GetInstance(id); m != nil && m.Character.IsInCombat() && m.Character.CurrentCombatTarget().UserId == userId {
			return id
		}
	}
	return 0
}

// mayStrike reports whether she may turn on this creature: whatever the
// plan or her nerve says, only what her owner could attack (harmAllowed).
// Going for the thing hurting her owner is not an exception, because a
// player defending themselves gets none either.
func mayStrike(owner *users.UserRecord, room *rooms.Room, mobInstanceId int) bool {
	ok, _ := harmAllowed(owner, room, mobInstanceId, 0)
	return ok
}

// mayStrikeCurrent is mayStrike for whoever she is already fighting, which
// is what a special move lands on. The engine's round chose that foe, not
// her; a move is her choice, so it is held to her owner's rules too, with
// the one allowance a player gets: a player already in a fight may use a
// move on their foe whoever it is (actions.StageMeleeTarget stages no
// target checks in combat), so she may use one on a foe that is fighting
// her. Never on her owner.
func mayStrikeCurrent(owner *users.UserRecord, room *rooms.Room, mob *mobs.Mob) bool {
	cur := mob.Character.CurrentCombatTarget()
	if foeFightingHer(owner, room, mob, cur.MobInstanceId, cur.UserId) {
		return true
	}
	ok, _ := harmAllowed(owner, room, cur.MobInstanceId, cur.UserId)
	return ok
}

// foeFightingHer reports whether her current foe, a creature or a person
// other than her owner, stands in her room and is fighting her.
func foeFightingHer(owner *users.UserRecord, room *rooms.Room, mob *mobs.Mob, foeMobId int, foeUserId int) bool {
	if room == nil {
		return false
	}
	if foeMobId > 0 {
		foe := mobs.GetInstance(foeMobId)
		return foe != nil && foe.Character.RoomId == room.RoomId &&
			foe.Character.CurrentCombatTarget().MobInstanceId == mob.InstanceId
	}
	if foeUserId > 0 && (owner == nil || foeUserId != owner.UserId) {
		foe := users.GetByUserId(foeUserId)
		return foe != nil && foe.Character != nil && foe.Character.RoomId == room.RoomId &&
			foe.Character.CurrentCombatTarget().MobInstanceId == mob.InstanceId
	}
	return false
}

// strikeCommand chooses a shot or a blade, by style, weapon and whether
// there is anything left to shoot. An archer with an empty quiver closes in
// rather than pulling a string at nothing.
func (m *AICompanionModule) strikeCommand(c *controller, mob *mobs.Mob, targetId int) string {
	if c.fight != nil && c.fight.Style == `ranged` && hasShootingWeapon(mob) && hasAmmo(mob) {
		return fmt.Sprintf(`fire #%d`, targetId)
	}
	return fmt.Sprintf(`attack #%d`, targetId)
}

func hasShootingWeapon(mob *mobs.Mob) bool {
	w := mob.Character.Equipment.Weapon
	return w.ItemId > 0 && w.GetSpec().Subtype == items.Shooting
}

// hasAmmo reports whether a shot is still possible: one already chambered,
// or a bundle of the right kind with shots left in it.
func hasAmmo(mob *mobs.Mob) bool {
	w := mob.Character.Equipment.Weapon
	if w.ItemId < 1 {
		return false
	}
	if w.Loaded {
		return true
	}
	tag := w.GetSpec().AmmoTag
	for i := range mob.Character.Items {
		spec := mob.Character.Items[i].GetSpec()
		if spec.Type == items.Ammo && spec.AmmoTag == tag && mob.Character.Items[i].Uses > 0 {
			return true
		}
	}
	return false
}

// moveReady reports whether a named special move would actually do
// something right now, using the engine's own gate. Which moves a
// companion has depends on its body, its gear and the shared special-move
// cooldown, not on its skills: skills decide how well a move lands.
func moveReady(mob *mobs.Mob, room *rooms.Room, move string) bool {
	if move == `` || move == `none` || move == `unchanged` {
		return false
	}
	return actions.CommandIsReady(actions.NewMobActorInRoom(mob, room), move)
}

// drinkablePotion names a potion in the pack, or "".
func drinkablePotion(mob *mobs.Mob) string {
	for i := range mob.Character.Items {
		it := &mob.Character.Items[i]
		spec := it.GetSpec()
		if spec.Type == items.Potion && spec.Subtype == items.Drinkable {
			return it.Name()
		}
	}
	return ``
}

// combatLine says or does one authored battle line, locally and at game
// speed (F13.7). Returns whether a line was used.
func (m *AICompanionModule) combatLine(c *controller, mob *mobs.Mob, round uint64, pool []string) bool {
	if len(pool) == 0 || ownerSilenced(c.ownerUserId) || util.Rand(10) >= 7 {
		return false
	}
	if c.fight != nil {
		if round < c.fight.LastLine+lineEveryRounds && c.fight.LastLine != 0 {
			return false
		}
		c.fight.LastLine = round
	}
	line := cleanText(pool[util.Rand(len(pool))], maxEmoteRunes)
	if line == `` {
		return false
	}
	kind := `emote`
	if strings.HasPrefix(line, `"`) {
		kind = `say`
		line = strings.Trim(line, `"`)
	}
	mob.Command(kind + ` ` + util.EscapeAnsiTags(line))
	lk := `emoted`
	if kind == `say` {
		lk = `said`
	}
	c.mind.addLine(Line{Speaker: c.profile.Name, Kind: lk, Text: line}, m.cfg.WorkingMemoryLines)
	return true
}

// applyCombatProposal takes the model's stance for the fight in progress.
func (m *AICompanionModule) applyCombatProposal(c *controller, p CombatProposal, u *users.UserRecord) {
	f := c.fight
	if f == nil {
		return
	}
	if p.Stance != `` && p.Stance != `unchanged` {
		f.Stance = p.Stance
	}
	if p.FleeAt != `` && p.FleeAt != `unchanged` {
		f.FleeAt = p.FleeAt
	}
	if p.Style != `` && p.Style != `unchanged` {
		f.Style = p.Style
	}
	if id, ok := f.Refs[p.Target]; ok && id > 0 {
		if mob := mobs.GetInstance(c.instanceId); mob != nil && mayStrike(u, rooms.LoadRoom(mob.Character.RoomId), id) {
			f.TargetId = id
		}
	}
	// Only a move that suits her: the profile's own list, so a healer is
	// not sent to grapple and an archer is not sent to bash.
	if p.Move == `none` {
		f.Move = `` // she has thought better of it
	} else if inSet(movesFor(c.profile), p.Move) {
		f.Move = p.Move
	}
	if p.Spell != `` {
		if mob := mobs.GetInstance(c.instanceId); mob != nil {
			f.Spell = resolveFightSpell(mob, f, p.Spell, p.SpellAt)
		}
	}
	m.setHoldBack(c, u, f.Stance == `hold_back`)
}

// setHoldBack switches the engine's auto-assist off while the companion
// holds back from a fight, and restores the owner's setting afterwards.
// This is the companion refusing to join in, not a new ability: it can
// still defend itself.
func (m *AICompanionModule) setHoldBack(c *controller, u *users.UserRecord, hold bool) {
	f := c.fight
	if f == nil || u == nil || u.Character == nil {
		return
	}
	comp := u.Character.GetCompanionByInstanceId(c.instanceId)
	if comp == nil {
		return
	}
	if hold && !f.AssistSet {
		f.AssistWas, f.AssistSet = comp.AutoAssist, true
		comp.AutoAssist = false
		// Remember the owner's setting in the mind as well, so it is put
		// back even if the session ends mid-fight.
		c.mind.AssistToRestore = `off`
		if f.AssistWas {
			c.mind.AssistToRestore = `on`
		}
		c.dirty = true
		// Auto-assist may already have sent her in this very round. Holding
		// back means stepping out again, unless her foe is fighting her, in
		// which case she defends herself.
		if mob := mobs.GetInstance(c.instanceId); mob != nil && mob.Character.IsInCombat() {
			cur := mob.Character.CurrentCombatTarget()
			if !foeFightingHer(u, rooms.LoadRoom(mob.Character.RoomId), mob, cur.MobInstanceId, cur.UserId) {
				mob.Character.EndAggro()
			}
		}
	} else if !hold && f.AssistSet {
		comp.AutoAssist = f.AssistWas
		f.AssistSet = false
		c.mind.AssistToRestore = ``
		c.dirty = true
	}
}

// restoreAssist puts back an owner's auto-assist setting that a session
// ended while holding back from a fight. Called when a controller attaches.
func restoreAssist(mind *Mind, comp *characters.CompanionInfo) {
	if mind.AssistToRestore == `` || comp == nil {
		return
	}
	comp.AutoAssist = mind.AssistToRestore == `on`
	mind.AssistToRestore = ``
}

// endFight sums the fight up, remembers it, and gives the companion a
// moment afterwards (F13.8).
func (m *AICompanionModule) endFight(c *controller, mob *mobs.Mob, u *users.UserRecord, round uint64) {
	f := c.fight
	m.setHoldBack(c, u, false)
	c.fight = nil
	c.bumpWorld()
	now := time.Now().Unix()

	var parts []string
	if len(f.Killed) > 0 {
		parts = append(parts, `fell: `+strings.Join(f.Killed, `, `))
	}
	if f.Fled {
		parts = append(parts, `you ran`)
	}
	if f.WorstSelf < 15 {
		parts = append(parts, `you nearly died`)
	} else if f.WorstSelf < 50 {
		parts = append(parts, `you were badly hurt`)
	}
	owner := ownerNameOf(u)
	if f.WorstOwner < 15 {
		parts = append(parts, owner+` nearly died`)
	} else if f.WorstOwner < 50 {
		parts = append(parts, owner+` was badly hurt`)
	}
	long := round-f.StartRound > 10
	if long {
		parts = append(parts, `it was a long fight`)
	}
	if recovered := m.gatherArrows(c, mob, f); recovered > 0 {
		parts = append(parts, fmt.Sprintf(`you got %d of your %s back`, recovered, c.profile.ammoWord()))
	}
	summary := `The fight with ` + enemyNames(f.Enemies, f.EnemyUsers) + ` is over`
	if len(parts) > 0 {
		summary += `: ` + strings.Join(parts, `; `)
	}
	summary += `.`

	importance := 4
	if f.WorstSelf < 15 || f.WorstOwner < 15 {
		importance += 3
	}
	if f.WorstSelf < 20 && f.WorstOwner < 20 {
		m.noteMilestone(c, `survived`)
	}
	if long {
		importance++
	}
	emotion := `relief`
	if f.Fled {
		emotion = `fear`
	}
	// The summary names her owner and anyone who fought them.
	if m.mayRemember(c) {
		c.mind.addMemory(Memory{Unix: now, Kind: `event`, Text: summary, Importance: importance, Emotion: emotion,
			People: []string{owner}, PlaceId: mob.Character.RoomId}, m.cfg.MaxMemories)
		c.mind.addLine(Line{Kind: `event`, Text: summary}, m.cfg.WorkingMemoryLines)
	}
	if !f.Fled && len(f.Killed) > 0 {
		m.combatLine(c, mob, round, c.profile.Combat.Lines.Victory)
	}
	c.dirty = true
	c.push(stimulus{Kind: `fight_over`, Text: summary, FromOwner: true})
}

// onMobDeath notes an enemy falling in a fight the companion is in.
func (m *AICompanionModule) onMobDeath(e events.Event) events.ListenerReturn {
	evt, ok := e.(events.MobDeath)
	if !ok || !m.cfg.Enabled {
		return events.Continue
	}
	for _, c := range m.ctrls {
		if c.fight == nil {
			continue
		}
		if name, fought := c.fight.Enemies[evt.InstanceId]; fought {
			c.fight.Killed = append(c.fight.Killed, name)
			if c.fight.TargetId == evt.InstanceId {
				c.fight.TargetId = 0
			}
			if mob := mobs.GetInstance(c.instanceId); mob != nil {
				m.combatLine(c, mob, util.GetRoundCount(), c.profile.Combat.Lines.EnemyDown)
			}
		}
	}
	return events.Continue
}

// onPlayerDeath remembers the owner falling.
func (m *AICompanionModule) onPlayerDeath(e events.Event) events.ListenerReturn {
	evt, ok := e.(events.PlayerDeath)
	if !ok || !m.cfg.Enabled {
		return events.Continue
	}
	c := m.ctrls[evt.UserId]
	if c == nil {
		return events.Continue
	}
	now := time.Now().Unix()
	if m.mayRemember(c) {
		c.mind.addMemory(Memory{Unix: now, Kind: `death`, Text: `I watched ` + evt.CharacterName + ` fall.`,
			Importance: 9, Emotion: `sadness`, People: []string{evt.CharacterName}, PlaceId: evt.RoomId}, m.cfg.MaxMemories)
		c.mind.addLine(Line{Kind: `event`, Text: evt.CharacterName + ` fell.`}, m.cfg.WorkingMemoryLines)
	}
	c.mind.Mood, c.mind.MoodSetUnix = `sad`, now
	c.mind.markDanger(evt.RoomId, 3, now, false)
	c.dirty = true
	return events.Continue
}

// refusesToFight reports whether a target is one the companion will not
// attack unprovoked: anyone keeping a shop, and anyone her profile's
// refusal list names. This is her character, not the rules. What nobody
// may attack (a companion, a non-combatant, the attack-immune) is the
// engine's to say, and harmAllowed asks it; the engine lets a player fight
// a shopkeeper who is not protected, and she simply will not.
func refusesToFight(p *Profile, m *mobs.Mob) bool {
	if m == nil {
		return false
	}
	if m.HasShop() {
		return true
	}
	name := strings.ToLower(m.Character.Name)
	for _, w := range p.Combat.Refuse {
		if w != `` && strings.Contains(name, strings.ToLower(w)) {
			return true
		}
	}
	return false
}

// unjustFight reports whether the owner started a fight with someone the
// companion will not fight, so the model can choose to hold back.
func unjustFight(p *Profile, f *fightState) (string, bool) {
	for id, name := range f.Enemies {
		if refusesToFight(p, mobs.GetInstance(id)) {
			return name, true
		}
	}
	return ``, false
}

// worstOdds is the least favourable power ratio among the enemies here, the
// same comparison a player gets from `consider`. Below 1 means the other
// side has the upper hand.
func worstOdds(room *rooms.Room, mob *mobs.Mob, f *fightState) float64 {
	self := actions.NewMobActorInRoom(mob, room)
	worst := 0.0
	for id := range f.Enemies {
		e := mobs.GetInstance(id)
		if e == nil || e.Character.RoomId != room.RoomId {
			continue
		}
		r := actions.Consider(self, actions.NewMobActorInRoom(e, room)).Ratio
		if worst == 0 || r < worst {
			worst = r
		}
	}
	return worst
}

// Arrows are not gone when they are loosed: they are in the dirt, in a
// tree, or in whatever they hit. An archer walks the ground afterwards and
// picks up what survived, which is anywhere between a handful and most of
// them. This is done for a bonded companion only: the player path is the
// engine's, and nothing here changes it.

// ammoOnHand counts the arrows a companion could loose right now, and the
// bundle they come out of, so what it spends in a fight can be measured.
func ammoOnHand(mob *mobs.Mob) (count int, itemId int) {
	w := mob.Character.Equipment.Weapon
	if w.ItemId < 1 {
		return 0, 0
	}
	tag := w.GetSpec().AmmoTag
	if tag == `` {
		return 0, 0
	}
	if w.Loaded {
		count++
	}
	for i := range mob.Character.Items {
		spec := mob.Character.Items[i].GetSpec()
		if spec.Type == items.Ammo && spec.AmmoTag == tag && mob.Character.Items[i].Uses > 0 {
			count += mob.Character.Items[i].Uses
			if itemId == 0 {
				itemId = mob.Character.Items[i].ItemId
			}
		}
	}
	return count, itemId
}

// gatherArrows puts some of what was loosed back in the quiver when the
// fighting stops. Returns how many were found.
func (m *AICompanionModule) gatherArrows(c *controller, mob *mobs.Mob, f *fightState) int {
	if !m.cfg.RecoverArrows || f == nil || f.AmmoAtStart <= 0 || f.AmmoItemId == 0 {
		return 0
	}
	now, _ := ammoOnHand(mob)
	spent := f.AmmoAtStart - now
	if spent <= 0 {
		return 0
	}
	// Somewhere between a tenth and most of them, and never all: a fight is
	// hard on arrows.
	lo, hi := m.cfg.RecoverArrowsMin, m.cfg.RecoverArrowsMax
	share := lo + (hi-lo)*float64(util.Rand(101))/100.0
	found := int(float64(spent) * share)
	if found <= 0 {
		return 0
	}

	// Back into the bundle they came from, or a fresh one if it emptied.
	for i := range mob.Character.Items {
		if mob.Character.Items[i].ItemId == f.AmmoItemId {
			mob.Character.Items[i].Uses += found
			m.noteGathered(c, mob, found)
			return found
		}
	}
	bundle := items.New(f.AmmoItemId)
	if bundle.ItemId == 0 {
		return 0
	}
	bundle.Uses = found
	if !mob.Character.StoreItem(bundle) {
		return 0
	}
	m.noteGathered(c, mob, found)
	return found
}

// noteGathered shows the companion walking the ground afterwards, and
// remembers it, so the arrows do not simply appear.
func (m *AICompanionModule) noteGathered(c *controller, mob *mobs.Mob, found int) {
	ammo := c.profile.ammoWord()
	mob.Command(fmt.Sprintf(`emote walks the ground and works %d %s loose from the dirt and the dead.`, found, ammo), 1.5)
	c.mind.addLine(Line{Kind: `event`, Text: fmt.Sprintf(`You recovered %d %s after the fight.`, found, ammo)}, m.cfg.WorkingMemoryLines)
	c.snapshotDue = true
	c.dirty = true
}
