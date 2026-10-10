// Round ticks for players
package hooks

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/behaviortree"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/crafting"
	"github.com/GoMudEngine/GoMud/internal/enchantments"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/gametime"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mutations"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
	"github.com/GoMudEngine/GoMud/internal/textutil"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/GoMudEngine/GoMud/internal/worldevents"
)

// enchantTierUpBlockedCooldown throttles the "this cannot deepen" line. The
// roll retries every combat round, so without a throttle a wearer sitting at
// the ceiling would be told the same thing several times a fight.
const enchantTierUpBlockedCooldown = `enchant-tierup-blocked`

// enchantTierUpWouldBreach reports whether advancing this item one enchantment
// tier would carry the wearer's total reservation past the ceiling.
//
// Tier-up is a PASSIVE breach with no action to refuse: it rolls every combat
// round on every Chrysalis-enchanted equipped item, and it DOUBLES the reserved
// fraction at low tiers, so a character sitting just under the ceiling can cross
// it mid-fight having done nothing. Grandfathering means it can never force a
// dismissal; it simply must not make things worse.
//
// Unlike the equip seam, nothing has been placed or displaced yet here, so the
// current reservation is the real one and the delta can be priced directly.
func enchantTierUpWouldBreach(ch *characters.Character, itm *items.Item) bool {
	if itm.ReservePool == `` {
		return false
	}
	pool := characters.Pool(itm.ReservePool)
	hands := itm.GetSpec().Hands
	added := ch.EnchantReserveAt(itm.EnchantType, itm.EnchantTier+1, hands, pool) -
		ch.EnchantReserveAt(itm.EnchantType, itm.EnchantTier, hands, pool)
	return ch.WouldBreachReservationCap(pool, added)
}

// enchantApplyWouldBreach reports whether binding a fresh tier-0 `enchantType`
// to `itm` would carry the wearer past the ceiling, on which pool, and by how
// much the binding would ADD. The added figure is returned rather than
// recomputed at the call site because ReservationRefusal needs it to tell a
// "this one thing is too heavy on its own" refusal from a "you are already
// full" one.
//
// Subtracting what the target already reserves is what makes RE-enchanting
// work: the old enchantment is replaced rather than stacked, so only the
// difference is new.
func enchantApplyWouldBreach(ch *characters.Character, itm *items.Item, enchantType string) (characters.Pool, int, bool) {
	def := enchantments.GetEnchantment(enchantType)
	if def == nil || def.ReservePool == `` {
		return ``, 0, false
	}
	pool := characters.Pool(def.ReservePool)
	added := ch.EnchantReserveAt(enchantType, 0, itm.GetSpec().Hands, pool) -
		ch.ItemReserveOnPool(*itm, pool)
	return pool, added, ch.WouldBreachReservationCap(pool, added)
}

// tickChrysalisEnchantments advances every equipped Chrysalis-enchanted item by
// one round of use and returns the lines to send the wearer, all of which
// belong to messaging.CategorySkillProgress.
//
// randN is the roll source (production passes util.Rand). It is a parameter so
// the ceiling behaviour can be driven deterministically from a test rather than
// waiting on a 2%-per-round die.
func tickChrysalisEnchantments(ch *characters.Character, randN func(int) int) []string {

	bal := configs.GetBalanceConfig()
	maxTier := int(bal.EnchantMaxTier)

	lines := []string{}

	for _, itemPtr := range ch.Equipment.GetAllItemPtrs() {
		if !itemPtr.HasChrysalisEnchantment() {
			continue
		}
		itemPtr.EnchantUses++

		eDef := enchantments.GetEnchantment(itemPtr.EnchantType)
		if eDef == nil {
			continue
		}

		currentTier := itemPtr.EnchantTier
		if currentTier >= maxTier || currentTier >= len(eDef.Tiers)-1 {
			continue
		}

		threshold := float64(bal.EnchantTierUsesBase) * math.Pow(float64(bal.EnchantTierUsesScale), float64(currentTier))
		if float64(itemPtr.EnchantUses) < threshold {
			continue
		}
		if randN(100) >= int(float64(bal.EnchantTierUpBaseChance)*100) {
			continue
		}

		if enchantTierUpWouldBreach(ch, itemPtr) {
			// Say why, but not every round. EnchantUses is deliberately NOT
			// reset: the item stays ready to advance the moment its wearer
			// makes room, rather than losing the progress it earned.
			if ch.TryCooldown(enchantTierUpBlockedCooldown, `200 rounds`) {
				lines = append(lines, `<ansi fg="yellow">Your `+itemPtr.DisplayName()+
					` strains to deepen, but your gear already holds too much of you in `+
					`reserve. Set another burden aside and it can grow.</ansi>`)
			}
			continue
		}

		itemPtr.EnchantTier++
		itemPtr.EnchantUses = 0
		enchantments.ApplyTier(itemPtr, eDef, itemPtr.EnchantTier)

		newTier := itemPtr.EnchantTier
		if newTier < len(eDef.Tiers) && eDef.Tiers[newTier].TierUpMessage != `` {
			lines = append(lines, fmt.Sprintf(`<ansi fg="magenta">%s</ansi>`, eDef.Tiers[newTier].TierUpMessage))
		}
	}

	return lines
}

//
// Player Round Tick
//

func UserRoundTick(e events.Event) events.ListenerReturn {

	roomsWithPlayers := rooms.GetRoomsWithPlayers()
	for _, roomId := range roomsWithPlayers {
		// Get rooom
		if room := rooms.LoadRoom(roomId); room != nil {
			room.RoundTick()

			// Mutation graph: project ally auras (Commanding Presence, …) onto
			// the room's other players, and enemy auras (Dissonance Organ, …)
			// onto its in-combat mobs, while the owner is in combat.
			applyRoomAllyAuras(room)
			applyRoomEnemyAuras(room)

			allowIdleMessages := true
			behaviortree.TryRoomBehavior(roomId, behaviortree.EventContext{
				EventType: "room_idle",
				RoomId:    roomId,
			})

			if allowIdleMessages {

				chanceIn100 := 5
				if room.RoomId == -1 {
					chanceIn100 = 20
				}

				var idleMsgs []string

				if len(room.IdleMessages) > 0 {
					idleMsgs = room.IdleMessages
				} else {
					if zCfg := rooms.GetZoneConfig(room.Zone); zCfg != nil {
						if len(zCfg.IdleMessages) > 0 {
							idleMsgs = zCfg.IdleMessages
						}
					}
				}

				idleMsgCt := len(idleMsgs)
				if idleMsgCt > 0 && util.Rand(100) < chanceIn100 {

					if targetRoomId, err := strconv.Atoi(idleMsgs[0]); err == nil {
						idleMsgCt = 0
						if tgtRoom := rooms.LoadRoom(targetRoomId); tgtRoom != nil {
							idleMsgs = tgtRoom.IdleMessages
							idleMsgCt = len(idleMsgs)
						}
					}

					if idleMsgCt > 0 {
						// pick a random message
						idleMsgIndex := uint8(util.Rand(idleMsgCt))

						// If it's a repeating message, treat it as a non-message
						// (Unless it's the only one)
						if idleMsgIndex != room.LastIdleMessage || idleMsgCt == 1 {

							room.LastIdleMessage = idleMsgIndex

							msg := idleMsgs[idleMsgIndex]
							if msg != `` {
								wrappedMsg := util.SplitStringNL(msg, 80)
								// Idle flavor text is visual; the visual
								// pipeline judges each reader's sight, dark room
								// or lit. The dark-room branch that tested the
								// nightvision FLAG is gone (lighting plan 5c).
								sendVisualRoomText(room, messaging.CategoryRoomDescription, wrappedMsg)
							}

						}
					}

				}
			}

			for _, uId := range room.GetPlayers() {

				user := users.GetByUserId(uId)
				if user == nil {
					continue
				}

				if user.Character.HasAdjective(`zombie`) {
					user.Command(`zombieact`)
				}

				// Roundtick any cooldowns
				user.Character.Cooldowns.RoundTick()

				if user.Character.Charmed != nil && user.Character.Charmed.RoundsRemaining > 0 {
					user.Character.Charmed.RoundsRemaining--
				}

				// #220: a light or darkness about to run out on this Trigger
				// has its room snapshotted first, for its end line at the prune.
				keepEndLineSnapshots(user.Character, room)

				if triggeredConditions := user.Character.Conditions.Trigger(); len(triggeredConditions) > 0 {

					//
					// Fire onTrigger for condition script
					//
					triggeredConditionIds := []int{}
					for _, condition := range triggeredConditions {

						trigConditionSpec := conditions.GetConditionSpec(condition.ConditionId)

						// Send YAML trigger text (if defined), including on
						// the condition's final, expiring trigger. PruneConditions'
						// own end narration still follows as a separate
						// line for the record's close. Matches the mob
						// round tick's tickMobConditions, which narrates its own
						// trigger text the same way.
						//
						// Bug found migrating Bleeding to a record (slice 1,
						// task 9): this used to gate the WHOLE loop body —
						// text AND the TickPool harm/restore below — behind
						// !Expired(), which silently dropped the tick/harm
						// effect on any condition's final trigger (invisible until
						// now because every existing record used a trigger
						// count far above 1: Warcry/Rally 25, ConvictionWard/
						// Regenerating/Poisoned 10). A record created with
						// exactly one trigger left, which slice 1's three-round
						// Bleeding produced for every ordinary duration, never
						// applied its one and only tick. tickMobConditions
						// never had this defect: it always applies TickAmount
						// and always narrates the flavor text too.
						//
						// Whole-branch review (slice 1): the text was still
						// gated on !Expired() even after the harm/restore fix
						// above, so a one-trigger record (slice 1's bleed
						// producers all made one-trigger records) applied its
						// harm silently and only the
						// prune pass's end line was ever seen. The harm AND
						// the text now land on every trigger, including the
						// expiring one; the prune pass's end line follows as
						// the intended second line, not a replacement for the
						// first.
						if trigConditionSpec != nil && trigConditionSpec.Narration(conditions.PhaseTrigger).Len() > 0 {
							roles := trigConditionSpec.Narrate(conditions.PhaseTrigger,
								user.Character.GetCharacterName(true),
								user.Character.GetCharacterName(false))
							if roles.Actee != "" {
								user.SendText(messaging.CategoryConditionApply, roles.Actee)
							}
							if roles.Observer != "" {
								if r := rooms.LoadRoom(user.Character.RoomId); r != nil {
									// HidingNames: sight-gating alone leans on tag-based
									// Anonymize, which cannot see a bare name, and 17
									// shipped condition observer lines authored an
									// {actee_plain}. See Condition_ApplyConditions.go.
									// Only to the players who perceive the holder:
									// a sneak-hidden player read to the room as
									// "Ordel Quist continues meditating." (#458).
									r.SendTextVisualHidingNames(messaging.CategoryConditionApply,
										roles.Observer,
										[]string{user.Character.GetCharacterName(false)},
										append(conditionLineUnseenBy(r, user.Character), user.UserId)...)
								}
							}
						}

						// Apply config-driven tick amount. A tick_pool
						// condition that arrived through the async event path
						// carries TickAmount 0; fillZeroTickAmount computes and
						// caches it (shared with the mob round tick). Runs on
						// EVERY trigger, including the final one that also
						// expires the condition (see the note above).
						if trigConditionSpec != nil && trigConditionSpec.TickPool != "" {
							tickAmt := fillZeroTickAmount(user.Character, condition, trigConditionSpec)
							// tickAmt is SIGNED: conditions.ComputeTickAmount returns a
							// negative value for TickPercent < 0, so this is a
							// damage-over-time delivery path as well as a regen one.
							// Routing it to ApplyRestore alone would silently delete
							// every DoT condition, because ApplyRestore no-ops on
							// non-positive input. Hence the sign split; ApplyHarm
							// takes a POSITIVE amount, so negate.
							//
							// The harm is the record's caster's (#240), so a tick
							// that kills names them; a record nobody cast harms
							// anonymously, as before. A player victim's damage map
							// is left alone, as a spell's direct damage leaves it
							// (creditSpellDamage credits mob victims only).
							switch trigConditionSpec.TickPool {
							case "health":
								if tickAmt > 0 {
									user.Character.ApplyRestore(characters.PoolHealth, tickAmt)
								} else if tickAmt < 0 {
									user.Character.ApplyHarm(characters.PoolHealth, -tickAmt, condition.Caster)
									// Damage is damage: wake a sleeper and drop
									// cancel-on-damage records, as the poison and
									// bleed hook always did (slice 1, change 5).
									cancelCraftOrSalvageOnDamage(user.Character)
									cancelDamageConditions(user.Character)
									// Capture the cause at the moment the tick lands: a
									// tick that is the record's last trigger arrives
									// already Expired (Conditions.Trigger decrements
									// TriggersLeft before returning it), and the record
									// can also be pruned before the death announcement
									// listener runs. See tickCauseFor and deathCauseFor.
									if cause := tickCauseFor(trigConditionSpec); cause != "" {
										user.Character.LastTickCause = cause
										user.Character.LastTickCauseRound = util.GetRoundCount()
									}
								}
							case "stamina":
								if tickAmt > 0 {
									user.Character.ApplyRestore(characters.PoolStamina, tickAmt)
								} else if tickAmt < 0 {
									user.Character.ApplyHarm(characters.PoolStamina, -tickAmt, condition.Caster)
								}
							case "conviction":
								if tickAmt > 0 {
									user.Character.ApplyRestore(characters.PoolConviction, tickAmt)
								} else if tickAmt < 0 {
									user.Character.ApplyHarm(characters.PoolConviction, -tickAmt, condition.Caster)
								}
							}
						}

						triggeredConditionIds = append(triggeredConditionIds, condition.ConditionId)

					}

					events.AddToQueue(events.ConditionsTriggered{UserId: user.UserId, ConditionIds: triggeredConditionIds})
				}

				// Stage 7.5: Attempt automatic recovery from prone (contested
				// if someone is holding the character down, free otherwise).
				// AFTER the condition tick, the order MobRoundTick uses: a failed or
				// gated attempt adds the one-round Recovering record (118,
				// attacks_cap 1), and when it ran before the tick, the tick
				// expired the record before DoCombat could read it, so a player
				// never felt the cap (slice 1b, owner ruling 2026-09-14).
				//
				// Guarded on Health/DeathQueued: a lethal bleed/poison tick just
				// above can queue this character's death, and standing it back
				// up (plus the progression award) mid-death is wrong. Matches
				// NewRound_MobRoundTick.go, which skips a dying mob's own
				// recovery the same way.
				if user.Character.Health > 0 && !user.Character.DeathQueued {
					if attemptMade, success := user.Character.AttemptRecovery(recoveryContest(user.Character)); attemptMade {
						if success {
							user.SendText(messaging.CategorySystem, "You scramble to your feet!")
							if room := rooms.LoadRoom(user.Character.RoomId); room != nil {
								sendVisualRoomText(room, messaging.CategoryEmote, "<ansi fg=\"username\">"+user.Character.Name+"</ansi> clambers to their feet in a rushed panic.", user.UserId)
							}
						} else {
							user.SendText(messaging.CategorySystem, "You attempt to stand, but slip back down in the chaos of battle!")
							if room := rooms.LoadRoom(user.Character.RoomId); room != nil {
								sendVisualRoomText(room, messaging.CategoryEmote, "<ansi fg=\"username\">"+user.Character.Name+"</ansi> attempts to stand, but slips and falls in the chaos of battle.", user.UserId)
							}
						}
					}
				}

				// Pinnacle item upkeep (procs are event-driven; this is the always-on layer).
				pinnacleUserTick(user, room)

				// Chrysifier: keep the Homunculus-apex owner supplied with their crafted twin.
				tickHomunculus(user, room)

				// Manifester: strengthen the owner's companions (Symbiotic Bond / bridges).
				tickCompanionEmpowerment(user, room)

				// Manifester: a Brood Mother is never petless.
				tickBroodMotherFloor(user, room)

				// Stage 12.2: Mutation progress — accumulates during combat, triggers acquisition or deepening
				// Stage 17.2: The Eye modulates how quickly mutations happen (0.5× at new moon, 1.5× at full)
				if user.Character.IsInCombat() {
					// Blood Frenzy: enter/refresh the frenzy state while wounded.
					if shouldFrenzy(mutations.HasMutationFlag(user.Character.Mutations, "battle-frenzy"), user.Character.Health, user.Character.HealthMax.Value) {
						user.AddCondition(bloodFrenzyConditionId, "blood-frenzy")
					}
					mb := configs.GetBalanceConfig()
					canAcquire := len(user.Character.Mutations) < int(mb.MutationMaxCount)
					canDeepen := mutations.CanDeepen(user.Character.Mutations)
					if canAcquire || canDeepen {
						eyeMult := 0.5 + gametime.GetEyePhase()
						// Phase 25.3: a mutation-rate condition quickens mutation progress
						// gain. The magnitude now lives on the condition: a condition with no
						// progress_mult is worth 2.0, the historic literal this line
						// used to hardcode. That 2.0 default is a balance number
						// living in Go rather than config.yaml and belongs on the
						// config audit list.
						mutCatalystMult := user.Character.Conditions.ProgressMult(conditions.MutationRate)
						user.Character.MutationProgress += float64(mb.MutationProgressGainPerRound) * eyeMult * mutCatalystMult
						// Phase 24.1: Use rarity-weighted load instead of flat event count
						load := mutations.GetMutationLoad(user.Character.Mutations)
						threshold := float64(mb.MutationBaseProgress) *
							math.Pow(float64(mb.MutationProgressScale), load)
						if user.Character.MutationProgress >= threshold {
							user.Character.MutationProgress = 0
							// Mutation-graph drift fades on each mutation event so recent behavior dominates.
							mutations.DecayAffinity(user.Character.ClusterAffinity, float64(mb.MutationAffinityDecay))
							// Decide: deepen existing mutation or acquire new one
							doDeepen := false
							if canAcquire && canDeepen {
								// Both possible — coin flip weighted toward deepening
								if util.Rand(100) < int(mb.MutationDeepenChance*100) {
									doDeepen = true
								}
							} else if canDeepen && !canAcquire {
								// At max count — must deepen
								doDeepen = true
							}
							// else: canAcquire && !canDeepen — acquire new (doDeepen stays false)

							if doDeepen {
								mutId := mutations.RollDeepening(user.Character.Mutations)
								if mutId != "" {
									user.Character.Mutations[mutId]++
									events.AddToQueue(mutations.Gained{
										UserId:     user.UserId,
										MutationId: mutId,
										Rank:       user.Character.Mutations[mutId],
										IsNew:      false,
									})
								}
							} else if canAcquire {
								// Pass the user's species so body-part requirements gate the pool.
								sp := species.GetSpecies(user.Character.SpeciesId)
								aff := mutations.EffectiveAffinity(user.Character.Mutations, user.Character.ClusterAffinity)
								pool := mutations.GetGraphPool(user.Character.Mutations, aff, sp)
								if len(pool) > 0 {
									mutId := mutations.RollAcquisition(pool)
									if user.Character.Mutations == nil {
										user.Character.Mutations = make(map[string]int)
									}
									user.Character.Mutations[mutId] = 1
									events.AddToQueue(mutations.Gained{
										UserId:     user.UserId,
										MutationId: mutId,
										Rank:       1,
										IsNew:      true,
									})
									spec := mutations.GetMutation(mutId)
									if spec != nil {
										// Emit world event for gossip system
										sig := worldevents.Regional
										if spec.Rarity >= 8 {
											sig = worldevents.Global
										}
										zone := user.Character.Zone
										region := ""
										if zCfg := rooms.GetZoneConfig(zone); zCfg != nil {
											region = zCfg.Region
										}
										worldevents.EmitWorldEvent(worldevents.WorldEvent{
											Type:         worldevents.PlayerMutationMilestone,
											Significance: sig,
											ZoneName:     zone,
											RegionName:   region,
											PlayerName:   user.Character.Name,
											Description: fmt.Sprintf("%s has undergone a mutation: %s.",
												user.Character.Name, spec.Name),
										})
									}
								}
							}
						}
					}
				}

				// Stage 13.1: Crafting/Salvaging tick — advance or complete via Activity machine.
				if user.Character.Activity != nil {
					switch user.Character.Activity.State() {
					case activity.Salvaging:
						// Salvaging tick — advance round via Activity machine.
						sd, complete := user.Character.Activity.AdvanceSalvagingRound()
						if !complete {
							user.SendText(messaging.CategorySystem, fmt.Sprintf(
								`<ansi fg="yellow">You continue salvaging... (%d/%d)</ansi>`,
								sd.RoundsComplete, sd.RoundsTotal))
						} else {
							// Determine salvage type from ItemUuid prefix.
							const corpsePrefix = "corpse:"
							if strings.HasPrefix(sd.ItemUuid, corpsePrefix) {
								mobIdStr := strings.TrimPrefix(sd.ItemUuid, corpsePrefix)
								_ = user.Character.Activity.TransitionToFree(state.TransitionReason{
									Trigger: activity.TriggerSalvageComplete,
									Actor:   user.Character.Activity.Self(),
								})
								resolveCorpseSalvage(user, mobIdStr)
							} else {
								// Parse item ID from UUID stored during TransitionToSalvaging.
								// ItemUuid holds the raw UUID string; item ID is recovered via
								// resolveSalvage's MiscData-free path using SalvagingData.
								_ = user.Character.Activity.TransitionToFree(state.TransitionReason{
									Trigger: activity.TriggerSalvageComplete,
									Actor:   user.Character.Activity.Self(),
								})
								resolveSalvageFromData(user, sd)
							}
						}

					case activity.Crafting:
						// Crafting tick — advance round via Activity machine.
						cd, complete := user.Character.Activity.AdvanceCraftingRound()
						if !complete {
							user.SendText(messaging.CategorySystem, fmt.Sprintf(
								`<ansi fg="yellow">You continue working on %s... (%d/%d)</ansi>`,
								cd.RecipeId, cd.RoundsComplete, cd.RoundsTotal))
						} else {
							recipe := crafting.GetRecipe(cd.RecipeId)
							enchantTargetSlot := cd.TargetSlot
							_ = user.Character.Activity.TransitionToFree(state.TransitionReason{
								Trigger: activity.TriggerCraftComplete,
								Actor:   user.Character.Activity.Self(),
							})
							if recipe != nil {
								sl := user.Character.Skills[recipe.Skill]

								// U10b-1b: craft is an ordinary contest. The
								// crafter scores stat + skill*SkillWeight; the
								// recipe supplies a difficulty built from its
								// SkillMinimum and the DEAREST MATERIAL ACTUALLY
								// BEING SPENT.
								//
								// SelectIngredients (not the recipe's declared
								// tags) is what makes that honest: it names the
								// concrete items ConsumeIngredients will take,
								// in the same order, so the roll and the
								// consumption cannot disagree.
								consumed := crafting.SelectIngredients(
									user.Character.Items, user.Character.ComponentItems, recipe)
								craftScore := crafting.CraftScore(
									float64(user.Character.GetStatValue(crafting.CraftPrimaryStat(recipe))), sl)
								// sight ramp (plan 5b): the crafter needs to see.
								craftScore *= messaging.SightMult(user.Character, room)
								craftDiff := crafting.CraftDifficulty(
									recipe.SkillMinimum, crafting.DearestMaterialTier(consumed))
								won := crafting.RunCraftContest(craftScore, craftDiff).Success

								// U10b-1 Task 16: awarded HERE, above the branch,
								// so a FAILED craft trains at
								// ProgressionFailureFraction instead of nothing.
								// This is the case the whole slice is justified
								// by: burning materials on a botched attempt and
								// learning literally nothing from it.
								//
								// U10b-3: the recipe-difficulty multiplier that
								// used to ride here is gone. skill_minimum now
								// decides what you can DISCOVER and shades which
								// recipe a discovery roll draws, rather than
								// making hard recipes train faster to make. With
								// no bonus left, plain AwardResolved is correct.
								user.Character.AwardResolved(user.UserId, won,
									user.Character.CandidateFor(recipe.Skill))

								if won {
									// The bottle's aging multiplier comes from the bottle ACTUALLY
									// BEING CONSUMED, read off the same selection that priced the
									// craft.
									//
									// 🔴 This was a real bug before U10b-1b, not a tidy-up. The old
									// code scanned Character.Items and THEN ComponentItems for the
									// first bottle with a multiplier, while ConsumeIngredients drew
									// from the component bag FIRST — so the potion could inherit the
									// aging speed of a bottle that was never spent, while the bottle
									// that WAS spent contributed nothing. Four bottles ship with
									// multipliers from 3.0 down to 0.25, so the two could differ by 12x.
									var bottleAgingMult float64
									for _, itm := range consumed {
										if spec := itm.GetSpec(); spec.ComponentTag == "bottle" && spec.BottleAgingMultiplier > 0 {
											bottleAgingMult = spec.BottleAgingMultiplier
											break
										}
									}

									if crafting.IsEnchantingRecipe(recipe) {
										// Enchanting: use the stored slot label to find the target
										targetItem := user.Character.Equipment.GetSlotPointer(enchantTargetSlot)
										if targetItem == nil || targetItem.ItemId < 1 {
											user.SendText(messaging.CategoryWarning, `<ansi fg="red">The item is no longer equipped. The enchanting fails, but your materials are returned.</ansi>`)
										} else if pool, added, breach := enchantApplyWouldBreach(user.Character, targetItem, recipe.EnchantType); breach {
											// U7b: craft.go refuses this before the work starts, but
											// the rounds in between are not free of change: a worn
											// enchantment can tier up mid-craft, and a lapsing condition
											// can shrink the pool the ceiling is measured against.
											// Refusing here still returns the materials, exactly as
											// the "no longer equipped" case above does.
											user.SendText(messaging.CategoryWarning, fmt.Sprintf(
												`<ansi fg="red">%s The enchanting fails, but your materials are returned.</ansi>`,
												user.Character.ReservationRefusal(pool, added)))
										} else {
											user.Character.Items, user.Character.ComponentItems = crafting.ConsumeIngredients(user.Character.Items, user.Character.ComponentItems, recipe)
											eDef := enchantments.GetEnchantment(recipe.EnchantType)
											if eDef != nil {
												targetItem.EnchantType = recipe.EnchantType
												targetItem.EnchantTier = 0
												targetItem.EnchantUses = 0
												targetItem.ReservePool = eDef.ReservePool
												enchantments.ApplyTier(targetItem, eDef, 0)
											}
										}
									} else {
										// Provident Hands may preserve the materials (efficient craft).
										if !user.Character.CraftMaterialsSaved() {
											user.Character.Items, user.Character.ComponentItems = crafting.ConsumeIngredients(user.Character.Items, user.Character.ComponentItems, recipe)
										}
										// Normal crafting: produce output item
										newItem := items.New(recipe.Output.ItemId)
										newItem.CraftedRound = util.GetRoundCount()
										newItem.CraftSkill = user.Character.CraftQualityLevel(user.Character.GetSkillLevel(skills.SkillTag(recipe.Skill))) // Faithwrought quality lift
										if bottleAgingMult > 0 {
											newItem.BottleMultiplier = bottleAgingMult
										}
										// Maker's mark for skilled crafters — see
										// crafting.ShouldStampMakerName for the policy (components
										// stamp regardless of Type; plain Objects don't).
										newSpec := newItem.GetSpec()
										if crafting.ShouldStampMakerName(newItem.CraftSkill, newSpec) {
											newItem.MakerName = user.Character.Name
										}
										user.Character.StoreItem(newItem)
										events.AddToQueue(events.ItemOwnership{UserId: user.UserId, Item: newItem, Gained: true})
									}
									successRoles := recipe.Narrate(crafting.PhaseSuccess, textutil.TokenContext{
										ActorName:      user.Character.GetCharacterName(true),
										ActorPlainName: user.Character.GetCharacterName(false),
									})
									user.SendText(messaging.CategorySystem, fmt.Sprintf(`<ansi fg="green">%s</ansi>`, successRoles.Actor))
									if successRoles.Observer != "" {
										sendVisualRoomText(room, messaging.CategoryEmote, successRoles.Observer, user.UserId)
									}

									// Stage 31.1: Recipe discovery roll
									bal := configs.GetBalanceConfig()
									knownCount := len(user.Character.KnownRecipes)
									craftSkillLevel := user.Character.GetSkillLevel(skills.SkillTag(recipe.Skill))
									discChance := configs.DiscoveryChance(configs.DiscoveryParams{
										Base:       float64(bal.RecipeDiscoveryBaseChance),
										Decay:      float64(bal.RecipeDiscoveryDecayRate),
										Known:      knownCount,
										Perception: user.Character.Stats.Perception.ValueAdj,
										Skill:      craftSkillLevel,
									})
									if util.Rand(100) < int(discChance) {
										eligible := crafting.GetEligibleRecipes(
											user.Character.KnownRecipes,
											user.Character.Skills,
											recipe.Skill)
										if len(eligible) > 0 {
											pick := eligible[configs.WeightedDiscoveryPick(crafting.SkillMinimumsFor(eligible), util.Rand)]
											if user.Character.LearnRecipe(pick) {
												if newRecipe := crafting.GetRecipe(pick); newRecipe != nil {
													user.SendText(messaging.CategorySkillProgress, fmt.Sprintf(
														`<ansi fg="yellow-bold">A new idea takes shape in your mind: %s!</ansi>`, newRecipe.Name))
												}
											}
										}
									}
								} else {
									user.Character.Items, user.Character.ComponentItems = crafting.ConsumeIngredients(user.Character.Items, user.Character.ComponentItems, recipe)
									failureRoles := recipe.Narrate(crafting.PhaseFailure, textutil.TokenContext{
										ActorName:      user.Character.GetCharacterName(true),
										ActorPlainName: user.Character.GetCharacterName(false),
									})
									user.SendText(messaging.CategorySystem, fmt.Sprintf(`<ansi fg="red">%s</ansi>`, failureRoles.Actor))
									if failureRoles.Observer != "" {
										sendVisualRoomText(room, messaging.CategoryEmote, failureRoles.Observer, user.UserId)
									}
								}
							}
						}
					}
				}

				// Stage 31.6: Chrysalis enchantment ticking (combat only)
				if user.Character.IsInCombat() {
					for _, line := range tickChrysalisEnchantments(user.Character, util.Rand) {
						user.SendText(messaging.CategorySkillProgress, line)
					}
				}

				// Recalculate all stats at the end of the round tick
				user.Character.Validate()

				// Town Justice 5.1c Task 9: release player when jail sentence expires.
				releaseIfSentenceServed(user.Character, uId, util.GetRoundCount())

			}

		}

	}

	return events.Continue
}

// resolveSalvageFromData resolves salvage completion using Activity SalvagingData
// as the sole source of truth. Delegates to actions.Salvage for roll, storage,
// messaging, and skill progression.
func resolveSalvageFromData(user *users.UserRecord, sd activity.SalvagingData) {
	actor := &actions.UserActor{
		User: user,
		Room: rooms.LoadRoom(user.Character.RoomId),
	}
	_ = actions.Salvage(actor, actions.SalvageOptions{
		TargetItemUuid: sd.ItemUuid,
		SpoiledPotion:  sd.SpoiledPotion,
	})
}

// resolveCorpseSalvage handles corpse salvage completion when CraftingState
// finishes. Delegates to actions.Salvage for roll, storage, messaging, and
// skill progression. Corpse identity keys are cleared from player MiscData
// here (activity teardown), then passed as filter opts to the action.
func resolveCorpseSalvage(user *users.UserRecord, mobIdStr string) {
	var mobId int
	fmt.Sscanf(mobIdStr, "%d", &mobId)

	// Pull stashed corpse identity (existing logic).
	roundCreatedInt, _ := user.Character.GetMiscData("salvage_corpse_round_created").(int)
	user.Character.SetMiscData("salvage_corpse_round_created", nil)

	actor := &actions.UserActor{
		User: user,
		Room: rooms.LoadRoom(user.Character.RoomId),
	}
	_ = actions.Salvage(actor, actions.SalvageOptions{
		TargetCorpse:             true,
		TargetCorpseMobId:        mobId,
		TargetCorpseRoundCreated: uint64(roundCreatedInt),
	})
}
