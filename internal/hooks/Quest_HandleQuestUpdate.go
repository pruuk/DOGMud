package hooks

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/acquire"
	"github.com/GoMudEngine/GoMud/internal/conversation"
	"github.com/GoMudEngine/GoMud/internal/crafting"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/factions"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/templates"
	"github.com/GoMudEngine/GoMud/internal/textutil"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// statGainDescriptions maps lowercase stat names to a flavourful gain phrase.
// No hard numbers — per the project no-hard-numbers rule.
var statGainDescriptions = map[string]string{
	"strength":   "You feel notably stronger — your muscles carry a new permanence.",
	"dexterity":  "Your movements sharpen; a new quickness settles into your limbs.",
	"perception": "The world comes into slightly sharper focus, details you once missed now clear.",
	"vitality":   "A deep resilience takes root — your body feels more enduring than before.",
	"willpower":  "Your mind firms, a quiet resolve that wasn't there before now anchoring you.",
	"charisma":   "Something in your bearing shifts; others may find you more compelling.",
}

//
// Handles quest progress
//

func HandleQuestUpdate(e events.Event) events.ListenerReturn {

	evt, typeOk := e.(events.Quest)
	if !typeOk {
		mudlog.Error("Event", "Expected Type", "Quest", "Actual Type", e.Type())
		return events.Cancel
	}

	// MarkerOnly events refresh the Char.Quests minimap marker on a mid-quest
	// step advance and nothing else — the token, banner, and rewards were all
	// handled synchronously by questengine GrantQuest. Doing any of that work
	// again here would double the progress banner, so bail (the GMCP
	// quest-progress handler, a separate listener, still refreshes the marker).
	// The step did land, so the talk markers on NPCs may have changed.
	if evt.MarkerOnly {
		conversation.RefreshMarkers(evt.UserId)
		return events.Continue
	}

	//mudlog.Debug(`Event`, `type`, evt.Type(), `UserId`, evt.UserId, `QuestToken`, evt.QuestToken)

	// Give them a token
	remove := false
	if evt.QuestToken[0:1] == `-` {
		remove = true
		evt.QuestToken = evt.QuestToken[1:]
	}

	questInfo := quests.GetQuest(evt.QuestToken)
	if questInfo == nil {
		return events.Continue
	}

	questUser := users.GetByUserId(evt.UserId)
	if questUser == nil {
		return events.Continue
	}

	if remove {
		questUser.Character.ClearQuestToken(evt.QuestToken)
		conversation.RefreshMarkers(questUser.UserId)
		return events.Continue
	}

	// Repeatable-quest gate: refuse to (re)start a repeatable quest on its
	// post-completion cooldown or at its lifetime repeat_limit. The talk
	// menu already hides such an offer (PlayerState.QuestOpen); this is the
	// backstop for any other grant path.
	{
		_, incomingStep := quests.TokenToParts(evt.QuestToken)
		if incomingStep == `start` && questInfo.Repeatable {
			if questUser.Character.RepeatCapReached(questInfo) {
				questUser.SendText(messaging.CategoryRefusal,
					`You have done all the work there is here.`)
				return events.Continue
			}
			if questUser.Character.QuestCooldownActive(questInfo.QuestId) {
				questUser.SendText(messaging.CategoryRefusal,
					`You have put in the work recently. Rest a while before taking this on again.`)
				return events.Continue
			}
		}
	}

	// Try to advance the quest. If it fails, check whether the quest engine
	// already set this token (GrantQuest sets it synchronously for chain
	// evaluation, then fires this event for rewards). In that case, proceed
	// with reward processing.
	freshlyGranted := questUser.Character.GiveQuestToken(evt.QuestToken)
	if !freshlyGranted {
		// Already at this step? The quest engine pre-set it. Continue to rewards.
		if !questUser.Character.HasQuest(evt.QuestToken) {
			return events.Continue
		}
	}

	// The token has landed: the talk markers on NPCs may have changed.
	conversation.RefreshMarkers(questUser.UserId)

	// Bridge legacy/dialogue token grants into the questengine's quest_granted
	// triggers. The questengine fires quest_granted only from its own internal
	// grant chain, so a token granted by DIALOGUE (grantsQuest) or any other
	// legacy path would otherwise never fire a questengine quest_granted trigger.
	// Notify ONLY on a fresh grant: a questengine-initiated grant pre-sets the
	// token synchronously (GiveQuestToken returns false above), so it is skipped
	// here — preventing a double-fire of the same trigger.
	if freshlyGranted {
		qbridge := questengine.NewGameBridge(questUser, questUser.Character.RoomId)
		questengine.GetEngine().Notify("quest_granted", questengine.EventDetails{
			UserId:     questUser.UserId,
			RoomId:     questUser.Character.RoomId,
			QuestToken: evt.QuestToken,
		}, qbridge, qbridge)
	}

	_, stepName := quests.TokenToParts(evt.QuestToken)
	if stepName == `start` {
		if !questInfo.Secret {

			questUser.EventLog.Add(`quest`, fmt.Sprintf(`Given a new quest: <ansi fg="questname">%s</ansi>`, questInfo.Name))

			questUpTxt, _ := templates.Process("character/questup", fmt.Sprintf(`You have been given a new quest: <ansi fg="questname">%s</ansi>!`, questInfo.Name), questUser.UserId)
			questUser.SendText(messaging.CategorySystem, questUpTxt)
		}
	} else if stepName == `end` {

		if !questInfo.Secret {

			questUser.EventLog.Add(`quest`, fmt.Sprintf(`Completed a quest: <ansi fg="questname">%s</ansi>`, questInfo.Name))

			questUpTxt, _ := templates.Process("character/questup", fmt.Sprintf(`You have completed the quest: <ansi fg="questname">%s</ansi>!`, questInfo.Name), questUser.UserId)
			questUser.SendText(messaging.CategorySystem, questUpTxt)
		}

		// Reward messages, through the quest store's door.
		//
		// Delivered through SendTrio (M4d Task 6). An observer line may name
		// the completing player with {actor} (quests 32 and 35 since
		// 2026-10-17, which had said "you" to the watcher); SendTrio hides
		// that name from a watcher who cannot see them. The questgiver NPC
		// is narrated by their own literal name, which is not the Actor or
		// Actee this Audience names. The actor line has no actee to hide
		// from it: a quest has no second party.
		rewardLines := questInfo.Rewards.Narrate(textutil.TokenContext{
			ActorName:      questUser.Character.GetCharacterName(true),
			ActorPlainName: questUser.Character.GetCharacterName(false),
		})
		var rewardRoom messaging.Broadcaster
		if room := rooms.LoadRoom(questUser.Character.RoomId); room != nil {
			rewardRoom = room
		}
		// CategorySystem is never wrapped by the pipeline, so the completing
		// player's line wraps itself to their width (as justice/arrest.go
		// does, #430); reward lines are authored prose and run long. The
		// room's line rides CategoryEmote, which the pipeline does not wrap
		// either, so it wraps at the 80-column rule (2026-10-17 newbie-duo
		// playtest: Tarn's quest 41 line read 83 wide).
		messaging.SendTrio(messaging.Trio{
			Actor:    messaging.Say(messaging.CategorySystem, messaging.WrapAnsi(rewardLines.Actor, questUser.GetLineWidth())),
			Actee:    messaging.NoLine,
			Observer: messaging.Say(messaging.CategoryEmote, messaging.WrapAnsi(rewardLines.Observer, 80)),
		}, messaging.Audience{
			Actor:     questUser,
			ActorId:   questUser.UserId,
			ActorName: questUser.Character.GetCharacterName(true),
			ActeeName: messaging.NoName,
			Room:      rewardRoom,
		})
		// New quest to start?
		if len(questInfo.Rewards.QuestId) > 0 {

			events.AddToQueue(events.Quest{
				UserId:     questUser.UserId,
				QuestToken: questInfo.Rewards.QuestId,
			})

		}
		// Gold reward?
		if questInfo.Rewards.Gold > 0 {
			questUser.SendText(messaging.CategoryLoot, fmt.Sprintf(`You receive <ansi fg="gold">%s gold</ansi>!`, util.FormatNumber(questInfo.Rewards.Gold)))
			questUser.Character.Gold += questInfo.Rewards.Gold

			events.AddToQueue(events.EquipmentChange{
				UserId:     questUser.UserId,
				GoldChange: questInfo.Rewards.Gold,
			})

		}
		// Item reward?
		if questInfo.Rewards.ItemId > 0 {
			newItm := items.New(questInfo.Rewards.ItemId)
			questUser.SendText(messaging.CategoryLoot, fmt.Sprintf(`You receive %s<ansi fg="itemname">%s</ansi>!`, items.ArticlePrefix(newItm.DisplayName()), items.MidSentenceName(newItm.DisplayName())))
			// The seam raises ItemOwnership{Gained}, and CheckItemQuests
			// grants the item's QuestToken from it, so the token is not
			// hand-fired here (it used to be, when this site raised no
			// event; firing both would grant it twice).
			acquire.AcquireItem(questUser, newItm, acquire.AcquireReward)
		}
		// Item-stockpile reward? Grants N of each listed item (see
		// quests.ParseItemGrants). Routes through the acquire seam (StoreItem
		// underneath) so potions auto-fill a
		// bandolier, components a component bag, etc. Reusable "starter kit"
		// reward, mirroring the skill/stat/recipe multi-grants above.
		for _, grant := range quests.ParseItemGrants(questInfo.Rewards.ItemInfo) {
			var firstName string
			granted := 0
			for i := 0; i < grant.Qty; i++ {
				newItm := items.New(grant.ItemId)
				if newItm.ItemId == 0 {
					break // unknown item id — skip the whole grant
				}
				firstName = newItm.DisplayName()
				acquire.AcquireItem(questUser, newItm, acquire.AcquireReward)
				granted++
			}
			if granted == 0 {
				mudlog.Warn("Quest ItemReward", "token", evt.QuestToken,
					"itemId", grant.ItemId, "error", "unknown item id — skipped")
				continue
			}
			if granted > 1 {
				questUser.SendText(messaging.CategoryLoot, fmt.Sprintf(`You receive <ansi fg="itemname">%s</ansi> (x%d)!`, firstName, granted))
			} else {
				questUser.SendText(messaging.CategoryLoot, fmt.Sprintf(`You receive %s<ansi fg="itemname">%s</ansi>!`, items.ArticlePrefix(firstName), items.MidSentenceName(firstName)))
			}
		}
		// Condition reward?
		if questInfo.Rewards.ConditionId > 0 {
			questUser.AddCondition(questInfo.Rewards.ConditionId, `quest`)
		}
		// Stage 3.5: XP rewards removed. Progression is skill-based.
		// Skill reward? Supports one OR several skills (see quests.ParseSkillGrants).
		// Each grant is a floor-raise — it never downgrades a player already
		// above the target (so a veteran replaying a newbie spoke keeps rank).
		for _, grant := range quests.ParseSkillGrants(questInfo.Rewards.SkillInfo) {
			currentLevel := questUser.Character.GetSkillLevel(skills.SkillTag(grant.Skill))
			if currentLevel < grant.Level {
				newLevel := questUser.Character.TrainSkill(grant.Skill, grant.Level)

				skillData := struct {
					SkillName  string
					SkillLevel int
				}{
					SkillName:  grant.Skill,
					SkillLevel: newLevel,
				}
				skillUpTxt, _ := templates.Process("character/skillup", skillData, questUser.UserId)
				questUser.SendText(messaging.CategorySkillProgress, skillUpTxt)
			}
		}
		// Stat reward? Additive — permanently increases the named stat's
		// Training value. Supports one OR several stats (see quests.ParseStatGrants).
		// Unlike skill grants these are NOT floor-raises; each grant always
		// adds its amount (quests are one-shot linear, so re-grant isn't
		// a concern).
		for _, grant := range quests.ParseStatGrants(questInfo.Rewards.StatInfo) {
			if questUser.Character.IncreaseStat(grant.Stat, grant.Amount) {
				msg, ok := statGainDescriptions[grant.Stat]
				if !ok {
					msg = "You feel a subtle but permanent improvement wash over you."
				}
				statUpTxt, _ := templates.Process("character/statup", msg, questUser.UserId)
				questUser.SendText(messaging.CategorySkillProgress, statUpTxt)
			} else {
				mudlog.Warn("Quest StatReward", "token", evt.QuestToken,
					"stat", grant.Stat, "error", "unknown stat name — skipped")
			}
		}
		// Recipe reward? Grants each recipe ID to the player's KnownRecipes.
		// Silently skips recipes the player already knows (idempotent).
		for _, recipeId := range quests.ParseRecipeGrants(questInfo.Rewards.RecipeInfo) {
			if questUser.Character.LearnRecipe(recipeId) {
				name := recipeId // fallback: use the id if the recipe spec isn't found
				if spec := crafting.GetRecipe(recipeId); spec != nil {
					name = spec.Name
				}
				questUser.SendText(messaging.CategorySkillProgress, fmt.Sprintf(
					`<ansi fg="cyan">You have learned the recipe for </ansi><ansi fg="itemname">%s</ansi><ansi fg="cyan">.</ansi>`,
					name))
			}
		}
		// Spell reward?
		if questInfo.Rewards.SpellId != "" {
			if questUser.Character.LearnSpell(questInfo.Rewards.SpellId) {
				if spellData := spells.GetSpell(questInfo.Rewards.SpellId); spellData != nil {
					questUser.SendText(messaging.CategorySkillProgress, fmt.Sprintf(
						`<ansi fg="magenta-bold">You have learned the spell: <ansi fg="cyan-bold">%s</ansi></ansi>`,
						spellData.Name))
				}
			}
		}
		// Move them to another room/area?
		if questInfo.Rewards.RoomId > 0 {
			questUser.SendText(messaging.CategorySystem, `You are suddenly moved to a new place!`)

			if room := rooms.LoadRoom(questUser.Character.RoomId); room != nil {
				sendVisualRoomText(room, messaging.CategoryEmote, fmt.Sprintf(`<ansi fg="username">%s</ansi> is suddenly moved to a new place!`, questUser.Character.Name), questUser.UserId)
			}

			rooms.MoveToRoom(questUser.UserId, questInfo.Rewards.RoomId)
		}
		// Faction reputation reward?
		if questInfo.Rewards.RepFaction != "" && questInfo.Rewards.RepAmount != 0 {
			factions.BumpRep(questInfo.Rewards.RepFaction, questUser.UserId, questInfo.Rewards.RepAmount)
		}

		// Repeatable quest: reset progress so it can be taken again, count
		// the run and stamp its cooldown (RecordCompletion). Non-repeatable
		// quests are unaffected.
		if questInfo.Repeatable {
			questUser.Character.ClearQuestToken(evt.QuestToken)
			questUser.Character.RecordCompletion(questInfo)
		}
	} else {
		if !questInfo.Secret {

			questUser.EventLog.Add(`quest`, fmt.Sprintf(`Made progress on a quest: <ansi fg="questname">%s</ansi>`, questInfo.Name))

			questUpTxt, _ := templates.Process("character/questup", fmt.Sprintf(`You've made progress on the quest: <ansi fg="questname">%s</ansi>!`, questInfo.Name), questUser.UserId)
			questUser.SendText(messaging.CategorySystem, questUpTxt)
		}
	}

	return events.Continue
}
