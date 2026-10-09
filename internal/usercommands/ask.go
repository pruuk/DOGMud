package usercommands

import (
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/behaviortree"
	"github.com/GoMudEngine/GoMud/internal/companionai"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/llm"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// deliverDialogue executes the YAML dialogue lookup and has the mob respond.
// It is called both by the normal (non-LLM) path and the LLM-unavailable fallback.
func deliverDialogue(df *dialogue.DialogueFile, mob *mobs.Mob, mobInstanceId int, userId int, topic string, ps *dialogue.PlayerState) {
	if df == nil {
		mob.Command(`emote shakes their head.`)
		return
	}

	// Tree nodes first — quest offers, quest progress, and gated content live
	// here.
	if nodeText, hints, moodChange, ok := dialogue.TreeAdvance(df, mobInstanceId, userId, topic, ps); ok {
		mob.Command(`say ` + nodeText)
		if hints != `` {
			if u := users.GetByUserId(userId); u != nil {
				u.SendText(messaging.CategoryDialogueHint, fmt.Sprintf(`<ansi fg="181">  [%s]</ansi>`, hints))
			}
		}
		dialogue.ShiftMood(mobInstanceId, moodChange, df.DefaultMood)
		return
	}

	response, moodChange, ok, usedFallback := dialogue.MatchWithFallbackInfo(df, mobInstanceId, topic, ps)

	// A pointed "do you have a quest?" that only reaches the generic catch-all
	// (or nothing) deserves a clear answer, not unrelated filler.
	if isQuestInquiry(topic) && (!ok || usedFallback) {
		mob.Command(`say I have no task for you right now.`)
		return
	}

	if ok {
		mob.Command(`say ` + response)
		dialogue.ShiftMood(mobInstanceId, moodChange, df.DefaultMood)
		return
	}

	mob.Command(`emote shakes their head.`)
}

// isQuestInquiry reports whether the player is asking an NPC specifically about
// quests/work, so a non-quest NPC can answer clearly instead of with filler.
func isQuestInquiry(topic string) bool {
	switch strings.ToLower(strings.TrimSpace(topic)) {
	case "quest", "quests", "task", "tasks", "job", "jobs", "work":
		return true
	}
	return false
}

func Ask(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {

	args := util.SplitButRespectQuotes(rest)

	if len(args) < 2 {

		for _, mId := range room.GetMobs(rooms.FindCharmed) {
			mob := mobs.GetInstance(mId)
			if mob == nil {
				continue
			}
			if mob.Character.IsCharmed(user.UserId) {
				mob.Command(`emote regards you blankly. It doesn't seem to understand.`)
				return true, nil
			}
		}

		user.SendText(messaging.CategorySystem, `You must <ansi fg="command">ask</ansi> <ansi fg="mobname">someone</ansi> <ansi fg="yellow">something</ansi>`)
		return true, nil
	}

	// #454, owner call 2: a typed name resolves only at full sight, or
	// asking would confirm who is there in the dark.
	searchName, refusal := actions.AimBySight(user.Character, user.UserId, room, args[0], `ask`)
	if refusal != `` {
		user.SendText(messaging.CategorySystem, refusal)
		return true, nil
	}

	// Only ask charmed players or mobs to do stuff
	target, err := actions.ResolveTargetActor(room, searchName, actions.ResolveTargetOptions{Viewer: user.Character})
	if err != nil {
		user.SendText(messaging.CategorySystem, `ask who what?`)
		return true, nil
	}
	if target.IsPlayer() {
		user.SendText(messaging.CategorySystem, `You can't ask another player.`)
		return true, nil
	}

	mob := target.(*actions.MobActor).Mob
	mobId := mob.InstanceId

	// A sleeping NPC cannot hold a conversation — and must not grant a quest.
	// Before this guard, `ask marek quest` while he slept produced his full
	// offer speech and handed over the quest item.
	if actions.RefuseMobIfAsleep(mob, user) {
		return true, nil
	}

	args = args[1:]

	if !mob.Character.IsCharmed() {
		// The topic is raw player input echoed to the whole room, and this
		// path has no mute gate — escape before it reaches AnsiParse.
		topic := util.EscapeAnsiTags(strings.Join(args, ` `))
		room.SendTextVisual(messaging.CategoryMobEmote, fmt.Sprintf(`<ansi fg="username">%s</ansi> asks <ansi fg="mobname">%s</ansi> about "%s"`, user.Character.Name, mob.Character.Name, topic), user.UserId)
	}

	// players may type "ask <mob> to <do something>"
	if len(args) > 1 && strings.ToLower(args[0]) == `to` {
		args = args[1:]
	}
	if len(args) > 1 && strings.ToLower(args[0]) == `about` {
		args = args[1:]
	}

	// A bonded AI companion converses. The aicompanion module claims the ask
	// when it drives this mob; for every other mob this returns false and the
	// normal paths below run unchanged.
	if companionai.RouteAsk(user.UserId, mob.InstanceId, strings.Join(args, ` `)) {
		return true, nil
	}

	// Companions can't be ordered around — they act on their own
	if mob.Character.IsCharmed(user.UserId) {
		mob.Command(`emote regards you blankly. It doesn't seem to understand.`)
		return true, nil
	}

	rest = strings.Join(args, ` `)

	if askNpcChain(user, mob, mobId, room, rest, true) {
		// A behaviour tree answered: it speaks for itself, and the original
		// command returned here without telling the neighbours.
		return true, nil
	}

	room.SendTextToExits(`You hear someone talking.`, true)

	return true, nil
}

// askNpcChain is everything that happens once a question has reached an NPC:
// the quest engine, the behaviour tree, an LLM dialogue profile if the mob
// has one, and the authored YAML dialogue as the fallback. Split out of Ask
// so a bonded AI companion can ask an NPC a question on its owner's behalf
// and get the same answer a player would (see AskNpcForOwner). The dialogue
// memory is keyed to the owner, so the NPC treats the pair as one party.
// allowQuest is false for a question the player did not ask for: the NPC
// still answers, but the player's quests are not touched by it.
// It reports whether a behaviour tree took the question, which in the
// original Ask ended the command before the "you hear someone talking"
// broadcast. Callers must respect that or every behaviour-tree answer
// leaks a line into the neighbouring rooms.
func askNpcChain(user *users.UserRecord, mob *mobs.Mob, mobId int, room *rooms.Room, rest string, allowQuest bool) (handled bool) {
	if allowQuest {
		// Quest engine: dialogue notification
		bridge := questengine.NewGameBridge(user, room.RoomId)
		questengine.GetEngine().Notify("dialogue", questengine.EventDetails{
			UserId: user.UserId,
			RoomId: room.RoomId,
			MobId:  int(mob.MobId),
			Topic:  rest,
		}, bridge, bridge)
	}

	// Build PlayerState for quest/item gating in dialogue
	ps := buildPlayerState(user)

	// Behavior tree: try before JS
	if behaviortree.TryMobBehavior(mobId, behaviortree.EventContext{
		EventType: "player_ask",
		UserId:    user.UserId,
		Text:      rest,
		RoomId:    room.RoomId,
	}) {
		return true
	}

	jsHandled := false

	// LLM path: fires if JS didn't handle it and the mob has an LLM profile configured.
	if !jsHandled && mob.LLMProfile != nil && bool(configs.GetLLMConfig().Enabled) {
		cfg := configs.GetLLMConfig()
		mem := dialogue.GetMemory(mobId, user.UserId)
		llmCtx := llm.ConversationContext{
			MobName:          mob.Character.Name,
			ZoneName:         mob.Zone,
			PlayerName:       user.Character.Name,
			CurrentMood:      string(dialogue.GetMood(mobId, mob.LLMProfile.DefaultMood)),
			RecentTopics:     mem.RecentTopics,
			QuestContext:     buildQuestContext(user, int(mob.MobId)),
			PlayerCondition:  buildPlayerCondition(user),
			TutorialProgress: buildTutorialContext(user),
		}
		mob.Command(`emote pauses thoughtfully.`)
		mobIdCopy := mobId
		userIdCopy := user.UserId
		restCopy := rest
		llm.AskAsync(mob.LLMProfile, string(cfg.Endpoint), int(cfg.Timeout),
			mobIdCopy, llmCtx, restCopy,
			func(response string) {
				m := mobs.GetInstance(mobIdCopy)
				if m != nil {
					// LLM output is untrusted: the prompt carries the player's
					// own text, so "reply with exactly </ansi><ansi fg=...>"
					// would launder an injection through a named NPC to the
					// whole room. Escaped here rather than in mobcommands.Say,
					// which also carries authored dialogue YAML whose <ansi>
					// markup is legitimate.
					m.Command(`say ` + util.EscapeAnsiTags(response))
					dialogue.UpdateMemory(mobIdCopy, userIdCopy, "", nil, restCopy)
				}
			},
			func() {
				// LLM unavailable — fall through to YAML dialogue.
				m := mobs.GetInstance(mobIdCopy)
				if m != nil {
					df := dialogue.Load(int(m.MobId), m.Zone)
					deliverDialogue(df, m, mobIdCopy, user.UserId, restCopy, ps)
				}
			},
		)
		jsHandled = true // prevent double-response from YAML block below
	}

	if !jsHandled {
		df := dialogue.Load(int(mob.MobId), mob.Zone)
		deliverDialogue(df, mob, mobId, user.UserId, rest, ps)
	}

	return false
}

// AskNpcForOwner lets a charmed companion put a question to an NPC as though
// its owner had asked it. Installed into internal/companionai at init, so
// modules can reach it without internal/ importing modules/. The owner must
// be in the room: the reply is spoken aloud there, and the companion hears
// it like anyone else. Unless authorized, the quest engine is not told,
// because the model must not be able to move a player through a quest on
// its own.
func AskNpcForOwner(ownerUserId int, mobInstanceId int, text string, authorized bool) bool {
	user := users.GetByUserId(ownerUserId)
	mob := mobs.GetInstance(mobInstanceId)
	text = strings.TrimSpace(text)
	if user == nil || user.Character == nil || mob == nil || text == `` {
		return false
	}
	if mob.Character.IsCharmed() || mob.Character.RoomId != user.Character.RoomId {
		return false
	}
	room := rooms.LoadRoom(mob.Character.RoomId)
	if room == nil {
		return false
	}
	// A companion chatting to a shopkeeper must not advance, grant or spend
	// its owner's quests. Only a question the owner asked for in the moment
	// carries that authority.
	if askNpcChain(user, mob, mob.InstanceId, room, text, authorized) {
		return true
	}
	room.SendTextToExits(`You hear someone talking.`, true)
	return true
}

func init() {
	companionai.SetNpcAsker(AskNpcForOwner)
}
