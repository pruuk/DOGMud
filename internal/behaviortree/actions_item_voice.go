package behaviortree

import (
	"fmt"
	"sync"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/narration"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/targeting"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// The voice nodes (item behaviour slice 2, Rules 16 to 18). All four need
// an item subject (Rule 8), and all four are the Pinnacle feature:
// PinnacleItemsEnabled off silences them.
//
// Pacing is split across two nodes so a tree reads the way the Pinnacle
// tick behaved. `chatter_ready` gates an AMBIENT line: the item's cooldown
// open, someone to hear it and every listener past their cap, then the
// level's chance, drawn last so a closed round draws nothing. `speak` sends
// a line and arms the item's cooldown, and for an item_idle line starts
// every listener's cap. An event branch (on_equip, on_kill...) speaks
// without chatter_ready: it bypasses the cap and the chance, and speak
// still refuses while the item's cooldown is closed.

// speakNextRoundKey is the item state key holding the round its cooldown
// opens again.
const speakNextRoundKey = `speak_next_round`

// The speak audiences: the holder and the room (default), or the holder
// alone.
const (
	speakToAll    = `all`
	speakToHolder = `holder`
)

var (
	listenerCapMu sync.Mutex
	// listenerCapNext is the round each listening player may next hear an
	// ambient item line, across every item (Rule 17). In memory only.
	listenerCapNext = map[int]uint64{}
)

// ResetItemListenerCapsForTest empties the listener caps and returns a
// restore func.
func ResetItemListenerCapsForTest() func() {
	listenerCapMu.Lock()
	orig := listenerCapNext
	listenerCapNext = map[int]uint64{}
	listenerCapMu.Unlock()
	return func() {
		listenerCapMu.Lock()
		listenerCapNext = orig
		listenerCapMu.Unlock()
	}
}

func pinnacleItemsOn() bool {
	return bool(configs.GetConfig().GamePlay.PinnacleItemsEnabled)
}

// itemVoiceOf is the subject item's voice, through its template's tree.
func itemVoiceOf(ctx *EvalContext) *ItemVoice {
	if ctx == nil || ctx.Item == nil {
		return nil
	}
	spec := items.GetItemSpec(ctx.Item.ItemId)
	if spec == nil || spec.Behavior == `` {
		return nil
	}
	return GetEngine().GetItemVoice(spec.Behavior)
}

// chatterPacing is a chatter level's cooldown in rounds and chance in
// percent, from config.yaml.
func chatterPacing(level string) (uint64, int) {
	b := configs.GetBalanceConfig()
	switch level {
	case ChatterQuiet:
		return uint64(b.ItemChatterQuietCooldownRounds), int(b.ItemChatterQuietChancePct)
	case ChatterChatty:
		return uint64(b.ItemChatterChattyCooldownRounds), int(b.ItemChatterChattyChancePct)
	}
	return uint64(b.ItemChatterNormalCooldownRounds), int(b.ItemChatterNormalChancePct)
}

func itemCooldownOpen(ctx *EvalContext, now uint64) bool {
	next, ok := characters.MiscRound(ctx.MobState.Get(speakNextRoundKey))
	return !ok || now >= next
}

// condChatterReady: an ambient line may go out this round. An item with no
// holder (on a floor) has nothing to speak through, so it fails before any
// roll is drawn.
func condChatterReady(_ map[string]any, ctx *EvalContext) Result {
	if !pinnacleItemsOn() {
		return Failure
	}
	if ctx == nil || ctx.Item == nil || (ctx.Item.UserId == 0 && ctx.Item.MobInstanceId == 0) {
		return Failure
	}
	voice := itemVoiceOf(ctx)
	if voice == nil {
		return Failure
	}
	now := util.GetRoundCount()
	if !itemCooldownOpen(ctx, now) {
		return Failure
	}
	room := rooms.LoadRoom(ctx.RoomId)
	if room == nil {
		return Failure
	}
	listeners := room.GetPlayers()
	if len(listeners) == 0 {
		return Failure
	}
	listenerCapMu.Lock()
	for _, uid := range listeners {
		if now < listenerCapNext[uid] {
			listenerCapMu.Unlock()
			return Failure
		}
	}
	listenerCapMu.Unlock()
	_, chance := chatterPacing(voice.Chatter)
	if util.Rand(100) >= chance {
		return Failure
	}
	return Success
}

// condHungerOverdue: the holder's hunger anchor (the round the item last
// fed, pinnacle_hunger_anchor) is more than `fraction` of the item's
// hunger window behind. Spec X14: the Pinnacle tick's warning state.
func condHungerOverdue(params map[string]any, ctx *EvalContext) Result {
	c := itemHolder(ctx)
	if c == nil {
		return Failure
	}
	spec := items.GetItemSpec(ctx.Item.ItemId)
	if spec == nil || spec.HungerRounds <= 0 {
		return Failure
	}
	anchor, ok := characters.MiscRound(c.GetMiscData(`pinnacle_hunger_anchor`))
	if !ok {
		return Failure
	}
	now := util.GetRoundCount()
	fraction := getFloatParam(params, `fraction`, 0.75)
	if now > anchor && float64(now-anchor) > float64(spec.HungerRounds)*fraction {
		return Success
	}
	return Failure
}

// actSpeak: `speak` with `pool` (a pool of the tree's speech), `to: all |
// holder` (default all) and `paced: true | false` (default true). It picks
// a line through the narration core (the picker seam, so a seeded
// util.Rand replays it), sends the holder "<Item> says" and, for `all`,
// the room "<Name>'s <Item> mutters" heard by everyone with the holder's
// name hidden at each listener's sight (spec X17; never deafen-filtered,
// owner ruling 6). A mob holder gets the room line only (ruling R6), so a
// mob holder with `to: holder` has no audience: Failure, nothing sent or
// armed. Paced: refuses while the item's cooldown is closed, and arms it.
// An item_idle line starts every listener's cap only when the room line was
// actually sent.
func actSpeak(params map[string]any, ctx *EvalContext) Result {
	if !pinnacleItemsOn() {
		return Failure
	}
	voice := itemVoiceOf(ctx)
	if voice == nil {
		return Failure
	}
	lines := voice.Speech[getStringParam(params, `pool`)]
	if len(lines) == 0 {
		return Failure
	}
	paced := true
	if _, ok := params[`paced`]; ok {
		paced = getBoolParam(params, `paced`)
	}
	now := util.GetRoundCount()
	if paced && !itemCooldownOpen(ctx, now) {
		return Failure
	}
	spec := items.GetItemSpec(ctx.Item.ItemId)
	line := narration.Render(narration.Variants{Actor: lines}, nil, nil).Actor
	if spec == nil || line == `` {
		return Failure
	}

	var holderName, nameTag string
	var holderUser *users.UserRecord
	switch {
	case ctx.Item.UserId > 0:
		holderUser = users.GetByUserId(ctx.Item.UserId)
		if holderUser == nil || holderUser.Character == nil {
			return Failure
		}
		holderName, nameTag = holderUser.Character.Name, `username`
	case ctx.Item.MobInstanceId > 0:
		m := mobs.GetInstance(ctx.Item.MobInstanceId)
		if m == nil {
			return Failure
		}
		holderName, nameTag = m.Character.Name, `mobname`
	default:
		return Failure // a floor item has no holder to speak through
	}

	toRoom := getStringParam(params, `to`) != speakToHolder
	if holderUser == nil && !toRoom {
		return Failure // a mob holder has no one to hear a holder-only line
	}

	if holderUser != nil {
		holderUser.SendText(messaging.CategorySystem, fmt.Sprintf(
			`<ansi fg="item">%s</ansi> says, "<ansi fg="yellow">%s</ansi>"`, spec.Name, line))
	}
	room := rooms.LoadRoom(ctx.RoomId)
	roomHeard := toRoom && room != nil
	if roomHeard {
		exclude := []int{}
		if holderUser != nil {
			exclude = append(exclude, holderUser.UserId)
		}
		room.SendTextHidingNames(messaging.CategorySystem, fmt.Sprintf(
			`<ansi fg="%s">%s</ansi>'s <ansi fg="item">%s</ansi> mutters, "<ansi fg="yellow">%s</ansi>"`,
			nameTag, holderName, spec.Name, line),
			[]string{holderName}, messaging.HideSpeakerNames, exclude...)
	}

	if paced {
		cooldown, _ := chatterPacing(voice.Chatter)
		ctx.MobState.Set(speakNextRoundKey, now+cooldown)
	}
	if ctx.Event.EventType == `item_idle` && roomHeard {
		capRounds := uint64(configs.GetBalanceConfig().ItemChatterListenerCapRounds)
		listenerCapMu.Lock()
		for _, uid := range room.GetPlayers() {
			listenerCapNext[uid] = now + capRounds
		}
		listenerCapMu.Unlock()
	}
	return Success
}

// actTauntPull: the Aegis's tank loop (spec X15, ruling R5). A player
// holder's current foe, a mob fighting someone else, is made to fight the
// holder, through the taunt hold so the per-round re-aggro cannot flip it
// straight back. A mob holder, a holder at peace, a non-combatant foe or a
// foe already on the holder: nothing to do. Always Success, so a tree's
// taunt branch never falls through to another line.
func actTauntPull(_ map[string]any, ctx *EvalContext) Result {
	if !pinnacleItemsOn() || ctx == nil || ctx.Item == nil || ctx.Item.UserId <= 0 {
		return Success
	}
	u := users.GetByUserId(ctx.Item.UserId)
	if u == nil || u.Character == nil {
		return Success
	}
	target := u.Character.CurrentCombatTarget()
	if target.MobInstanceId <= 0 {
		return Success
	}
	foe := mobs.GetInstance(target.MobInstanceId)
	if foe == nil || foe.IsNonCombatant() {
		return Success
	}
	if foe.Character.CurrentCombatTarget().UserId == u.UserId {
		return Success
	}
	holdRounds := int(configs.GetBalanceConfig().TauntHoldRounds)
	targeting.CommitTaunt(&foe.Character, state.ActorRef{UserId: u.UserId}, holdRounds)
	return Success
}
