package hooks

import (
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
	"github.com/GoMudEngine/GoMud/internal/state/combatphase"
	"github.com/GoMudEngine/GoMud/internal/state/life"
)

// wireLifeCrossMachineCascades registers cascade handlers on
// each character's Life machine. On Alive→Dead, force other
// state machines to terminal states and clear scattered state.
// On Dead→Respawning, reset resources + apply grace condition.
//
// Per-actor concrete effects (loot, teleport, decay, KD
// tracking) live in their own observer files
// (Death_*.go, Respawn_*.go) in chunk-2 Tasks 6-8.
//
// Chunks 3 and 4 will repoint the Activity and Position
// pre-wires (currently raw field manipulation) to proper
// machine queries once those machines land.
func wireLifeCrossMachineCascades(c *characters.Character) {
	c.Life.Inner().AfterTransition("life_cross_machine",
		func(from, to life.State, r state.TransitionReason) {
			switch {
			case from == life.Alive && to == life.Dead:
				// Cross-machine cleanup on death.

				// 1. Combat Phase → Idle
				c.CombatPhase.ForceIdle(state.TransitionReason{
					Trigger: combatphase.TriggerForceIdle,
				})

				// 2. Awareness → Visible (via ForceVisible)
				c.Awareness.ForceVisible(state.TransitionReason{
					Trigger: awareness.TriggerDeath,
				})

				// 3. Activity → Free is now handled by Activity_Cascades.go
				//    (chunk 3). The legacy CastingState/CraftingState pointer
				//    nil-outs that lived here are gone — Activity.TransitionToFree
				//    (fired by the activity_life_dead AfterTransition observer)
				//    clears the per-state data slots inside the machine.
				//
				// Transition-window note: legacy CastingState/CraftingState
				// fields are still being written by skill.cast.go and craft.go
				// until Tasks 6-7 add parallel-writes to the Activity machine.
				// During that window, a player who dies mid-activity will have
				// stale legacy pointer state. Task 11 deletes the legacy fields
				// entirely; Tasks 6-7 keep them in sync via parallel-write.

				// 4. Position → Standing is now handled by the
				//    Position FSM cascade (Position_Cascades.go) which
				//    fires on Life Alive→Dead. The legacy pre-wire that
				//    lived here is removed in chunk-4b R4 now that all
				//    production readers go through the FSM predicates.

				// 5. Conditions → cancel all, permanent ones included (All skips
				//    only already-expired records, never Permanent).
				c.CancelConditionsWithFlag(conditions.All)

				// 5a. End this life's epoch. THIS MUST STAY BESIDE THE CONDITION
				// STRIP ABOVE. The strip clears every condition the character holds,
				// but a condition still QUEUED on events.Condition is not held yet: the
				// killing swing's on-hit condition is queued after its CharacterDied,
				// the queue is FIFO, and Die cascades a player back to Alive
				// with DeathQueued cleared before that condition flushes. Neither
				// IsAlive nor DeathQueued can tell it apart from a live condition by
				// then, so ApplyConditions compares the epoch the event was stamped
				// with instead. A Rending Bleed from the killing blow bled a
				// respawned player to a second death in the Mending Hut
				// (playtest 7d0dad99c4709fc0).
				c.LifeEpoch++

				// 5b. Toxicity → clear. THIS MUST STAY BESIDE THE CONDITION STRIP
				// ABOVE, because that strip is what justifies it: toxicity is
				// the price of a potion's effect, and the line above has just
				// removed every effect the player paid for. Leaving toxicity
				// behind would charge them twice for a benefit they no longer
				// hold -- they have already lost the potions, the materials and
				// the brewing time. Separating these two lines is what
				// re-creates the bug, and it also reopens a death spiral:
				// toxicity above 90% deals acute HP damage and decays at half
				// speed, so a player could respawn at 5% health still critical
				// and die again with no way out.
				c.Toxicity = 0

			case from == life.Dead && to == life.Respawning:
				// Player respawn cascade: resource reset + grace condition.
				// (Mobs don't reach Respawning; their instances
				// get cleaned up by the despawn observer.)

				// 0. Remove every expired-but-held record, SILENTLY. The death
				// strip only expires records, so the next NewTurn prune used
				// to remove them and narrate every end line to wherever the
				// player now stood ("Your wounds stop bleeding." in the
				// Mending Hut, playtest 7d0dad99c4709fc0). This is not limited
				// to the strip's records: one that expired earlier the same
				// turn (cancel-on-combat just before the killing blow) and was
				// not yet pruned is removed silently too. A record that ran out
				// or was cancelled and pruned before the death still narrated.
				//
				// Here and not beside the strip: the strip runs INSIDE the
				// Alive -> Dead observers, and the death announcement's
				// deathCauseFor reads the held Bleeding and Poisoned records
				// by id. Pruning at the strip would hide them from any
				// observer registered after this cascade. By the time Dead ->
				// Respawning fires, every Dead observer has returned.
				//
				// Before the pool resets below, because Validate reconciles
				// stats and clamps pools. ConditionsTriggered is not narration: it
				// is what refreshes the client's conditions panel, as the
				// prune pass does.
				if pruned := c.Conditions.Prune(); len(pruned) > 0 {
					_ = c.Validate()
					if uid := c.GetUserId(); uid != 0 {
						prunedIds := make([]int, 0, len(pruned))
						for _, b := range pruned {
							prunedIds = append(prunedIds, b.ConditionId)
						}
						events.AddToQueue(events.ConditionsTriggered{UserId: uid, ConditionIds: prunedIds})
					}
				}

				// Pools come back at GamePlay.Death.RespawnPoolFraction of max
				// (Validate defaults it to 0.05 when config.yaml omits it).
				frac := float64(configs.GetGamePlayConfig().Death.RespawnPoolFraction)
				c.SetPoolFraction(characters.PoolHealth, frac)
				c.SetPoolFraction(characters.PoolStamina, frac)
				c.SetPoolFraction(characters.PoolConviction, frac)
				_ = c.AddCondition(81, false) // NoAggroTarget grace

				// Clear damage ledger and notify vitals subscribers.
				clear(c.PlayerDamage)
				if uid := c.GetUserId(); uid != 0 {
					events.AddToQueue(events.CharacterVitalsChanged{UserId: uid})
				}
			}
		})
}

func init() {
	characters.OnCharacterCreated(wireLifeCrossMachineCascades)
}
