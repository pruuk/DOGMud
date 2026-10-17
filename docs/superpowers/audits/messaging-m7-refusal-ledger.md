# Messaging M7 slice 3: refusal ledger

Every `messaging.CategorySystem` send in a non-test Go file under `internal/` and
`modules/` (admin files `internal/usercommands/admin.*.go` excluded: slice 4 retags
them by file), ruled one by one. Inventory: `tools/refusal_classify.py --tsv` at master
`1abb49f12` (2026-10-16), 1325 rows. Rulings: five read-only agents read each site in
source; the controller reconciled the counts (1325 rows, keys identical to the
inventory) and amended 22 rows (marked "controller:"), plus rows found during the
build (marked "controller (Task N):"). Line numbers are as of `1abb49f12`; match
by text once a file has been edited. Two more sites outside this inventory (they
sent on `CategorySpellManifestation`) moved to `refusal.CompanionsFull` in Task 4:
`hooks/companion_summon.go:47`, `usercommands/skill.cast.go:193`.

Plan: `docs/superpowers/plans/completed/2026-10-16-messaging-m7-slice3-refusals.md`. Spec:
`docs/superpowers/specs/completed/2026-10-14-messaging-m7-refusals-design.md` section 6.

| Ruling | Rows | Slice 3 does |
|---|---|---|
| FAM | 103 | call the named `internal/refusal` helper |
| RETAG | 352 | change the category to `CategoryRefusal`, words unchanged |
| REWORD | 51 | new words from the plan Wording table, on `CategoryRefusal` |
| FAIL | 14 | `refusal.Failed` (logs the cause, sends on `CategoryError`) |
| NARR | 79 | nothing (slice 4 moves narration out of System) |
| USAGE | 47 | nothing (slice 4: `CategoryUsage`) |
| NOT | 679 | nothing (reports, confirmations, outcomes, help, staff output) |
| total | 1325 | |

Slice 3 converted every FAM, RETAG, REWORD and FAIL row in PR #559; the re-inventory after Task 9 left exactly the 805 NOT, NARR and USAGE rows on `CategorySystem`.

Slice 4 (plan `docs/superpowers/plans/completed/2026-10-16-messaging-m7-slice4-tags.md`, ruling R4)
amended the NOT rows its root guards found to be narration or a notice to someone
other than the acting player (marked "controller (slice 4 N1):" for the recipient
guard `system_recipient_guard_test.go`, "controller (slice 4 N2):" for the subject
guard `system_subject_guard_test.go`); the counts above include them.

Slice 4 converted the Admin, Usage, Refusal, notice and narration rows in PR #562.

Columns: line, ruling, what the player reads today (args as `%s`), the proposal,
the note. NOT rows are listed too, so a later reader can see each was looked at.

## `internal/actions/bauble_admin.go`

NOT 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 38 | NOT | Bauble %s is now %s (%s gold, %s lb): %s / Bauble %s was not regenerated: %s. |  | admin-only output (tellBaubleAdmin, admin bauble command) |

## `internal/actions/buy.go`

NOT 1 RETAG 3

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 340 | RETAG | Visit a merchant to purchase objects or services. |  | named merchant did not resolve; no echo |
| 352 | RETAG | Visit a merchant to purchase objects or services. |  | no merchant in the room |
| 363 | RETAG | You can't make out the goods well enough to deal. |  | ShopSightRefusalText keeps its logic in shop_sight.go; send on CategoryRefusal |
| 389 | NOT | Purchased %s of %s before running short. |  | outcome report of a multi-unit buy |

## `internal/actions/cast.go`

RETAG 5

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 144 | RETAG | You cannot bend another person's mind to your will. |  | charm aimed at a player |
| 159 | RETAG | Fighting is not allowed here. / PVP is disabled. / You need more experience before engaging in PVP. / This is not a PVP area. |  | room.CanPvp returns player-facing refusal copy as an error (rooms.go:3148-3169); not a server fault |
| 219 | RETAG | Fighting is not allowed here. / PVP is disabled. / You need more experience before engaging in PVP. / This is not a PVP area. |  | same CanPvp errors as :159 |
| 450 | RETAG | You can't target a companion with a harmful spell. |  | wording fine |
| 461 | RETAG | You can't target %s with a harmful spell. |  | name is HideNames-gated at the caster's sight; target was resolved by FindByNameSeenBy |

## `internal/actions/consider.go`

NOT 3

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 81 | NOT | (opener: how clearly you read %s) |  | consider report |
| 82 | NOT | (verdict line) |  | consider report |
| 83 | NOT | (pace line) |  | consider report |

## `internal/actions/defuse.go`

FAM 2 NOT 3 RETAG 6

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 66 | RETAG | You aren't advanced enough at skullduggery for that. |  | same literal 6x in this group (defuse:66, steal:256,446,543,872); one wording, so no NEW helper is required, though a skill-rank helper could hold it |
| 82 | FAM | You can't do that while you are under attack! | refusal.WhileUnderAttack(actor, "defuse a trap") | controller: under attack is not in combat; own helper; agent: same gate family as the in-combat check at :72 (which already says this); 3 sites (defuse:82, plant:66, steal:115) |
| 118 | RETAG | You need a disarm kit to attempt this. |  | wording fine |
| 164 | NOT | The trap triggers as you fumble the mechanism! |  | outcome of a failed roll |
| 208 | RETAG | There is no lock there. |  | container matched by name |
| 212 | RETAG | You don't detect any traps on that. |  | container matched by name; wording fine |
| 229 | RETAG | There is no lock there. |  | exit matched by name |
| 233 | RETAG | You don't detect any traps on that. |  | exit matched by name; wording fine |
| 245 | FAM | There is no such exit or container. | refusal.NotHereThing(actor) | an undiscovered hidden container falls through to this line, so it is a not-here thing |
| 257 | NOT | You carefully disarm the trap mechanism. |  | success outcome |
| 272 | NOT | You carefully disarm the trap mechanism. |  | success outcome |

## `internal/actions/drink.go`

FAM 1 NOT 8 RETAG 3

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 180 | FAM | You can't drink while focused on your work. Finish or be interrupted first. | refusal.WhileBusy(actor, "drink") | planned helper; today's text is busyRefusalText inline |
| 187 | RETAG | Your hands are committed to the grapple: you can't reach for that. |  | grapple gate; wording fine |
| 216 | RETAG | You can't drink %s. |  | item is from the player's own backpack; echo allowed |
| 252 | NOT | You drink the %s... |  | outcome |
| 254 | NOT | The potion has gone bad! You retch as the foul liquid burns your throat. |  | outcome |
| 270 | NOT | The foul taste teaches you something about how the ingredients interact... |  | outcome |
| 284 | RETAG | Your body rejects the potion: too much toxicity. |  | refused before the item is consumed (DrinkRefuseToxicity) |
| 316 | NOT | You drink the %s. |  | confirmation |
| 328 | NOT | The potion is freshly brewed. It should do the job. |  | outcome |
| 330 | NOT | The potion has fermented nicely. You feel it working stronger than expected. |  | outcome |
| 332 | NOT | The potion is at its peak. You feel its full potency. |  | outcome |
| 334 | NOT | The potion tastes a bit stale. Its effects are diminished. |  | outcome |

## `internal/actions/forage.go`

FAM 1 NOT 4 RETAG 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 56 | RETAG | There is nothing here worth foraging. Try an outdoor area. |  | wrong biome; wording fine |
| 66 | FAM | You need to wait %s more rounds before you can forage again. | refusal.Cooldown(actor, "forage again") | rule 2 (raw round count); 4 round-count cooldown sites in group (forage:66, search:190, track:206,229) |
| 78 | NOT | You crouch low and begin searching the ground carefully... |  | begin line |
| 138 | NOT | You find nothing of use this time. |  | outcome of the roll (awarded) |
| 147 | NOT | You find something, but it crumbles in your hands. |  | outcome |
| 163 | NOT | You find a %s. |  | outcome |

## `internal/actions/melee_target.go`

FAM 1 RETAG 5

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 164 | FAM | You can't %s while focused on your work. Finish or be interrupted first. | refusal.WhileBusy(user, v) | v is opts.CraftingVerb, else opts.Verb (craftingMsg) |
| 189 | RETAG | You can't %s yourself. |  | selfTargetMsg (pounce overrides with "on") |
| 204 | RETAG | You can't %s a companion. |  | charmedMsg |
| 207 | RETAG | You can't attack %s. |  | name HideNames-gated at the attacker's sight |
| 243 | RETAG | Fighting is not allowed here. / PVP is disabled. / You need more experience before engaging in PVP. / This is not a PVP area. |  | CanPvp refusal copy |
| 252 | RETAG | %s is in your party! |  | HideNames-gated; party members are known to the player |

## `internal/actions/move.go`

NARR 5 NOT 4

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 257 | NARR | (spotted on arrival: the spotter, named at the mover's sight, notices you) | CategoryMobEmote (CategoryEmote when the spotter is a player) | the mover is told another creature caught them |
| 351 | NARR | %s slips into the room but you notice them. | CategoryEmote (CategoryMobEmote when the mover is a mob) | told to an observer about the mover |
| 405 | NARR | %s enters the room and notices you! | CategoryEmote (CategoryMobEmote when the mover is a mob) | told to a hidden player about the newcomer |
| 408 | NARR | Someone enters the room and notices you! | CategoryEmote (CategoryMobEmote when the mover is a mob) | as :405 |
| 412 | NOT | You notice %s lurking in the shadows. |  | outcome of the mover's own detection roll |
| 416 | NOT | You notice someone lurking in the shadows. |  | outcome of the mover's own detection roll |
| 442 | NOT | You notice %s lurking in the shadows! |  | outcome of the mover's own detection roll |
| 446 | NOT | You notice something lurking in the shadows! |  | outcome of the mover's own detection roll |
| 452 | NARR | %s spots %s hiding in the shadows! | CategoryMobEmote | room broadcast through SendTextVisual |

## `internal/actions/mutation_helpers.go`

FAM 1 RETAG 3

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 73 | FAM | You can't use that ability while focused on your work. Finish or be interrupted first. | refusal.WhileBusy(actor, "use that ability") | planned helper; text is busyRefusalText inline |
| 82 | RETAG | You don't have that ability. |  | mutation ownership gate; wording fine |
| 90 | RETAG | You must be in combat to use %s! |  | %s is the mutation key the player typed (venom-coat, cocoon) |
| 114 | RETAG | You're too exhausted! |  | stamina refusal; refusal.Cost(actor, result) would fit only if mutationPreamble moved from ApplyCost (bool) to the quote/commit path (admitFullCost) |

## `internal/actions/plant.go`

FAM 3 NARR 4 NOT 2

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 66 | FAM | You can't do that while you are under attack! | refusal.WhileUnderAttack(actor, "plant anything") | controller: as defuse.go:82; agent: matches the in-combat gate at :55 |
| 124 | FAM | They seem to have vanished. | refusal.NotHereCreature(actor) | target gone between resolve and act; 6 sites (plant:124,270, shadow:104,150, steal:221,440) |
| 176 | NOT | You deftly slip the %s into %s's belongings unnoticed. |  | success outcome |
| 189 | NARR | %s catches you in the act! | CategoryMobEmote | controller (slice 4 N2): a mob catches the planter; the room line rides MobEmote; agent: failure outcome |
| 270 | FAM | They seem to have vanished. | refusal.NotHereCreature(actor) | as :124 |
| 284 | NARR | %s catches you in the act! | CategoryEmote | controller (slice 4 N2): a player catches the planter (player acts on player); agent: failure outcome |
| 333 | NARR | %s slips something into your belongings! | CategoryMobEmote | told to the victim about a mob's plant |
| 434 | NARR | %s spots you slipping something into the %s! | CategoryMobEmote | controller (slice 4 N2): a mob spotter; the room line rides MobEmote; agent: failure outcome |
| 476 | NOT | You quietly slip your %s into the %s. |  | success outcome |

## `internal/actions/relocate_mob.go`

NOT 2

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 40 | NOT | You turn your attention to %s! |  | controller (slice 4 review): read again; each player in the room reads only their OWN automatic retarget, the same notice `hooks/NewRound_DoCombat.go:155` and `hooks/NewRound_DoCombat_unified.go:1039` send on System, so it stays on the category its event's other lines use (System); retarget notice (RetargetNotice), HideNames-gated |
| 50 | NOT | You turn your attention to %s! |  | controller (slice 4 review): as line 40 (the companion-defence branch of the same retarget); retarget notice |

## `internal/actions/scan.go`

NOT 8

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 114 | NOT | You scan the surrounding area... |  | scan report header |
| 115 | NOT | (blank line) |  | report spacing |
| 117 | NOT | There are no visible exits to scan. |  | a line inside the scan report after its header, not a refusal |
| 146 | NOT | %s: the exit is locked |  | scan report row |
| 175 | NOT | %s: too dark to make anything out |  | scan report row |
| 178 | NOT | %s: %s |  | scan report row |
| 181 | NOT | %s: nothing of interest |  | scan report row |
| 185 | NOT | (blank line) |  | report spacing |

## `internal/actions/search.go`

FAM 1 NARR 2 NOT 9

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 84 | NOT | (room roster of the hiders found) |  | search report |
| 190 | FAM | You need to wait %s more rounds to do that again. | refusal.Cooldown(actor, "search again") | rule 2 (raw round count) |
| 204 | NOT | You search the %s and snoop around for a bit... |  | begin line |
| 212 | NOT | You snoop around for a bit... |  | begin line |
| 256 | NOT | You found a secret exit: %s |  | find |
| 275 | NOT | You discover a hidden %s! |  | find |
| 300 | NOT | (ground list of stashed items found) |  | find |
| 392 | NOT | You discover something: %s |  | find |
| 394 | NOT | (the hidden noun's description) |  | find |
| 456 | NOT | You find nothing of interest. |  | outcome (deliberately identical for empty and missed) |
| 498 | NARR | %s searches the room and spots you! | messaging.EmoteCategoryFor(searcher kind) | controller (slice 4 review): Emote for a player searcher, MobEmote for a mob one (revealSpotted calls it only for a player today; slice F opens the mob path), name capitalised at the source; told to the hider about a player's search |
| 501 | NARR | Someone searches the room and spots you! | messaging.EmoteCategoryFor(searcher kind) | as :498 |

## `internal/actions/search_bauble.go`

NOT 2

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 107 | NOT | (bauble find delivery lines) |  | find delivery |
| 530 | NOT | Something glints among the clutter. You set about working it loose... |  | find |

## `internal/actions/search_feature.go`

NOT 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 181 | NOT | Something glints about the %s. You set about working it loose... |  | find |

## `internal/actions/sell.go`

RETAG 3

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 76 | RETAG | You can't make out the goods well enough to deal. |  | ShopSightRefusalText |
| 187 | RETAG | Quest items cannot be sold! |  | wording fine |
| 311 | RETAG | Quest items cannot be sold! |  | wording fine |

## `internal/actions/shadow.go`

FAM 2 NARR 1 NOT 3 RETAG 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 62 | RETAG | You must be hidden to shadow someone. Try sneak first. |  | already carries a command hint |
| 104 | FAM | They seem to have vanished. | refusal.NotHereCreature(actor) | as plant:124 |
| 113 | NOT | You begin shadowing %s, moving silently in their wake. |  | confirmation |
| 150 | FAM | They seem to have vanished. | refusal.NotHereCreature(actor) | as plant:124 |
| 159 | NOT | You begin shadowing %s, watching their every move. |  | confirmation |
| 213 | NARR | You sense someone following close behind you. | CategoryEmote | told to the shadowed target about the shadower (shadowSensedLine) |
| 256 | NOT | (EndShadow reason) You stop shadowing your target. / shadowSpottedLine |  | confirmation or outcome passed in by the callers |

## `internal/actions/sleep.go`

FAIL 1 FAM 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 46 | FAM | You can't sleep right now. | refusal.WhileFighting(actor, "sleep") | in-combat gate |
| 65 | FAIL | You can't seem to settle into sleep right now. | refusal.Failed(actor, "sleep", err, "userId", actor.GetUserId()) | AddCondition failure is a server fault; it is already logged and the line hides the error, so this is for consistency (Failed replaces the mudlog call); RETAG if the owner prefers the flavor line |

## `internal/actions/sleeping_target.go`

FAM 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 60 | FAM | %s is fast asleep. You could make some noise. Try shout wake up. | refusal.TargetAsleep(user, name) | planned helper; keep the HideNames gating at the player's sight inside it |

## `internal/actions/sneak.go`

NARR 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 223 | NARR | %s tries to hide but you notice them. | CategoryEmote (CategoryMobEmote when the sneaker is a mob) | told to an observer about the sneaker |

## `internal/actions/steal.go`

FAM 3 NARR 5 NOT 1 RETAG 6 REWORD 4

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 115 | FAM | You can't do that while you are under attack! | refusal.WhileUnderAttack(actor, "steal") | controller: as defuse.go:82; agent: matches the in-combat gate at :104 |
| 131 | RETAG | Your hand is still in someone's pocket. |  | busy gate; WhileBusy's "focused on your work" does not fit a pickpocket pause |
| 221 | FAM | They seem to have vanished. | refusal.NotHereCreature(actor) | as plant:124 |
| 231 | REWORD | %s is someone's companion. You can't steal from them. | same text, name through messaging.HideNames at the thief's sight (melee_target.go:207 pattern) | controller: keep the name for a thief who sees it; agent: rule 1: AimBySight admits `steal shape` at shapes and this line echoes the real mob name unhidden; alternative is messaging.HideNames as melee_target.go:207 does |
| 242 | REWORD | You can't steal from %s. | as :231 | controller: as :231; agent: rule 1, as :231 |
| 256 | RETAG | You aren't advanced enough at skullduggery for that. |  | see defuse:66 |
| 383 | NOT | You deftly rifle through %s's belongings but find nothing worth taking. |  | outcome after the contest |
| 406 | NARR | %s catches you in the act! | CategoryMobEmote | controller (slice 4 N2): caughtByMob; the room line rides MobEmote; agent: failure outcome |
| 440 | FAM | They seem to have vanished. | refusal.NotHereCreature(actor) | as plant:124 |
| 446 | RETAG | You aren't advanced enough at skullduggery for that. |  | see defuse:66 |
| 463 | NARR | %s catches you in the act! | CategoryEmote | controller (slice 4 N2): a player catches the thief (player acts on player); agent: failure outcome |
| 517 | NARR | %s lifts %s gold from your pocket! | CategoryMobEmote | told to the victim about a mob's theft |
| 543 | RETAG | You aren't advanced enough at skullduggery for that. |  | see defuse:66 |
| 548 | RETAG | You root around inside the %s but find it empty. |  | refused before the contest and the cooldown; the container was matched by name |
| 576 | NARR | %s spots you reaching into the %s! | CategoryMobEmote | controller (slice 4 N2): a mob spotter; the room line rides MobEmote; agent: failure outcome |
| 872 | RETAG | You aren't advanced enough at skullduggery for that. |  | see defuse:66 |
| 877 | REWORD | You can't carry the %s - you're already overloaded! | `You can't carry that. You're already overloaded!` | rule 1: householdBaubleNamed (usercommands/skill.skullduggery.steal.go:135) matches floor items with no sight check, so a player in the dark who types part of a name reads the full bauble name before any roll |
| 897 | NARR | %s spots you reaching for the %s! | CategoryMobEmote | controller (slice 4 N2): a mob spotter; the room line rides MobEmote; agent: failure outcome |
| 917 | REWORD | You can't carry the %s - you're already overloaded! | `You can't carry that. You're already overloaded!` | rule 1, kept identical to :877 (post-contest storage failure; the thief may still be in the dark) |

## `internal/actions/steal_pocket.go`

NARR 1 NOT 2

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 149 | NOT | You attempt to pick %s's pocket... |  | begin line |
| 310 | NOT | You lose your chance at %s's pocket. |  | outcome |
| 398 | NARR | %s felt your hand in their pocket. A cry of "Thief!" rings out. | CategoryMobEmote | controller (slice 4 N2): the mark's room line rides MobEmote; agent: outcome of the thief's failed roll |

## `internal/actions/stolen_bauble.go`

NARR 2

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 245 | NARR | %s stares at the %s you are carrying. "That's mine! Thief!" | CategoryMobEmote | a mob's reaction told to the carrier |
| 320 | NARR | %s turns the %s over in disbelief. "My %s! I never thought to see it again." | CategoryMobEmote | a mob's reaction told to the giver |

## `internal/actions/track.go`

FAM 2 NOT 9

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 96 | NOT | You stop tracking. |  | confirmation |
| 125 | NOT | You read %s's sign in the scuffed ground. It is close, here in the open with you. |  | positive read |
| 194 | NOT | You don't see any tracks. |  | lost roll: a resolved, awarded attempt, not a refusal |
| 206 | FAM | You need to wait %s more rounds to use that skill again. | refusal.Cooldown(actor, "track again") | rule 2 (raw round count) |
| 229 | FAM | You need to wait %s more rounds to use that skill again. | refusal.Cooldown(actor, "track again") | rule 2 (raw round count) |
| 257 | NOT | %s is in the room with you! |  | positive read, awarded; tracker sees faces and perceives the target |
| 267 | NOT | %s is in the room with you! |  | positive read, awarded |
| 296 | NOT | You cast about for the trail, but lose it. |  | lost the opposed contest |
| 323 | NOT | You don't see any tracks. |  | resolved attempt that found nothing (awarded) |
| 489 | NOT | You don't see any tracks. |  | empty trail report |
| 494 | NOT | (descriptions/track template) |  | trail report |

## `internal/behaviortree/actions_item_voice.go`

NARR 2

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 227 | NARR | %s says, "%s" | CategoryNPCDialogue | an item speaking to its holder |
| 237 | NARR | %s's %s mutters, "%s" | CategoryNPCDialogue | room broadcast of an item speaking |

## `internal/bountyhunter/bountyhunter.go`

NARR 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 135 | NARR | Word reaches you that a hunter has taken the contract on your head. | CategoryWarning | world event notice |

## `internal/combat/combat.go`

NOT 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 380 | NOT | Exhaustion keeps your training from coming cleanly through. |  | shortage outcome; the round still resolves |

## `internal/combat/combat_helpers.go`

NOT 2

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 200 | NOT | (comparison, not a send) |  | must change in step with :204 (dedupe check reads the category) |
| 204 | NOT | You mount a desperate response, too spent to bring practiced technique to it. |  | shortage outcome of a defence that happened |

## `internal/hooks/Achievements_Poll.go`

NOT 3

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 63 | NOT | (You just earned your first achievement, a milestone you unlock simply by playing. Type achievements anytime to see the ones you've earned and what else you can chase.) |  | notice |
| 68 | NOT | *** Achievement unlocked: %s *** |  | notice |
| 70 | NOT | %s  (+%s points) |  | notice; achievement points are a report value |

## `internal/hooks/CombatPhase_FleeCancellation.go`

NOT 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 31 | NOT | The fight ends before you need to flee. |  | outcome of an admitted flee |

## `internal/hooks/Death_PlayerAnnouncement.go`

NOT 2

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 57 | NOT | You have been expelled from the instance. There is no return. |  | state notice after a subdue |
| 144 | NOT | You have been expelled from the instance. There is no return. |  | state notice after death |

## `internal/hooks/NewRound_DoCombat.go`

NOT 2

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 95 | NOT | You shift your focus to %s! |  | retarget notice |
| 155 | NOT | You turn your attention to %s! |  | retarget notice |

## `internal/hooks/NewRound_DoCombat_helpers.go`

NARR 2 RETAG 2

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 988 | RETAG | You can't flee while grappled! |  | controller: a queued flee refused at round time is a refusal; agent: round-time outcome of an admitted flee; the command-time refusal is usercommands.fleeRefusalText |
| 999 | NARR | %s blocks you from fleeing! | CategoryMobEmote or CategoryEmote | controller (slice 4 N2): blockCat, the room line's category, by the blocker's kind; agent: round-time flee outcome |
| 1010 | NARR | %s is blocked from fleeing by %s! | CategoryMobEmote | room broadcast (SendTextVisualHidingNames) |
| 1013 | RETAG | You can't find an exit! |  | controller: as :988; agent: round-time flee outcome |

## `internal/hooks/NewRound_DoCombat_unified.go`

NOT 3 RETAG 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 88 | RETAG | You can't seem to find your target. |  | controller: a queued attack refused at round time; agent: per-round combat notice (hidden defender), not a command refusal |
| 212 | NOT | Your target can't be found. |  | per-round disengage notice |
| 228 | NOT | Your target can't be found. |  | per-round disengage notice |
| 1039 | NOT | You turn your attention to %s! |  | retarget notice |

## `internal/hooks/NewRound_UserRoundTick.go`

NOT 2

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 391 | NOT | You scramble to your feet! |  | outcome of the recovery roll |
| 396 | NOT | You attempt to stand, but slip back down in the chaos of battle! |  | outcome of the recovery roll |

## `internal/hooks/PlayerSpawn_HandleJoin.go`

NOT 2 RETAG 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 296 | RETAG | You must change your password before doing anything else. |  | controller: refuses every command until the password changes; agent: login notice, not a refusal of a command |
| 297 | NOT | Type password to set a new password. |  | login notice |
| 347 | NOT | [%s] %s |  | guild MOTD |

## `internal/hooks/Quest_HandleQuestUpdate.go`

NOT 5 RETAG 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 208 | RETAG | You have put in the work recently. Rest a while before taking this on again. |  | repeatable-quest cooldown refusal; no number shown |
| 249 | NOT | (questup template) You have been given a new quest: %s! |  | banner |
| 258 | NOT | (questup template) You have completed the quest: %s! |  | banner |
| 281 | NOT | (quest reward actor line) |  | reward narration |
| 421 | NOT | You are suddenly moved to a new place! |  | reward outcome |
| 447 | NOT | (questup template) You've made progress on the quest: %s! |  | banner |

## `internal/hooks/charm_spell.go`

FAM 2 RETAG 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 74 | FAM | You are already sustaining as many companions as your will can hold. | refusal.CompanionsFull(user) | NEW:CompanionsFull "You are already sustaining as many companions as your will can hold." (3 sites, 2 wordings: charm_spell:74,121, companion_summon:130) |
| 79 | RETAG | (Character.ReservationRefusal for conviction) |  | type-owned helper keeps its logic; caller sends on CategoryRefusal |
| 121 | FAM | You cannot maintain any more companions. | refusal.CompanionsFull(user) | NEW:CompanionsFull, as :74 |

## `internal/hooks/companion_follow.go`

NOT 2

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 111 | NOT | Your %s rejoins you. |  | companion notice |
| 223 | NOT | Your %s is swept away! |  | companion notice |

## `internal/hooks/companion_reserve_backfill.go`

NOT 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 85 | NOT | Your bonds were re-priced while you were away. ... you cannot take on more until you make room. |  | login disclosure |

## `internal/hooks/companion_summon.go`

FAM 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 130 | FAM | You cannot maintain any more companions. | refusal.CompanionsFull(user) | NEW:CompanionsFull, as charm_spell:74 |

## `internal/hooks/pinnacle_tick.go`

NOT 3

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 188 | NOT | The blade feeds on you, a cold pull beneath your grip. |  | item upkeep notice |
| 326 | NOT | The %s stirs, drawing in the essence of what you have slotted; it will take a moment to attune. |  | item notice |
| 358 | NOT | The %s settles into resonance. Its stored virtues suffuse you. |  | item notice |

## `internal/hooks/spell_resolution.go`

FAM 1 NOT 2

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 441 | NOT | You mount a desperate response, too spent to bring practiced technique to it. |  | defence shortage outcome |
| 551 | NOT | You consume a %s as a spell component. |  | outcome |
| 863 | FAM | You can't seem to identify that. | refusal.NotCarrying(user, itemName) | identify looked in backpack and equipment (FindItem); the item is the player's own |

## `internal/justice/arrest.go`

NOT 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 412 | NOT | A guard seizes you and hauls you to the holding cell. You have been placed under arrest by the %s. Type fine to see what you owe. |  | arrest notice |

## `internal/mobcommands/shoot.go`

NOT 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 89 | NOT | You mount a desperate response, too spent to bring practiced technique to it. |  | defence shortage outcome |

## `internal/mobcommands/skill_move_defence.go`

NOT 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 63 | NOT | You mount a desperate response, too spent to bring practiced technique to it. |  | defence shortage outcome |

## `internal/mobcommands/taunt.go`

NOT 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 211 | NOT | You mount a desperate response, too spent to bring practiced technique to it. |  | defence shortage outcome |

## `internal/questengine/bridge.go`

NOT 2 RETAG 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 124 | NOT | (questup template) You have been given a new quest: %s! / You've made progress on the quest: %s! |  | banner |
| 168 | RETAG | You are carrying too much to take the %s. Put something down, then try again. |  | the quest is giving the item, so its name is known |
| 290 | NOT | You have learned the %s spell. |  | confirmation; note it prints the spell id, not its name (Quest_HandleQuestUpdate.go:413 uses spellData.Name) |

## `internal/rooms/roommanager.go`

NARR 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 293 | NARR | The rune you had enscribed here has faded away. | CategoryRoomDescription | world event; the sibling "A sign crumbles to dust." at :289 already rides CategoryRoomDescription; "enscribed" is misspelled |

## `modules/aicompanion/commands.go`

NARR 1 NOT 17 RETAG 1 USAGE 8

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 54 | USAGE | Usage: aicompanion status / profiles / grant / revoke / pause / resume / mind / trace / prompt / models / breaker reset |  | admin-only command |
| 57 | NOT | (grant result) |  | admin output |
| 60 | USAGE | (aiCompanionUsage block) |  | admin-only command |
| 63 | NOT | (revoke result) |  | admin output |
| 66 | USAGE | (aiCompanionUsage block) |  | admin-only command |
| 69 | NOT | (mind report) |  | admin output |
| 72 | USAGE | (aiCompanionUsage block) |  | admin-only command |
| 75 | NOT | (trace or prompt report) |  | admin output |
| 81 | NOT | Circuit breaker reset; model calls resume. |  | admin output |
| 84 | USAGE | (aiCompanionUsage block) |  | admin-only command |
| 87 | NOT | (pause or resume result) |  | admin output |
| 89 | USAGE | (aiCompanionUsage block) |  | admin-only command |
| 174 | NOT | (status report) |  | admin output |
| 179 | NOT | No companion profiles are loaded. |  | admin report |
| 192 | NOT | (profiles list) |  | admin output |
| 436 | NOT | (models report) |  | admin output |
| 493 | RETAG | You reset your companion only a moment ago. Give it a minute. |  | cooldown refusal with no raw number |
| 506 | NOT | Your companion has been reset. Their memories are untouched. |  | confirmation |
| 524 | NOT | %s has gone their own way. |  | confirmation |
| 531 | NARR | %s looks at you and waits. Type companion-part again within a minute to part ways for good; ... | CategoryMobEmote | controller (slice 4 N2): the companion (a mob) acts on the owner; the guard cannot see this site (msg is a call result), converted by hand; agent: confirmation prompt (confirmPart) |
| 549 | NOT | (courtStep result) |  | mixed: in-fiction answers ("%s is not looking for that, and says so kindly.", "%s hears you, and it is plain that %s.") and confirmations; split it if those answers should be refusals |
| 564 | NOT | (setBoundary result) |  | confirmations; its default branch is a usage-style hint |
| 627 | USAGE | Usage: companion-ask <who> about <what> |  | usage line |
| 688 | NOT | %s will %s |  | confirmation |
| 691 | NOT | %s will %s (companion-stay close\|normal\|free) |  | status |
| 694 | USAGE | Usage: companion-stay close\|normal\|free |  | usage line (unknown argument) |
| 732 | NOT | (companion-ai on/off/status lines) |  | confirmations and status |

## `modules/aicompanion/meeting.go`

NOT 5

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 192 | NOT | (%s is driven by OpenAI. ... the question itself is only asked this once.) |  | consent question |
| 216 | NOT | Your companion has gone their own way. |  | notice |
| 249 | NOT | (If you truly want %s to leave for good, type companion-part within %s minutes. Otherwise just carry on; they will stay.) |  | notice; shows a minute count (not a refusal, outside this sweep) |
| 394 | NOT | (Agreed. %s will answer in their own words from here on. ...) |  | confirmation |
| 404 | NOT | (Understood. %s stays with you as a plain companion, ...) |  | confirmation |

## `modules/aicompanion/romance.go`

NOT 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 379 | NOT | (If you feel the same, companion-court says so. companion-boundary friendship keeps things as they are.) |  | notice |

## `modules/aicompanion/tiers.go`

NARR 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 362 | NARR | (%s falls back on a few set words: your key's provider did not answer.) | CategoryWarning | controller (slice 4 N1): the owner is told when a companion call settles, not as the result of a command they typed (the recipient is looked up from ownerId); a notice like R3's, so CategoryWarning, which also never wraps; agent: notice (tellOwner) |

## `modules/auctions/auctions.go`

NOT 11 RETAG 9 REWORD 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 237 | REWORD | Auctions are disabled. See help set for learn how to change this. | `Auctions are disabled. See <ansi fg="command">help set</ansi> to learn how to change this.` | rule 4: "for learn how" is broken English |
| 252 | NOT | (auction-update template) |  | status report |
| 254 | NOT | No current auctions. You can auction something, though! |  | status report for a bare `auction` |
| 295 | NOT | (Past Auctions table) |  | report |
| 303 | RETAG | There is not an auction to bid on. |  | wording fine |
| 308 | RETAG | You cannot bid on your own auction. |  | wording fine |
| 313 | RETAG | You are already the highest bidder. |  | wording fine |
| 329 | RETAG | You must bid at least %s gold. |  | gold the player must pay is allowed |
| 345 | RETAG | There is not an auction to bid on. / You are already the highest bidder. / The minimum bid is %s gold / You only have %s gold in the bank. / User not found. |  | Bid returns refusal copy as errors (auctions.go:1051-1078); its "User not found." branch is a server fault that belongs on refusal.Failed |
| 381 | RETAG | There is already an auction in progress. |  | wording fine |
| 396 | RETAG | The %s is too trivial to interest the auction house. |  | item from the player's backpack |
| 404 | RETAG | The auction house will not list the %s: it was reported stolen, and not long ago. |  | item from the player's backpack |
| 416 | NOT | Aborting auction |  | confirms the player's No |
| 428 | RETAG | Aborting auction |  | refuses a buyout below 1 or not a number |
| 435 | NOT | Auctioning your %s: buyout %s gold, reserve %s gold. |  | confirmation |
| 533 | NOT | You have won the auction for the %s! It has been added to your backpack. |  | notice |
| 570 | NOT | Your auction of the %s has ended. The highest bid was made by %s for %s gold (after the auction house's cut). |  | notice |
| 916 | NOT | Your seized %s found no buyer and was disposed. |  | notice |
| 959 | NOT | You have won the auction for %s! It has been added to your backpack. |  | notice |
| 991 | NOT | Your seized %s sold at auction. After the debt and the house's cut, %s gold was returned to your account. |  | notice |
| 1185 | NOT | Your auction ended with no bids; the item was returned to your bank storage. |  | notice |

## `modules/cleanup/cleanup.go`

NOT 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 85 | NOT | You trash the %s for good. |  | confirmation |

## `modules/follow/follow.go`

NARR 8 NOT 7 RETAG 3

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 211 | NARR | %s stopped following you. | CategoryEmote | told to the followed player about the follower (follow expiry) |
| 214 | NOT | You are no longer following %s. |  | the follower's own follow ended |
| 222 | NARR | %s stopped following you. | CategoryMobEmote | told to the followed player about a mob |
| 236 | NOT | You are no longer following %s. |  | the follower's own follow ended |
| 400 | RETAG | You can't use this command while in a party. |  | wording fine |
| 451 | RETAG | You can't target yourself. |  | wording fine |
| 468 | NARR | You are no longer following %s. | CategoryEmote | told to followers because the leader used `follow lose` |
| 476 | NOT | Nobody is following you. |  | confirmation of `follow lose` (sent whether or not anyone was lost) |
| 494 | NARR | %s stopped following you. | CategoryEmote | told to the followed player |
| 497 | NOT | You are no longer following %s. |  | confirmation of `follow stop` |
| 508 | NOT | You are no longer following %s. |  | confirmation of `follow stop` |
| 516 | RETAG | You aren't following anyone. |  | follow stop with nothing to stop |
| 531 | NOT | You start following %s. |  | confirmation |
| 535 | NARR | %s is following you. | CategoryEmote | told to the target about the follower |
| 548 | NOT | You start following %s. |  | confirmation |
| 615 | NARR | You are no longer following %s. | CategoryMobEmote | told to followers because a mob used `follow lose` |
| 637 | NARR | %s stopped following you. | CategoryMobEmote | told to the followed player about a mob |
| 659 | NARR | %s is following you. | CategoryMobEmote | told to the target about a mob; note the mob name is not HideNames-gated here, unlike :637 |

## `modules/gmcp/gmcp.Mudlet.go`

NOT 18 RETAG 2 USAGE 10

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 571 | NOT | (toggle enabled message) |  | toggle state |
| 573 | NOT | (toggle disabled message) |  | toggle state |
| 587 | RETAG | This command is only available for Mudlet clients. You are currently using: %s |  | note: on a cache miss (!ok) gmcpData is not checked before .Client.Name is read |
| 605 | NOT | %s Mudlet UI Management |  | help/status block |
| 606 | NOT | Status: |  | help/status block |
| 607 | NOT | Login message display: %s |  | help/status block |
| 608 | NOT | Available Commands: |  | help/status block |
| 609 | NOT | mudletui install - Install the Mudlet UI package |  | help/status block |
| 610 | NOT | mudletui remove - Remove the Mudlet UI package |  | help/status block |
| 611 | NOT | mudletui update - Manually check for updates to the Mudlet UI package |  | help/status block |
| 612 | NOT | mudletui hide - Hide login messages |  | help/status block |
| 613 | NOT | mudletui show - Enable login messages |  | help/status block |
| 614 | NOT | For more information, type help mudletui |  | help/status block |
| 623 | NOT | UI installation package sent to your Mudlet client. If it doesn't install automatically, you may need to accept the installation prompt in Mudlet. |  | confirmation |
| 627 | NOT | UI removal command sent to your Mudlet client. |  | confirmation |
| 631 | NOT | Manual UI update check sent to your Mudlet client. |  | confirmation |
| 637 | NOT | You can use mudletui show in the future if you want to see the prompts again. |  | confirmation hint |
| 643 | NOT | You can use mudletui hide in the future if you want to hide the prompts again. |  | confirmation hint |
| 646 | USAGE | Usage: mudletui install\|remove\|update\|hide\|show / Type 'help mudletui' for more information. |  | usage block |
| 679 | NOT | We have detected you are using Mudlet as a client. |  | login notice |
| 690 | RETAG | This command is only available for Mudlet clients. You are currently using: %s |  | same cache-miss note as :587 |
| 697 | USAGE | Usage: discord area on\|off\|party on\|off\|name on\|off\|level on\|off\|info on\|off\|status on\|off |  | usage line |
| 710 | USAGE | Usage: discord area on\|off |  | usage line |
| 719 | USAGE | Usage: discord party on\|off |  | usage line |
| 728 | USAGE | Usage: discord name on\|off |  | usage line |
| 737 | USAGE | Usage: discord level on\|off |  | usage line |
| 755 | USAGE | Usage: discord info on\|off |  | usage line |
| 766 | USAGE | Usage: discord status on\|off |  | usage line |
| 770 | USAGE | Usage: discord area on\|off\|party on\|off\|name on\|off\|level on\|off\|info on\|off\|status on\|off |  | usage line |
| 773 | USAGE | Usage: discord area on\|off\|party on\|off\|name on\|off\|level on\|off\|info on\|off\|status on\|off |  | usage line |

## `modules/leaderboards/leaderboards.go`

NOT 2

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 162 | NOT | (blank line) |  | report spacing |
| 163 | NOT | (leaderboard table) |  | report |

## `modules/playtest/commands.go`

NOT 7

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 22 | NOT | Usage: ai-flag <username> [on\|off] |  | admin dev tool, ruled NOT per the group brief; slice 4's Usage/Admin tag guard decides its category |
| 32 | NOT | No such account: %s |  | admin dev tool |
| 37 | NOT | Failed to save: %s |  | admin dev tool; error text goes to an admin |
| 40 | NOT | %s IsAI = %s |  | admin dev tool |
| 59 | NOT | No AI-flagged accounts. |  | admin dev tool |
| 62 | NOT | AI-flagged accounts: |  | admin dev tool |
| 64 | NOT | %s (online) / %s (offline) |  | admin dev tool |

## `modules/weather/weather.go`

NOT 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 147 | NOT | (sendLine: weather report, admin rebuild result, admin usage) |  | shared sender for every weather line; slice 4 needs a category per caller (weather_commands.go:56-104, adminUsage passes through it) |

## `internal/usercommands/achievements.go`

NOT 6

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 41 | NOT | There are no accolades to earn yet. |  | empty-list report: the achievements command has no definitions loaded; nothing the player asked for is refused |
| 53 | NOT | <ansi fg="yellow-bold">Achievements</ansi>: earned <ansi fg="white-bold">%s/%s</ansi>, <ansi fg="yellow">%s points</ansi> |  | report header |
| 73 | NOT | (blank line) |  | report spacer |
| 74 | NOT | <ansi fg="cyan-bold">%s%s</ansi> |  | report category heading |
| 79 | NOT |   <ansi fg="green">[x] %s</ansi> <ansi fg="yellow">(%s)</ansi>: %s |  | report row |
| 82 | NOT |   <ansi fg="black-bold">[ ] %s (%s): %s</ansi>  <ansi fg="white">(%s)</ansi> |  | report row |

## `internal/usercommands/afk.go`

NOT 3

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 28 | NOT | You are no longer AFK. |  | toggle state confirmation |
| 46 | NOT | You are now AFK: "%s" |  | toggle state confirmation |
| 51 | NOT | You are now AFK. Type <ansi fg="command">afk</ansi> again to return. |  | toggle state confirmation |

## `internal/usercommands/alias.go`

NOT 6

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 30 | NOT | <ansi fg="yellow">Custom Alias Added:</ansi> <ansi fg="command">%s</ansi>=<ansi fg="command">%s</ansi> |  | confirmation |
| 33 | NOT | <ansi fg="yellow">Custom Alias Removed:</ansi> <ansi fg="command">%s</ansi> |  | confirmation |
| 71 | NOT | (tables/generic: Built in Aliases) |  | table |
| 84 | NOT | (tables/generic: Custom Aliases) |  | table |
| 87 | NOT | <ansi fg="yellow"><ansi fg="command">help alias</ansi> for more information on setting custom aliases.</ansi> |  | help pointer under the alias table |
| 88 | NOT | (blank line) |  | spacer |

## `internal/usercommands/appraise.go`

NOT 4 RETAG 2

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 24 | RETAG | You can't make out the goods well enough to deal. |  | actions.ShopSightRefusalText (type-level helper stays where it is; caller sends on CategoryRefusal) |
| 30 | RETAG | You need to be at a merchant to appraise items. |  | no merchant in the room |
| 94 | NOT | You give <ansi fg="mobname">%s</ansi> %s gold to appraise <ansi fg="itemname">%s</ansi>. |  | confirmation of the paid appraisal |
| 98 | NOT | (descriptions/identify template) |  | report |
| 117 | NOT | <ansi fg="mobname">%s</ansi> turns <ansi fg="itemname">%s</ansi> over in their hands. |  | outcome line for the actor (room gets its own MobEmote line) |
| 133 | NOT | (bauble appraisal block: name, description, Made of, Weight, Worth, Found in) |  | report |

## `internal/usercommands/ask.go`

FAM 1 RETAG 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 89 | FAM | You must <ansi fg="command">ask</ansi> <ansi fg="mobname">someone</ansi> <ansi fg="yellow">something</ansi> | refusal.Prompt(user, "ask", refusal.What, "ask <name> <topic>") | fewer than two args (bare `ask` or `ask <name>` with no topic); today's line also lacks a full stop. Gives "Ask what? Try ask <name> <topic>." |
| 108 | RETAG | You can't ask another player. |  | could also point at say; left as is (no taste rewords) |

## `internal/usercommands/assess.go`

FAM 1 NOT 7

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 35 | NOT | These remains were bound to a master. The essence is spent. There is nothing left to raise. |  | the reading of this corpse (an assessment result), not a refusal of the command |
| 40 | FAM | You need to wait %s more rounds before you can assess again. | refusal.Cooldown(user, "assess again") | raw round count today (rule 2); gives "You need a moment to recover before you can assess again." |
| 75 | NOT | You study the remains of <ansi fg="mob-corpse">%s</ansi>. |  | outcome (HideNames at sight) |
| 78 | NOT | You sense %s. |  | report |
| 90 | NOT | Whatever this was in life cannot be called back. The dead of your own kind are beyond your craft. |  | part of the reading, sent after the essence line and before the skill award; the assessment happened |
| 143 | NOT | The essence is too faint to animate any form. |  | report |
| 145 | NOT | It could sustain: %s. |  | report |
| 154 | NOT | Raising %s would set aside %s of your conviction while it serves.[ You could not spare that right now.] |  | report (banded, no numbers) |

## `internal/usercommands/assist.go`

FAM 1 RETAG 4

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 22 | FAM | You are not in a party. | refusal.NotInParty(user) | controller (Task 4): same situation as party.go:61, converted with it; agent: only one "not in a party" site in this group; a NotInParty helper may be worth it if other groups repeat it |
| 63 | RETAG | No party member found to assist. |  | does not echo the typed name |
| 68 | RETAG | <ansi fg="username">%s</ansi> is not fighting anyone. |  | echoes a party member's name (own party, known) |
| 73 | RETAG | <ansi fg="username">%s</ansi> is not here. |  | echoes a party member's name (own party, known) |
| 90 | RETAG | <ansi fg="username">%s</ansi>'s target is no longer here. |  | refusal (nothing to attack); names the party member only |

## `internal/usercommands/attack.go`

FAM 1 RETAG 4

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 124 | FAM | There is nothing here to attack. | refusal.NoTarget(user, "attack") | bare `attack` with no foe to pick up is the no-target situation; gives "You have no target. Try attack <name>." The const attackNothingHereLine and any test on it go with it |
| 206 | RETAG | %s is someone's companion! |  | name passes HideNames at the attacker's sight; target was resolved by AimBySight |
| 211 | RETAG | You can't attack <ansi fg="mobname">%s</ansi>. |  | HideNames at sight |
| 333 | RETAG | (room.CanPvp error) Fighting is not allowed here. / PVP is disabled. / You need more experience before engaging in PVP. / This is not a PVP area. |  | the error carries authored player sentences (rooms.go:3135), not internal text; dormant while PVP ships disabled |
| 339 | RETAG | <ansi fg="username">%s</ansi> is in your party! |  | own party member, HideNames at sight |

## `internal/usercommands/ban.go`

NARR 1 NOT 7 USAGE 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 24 | USAGE | <ansi fg="yellow">Usage: ban <name> [reason]  \|  ban ip <name\|ip> [reason]</ansi> |  | staff command |
| 40 | NOT | <ansi fg="red">No online player named "%s" (and "%s" is not an IP).</ansi> |  | staff-only command output (admin-ish); a CategoryAdmin candidate though not an admin.*.go file |
| 45 | NOT | <ansi fg="red">Could not read that player's connection.</ansi> |  | staff-only output |
| 59 | NOT | <ansi fg="alert-5">IP ban FAILED to save:</ansi> %s. The ban is NOT in effect. %s |  | staff-only; the raw error is deliberate for staff (review finding 7). Not Failed: staff need the cause on screen |
| 62 | NOT | <ansi fg="alert-5">IP banned:</ansi> %s |  | staff confirmation |
| 84 | NOT | <ansi fg="alert-5">Ban FAILED to save:</ansi> <ansi fg="username">%s</ansi> is NOT banned. %s |  | staff-only; raw error deliberate for staff |
| 90 | NARR | <ansi fg="alert-5">You have been banned.</ansi> %s | CategoryWarning | sent to the banned player (another player), not to the issuer; a moderation notice, not System output for the actor |
| 93 | NOT | <ansi fg="yellow">Note: no account named "%s" is currently known. The name is banned and will apply if it is ever registered.</ansi> |  | staff note |
| 96 | NOT | <ansi fg="username">%s</ansi> has been <ansi fg="alert-5">banned</ansi>. Reason: %s |  | staff confirmation |

## `internal/usercommands/bank.go`

NOT 10 RETAG 3

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 19 | NOT | (blank line) |  | spacer |
| 22 | RETAG | You are not at a bank. |  | trailing CRLF kept |
| 27 | NOT | You have <ansi fg="gold">%s gold</ansi> on hand and <ansi fg="gold">%s gold</ansi> in the bank. |  | report |
| 28 | NOT | You can <ansi fg="command">deposit</ansi> to or <ansi fg="command">withdraw</ansi> from the bank. |  | help line under the balance report |
| 40 | RETAG | Try <ansi fg="command">help bank</ansi> for more information about banking. |  | bad subcommand (not deposit/withdraw or no amount): the refusal of malformed input, already a hint |
| 50 | RETAG | You must specify an amount greater than zero to %s. |  | amount typed but zero or not a number; %s is deposit/withdraw |
| 60 | NOT | You don't have that much gold on hand, but what you do have you deposit. |  | the deposit proceeds with what is on hand: a notice on an outcome, not a refusal |
| 72 | NOT | You deposit <ansi fg="gold">%s gold</ansi>. |  | confirmation |
| 73 | NOT | You now have <ansi fg="gold">%s gold</ansi> on hand and <ansi fg="gold">%s gold</ansi> in the bank. |  | report |
| 82 | NOT | You don't have that much gold in the bank, but you withdraw what is there. |  | the withdrawal proceeds: notice on an outcome |
| 94 | NOT | You withdraw <ansi fg="gold">%s gold</ansi>. |  | confirmation |
| 95 | NOT | You now have <ansi fg="gold">%s gold</ansi> on hand and <ansi fg="gold">%s gold</ansi> in the bank. |  | report |
| 98 | NOT | (blank line) |  | spacer |

## `internal/usercommands/bash.go`

FAM 2 RETAG 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 39 | FAM | (actions.CostRefusalText(bashResult.Cost)) | refusal.Cost(user, bashResult.Cost) | planned Cost helper |
| 45 | FAM | <ansi fg="red">You can't bash while focused on your work. Finish or be interrupted first.</ansi> | refusal.WhileBusy(user, "bash") | inline copy of busyRefusalText("bash") |
| 51 | RETAG | You need a shield equipped to perform a shield bash! |  | missing equipment gate |

## `internal/usercommands/biome.go`

NOT 1 RETAG 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 18 | RETAG | No biome information found about this area. |  | the room's biome is missing from the registry (a data gap), but the line carries no internal text; the returned Go error goes to the log, not the player |
| 23 | NOT | (descriptions/biome template) |  | report |

## `internal/usercommands/boot.go`

NARR 1 NOT 2 USAGE 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 21 | USAGE | <ansi fg="yellow">Usage: boot <name> [reason]</ansi> |  | staff command |
| 32 | NOT | <ansi fg="red">No online player named "%s".</ansi> |  | staff-only command output (admin-ish) |
| 37 | NARR | <ansi fg="alert-5">You have been disconnected by staff.</ansi> %s | CategoryWarning | sent to the booted player (another player), a moderation notice |
| 41 | NOT | <ansi fg="username">%s</ansi> has been <ansi fg="alert-5">booted</ansi>. |  | staff confirmation |

## `internal/usercommands/break.go`

NOT 1 RETAG 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 17 | NOT | You break off combat. |  | confirmation |
| 23 | RETAG | You aren't in combat! |  | wrong state |

## `internal/usercommands/bug.go`

FAIL 3 NOT 2 USAGE 3

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 23 | USAGE | <ansi fg="yellow">Usage: <ansi fg="cyan-bold">bug <description></ansi> |  | usage block line 1 (the outer yellow tag is never closed) |
| 24 | USAGE | <ansi fg="yellow">Use this command to report bugs or typos you encounter in the game.</ansi> |  | usage block line 2 |
| 25 | USAGE | <ansi fg="yellow">Your character name, location, and timestamp will be recorded automatically.</ansi> |  | usage block line 3 |
| 31 | FAIL | <ansi fg="red">Could not save bug report. Please notify an admin.</ansi> | refusal.Failed(user, "bug report", err, "userId", user.UserId) | os.MkdirAll error is a server fault and is dropped today (never logged) |
| 38 | FAIL | <ansi fg="red">Could not save bug report. Please notify an admin.</ansi> | refusal.Failed(user, "bug report", err, "userId", user.UserId) | os.OpenFile error, never logged today |
| 53 | FAIL | <ansi fg="red">Could not save bug report. Please notify an admin.</ansi> | refusal.Failed(user, "bug report", err, "userId", user.UserId) | f.WriteString error, never logged today |
| 57 | NOT | <ansi fg="green">Bug report submitted. Thank you for helping improve the game!</ansi> |  | confirmation |
| 58 | NOT | <ansi fg="yellow">(You can also use <ansi fg="cyan-bold">bug</ansi> to report typos.)</ansi> |  | confirmation tail |

## `internal/usercommands/busy_refuse.go`

FAM 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 26 | FAM | <ansi fg="red">You can't %s while focused on your work. Finish or be interrupted first.</ansi> | refusal.WhileBusy(user, verb) | the planned helper's source; busyRefusalText moves into the package |

## `internal/usercommands/cancel.go`

NOT 2 RETAG 2

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 35 | RETAG | You aren't doing anything to cancel. |  | bare cancel while free |
| 52 | NOT | You stop casting. |  | confirmation |
| 129 | NOT | You let the spell go. |  | confirmation |
| 135 | RETAG | You have no %s you can let go of. |  | echoes the typed spell or condition name (the player's own, known); raw typed text, not passed through the package echo |

## `internal/usercommands/channel.go`

NOT 3 RETAG 1 USAGE 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 31 | USAGE | Usage: <ansi fg="command">%s</ansi> &lt;message&gt; |  | chat/newbie/trade with no message |
| 81 | NOT | <ansi fg="yellow">Channels</ansi> <ansi fg="8">(channels &lt;name&gt; to toggle):</ansi> |  | report header |
| 87 | NOT |   <ansi fg="%s">%s</ansi>: %s |  | report row |
| 98 | RETAG | No such channel. Valid channels: %s |  | lists the channel names (known) |
| 117 | NOT | Channel <ansi fg="%s">%s</ansi> is now <ansi fg="%s">%s</ansi>. |  | toggle state |

## `internal/usercommands/character.go`

NARR 1 NOT 12 RETAG 9 REWORD 3

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 45 | REWORD | <ansi fg="203">Alt character are disabled on this server.</ansi> | `<ansi fg="203">Alt characters are disabled on this server.</ansi>` | rule 4: broken English (plural noun, "are") |
| 50 | REWORD | <ansi fg="203">You must gain at least 20 total skill ranks to access character alts.</ansi> | `<ansi fg="203">You need more skill training before you can have alt characters.</ansi>` | rule 2: prints a raw skill-rank threshold (20 is hardcoded here, not a config knob) |
| 79 | NOT | (blank line) |  | spacer before alt table |
| 80 | NOT | (tables/generic: Your alt characters) |  | table |
| 132 | RETAG | <ansi fg="mobname">%s</ansi> is currently hired out. |  | names the player's own alt |
| 142 | RETAG | <ansi fg="203">You already have too many alts.</ansi> |  | first line of a two-line refusal |
| 143 | RETAG | <ansi fg="203">You'll need to delete one to create a new one.</ansi> |  | second line of the same refusal (the next step) |
| 188 | NOT | (blank line) |  | spacer |
| 189 | NOT | (tables/generic: Your alt characters) |  | table |
| 211 | NOT | <ansi fg="203">Okay. Aborting.</ansi> |  | confirmation of the player's own "no" |
| 226 | NOT | <ansi fg="username">%s</ansi> <ansi fg="red">is deleted.</ansi> |  | confirmation |
| 232 | RETAG | <ansi fg="203">No character with the name <ansi fg="username">%s</ansi> found.</ansi> |  | echoes the typed alt name (own alts, known) but raw: rawResponse is not escaped, so typed markup styles the line; route it through the package echo when touched. Same wording at 311, 357, 458 |
| 242 | NOT | (blank line) |  | spacer |
| 243 | NOT | (tables/generic: Your alt characters) |  | table |
| 265 | NOT | <ansi fg="203">Okay. Aborting.</ansi> |  | confirmation of the player's own "no" |
| 303 | NOT | You dematerialize as <ansi fg="username">%s</ansi>. and rematerialize as <ansi fg="username">%s</ansi>! |  | outcome (note: stray full stop before "and"; not a refusal, so out of scope) |
| 311 | RETAG | <ansi fg="203">No character with the name <ansi fg="username">%s</ansi> found.</ansi> |  | same as 232 (raw unescaped echo) |
| 321 | NOT | (blank line) |  | spacer |
| 322 | NOT | (tables/generic: Your alt characters) |  | table |
| 357 | RETAG | <ansi fg="203">No character with the name <ansi fg="username">%s</ansi> found.</ansi> |  | same as 232 (raw unescaped echo) |
| 376 | RETAG | <ansi fg="mobname">%s</ansi> is already hired out. |  | own alt name |
| 400 | REWORD | You only have <ansi fg="gold">%s gold</ansi> and it would cost <ansi fg="gold">%s gold</ansi> to hire <ansi fg="username">%s</ansi>. | same text; the first amount becomes user.Character.Gold | controller: bug, both amounts were charValue; agent: refusal (not enough gold), classifier said other. BUG: the Sprintf passes charValue for both amounts, so it reads "You only have 500 gold and it would cost 500 gold"; the first arg should be user.Character.Gold |
| 408 | RETAG | You can only have %s mobs following you at a time. |  | refusal; the follower cap is a count, not a damage/duration/skill value. "mobs" is engine jargon but not one of the reword rules |
| 448 | NARR | <ansi fg="username">%s</ansi> appears to help you out! | CategoryMobEmote | controller (slice 4 N2): the hired alt appears; the room line rides MobEmote; agent: outcome |
| 458 | RETAG | <ansi fg="203">No character with the name <ansi fg="username">%s</ansi> found.</ansi> |  | same as 232 (raw unescaped echo) |

## `internal/usercommands/companion.go`

FAM 1 NOT 15 RETAG 2 REWORD 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 31 | FAM | You have no companions. | refusal.NoCompanion(user) | no companion records at all, so "You have no companion travelling with you." is true; same situation as dismiss.go:61 |
| 35 | NOT | <ansi fg="yellow">━━━ Companions ━━━</ansi> |  | report header |
| 45 | NOT |   <ansi fg="mobname">%s</ansi> [%s] <ansi fg="239">[offline]</ansi> (assist: %s) |  | report row |
| 70 | NOT |   <ansi fg="mobname">%s</ansi> [%s] (assist: %s) |  | report row |
| 74 | NOT |     HP: %s  SP: %s  CP: %s |  | report row (bars) |
| 79 | NOT |     Mutations: %s |  | report row |
| 106 | RETAG | You have no companion named "%s". |  | echoes the typed companion name (own, known); raw, not through the package echo. Same line at dismiss.go:67 |
| 116 | RETAG | <ansi fg="mobname">%s</ansi> already has a name. |  | bonded companion rename refused |
| 122 | REWORD | (validateCompanionName error) Companion names must be between 2 and 20 characters. / Companion names may only contain letters. / name must be between %s and %s characters long / %s (NameRejectReason) / that name matched the prohibited pattern: %s / that name is in use / that name is already in use / that name is in use by a companion | Task 3: ValidateActorName returns sentences; the caller sends err.Error() unchanged | controller: one fix at the source covers companion, pet, renameself and signup; agent: player's bad input, so not Failed. Rules 3 and 4: the ValidateActorName strings are lowercase fragments with no full stop, and the banned case prints the internal prohibited regex pattern. Alternative: keep per-reason lines but capitalise them and drop the pattern |
| 141 | NOT | <ansi fg="mobname">%s</ansi> is now known as <ansi fg="mobname">%s</ansi>. |  | confirmation (classifier said refusal) |
| 156 | NOT | <ansi fg="mobname">%s</ansi>'s auto-assist is now <ansi fg="yellow">%s</ansi>. |  | toggle state |
| 172 | NOT | <ansi fg="yellow">━━━ %s [%s] ━━━</ansi> |  | report header |
| 178 | NOT |   This companion is not currently present. |  | report row (detail view of an offline companion) |
| 179 | NOT |   Auto-Assist: %s |  | report row |
| 184 | NOT |   Health: %s Stamina: %s Conviction: %s |  | report row (bands) |
| 192 | NOT |   <ansi fg="yellow">Strength:</ansi>   %s <ansi fg="yellow">Dexterity:</ansi>  %s <ansi fg="yellow">Perception:</ansi> %s |  | report row |
| 198 | NOT |   <ansi fg="yellow">Vitality:</ansi>   %s <ansi fg="yellow">Willpower:</ansi>  %s <ansi fg="yellow">Charisma:</ansi>   %s |  | report row |
| 206 | NOT |   <ansi fg="yellow">Mutations:</ansi>  %s |  | report row |
| 209 | NOT |   Auto-Assist: %s |  | report row |

## `internal/usercommands/conditions.go`

NOT 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 31 | NOT | (character/conditions template) |  | report |

## `internal/usercommands/consider.go`

RETAG 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 51 | RETAG | You can't fight <ansi fg="mobname">%s</ansi>. |  | HideNames at sight; target resolved through AimBySight |

## `internal/usercommands/cooldowns.go`

NOT 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 14 | NOT | (character/cooldowns template) |  | report |

## `internal/usercommands/craft.go`

FAM 3 NOT 23 RETAG 17 REWORD 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 83 | RETAG | You can't see well enough to work on anything here. |  | sight refusal, no echo |
| 121 | RETAG | You can't see well enough to work on anything here. |  | via craftDeliver; unreachable today (early return at 83) |
| 129 | RETAG | You know more than one recipe like that: %s. Type more of the name to pick one. |  | via craftDeliver; lists known recipes (known names) |
| 135 | REWORD | <ansi fg="red">No recipe found for "%s". Type <ansi fg="cyan-bold">craft list</ansi> to see available recipes.</ansi> | fmt.Sprintf(`<ansi fg="red">No recipe matches '%s'.</ansi> Try <ansi fg="command">craft list</ansi>.`, rest) | rule 4: 64 columns before the echo, so any typed name over 16 columns passes 80; the raw typed text is unescaped and uncapped (pass it through the package echo). Via craftDeliver |
| 141 | RETAG | <ansi fg="red">You don't know that recipe yet. Keep crafting to discover new ones!</ansi> |  | via craftDeliver. Same line at 315 |
| 145 | FAM | <ansi fg="red">You are already working on something. Finish or be interrupted first.</ansi> | refusal.WhileBusy(user, "craft") | via craftDeliver (self-only SendTrio; the helper can send directly). Busy gate. Same line at 321, 416 |
| 149 | RETAG | <ansi fg="red">That recipe is just beyond your %s skill. A little more practice should do it.</ansi> / <ansi fg="red">That recipe is well beyond your %s skill for now.</ansi> |  | craftSkillTooLowText (type-level helper stays); caller sends on CategoryRefusal |
| 154 | RETAG | <ansi fg="red">You need to be at a %s to craft that.</ansi> |  | station name, no echo of typed input |
| 160 | RETAG | <ansi fg="red">You are missing: %s.</ansi> |  | %s is the storage-aware missing ingredient tag (e.g. copper-wire) |
| 165 | RETAG | <ansi fg="red">The %s must be your own work. It bears another maker's mark.</ansi> |  | component name from the player's own inventory |
| 286 | RETAG | You're too encumbered to draw any more from storage. |  | refusal of the storage pull mid-way |
| 315 | RETAG | <ansi fg="red">You don't know that recipe yet. Keep crafting to discover new ones!</ansi> |  | enchanting path; same as 141 |
| 321 | FAM | <ansi fg="red">You are already working on something. Finish or be interrupted first.</ansi> | refusal.WhileBusy(user, "craft") | enchanting path; same as 145 |
| 328 | RETAG | <ansi fg="red">That recipe is just beyond your %s skill. A little more practice should do it.</ansi> / <ansi fg="red">That recipe is well beyond your %s skill for now.</ansi> |  | craftSkillTooLowText, enchanting path |
| 335 | RETAG | <ansi fg="red">You need to be at a %s to craft that.</ansi> |  | enchanting path |
| 344 | RETAG | <ansi fg="red">You are missing: %s.</ansi> |  | enchanting path |
| 351 | RETAG | <ansi fg="red">The %s must be your own work. It bears another maker's mark.</ansi> |  | enchanting path |
| 370 | RETAG | <ansi fg="red">%s</ansi> (resolveEnchantSlot: You don't have a %s equipped to enchant. / Which slot? You have multiple options: + list / That recipe targets %s slots, not %s. / There is no %s#%s equipped. / That equipment slot is empty.) |  | strings from enchant_slot.go; all refusals about the player's own equipment, wording fine. The "You don't have a %s equipped" line may need its own phrase-guard treatment later |
| 375 | RETAG | <ansi fg="red">Could not find a valid item in that slot.</ansi> |  | enchant target slot resolved to nothing |
| 391 | RETAG | <ansi fg="red">%s</ansi> (Character.ReservationRefusal) |  | type-level helper stays; caller sends on CategoryRefusal |
| 416 | FAM | <ansi fg="red">You are already working on something. Finish or be interrupted first.</ansi> | refusal.WhileBusy(user, "craft") | TransitionToCrafting error read as busy; the err is discarded. If the transition can fail for another reason it should be Failed instead |
| 495 | NOT | (blank line) |  | spacer |
| 496 | NOT | <ansi fg="green-bold"> .:. Ready to Craft .:.</ansi> |  | report header |
| 499 | NOT | (blank line) |  | spacer |
| 500 | NOT | <ansi fg="yellow">Nothing you can craft here right now. Try <ansi fg="cyan-bold">craft list</ansi> to see everything you know.</ansi> |  | empty state of the bare craft report, printed under its header |
| 501 | NOT | (blank line) |  | spacer |
| 514 | NOT | (blank line) |  | spacer |
| 515 | NOT | <ansi fg="yellow">%s</ansi> |  | report skill heading |
| 519 | NOT | (craftRecipeRow) |  | report row |
| 523 | NOT | (blank line) |  | spacer |
| 524 | NOT | <ansi fg="cyan">Type <ansi fg="cyan-bold">craft list</ansi> to see every recipe you know.</ansi> |  | report footer |
| 525 | NOT | (blank line) |  | spacer |
| 534 | NOT | <ansi fg="yellow">No crafting recipes are currently available.</ansi> |  | empty state of craft list (no recipes loaded) |
| 547 | NOT | <ansi fg="yellow">You haven't discovered any crafting recipes yet.</ansi> |  | empty state of craft list |
| 585 | NOT | (blank line) |  | spacer |
| 586 | NOT | <ansi fg="cyan-bold"> .:. Crafting Recipes .:.</ansi> |  | report header |
| 603 | NOT | (blank line) |  | spacer |
| 604 | NOT |  %s |  | section header |
| 619 | NOT |   <ansi fg="yellow">%s</ansi> <ansi fg="white">(%s)</ansi> |  | report skill heading |
| 637 | NOT |     <ansi fg="red">[%s]</ansi> <ansi fg="white">%s</ansi>: %s  <ansi fg="red">%s</ansi><ansi fg="dark-cyan">%s, %s</ansi> |  | report row |
| 641 | NOT |     <ansi fg="green">[%s]</ansi> <ansi fg="white">%s</ansi>: %s  <ansi fg="dark-cyan">%s, %s</ansi> |  | report row |
| 651 | NOT | (blank line) |  | spacer |
| 652 | NOT | <ansi fg="cyan">Overall recipe knowledge: %s</ansi> |  | report footer |
| 653 | NOT | (blank line) |  | spacer |

## `internal/usercommands/deletecharacter.go`

NOT 2 RETAG 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 27 | NOT | Aborted. |  | confirmation of the player's own "no" |
| 40 | RETAG | That doesn't match. Aborted. |  | typed confirmation name did not match |
| 51 | NOT | Your form dissolves into shimmering dust. Farewell, <ansi fg="username">%s</ansi>. |  | outcome |

## `internal/usercommands/dismiss.go`

FAM 1 NARR 1 NOT 4 RETAG 1 REWORD 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 61 | FAM | You have no companions to dismiss. | refusal.NoCompanion(user) | no companion records at all; same situation as companion.go:31 |
| 67 | RETAG | You have no companion named "%s". |  | same as companion.go:106 (own name, raw echo) |
| 86 | REWORD | <ansi fg="mobname">%s</ansi> is not yours to dismiss. If you want to part ways, you will have to tell them. | fmt.Sprintf(`<ansi fg="mobname">%s</ansi> is not yours to dismiss. To part ways, tell them.`, compName) | rule 4: the fixed text is 79 visible columns, so any companion name pushes the line past 80 |
| 99 | NOT | Your bond with <ansi fg="mobname">%s</ansi> fades away. |  | outcome |
| 128 | NOT | You part ways with <ansi fg="mobname">%s</ansi>, who turns away and takes a different road. |  | outcome |
| 145 | NOT | You release <ansi fg="mobname">%s</ansi>. It dissolves back into the energies that shaped it. |  | outcome |
| 162 | NOT | You sever the bond with <ansi fg="mobname">%s</ansi>. |  | outcome |
| 178 | NARR | <ansi fg="mobname">%s</ansi> turns on you with fury! | CategoryMobEmote | controller (slice 4 N2): the dismissed companion acts on the owner; the room line rides MobEmote; agent: outcome to the actor (room gets its own MobEmote line) |

## `internal/usercommands/drain.go`

FAM 1 RETAG 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 33 | FAM | (actions.CostRefusalText(res.Cost)) | refusal.Cost(user, res.Cost) | planned Cost helper |
| 38 | RETAG | Your body cannot drain the life from another. |  | species gate (not a life drainer) |

## `internal/usercommands/drop.go`

FAIL 1 FAM 1 NOT 4

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 93 | FAM | Oops! | refusal.Prompt(user, "drop", refusal.HowMuchGold, "drop <amount> gold") | `drop <N> gold` with N below 1 or not a number: the player's bad input. "Oops!" is unclear (rule 4). Matches Give's zero-amount answer ("Give how much gold?") in context.md |
| 105 | FAIL | Oops! | refusal.Failed(user, "drop", err, "userId", user.UserId) | FloorDropGold can only fail here on a server fault (amount was already checked >= 1 and <= Gold); err is dropped today |
| 114 | NOT | You drop <ansi fg="gold">%s gold</ansi> on the floor. |  | confirmation |
| 141 | NOT | You drop %s. |  | confirmation |
| 161 | NOT | You drop the <ansi fg="item">%s</ansi>. |  | confirmation |
| 171 | NOT | Todo. Grenades disabled for now. |  | sent after a grenade is dropped (the drop happened), so not a refusal; it is debug text shown to players and should be deleted (rule 3), outside this slice |

## `internal/usercommands/eat.go`

NOT 1 RETAG 3

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 23 | RETAG | <ansi fg="red">Your hands are committed to the grapple, so you can't reach for that.</ansi> |  | grapple gate; only one in this group |
| 45 | RETAG | You can't eat <ansi fg="itemname">%s</ansi>. |  | names the player's own carried item |
| 53 | RETAG | <ansi fg="red">The food has gone bad! It reeks of decay and is clearly inedible.</ansi> |  | refused before eating (#277) |
| 59 | NOT | You eat some of the <ansi fg="itemname">%s</ansi>. |  | confirmation |

## `internal/usercommands/equip.go`

NOT 7 RETAG 2

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 36 | NOT | <ansi fg="yellow">%s</ansi> (Character.ReservationIncreaseNotice) |  | disclosure after a successful equip |
| 48 | NOT | <ansi fg="yellow">%s</ansi> (Character.ReservationDecreaseNotice) |  | disclosure after a remove |
| 108 | RETAG | Your <ansi fg="item">%s</ansi> doesn't look very fashionable. |  | not equipment; names the player's own item |
| 144 | NOT | You remove your <ansi fg="item">%s</ansi> and return it to your backpack. |  | confirmation |
| 156 | NOT | You equip your <ansi fg="item">%s</ansi> in your %s. |  | confirmation |
| 158 | NOT | You wield your <ansi fg="item">%s</ansi> in your %s. |  | confirmation |
| 171 | NOT | You wear your <ansi fg="item">%s</ansi>.%s |  | confirmation |
| 183 | NOT | You wield your <ansi fg="item">%s</ansi>. You're feeling dangerous. |  | confirmation |
| 222 | RETAG | %s (EquipItemResult.FailureReason from Character.Wear, e.g. CursedRefusal, ReservationRefusal, strength gate) / fallback: You can't figure out how to equip the <ansi fg="item">%s</ansi>. |  | refusals produced by type-level helpers; caller sends on CategoryRefusal |

## `internal/usercommands/flee.go`

NOT 2 RETAG 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 35 | RETAG | (fleeRefusalText) You're locked in, with nowhere to flee to. / You can't break off to flee right now. You can only fight. / You're already trying to break away. Give it a moment. / You're not in combat; there's nothing to flee from. / <ansi fg="red">You can't flee while grappled!</ansi> / <ansi fg="red">You can't flee from the ground. Stand up first!</ansi> / You can't break away just yet. |  | type-level helper map stays; caller sends on CategoryRefusal |
| 39 | NOT | You break away on instinct rather than technique, too spent to use your training. |  | notice on a flee that proceeds (shortage), not a refusal |
| 41 | NOT | You attempt to flee... |  | confirmation |

## `internal/usercommands/gearup.go`

RETAG 2

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 45 | RETAG | You have nothing to wear. |  | wording fine |
| 47 | RETAG | You're already wearing everything you can! |  | nothing left to wear: a refusal of the command |

## `internal/usercommands/get.go`

FAM 3 NOT 16 RETAG 29 REWORD 3

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 66 | RETAG | You can't carry the <ansi fg="itemname">%s</ansi> - you're already overloaded! |  | item was found and seen (TakeFloorItem passed the dark gate), so the echo is not a sight leak; this exact line repeats 8 times across get.go and loot.go, a helper candidate though the wording never varies |
| 85 | NOT | You pick up %s. |  | confirmation |
| 108 | RETAG | You can't see anything to pick up! |  | wording fine |
| 127 | RETAG | The shipping crate is sealed and bound for the caravan; you can't get into it. |  | wording fine (78 columns) |
| 138 | RETAG | Your component bag is empty. |  | get all bag with nothing to move |
| 143 | NOT | You move %s from your component bag to your backpack. |  | confirmation |
| 149 | RETAG | Your bandolier is empty. |  | get all bandolier with nothing to move |
| 154 | NOT | You move %d potion(s) from your bandolier to your backpack. |  | confirmation |
| 182 | RETAG | There's nothing in the <ansi fg="container">%s</ansi>. |  | container resolved and visible/discovered; a state refusal, not a miss |
| 203 | RETAG | This isn't your kill. |  | wording fine |
| 215 | RETAG | There's nothing left on the %s. |  | corpse name via corpseNameFor (sight-safe) |
| 274 | RETAG | There is nothing here to pick up. |  | wording fine |
| 356 | FAM | You don't see a "%s" in your %s. | refusal.NotCarrying(user, rest) | own component bag or bandolier: echo allowed; loses the "in your bag" label; the phrase "You don't see" belongs to the not-here family |
| 368 | NOT | You move the <ansi fg="itemname">%s</ansi> from your %s to your backpack. |  | confirmation |
| 413 | REWORD | You can't do that! | `You can only take things from your own pet.` | rule 4 unclear: the player is trying to get from another player's pet |
| 439 | RETAG | This isn't your kill. |  | wording fine |
| 458 | RETAG | You don't see a %s carried by %s. |  | own pet, so the echo is allowed; echoes raw typed rest (not trimmed or tag-stripped as refusal.echo does); a pet-flavoured NotCarrying would fit if one is added |
| 464 | RETAG | You can't carry the <ansi fg="itemname">%s</ansi> - you're already overloaded! |  | wording fine (see 66) |
| 475 | NOT | You remove a <ansi fg="itemname">%s</ansi> from %s. |  | confirmation |
| 501 | RETAG | (blank line) |  | spacer of the locked-container refusal at 502; moves with it |
| 502 | RETAG | The <ansi fg="container">%s</ansi> is locked. |  | wording fine |
| 503 | RETAG | (blank line) |  | spacer of the locked-container refusal at 502; moves with it |
| 512 | RETAG | There's no gold to grab. |  | wording fine |
| 527 | NOT | You pick up <ansi fg="gold">%d gold</ansi> from the <ansi fg="container">%s</ansi>. |  | confirmation |
| 542 | FAM | You don't see a %s in the <ansi fg="container">%s</ansi>. | refusal.NotHereThing(user) | rule 1: echoes a typed name for an item in a room container |
| 560 | NOT | You take the <ansi fg="itemname">%s</ansi> from the <ansi fg="container">%s</ansi>. |  | confirmation |
| 574 | RETAG | You can't carry the <ansi fg="itemname">%s</ansi> - you're already overloaded! |  | wording fine (see 66) |
| 588 | RETAG | There's no gold to grab. |  | wording fine |
| 600 | NOT | You pick up <ansi fg="gold">%d gold</ansi>. |  | confirmation |
| 626 | RETAG | You can't pick that up, it's about to explode! |  | wording fine |
| 636 | REWORD | The <ansi fg="itemname">%s</ansi> belongs to this household. To take it anyway, <ansi fg="command">steal %s</ansi>. | `The <ansi fg="itemname">%s</ansi> belongs to this household. Try <ansi fg="command">steal %s</ansi>.` | rule 4: 62 visible columns before the item name and steal word, so a typical bauble name runs it past 80; keep in step with 904 |
| 642 | RETAG | You can't carry the <ansi fg="itemname">%s</ansi> - you're already overloaded! |  | wording fine (see 66) |
| 660 | RETAG | You can't pick that up, it's about to explode! |  | wording fine |
| 664 | RETAG | You can't carry the <ansi fg="itemname">%s</ansi> - you're already overloaded! |  | wording fine (see 66) |
| 685 | NOT | You dig out the <ansi fg="itemname">%s</ansi> from where it was stashed. |  | confirmation |
| 693 | NOT | You pick up the <ansi fg="itemname">%s</ansi>. |  | confirmation |
| 714 | RETAG | You can't get the <ansi fg="noun">%s</ansi> |  | room noun (scenery), not a creature or item; missing its full stop, which the normalizer's end-punctuation stage may already add |
| 723 | RETAG | You can't pick up corpses. What would people think? |  | wording fine |
| 736 | RETAG | You can't pick up the <ansi fg="container">%s</ansi>. Try looking at it. |  | container resolved and visible/discovered |
| 751 | NOT | <ansi fg="red-bold">You are severely overloaded!</ansi> Your combat and movement are heavily penalized. |  | encumbrance status after a pickup; a warning, not a refusal (CategoryWarning is a candidate) |
| 753 | NOT | <ansi fg="red">You are heavily encumbered.</ansi> Your combat and movement are significantly penalized. |  | encumbrance status, as 751 |
| 755 | NOT | <ansi fg="yellow">You are encumbered.</ansi> Your combat and movement are penalized. |  | encumbrance status, as 751 |
| 757 | NOT | <ansi fg="yellow">You are carrying a moderate load.</ansi> |  | encumbrance status, as 751 |
| 776 | RETAG | There's no gold to grab. |  | corpse with no gold |
| 785 | NOT | You take <ansi fg="gold">%d gold</ansi> from the %s. |  | confirmation |
| 801 | FAM | You don't see a %s in the %s. | refusal.NotHereThing(user) | rule 1: echoes a typed name for an item on a corpse |
| 809 | RETAG | The leader is holding the loot. / That's not your share. |  | corpseItemGateMessage, by loot mode |
| 816 | RETAG | You can't carry the <ansi fg="itemname">%s</ansi> - you're already overloaded! |  | wording fine (see 66) |
| 828 | NOT | You take the <ansi fg="itemname">%s</ansi> from the %s. |  | confirmation |
| 897 | RETAG | The <ansi fg="itemname">%s</ansi> is fixed in place. |  | fixedInPlace: the fixture was found by name |
| 904 | REWORD | You leave the <ansi fg="itemname">%s</ansi>: it belongs to this household. To take it anyway, <ansi fg="command">steal %s</ansi>. | `You leave the <ansi fg="itemname">%s</ansi>; it belongs to this household. Try <ansi fg="command">steal %s</ansi>.` | rule 4: 76 visible columns before the item name and steal word, so it always passes 80; per-item refusal inside a get all sweep |

## `internal/usercommands/give.go`

NARR 2 NOT 7

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 128 | NOT | You give the <ansi fg="item">%s</ansi> to <ansi fg="username">%s</ansi>. |  | confirmation |
| 132 | NARR | <ansi fg="username">%s</ansi> gives you their <ansi fg="item">%s</ansi>. | CategoryLoot | the recipient's line about another player's give; the room line on the same event rides CategoryLoot |
| 187 | NOT | You give <ansi fg="gold">%d gold</ansi> to <ansi fg="username">%s</ansi>. |  | confirmation |
| 212 | NOT | You give the <ansi fg="item">%s</ansi> to <ansi fg="mobname">%s</ansi>. |  | confirmation |
| 244 | NOT | You give the <ansi fg="item">%s</ansi> to <ansi fg="mobname">%s</ansi>. |  | confirmation |
| 328 | NOT | You count out <ansi fg="gold">%d gold</ansi> and put it back in your pocket. |  | confirmation (give to self) |
| 353 | NOT | You give <ansi fg="gold">%d gold</ansi> to <ansi fg="username">%s</ansi>. |  | confirmation |
| 357 | NARR | <ansi fg="username">%s</ansi> gives you <ansi fg="gold">%d gold</ansi>. | CategoryLoot | recipient's line, as 132 |
| 374 | NOT | You give the <ansi fg="itemname">%s</ansi> to %s. |  | confirmation (pet) |

## `internal/usercommands/go.go`

FAIL 1 FAM 1 NARR 4 NOT 2 RETAG 6 REWORD 2

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 42 | NOT | You know this lock well, you quickly pick it. / You use the key on your key ring to unlock the %s exit. / You use your %s to unlock the %s exit, and add it to your key ring for the future. |  | unlockExit playerMsg: confirmations |
| 71 | FAM | You can't do that! You are in combat! | refusal.WhileFighting(user, "move") | reads "You can't move while fighting." |
| 77 | RETAG | (quest sequence lock text, authored per quest) |  | questSequenceLock temp data; the wording is quest content |
| 91 | REWORD | You can't do that! | `You can't move right now.` | rule 4 unclear: a NoMovement condition holds the player |
| 113 | RETAG | You can't do that while you're dead. Hold on; you'll be pulled back to safety. |  | wording fine (78 columns) |
| 179 | REWORD | There's a lock preventing you from going that way. You'll need a <ansi fg="item">Key</ansi> or to <ansi fg="command">pick</ansi> the lock with <ansi fg="item">lockpicks</ansi>. | `That way is locked. You need a <ansi fg="item">key</ansi>, or <ansi fg="command">pick</ansi> the lock with <ansi fg="item">lockpicks</ansi>.` | rule 4: 104 visible columns |
| 207 | RETAG | You're too encumbered to move (<ansi fg="command">help encumbrance</ansi>)! |  | wording fine |
| 210 | RETAG | You're too tired to move (slow down)! |  | wording fine |
| 214 | RETAG | You're too exhausted to move! Rest and recover your stamina. |  | wording fine |
| 218 | NOT | <ansi fg="yellow">You're feeling winded. Consider resting to recover your stamina.</ansi> |  | warning; the move goes ahead (CategoryWarning is a candidate) |
| 252 | FAIL | Oops, couldn't move there! | refusal.Failed(user, "move", err, "userId", user.UserId) | rooms.MoveToRoom error: a server fault; the err is dropped today |
| 313 | NARR | %s follows you. | CategoryRoomExit | controller (slice 4 N2): the pet follows on departure, as the party follow line (go.go "You follow the party leader.") rides RoomExit; agent: confirmation: own pet follows |
| 352 | NARR | You follow the party leader. | CategoryRoomExit | sent to each party member when the leader moves, an event another player caused |
| 461 | NARR | <ansi fg="mobname">%s</ansi> notices you as you enter! | CategoryMobEmote | world event: an aggro mob notices the arriving player |
| 463 | NARR | <ansi fg="yellow">Something notices you in the darkness!</ansi> | CategoryMobEmote | world event, dark form of 461 |
| 571 | RETAG | You're bumping into walls. |  | compass direction with no exit |

## `internal/usercommands/gore.go`

FAM 1 RETAG 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 33 | FAM | You are too spent to manage that right now. / You cannot muster the resolve for that right now. | refusal.Cost(user, res.Cost) | actions.CostRefusalText path |
| 38 | RETAG | You have no horns to gore with. |  | wording fine |

## `internal/usercommands/grapple.go`

FAM 2 RETAG 5 REWORD 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 34 | FAM | You are too spent to manage that right now. / You cannot muster the resolve for that right now. | refusal.Cost(user, res.Cost) | actions.CostRefusalText path |
| 40 | FAM | <ansi fg="red">You can't grapple while focused on your work. Finish or be interrupted first.</ansi> | refusal.WhileBusy(user, "grapple") | hand-written copy of busyRefusalText |
| 54 | RETAG | %s is already grappling. |  | target was staged through the sight-gated special-move target |
| 67 | RETAG | Your body has nothing that can take hold. Grappling is not a thing you can do. |  | own body (species): before any roll |
| 70 | RETAG | Grappling means seizing and holding, and you have no arms to do it with. |  | own body (no arms) |
| 73 | REWORD | You get both hands on %s and heave. It might as well be a boulder. Nothing shifts. | `You heave at %s with both hands. Nothing shifts.` | rule 4: 82 visible columns before the target name; "might as well be" is also an idiom; immune target, no roll happens, so a refusal not an outcome |
| 81 | RETAG | You cannot get a grip on that. |  | wording fine |
| 83 | RETAG | You reach for %s but your hands pass right through! |  | incorporeal target: refusal before any roll |

## `internal/usercommands/guild.go`

FAIL 1 FAM 8 NARR 5 NOT 38 RETAG 25 REWORD 7 USAGE 7

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 74 | RETAG | Unknown guild command. Try <ansi fg="command">help guild</ansi>. |  | wording fine |
| 86 | NARR | (announceGuild msg: "%s has left the guild.", "%s deposited %d gold into the guild treasury.", etc.) | CategoryBroadcast | sent to every other online guild member; no guild channel category exists, so a new one shared with party notices may be better |
| 97 | NOT | <ansi fg="yellow-bold">%s</ansi> <ansi fg="cyan">[%s]</ansi>: %d member(s) |  | guild info report |
| 101 | NOT | <ansi fg="cyan">"%s"</ansi> |  | guild info report (MOTD) |
| 107 | NOT |   <ansi fg="white">%-20s</ansi> <ansi fg="black-bold">%s</ansi> |  | guild roster row |
| 115 | RETAG | You are already in a guild. |  | wording fine |
| 120 | USAGE | Usage: <ansi fg="command">guild create <TAG> <name></ansi>  (TAG is 2-4 letters/digits). |  | usage line |
| 134 | NOT | Guild founding cancelled. |  | answer to the confirm prompt |
| 139 | RETAG | You need %d gold in the bank to found a guild. |  | gold the player must pay: allowed |
| 145 | REWORD | Cannot found the guild: %s | fmt.Sprintf(`You can't found that guild: %s.`, err) | rule 3: raw Go error appended, no full stop; every guilds.Create error is the player's mistake (tag/name rules at guilds/guilds.go:91-106, tag or name taken at registry.go:99-102), so not FAIL; better still, map each sentinel to its own line |
| 150 | NOT | You found <ansi fg="yellow-bold">%s</ansi> <ansi fg="cyan">[%s]</ansi>! You are its leader. Invite others with <ansi fg="command">guild invite <player></ansi>. |  | confirmation |
| 156 | NOT | There are no guilds yet. Found one with <ansi fg="command">guild create</ansi>. |  | empty guild list report |
| 160 | NOT | <ansi fg="yellow-bold">Guilds:</ansi> |  | list header |
| 169 | NOT |   <ansi fg="cyan">[%s]</ansi> <ansi fg="white">%s</ansi>: %d member(s), led by %s |  | list row |
| 181 | RETAG | As leader you must <ansi fg="command">guild transfer <player></ansi> or <ansi fg="command">guild disband</ansi> first. |  | leader cannot leave a guild with members |
| 192 | NOT | Cancelled. |  | answer to the confirm prompt |
| 197 | NOT | You disband the guild. |  | confirmation |
| 203 | NOT | You leave <ansi fg="yellow-bold">%s</ansi>. |  | confirmation |
| 216 | FAM | Only the guild leader can disband the guild. | refusal.NotGuildLeader(user, "disband the guild") | NEW:NotGuildLeader "Only the guild leader can <action>." |
| 226 | NOT | Cancelled. |  | answer to the confirm prompt |
| 231 | NOT | You disband the guild. |  | confirmation |
| 268 | FAM | Only officers and the leader can invite. | refusal.NotGuildOfficer(user, "invite") | NEW:NotGuildOfficer "Only officers and the leader can <action>." |
| 273 | USAGE | Usage: <ansi fg="command">guild invite <player></ansi> |  | usage line |
| 278 | RETAG | No player by that name is online to invite. |  | online-player lookup, not sight |
| 282 | RETAG | You can't invite yourself. |  | wording fine |
| 286 | RETAG | That player is already in a guild. |  | wording fine |
| 290 | RETAG | That player already has a pending invite. |  | wording fine |
| 294 | NARR | <ansi fg="username">%s</ansi> invites you to join <ansi fg="yellow-bold">%s</ansi> <ansi fg="cyan">[%s]</ansi>. <ansi fg="command">guild accept</ansi> or <ansi fg="command">guild decline</ansi>. | CategoryBroadcast | the invited player's line about another player's action; same home as 86 |
| 295 | NOT | You invite <ansi fg="username">%s</ansi> to the guild. |  | confirmation |
| 300 | RETAG | You are already in a guild. |  | wording fine |
| 305 | RETAG | You have no pending guild invitation. |  | guild twin of refusal.NoPartyInvite |
| 309 | REWORD | Cannot join: %s | `That guild no longer exists.` | rule 3: raw Go error; guilds.AddMember can only fail here with "no such guild" (the guild was disbanded after the invite), since already-in-a-guild is checked at 299 |
| 312 | NOT | You join <ansi fg="yellow-bold">%s</ansi> <ansi fg="cyan">[%s]</ansi>! |  | confirmation |
| 320 | RETAG | You have no pending guild invitation. |  | wording fine |
| 324 | NOT | You decline your pending guild invitation(s). |  | confirmation |
| 334 | FAM | Only officers and the leader can kick. | refusal.NotGuildOfficer(user, "kick") | NEW:NotGuildOfficer |
| 339 | RETAG | No guild member by that name. |  | guild roster lookup, not sight |
| 343 | RETAG | To leave, use <ansi fg="command">guild leave</ansi>. |  | refuses kicking yourself |
| 347 | RETAG | You can't kick someone of equal or higher rank. |  | wording fine |
| 353 | NARR | You have been removed from <ansi fg="yellow-bold">%s</ansi>. | CategoryBroadcast | the kicked player's line about another player's action; same home as 86 |
| 355 | NOT | You remove <ansi fg="username">%s</ansi> from the guild. |  | confirmation |
| 368 | FAM | Only the guild leader can change ranks. | refusal.NotGuildLeader(user, "change ranks") | NEW:NotGuildLeader |
| 373 | RETAG | Name another guild member. |  | no such member, or yourself |
| 380 | RETAG | That member is already an officer. |  | wording fine |
| 384 | NOT | You promote <ansi fg="username">%s</ansi> to %s. |  | confirmation |
| 387 | RETAG | That member is not an officer. |  | refuses demoting a plain member |
| 391 | NOT | You demote <ansi fg="username">%s</ansi> to %s. |  | confirmation |
| 394 | NARR | Your guild rank has changed. | CategoryBroadcast | the promoted or demoted player's line; same home as 86 |
| 405 | FAM | Only the guild leader can transfer leadership. | refusal.NotGuildLeader(user, "transfer leadership") | NEW:NotGuildLeader |
| 410 | RETAG | Name another guild member to lead. |  | wording fine |
| 421 | NOT | Cancelled. |  | answer to the confirm prompt |
| 425 | NOT | You hand leadership to <ansi fg="username">%s</ansi> and become an officer. |  | confirmation |
| 457 | USAGE | Usage: <ansi fg="command">guild chat &lt;message&gt;</ansi>  (or <ansi fg="command">gc &lt;message&gt;</ansi>) |  | usage line |
| 466 | NARR | <ansi fg="cyan">(guild)</ansi> <ansi fg="username">%s</ansi>: <ansi fg="white">%s</ansi> | CategoryOOC | chat other members read; party chat (party.go:813) is the same case, so one channel category for both may be better |
| 469 | NOT | <ansi fg="cyan">(guild)</ansi> You: <ansi fg="white">%s</ansi> |  | sender's own echo; should move with 466 whatever category that takes |
| 530 | USAGE | Usage: <ansi fg="command">guild deposit <amount\|all></ansi>  (gold comes from your bank). |  | usage line |
| 534 | RETAG | You only have <ansi fg="gold">%d gold</ansi> in the bank. |  | the player's own gold |
| 538 | RETAG | Deposit a positive amount. |  | reached by "deposit all" with an empty bank |
| 542 | FAIL | Cannot deposit: %s | refusal.Failed(user, "deposit", err, "userId", user.UserId) | guilds.DepositGold can only fail here with "no such guild" (guild deleted mid-command); "amount must be positive" is pre-checked at 537 |
| 547 | NOT | You deposit <ansi fg="gold">%d gold</ansi> into the guild treasury. |  | confirmation |
| 558 | REWORD | Only the leader can withdraw (unless the leader has delegated treasury access to officers). | `Only the leader can withdraw, unless officers have treasury access.` | rule 4: 91 visible columns |
| 563 | USAGE | Usage: <ansi fg="command">guild withdraw <amount\|all></ansi>  (gold goes to your bank). |  | usage line |
| 567 | RETAG | The treasury is empty. |  | withdraw all from an empty treasury |
| 572 | REWORD | Cannot withdraw: %s | `The treasury doesn't hold that much gold.` | rule 3: raw Go error; the reachable error is the player's ("the treasury does not hold that much gold", registry.go:338); "no such guild" is a race that could go to Failed |
| 577 | NOT | You withdraw <ansi fg="gold">%d gold</ansi> from the guild treasury to your bank. |  | confirmation |
| 589 | USAGE | Usage: <ansi fg="command">guild donate <item></ansi> |  | usage line |
| 601 | REWORD | Cannot donate: %s | `The guild vault is full.` | rule 3: raw Go error; the reachable error is "the guild vault is full" (registry.go:356); "no such guild" is a race that could go to Failed |
| 606 | NOT | You donate %s to the guild vault. |  | confirmation |
| 617 | REWORD | Only the leader can take from the vault (unless the leader has delegated treasury access to officers). | `Only the leader can take from the vault, unless officers have treasury access.` | rule 4: 102 visible columns |
| 623 | RETAG | No such item in the guild vault. See <ansi fg="command">guild treasury</ansi>. |  | vault lookup, not sight |
| 630 | RETAG | Your pack is too full to carry that. |  | wording fine |
| 637 | RETAG | Cannot take that item right now. |  | vault changed under the taker; plain words, no error text |
| 641 | NOT | You take %s from the guild vault. |  | confirmation |
| 654 | FAM | Only the leader can delegate treasury access. | refusal.NotGuildLeader(user, "delegate treasury access") | NEW:NotGuildLeader (adds "guild" before leader) |
| 668 | NOT | Officers may now withdraw gold and take items from the vault. |  | toggle confirmation |
| 671 | NOT | Only you may now withdraw gold and take items from the vault. |  | toggle confirmation (delegation off), not a refusal |
| 678 | NOT | <ansi fg="yellow-bold">%s</ansi> treasury: <ansi fg="gold">%d gold</ansi> |  | treasury report |
| 680 | NOT | Vault: empty (0/%d). |  | treasury report |
| 682 | NOT | Vault (%d/%d): |  | treasury report |
| 684 | NOT |   <ansi fg="black-bold">%2d.</ansi> %s |  | treasury report row |
| 688 | NOT | Treasury access: leader and officers. |  | treasury report |
| 690 | NOT | Treasury access: leader only. |  | treasury report |
| 714 | FAM | Only the guild leader can rename ranks. | refusal.NotGuildLeader(user, "rename ranks") | NEW:NotGuildLeader |
| 719 | USAGE | Usage: <ansi fg="command">guild title <member\|officer\|leader> [new name]</ansi>  (omit the name to reset). |  | usage line |
| 724 | RETAG | Name a rank: <ansi fg="white">member</ansi>, <ansi fg="white">officer</ansi>, or <ansi fg="white">leader</ansi>. |  | bad rank typed |
| 730 | NOT | You reset the <ansi fg="white">%s</ansi> rank to its default name. |  | confirmation |
| 734 | REWORD | Invalid title: %s | fmt.Sprintf(`You can't use that title: %s.`, err) | rule 3: raw Go error, no full stop; ValidRankTitle errors are the player's mistake ("a rank title must be 2-20 characters", "... may only contain letters, digits, and spaces", guilds.go:79-83) |
| 738 | NOT | Members of <ansi fg="white">%s</ansi> rank are now titled <ansi fg="cyan">%s</ansi>. |  | confirmation |
| 748 | FAM | Only officers and the leader can set the MOTD. | refusal.NotGuildOfficer(user, "set the MOTD") | NEW:NotGuildOfficer |
| 754 | NOT | You clear the guild's message of the day. |  | confirmation |
| 756 | NOT | You set the guild's message of the day. |  | confirmation |

## `internal/usercommands/help.go`

NOT 1 RETAG 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 89 | RETAG | No help found for "%s" |  | echoes a typed help topic (allowed); no full stop |
| 97 | NOT | (help topic text) |  | help; note the help/help template error path at help.go:79-82 sends err.Error() as helpTxt, a FAIL case on this same send |

## `internal/usercommands/history.go`

NOT 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 44 | NOT | (History table) |  | report |

## `internal/usercommands/hood.go`

NOT 2 RETAG 4

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 22 | RETAG | You have no lantern with a hood. |  | wording fine |
| 33 | RETAG | Your <ansi fg="item">%s</ansi> has no hood. |  | worn light: own equipment, echo allowed |
| 48 | RETAG | Your lantern is already hooded. |  | wording fine |
| 63 | NOT | You lower the hood over your lantern, and its light narrows to nothing. |  | confirmation |
| 84 | RETAG | Your lantern's hood is already open. |  | wording fine |
| 94 | NOT | You throw back the hood of your lantern, and light floods out around you. |  | confirmation |

## `internal/usercommands/inbox.go`

NOT 9 REWORD 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 22 | NOT | You have %d unread messages and %d old messages. Type inbox to view your messages. |  | inbox report (language key Inbox.UnreadMessageWithCheck) |
| 26 | NOT | You have %d unread messages and %d old messages. |  | inbox report (Inbox.UnreadMessage) |
| 33 | NOT | (mail border rule) |  | report |
| 46 | NOT | (mail/message template) |  | report |
| 48 | NOT | (mail border rule) |  | report |
| 55 | REWORD | Your pack is too full to receive the <ansi fg="item">%s</ansi>. Free some space and check your mail again. | `Your pack is too full for the <ansi fg="item">%s</ansi>. Make room and check your mail again.` | rule 4: 83 visible columns before the item name |
| 64 | NOT | (blank line) |  | report footer spacer |
| 65 | NOT | Type inbox old to read old messages. |  | report footer (Inbox.ReadOldMessages) |
| 66 | NOT | Type inbox clear to clear all messages in your inbox. |  | report footer (Inbox.ClearMessages) |
| 67 | NOT | (blank line) |  | report footer spacer |

## `internal/usercommands/inventory.go`

NARR 3 NOT 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 68 | NARR | <ansi fg="red-bold">A <ansi fg="itemname">%s</ansi> in your pack detonates! (%s)</ansi> | CategoryHitRanged | world event (spoiled grenade harms the holder); grenade blasts in throw.go ride CategoryHitRanged |
| 77 | NARR | <ansi fg="red-bold">A <ansi fg="itemname">%s</ansi> in your pack goes off!</ansi> | CategoryHitRanged | world event, condition grenade, as 68 |
| 94 | NARR | <ansi fg="yellow">%d grenade(s) have destabilized and dissolved into putrid residue.</ansi> | CategoryEquipment | world event: items in the pack change |
| 446 | NOT | (character/inventory template) |  | report |

## `internal/usercommands/jail.go`

NOT 3 RETAG 2 REWORD 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 38 | RETAG | You aren't being held anywhere. |  | wording fine |
| 42 | NOT | Your sentence is all but served. You'll be released shortly. |  | fine report |
| 45 | NOT | Your fine to walk free now is <ansi fg="gold">%d gold</ansi>. It drops the longer you sit. Pay it with <ansi fg="command">payfine</ansi>. |  | fine report (gold the player must pay) |
| 56 | RETAG | You aren't being held anywhere. |  | wording fine |
| 66 | REWORD | You can't cover the <ansi fg="gold">%d gold</ansi> fine, even counting your bank. You'll have to sit it out. | You can't pay the <ansi fg="gold">%d gold</ansi> fine, even with your bank. You must serve your time. | controller: keep the consequence; plain words; agent: rule 4: 85 visible columns before the amount; "sit it out" is an idiom |
| 84 | NOT | You count out <ansi fg="gold">%d gold</ansi> and settle your fine with the guards. |  | confirmation |

## `internal/usercommands/keyring.go`

NOT 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 166 | NOT | (Your Keyring table) |  | report |

## `internal/usercommands/kick.go`

FAM 2

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 33 | FAM | You are too spent to manage that right now. / You cannot muster the resolve for that right now. | refusal.Cost(user, res.Cost) | actions.CostRefusalText path |
| 39 | FAM | <ansi fg="red">You can't kick while focused on your work. Finish or be interrupted first.</ansi> | refusal.WhileBusy(user, "kick") | hand-written copy of busyRefusalText |

## `internal/usercommands/killstats.go`

NOT 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 127 | NOT | (kill stats table plus "Also try:" line) |  | report |

## `internal/usercommands/list.go`

NOT 2 RETAG 2 REWORD 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 41 | RETAG | You can't make out the goods well enough to deal. |  | actions.ShopSightRefusalText keeps its home; the caller sends on CategoryRefusal |
| 113 | REWORD | <ansi fg="mobname">%s</ansi> has closed up for the night and gone to bed. Come back in the morning, or make some noise nearby. | `<ansi fg="mobname">%s</ansi> has gone to bed. Come back in the morning, or make some noise.` | rule 4: 100 visible columns before the name; the keeper is away, so the name is not a sight echo |
| 118 | RETAG | Visit a merchant to list and buy objects. |  | no merchant here |
| 481 | NOT | (tables/shoplist template) |  | report |
| 482 | NOT | %s%s |  | shop help text under the table |

## `internal/usercommands/lock.go`

NOT 4 RETAG 6

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 26 | RETAG | The shipping crate is already sealed. |  | wording fine |
| 45 | RETAG | That's already locked. |  | wording fine |
| 64 | NOT | You use a key to relock the <ansi fg="container">%s</ansi>. |  | confirmation |
| 86 | NOT | You use your <ansi fg="item">%s</ansi> to lock the <ansi fg="container">%s</ansi>, and add it to your key ring for the future. |  | confirmation |
| 91 | RETAG | You do not have the key for that. |  | wording fine |
| 101 | RETAG | That's already locked. |  | wording fine |
| 120 | NOT | You use a key to relock the <ansi fg="exit">%s</ansi> lock. |  | confirmation |
| 142 | NOT | You use your <ansi fg="item">%s</ansi> to lock the <ansi fg="exit">%s</ansi> exit, and add it to your key ring for the future. |  | confirmation |
| 147 | RETAG | You do not have the key for that. |  | wording fine |
| 154 | RETAG | There is no such exit or container. |  | no echo; hidden containers already gated by discovery |

## `internal/usercommands/look.go`

NOT 10 RETAG 8

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 53 | NOT | You run your hands over your <ansi fg="item">%s</ansi>. |  | report (own gear by touch) |
| 58 | RETAG | You can't see anything! / It is too dark to see. You need light, or eyes that do not need it. |  | noSightRefusal keeps its home; the caller sends on CategoryRefusal |
| 156 | RETAG | It's too dark to see anything in that direction. |  | wording fine |
| 160 | RETAG | The %s exit is locked. |  | exit name: allowed echo |
| 168 | NOT | You peer %s. |  | report |
| 174 | NOT | It's too dark to see that way, and nothing warm moves there. |  | report of what heat sense shows |
| 178 | NOT | It's too dark to see that way, but you sense the warmth of %s. |  | report of what heat sense shows |
| 183 | NOT | You peer %s. |  | report |
| 217 | RETAG | (blank line) |  | spacer of the locked-container refusal at 218; moves with it |
| 218 | RETAG | The <ansi fg="container">%s</ansi> is locked. |  | wording fine |
| 219 | RETAG | (blank line) |  | spacer of the locked-container refusal at 218; moves with it |
| 240 | NOT | (blank line) |  | recipe report spacer |
| 241 | NOT | You can <ansi fg="command">use</ansi> the <ansi fg="container">%s</ansi> if you put the following objects inside: |  | recipe report |
| 251 | NOT | (blank line) |  | recipe report spacer |
| 254 | NOT |     <ansi fg="230">To receive 1 <ansi fg="itemname">%s</ansi>:</ansi>  |  | recipe report |
| 265 | NOT |         <ansi fg="%s">[%d/%d]</ansi> <ansi fg="itemname">%s</ansi> |  | recipe report row |
| 289 | RETAG | There is no exit in that direction. |  | wording fine |
| 541 | RETAG | You can only make out shapes here. |  | deliberate shapes-sight answer to an unresolved name (it differs from NotHereThing on purpose, per the comment at 537) |

## `internal/usercommands/loot.go`

NOT 3 RETAG 9

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 27 | RETAG | You can't see anything to loot! |  | wording fine |
| 87 | RETAG | This isn't your kill. |  | wording fine |
| 92 | RETAG | There's nothing to loot from the %s. |  | corpse name via corpseNameFor (sight-safe) |
| 115 | NOT | You take the <ansi fg="itemname">%s</ansi> from the %s. |  | confirmation |
| 120 | RETAG | You can't carry the <ansi fg="itemname">%s</ansi> - you're already overloaded! |  | wording fine (see get.go 66) |
| 133 | NOT | You take <ansi fg="gold">%d gold</ansi> from the %s. |  | confirmation |
| 171 | RETAG | There's nothing to pass. Loot passing only applies to round-robin looting. |  | wording fine (74 columns) |
| 177 | RETAG | There's nothing to pass. |  | wording fine |
| 193 | RETAG | There's no one to pass your share to. |  | wording fine |
| 205 | RETAG | There's nothing to pass. |  | wording fine |
| 221 | RETAG | You have no share to pass on the %s. |  | corpse name via corpseNameFor (sight-safe) |
| 231 | NOT | You pass your share of the %s to <ansi fg="username">%s</ansi>. |  | confirmation |

## `internal/usercommands/macros.go`

NOT 7 RETAG 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 17 | RETAG | You have no macros set. |  | controller: refusal-shaped empty report; rides the refusal category; agent: empty macro list report |
| 28 | NOT | <ansi fg="226">Your macros:</ansi> |  | report |
| 31 | NOT | (blank line) |  | report spacer |
| 33 | NOT |   <ansi fg="228">%s</ansi>: |  | report row |
| 46 | NOT |       %s) <ansi fg="command">%s</ansi> %s |  | report row |
| 49 | NOT | (blank line) |  | report spacer |
| 50 | NOT | To use a macro, type <ansi fg="command">={num}</ansi>. |  | report footer |
| 51 | NOT | Some terminals support pressing the associated F-Key (<ansi fg="228">F1</ansi>, <ansi fg="228">F2</ansi>, etc.) |  | report footer |

## `internal/usercommands/mail.go`

FAM 1 NOT 3 RETAG 6

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 95 | RETAG | You must wait a while before sending more mail. |  | mail cooldown, no number; refusal.Cooldown's "recover" wording does not fit mail |
| 118 | RETAG | You can't mail yourself. |  | wording fine |
| 120 | RETAG | No adventurer by that name has ever been recorded. |  | character-name lookup, not sight |
| 138 | RETAG | A message is required. |  | empty message at the compose prompt |
| 157 | RETAG | You aren't carrying that much gold. |  | wording fine |
| 175 | FAM | Could not find item: %s | refusal.NotCarrying(user, question.Response) | own backpack: echo allowed; the helper trims, strips tags and caps the raw answer |
| 185 | NOT | (mail/message template preview) |  | report |
| 190 | NOT | Cancelling the mail. |  | answer to the confirm prompt |
| 201 | RETAG | Your mail to <ansi fg="username">%s</ansi> could not be delivered. Nothing was sent. |  | nothing left the sender; plain words; if deliverMail returned the LoadUser error this could become Failed |
| 215 | NOT | Your mail to <ansi fg="username">%s</ansi> is on its way. |  | confirmation |

## `internal/usercommands/maul.go`

FAM 1 RETAG 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 33 | FAM | You are too spent to manage that right now. / You cannot muster the resolve for that right now. | refusal.Cost(user, res.Cost) | actions.CostRefusalText path |
| 38 | RETAG | You have no fangs to maul with. |  | wording fine |

## `internal/usercommands/mutations.go`

NOT 10

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 17 | NOT | <ansi fg="magenta">The Chrysalis has not yet reshaped you. No mutations have emerged.</ansi> |  | empty mutation list report |
| 21 | NOT | (blank line) |  | report spacer |
| 22 | NOT | <ansi fg="magenta"> .:. <ansi fg="yellow">Your Mutations</ansi> .:.</ansi> |  | report header |
| 23 | NOT | (blank line) |  | report spacer |
| 29 | NOT |   <ansi fg="yellow">%s</ansi> (%s)  <ansi fg="red">[data missing]</ansi> |  | report row; prints the internal mutation id when its spec is missing (a data fault), worth a log line instead |
| 32 | NOT |   <ansi fg="yellow">%s</ansi> <ansi fg="magenta">(%s)</ansi> |  | report row |
| 33 | NOT |     <ansi fg="white">%s</ansi> |  | report row |
| 39 | NOT |       <ansi fg="green">+ %s</ansi> |  | report row |
| 44 | NOT |       <ansi fg="red">- %s</ansi> |  | report row |
| 49 | NOT | (blank line) |  | report spacer |

## `internal/usercommands/offer.go`

RETAG 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 34 | RETAG | You can't make out the goods well enough to deal. |  | actions.ShopSightRefusalText keeps its home; the caller sends on CategoryRefusal |

## `internal/usercommands/online.go`

NOT 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 205 | NOT | (users online table) |  | report |

## `internal/usercommands/party.go`

FAM 2 NARR 14 NOT 33 RETAG 7 REWORD 4 USAGE 8

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 61 | FAM | You are not attached to a party. | refusal.NotInParty(user) | NEW:NotInParty "You aren't in a party. Try <ansi fg="command">party create</ansi>." Same situation worded 3 ways: party.go:61, report.go:38, assist.go:22 (other group) |
| 78 | RETAG | You haven't accepted an invitation to the party. |  | invited-not-joined gate before member subcommands |
| 155 | RETAG | You already have a pending party invite. Try <ansi fg="command">party accept/decline</ansi> first |  | party create while invited; missing stop is added by the end-punctuation stage |
| 157 | REWORD | You already own a party Type <ansi fg="command">party list</ansi> for more info. | `You already lead a party. Try <ansi fg="command">party list</ansi>.` | rule 4: run-on, no stop before "Type"; party create while leading |
| 159 | REWORD | You are already party of a party. | `You are already in a party.` | rule 4: broken English ("party of a party"); party create while a member |
| 166 | NOT | You started a new party! |  | confirmation |
| 203 | FAM | %s not found. | refusal.NotHereCreature(user) | rule 1: echoes the typed invitee name (a room occupant found by looking) after AimBySight; ResolveTargetActor failed |
| 207 | RETAG | You can only invite players to your party. |  | target resolved to a mob; no echo |
| 215 | RETAG | You can't invite yourself. |  | self-invite |
| 220 | RETAG | That player is already in a party. |  | invitee already in a party |
| 235 | NOT | You invited <ansi fg="username">%s</ansi> to your party. |  | confirmation (HideNames) |
| 240 | NARR | <ansi fg="username">%s</ansi> invited you to their party. Type <ansi fg="command">party accept</ansi> or <ansi fg="command">party decline</ansi> to respond. | party notice: no fitting category today; new CategoryParty, or keep System | sent to the invitee (another player) |
| 272 | NOT | You joined the party! |  | confirmation |
| 278 | NARR | <ansi fg="username">%s</ansi> joined the party! | party notice: new CategoryParty, or keep System | sent to the other members |
| 301 | NARR | <ansi fg="username">%s</ansi> declined the invitation. | party notice: new CategoryParty, or keep System | sent to the leader |
| 303 | NOT | You decline the invitation. |  | confirmation |
| 467 | NOT | (Party Members table, tables/generic) |  | report/table |
| 470 | NOT | Type <ansi fg="command">party accept/decline</ansi> to finalize your party membership. |  | hint under the party list for an invitee |
| 482 | NOT | You already have auto-attack enabled. |  | toggle state (setting is written either way) |
| 484 | NOT | You are now auto-attacking with your party. |  | toggle confirmation |
| 489 | NOT | You are no longer auto-attacking with your party. |  | toggle confirmation |
| 491 | NOT | You already have auto-attacking disabled. |  | toggle state (setting is written either way) |
| 494 | USAGE | Usage: <ansi fg="command">party autoattack [on/off]</ansi> |  | any other argument |
| 513 | NOT | You disbanded the party. |  | confirmation |
| 544 | NARR | You are now the leader of the party. | party notice: new CategoryParty, or keep System | sent to the member promoted when the leader leaves, not the actor |
| 546 | NARR | <ansi fg="username">%s</ansi> is now the leader of the party. | party notice: new CategoryParty, or keep System | sent to the other members (classifier guessed refusal) |
| 556 | NOT | You left the party. |  | confirmation |
| 565 | NOT | You left the party. |  | confirmation |
| 572 | NARR | <ansi fg="username">%s</ansi> left the party. | party notice: new CategoryParty, or keep System | sent to the other members |
| 594 | NARR | <ansi fg="username">%s</ansi> disbanded the party. | party notice: new CategoryParty, or keep System | sent to the other members |
| 599 | NARR | <ansi fg="username">%s</ansi> disbanded the party. | party notice: new CategoryParty, or keep System | sent to invitees |
| 607 | NOT | You disbanded the party. |  | confirmation |
| 624 | REWORD | %s not found. | `No one in your party is named '%s'.` | rule 4: `party kick` with no name prints " not found."; give the empty case refusal.Prompt(user, "kick", refusal.Whom, "party kick <name>"). Party member names may echo (own party) but should go through the echo sanitizer; today rest is raw |
| 633 | NARR | You were kicked from the party. | party notice: new CategoryParty, or keep System | sent to the kicked player, not the actor |
| 638 | NARR | <ansi fg="username">%s</ansi> was kicked from the party. | party notice: new CategoryParty, or keep System | sent to every remaining member, the leader included |
| 651 | REWORD | %s not found. | `No one in your party is named '%s'.` | rule 4: `party promote` with no name prints " not found."; empty case refusal.Prompt(user, "promote", refusal.Whom, "party promote <name>"); same echo note as :624 |
| 661 | NARR | You have been promoted to party leader. | party notice: new CategoryParty, or keep System | sent to the promoted member, not the actor |
| 667 | NARR | <ansi fg="username">%s</ansi> is now the party leader. | party notice: new CategoryParty, or keep System | sent to every member but the promoted one, the actor included (classifier guessed refusal) |
| 699 | USAGE | Usage: <ansi fg="command">party loot [ffa/roundrobin/leaderhold]</ansi> |  | heads the usage block (:700-:703) |
| 700 | USAGE |   <ansi fg="command">ffa</ansi>         - Free-for-all: anyone can loot corpses. |  | usage block line |
| 701 | USAGE |   <ansi fg="command">roundrobin</ansi>  - Loot rights rotate through party members. |  | usage block line |
| 702 | USAGE |   <ansi fg="command">leaderhold</ansi>  - Only the leader may loot corpses. |  | usage block line |
| 703 | NOT | The current loot mode is <ansi fg="magenta-bold">%s</ansi>. |  | report (ends the usage block) |
| 726 | NOT | You set the party loot mode to <ansi fg="magenta-bold">%s</ansi>. |  | controller (slice 4 N1): stays on System, now sent through the `user` parameter (the actor's branch of the loop over members, where `u` was the same record); the others' copy (:728) rides CategoryGroup; agent: confirmation |
| 728 | NARR | <ansi fg="username">%s</ansi> set the party loot mode to <ansi fg="magenta-bold">%s</ansi>. | party notice: new CategoryParty, or keep System | sent to the other members |
| 786 | RETAG | The party pool is empty. |  | party gold split with nothing to split |
| 793 | NOT | The party pool holds <ansi fg="yellow-bold">%d gold</ansi>. |  | report |
| 795 | NOT | Type <ansi fg="command">party gold split</ansi> to divide it among members. |  | hint after report |
| 814 | NARR | <ansi fg="magenta">(party)</ansi> <ansi fg="username">%s</ansi> says, "<ansi fg="yellow">%s</ansi>" | party channel speech: new CategoryParty (CategorySpeech normalizes and wraps differently; the line is pre-split at 80) | party chat to the other members |
| 818 | NOT | <ansi fg="magenta">(party)</ansi> You say, "<ansi fg="yellow">%s</ansi>" |  | speaker's own echo; should follow :814 if that moves |
| 832 | RETAG | You do not have <ansi fg="command">party.debug</ansi> permission. |  | permission refusal a non-admin can reach (party admin) |
| 852 | USAGE | Usage: <ansi fg="command">party admin list-npc</ansi> |  | admin usage |
| 853 | USAGE |        <ansi fg="command">party admin show <id></ansi> |  | usage block line |
| 884 | NOT | No active NPC parties. |  | admin output (party.debug); CategoryAdmin in slice 4 |
| 887 | NOT | NPC Parties: |  | admin output |
| 889 | NOT | (NPC party row: #id Leader Members HomeRoom HelpRoom) |  | admin output |
| 896 | USAGE | Usage: <ansi fg="command">party admin show <id></ansi> |  | admin usage |
| 901 | NOT | Invalid party id: %s |  | admin output (party.debug); bad id refusal, CategoryAdmin in slice 4 |
| 913 | NOT | No party with id %d. |  | admin output (party.debug); CategoryAdmin in slice 4 |
| 926 | NOT | Party #%d |  | admin output |
| 934 | NOT |   Leader: %s (player %d, room %d) |  | admin output |
| 941 | NOT |   Leader: %s (mob %d, room %d) |  | admin output |
| 949 | NOT |   Leader: (none) |  | admin output |
| 952 | NOT |   Members: |  | admin output |
| 963 | NOT |     %s (player %d, room %d)%s |  | admin output |
| 968 | NOT |     %s (mob %d, room %d)%s |  | admin output |
| 975 | NOT |   Home room: %s |  | admin output |
| 976 | NOT |   Help room: %s |  | admin output |

## `internal/usercommands/password.go`

NOT 1 RETAG 2 REWORD 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 22 | RETAG | <ansi fg="alert-5">Sorry, your password was incorrect.</ansi> |  | wrong current password |
| 43 | RETAG | <ansi fg="alert-5">Sorry, your new password and the confirmation password did not match.</ansi> |  | confirmation mismatch |
| 49 | REWORD | <ansi fg="alert-5">%s</ansi> (err.Error(): "password must be between %d and %d characters long" or "failed to hash password: %s") | `<ansi fg="alert-5">Your new password must be between %d and %d characters long.</ansi>` | rule 3: raw Go error string. SetPassword has two errors: the length one is the player's mistake (this line); the bcrypt one is a server fault and should go to refusal.Failed(user, "password change", err, "userId", user.UserId) |
| 56 | NOT | <ansi fg="alert-1">Your password has been changed!</ansi> |  | confirmation |

## `internal/usercommands/pet.go`

NOT 2 RETAG 2 REWORD 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 27 | RETAG | You have no pet to name. |  | pet name with no pet |
| 32 | RETAG | %s already has a name. |  | own pet's display name; echo allowed |
| 39 | REWORD | That name won't work: %s (err.Error() from users.ValidateActorName) | Task 3: send err.Error() (a sentence) without the "That name won't work: " prefix | controller: as companion.go:122; agent: rule 3: ValidateActorName's banned-pattern error prints the moderation pattern (`that name matched the prohibited pattern: %q`). Fix in validate_actor_name.go:40 so pet.go and renameself.go both read it |
| 47 | NOT | You name your pet: %s. |  | confirmation |
| 78 | NOT | You pet %s. |  | confirmation |

## `internal/usercommands/petition.go`

FAIL 1 NARR 1 NOT 1 RETAG 2 USAGE 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 30 | USAGE | <ansi fg="yellow">Usage: <ansi fg="cyan-bold">petition <message></ansi>: send a message to the staff (harassment, a stuck quest, anything).</ansi> |  | empty petition |
| 37 | RETAG | <ansi fg="yellow">That's a bit long. Please keep your petition under %d characters.</ansi> |  | number is a typed-length limit, not a balance value |
| 50 | RETAG | <ansi fg="yellow">You've just sent a petition. Give the staff a moment before sending another.</ansi> |  | anti-spam cooldown; names no duration |
| 62 | FAIL | <ansi fg="red">Your petition could not be filed. Please try again or find an admin.</ansi> | refusal.Failed(user, "petition", err, "userId", user.UserId) | moderation.Add error is dropped unlogged at this site; FailedText loses "try again or find an admin" |
| 70 | NOT | <ansi fg="green">Your petition has been sent to the staff. They'll review it as soon as they can.</ansi> |  | confirmation |
| 77 | NARR | <ansi fg="alert-5">[PETITION #%d]</ansi> <ansi fg="username">%s</ansi> (%s): %s <ansi fg="black-bold">(Type 'petitions' to review.)</ansi> | CategoryAdmin (slice 4) | staff alert sent to every non-user-role player online |

## `internal/usercommands/petitions.go`

NOT 13 USAGE 2

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 26 | NOT | <ansi fg="green">No open petitions.</ansi> |  | admin command (petitions is admin only, usercommands.go:165); report |
| 29 | NOT | <ansi fg="cyan-bold">%d open petition(s):</ansi> |  | admin report |
| 31 | NOT |   <ansi fg="yellow">#%d</ansi> <ansi fg="username">%s</ansi>: %s |  | admin report |
| 38 | NOT | <ansi fg="green">No petitions on record.</ansi> |  | admin report |
| 42 | NOT |   <ansi fg="yellow">#%d</ansi> [%s] <ansi fg="username">%s</ansi>: %s |  | admin report |
| 48 | USAGE | <ansi fg="yellow">Usage: petitions resolve <id> [note]</ansi> |  | admin usage |
| 53 | NOT | <ansi fg="red">That is not a valid petition id.</ansi> |  | admin output; CategoryAdmin in slice 4 |
| 58 | NOT | <ansi fg="red">%s</ansi> (err.Error() from moderation.Resolve: "petition %d already resolved" / "no petition with id %d") |  | admin output; raw Go error strings, admin-facing only; CategoryAdmin in slice 4 |
| 61 | NOT | <ansi fg="green">Petition #%d marked resolved.</ansi> |  | admin confirmation |
| 67 | USAGE | <ansi fg="yellow">Usage: petitions \| petitions all \| petitions <id> \| petitions resolve <id> [note]</ansi> |  | admin usage |
| 72 | NOT | <ansi fg="red">No petition with that id.</ansi> |  | admin output; CategoryAdmin in slice 4 |
| 75 | NOT | <ansi fg="cyan-bold">Petition #%d</ansi> [%s] |  | admin report |
| 76 | NOT |   From: <ansi fg="username">%s</ansi>  When: %s  Room: %d (%s) |  | admin report |
| 77 | NOT |   Message: %s |  | admin report |
| 79 | NOT |   Resolved by <ansi fg="username">%s</ansi>: %s |  | admin report |

## `internal/usercommands/picklock.go`

FAM 1 NOT 20 RETAG 6 REWORD 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 39 | RETAG | You need <ansi fg="item">lockpicks</ansi> to pick a lock. |  | no lockpicks carried |
| 46 | FAM | You wanna pock a lock? Specify where it is. | refusal.Prompt(user, "pick", refusal.What, "picklock <exit or container>") | rule 4 too (typo "pock", slang "wanna"); a bare prompt slice 2 missed |
| 51 | REWORD | The shipping crate has no lock to pick. It's sealed by the caravan's binding, not by mechanism. | `The shipping crate has no lock. The caravan's binding seals it.` | rule 4: 95 visible columns, over 80 |
| 75 | RETAG | There is no lock there. |  | container without a lock; no echo |
| 80 | RETAG | It's already unlocked. |  | container |
| 98 | RETAG | There is no lock there. |  | exit without a lock |
| 103 | RETAG | It's already unlocked. |  | exit |
| 114 | RETAG | There is no such exit or container. |  | no echo; covers exits too, so NotHereThing does not fit |
| 143 | NOT | (blank line) |  | spacer before the auto-pick outcome |
| 144 | NOT | Your keyring already has this lock on it. |  | outcome: the lock is then picked from the keyring |
| 148 | NOT | (blank line) |  | spacer |
| 149 | NOT | <ansi fg="yellow-bold">***</ansi> <ansi fg="green-bold">You Successfully picked the lock!</ansi> <ansi fg="yellow-bold">***</ansi> |  | outcome |
| 150 | NOT | <ansi fg="yellow-bold">***</ansi> <ansi fg="green-bold">You can automatically pick this lock any time as long as you carry <ansi fg="item">lockpicks</ansi>!</ansi> <ansi fg="yellow-bold">***</ansi> |  | outcome (over 80 columns, out of refusal scope) |
| 151 | NOT | (blank line) |  | spacer |
| 185 | NOT | (lock sequence table, tables/lockpicking) |  | minigame render |
| 200 | NOT | Type '<ansi fg="command">help picklock</ansi>' for more information on picking locks. |  | help pointer after quitting the minigame |
| 226 | NOT | (blank line) |  | spacer |
| 227 | NOT | <ansi fg="yellow-bold">***</ansi> <ansi fg="red-bold">Oops! Your <ansi fg="item">%s</ansi> break off in the lock, resetting the lock. You'll have to start all over.</ansi> <ansi fg="yellow-bold">***</ansi> |  | outcome of a wrong pin |
| 228 | NOT | (blank line) |  | spacer |
| 234 | NOT | <ansi fg="yellow-bold">***</ansi> <ansi fg="alert-5">A trap was triggered!</ansi> <ansi fg="yellow-bold">***</ansi> |  | outcome |
| 235 | NOT | (blank line) |  | spacer |
| 248 | NOT | (blank line) |  | spacer |
| 249 | NOT | <ansi fg="green-bold">A satisfying *click* tells you that you're making progress...</ansi> |  | outcome |
| 255 | NOT | (lock sequence table, tables/lockpicking) |  | minigame render |
| 264 | NOT | (blank line) |  | spacer |
| 265 | NOT | <ansi fg="yellow-bold">***</ansi> <ansi fg="green-bold">You Successfully picked the lock!</ansi> <ansi fg="yellow-bold">***</ansi> |  | outcome |
| 266 | NOT | <ansi fg="yellow-bold">***</ansi> <ansi fg="green-bold">You can automatically pick this lock any time as long as you carry <ansi fg="item">lockpicks</ansi>!</ansi> <ansi fg="yellow-bold">***</ansi> |  | outcome |
| 267 | NOT | (blank line) |  | spacer |

## `internal/usercommands/pounce.go`

FAM 1 RETAG 2

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 36 | FAM | (actions.CostRefusalText(res.Cost)) | refusal.Cost(user, res.Cost) | planned Cost helper |
| 41 | RETAG | You can't pounce from a clinch. |  | grappling |
| 45 | RETAG | You aren't built to pounce. |  | not a predator |

## `internal/usercommands/print.go`

NOT 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 58 | NOT | (printline ruler) |  | layout tool output |

## `internal/usercommands/put.go`

FAM 1 NOT 4 RETAG 2 REWORD 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 36 | REWORD | The shipping crate is sealed; the caravan only lets through what they put in themselves. | `The shipping crate is sealed. Only the caravan can put things in it.` | rule 4: 88 visible columns, over 80 |
| 55 | FAM | No container found by that name | refusal.NotHereThing(user) | room container is a thing found by looking |
| 62 | NOT | (blank line) |  | spacer above :63; moves with it |
| 63 | RETAG | The <ansi fg="container">%s</ansi> is locked. |  | echoes the matched container name; put has no sight gate today, so this echo is a #454 risk if one is added |
| 64 | NOT | (blank line) |  | spacer below :63; moves with it |
| 100 | RETAG | You don't have that much gold. |  | same literal in 8 places repo-wide; a NotEnoughGold helper candidate across groups |
| 113 | NOT | You place <ansi fg="gold">%d gold</ansi> into the <ansi fg="container">%s</ansi> |  | confirmation |
| 128 | NOT | You place your <ansi fg="itemname">%s</ansi> into the <ansi fg="container">%s</ansi> |  | confirmation |

## `internal/usercommands/pvp.go`

NOT 5

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 15 | NOT | (blank line) |  | spacer |
| 17 | NOT | PVP is <ansi fg="alert-5">disabled</ansi> on this server. You cannot fight other players. |  | pvp info command report (classifier guessed refusal) |
| 19 | NOT | PVP is <ansi fg="green-bold">enabled</ansi> on this server. You can fight other players anywhere. |  | report |
| 21 | NOT | PVP is <ansi fg="yellow">limited</ansi> on this server. You can fight other players in places labeled with: <ansi fg="11" bg="52"> ☠ PK Area ☠ </ansi>. |  | report |
| 23 | NOT | (blank line) |  | spacer |

## `internal/usercommands/quests.go`

NOT 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 109 | NOT | (quest list, character/quests template) |  | report |

## `internal/usercommands/quit.go`

FAM 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 13 | FAM | You're too busy to quit right now! | refusal.WhileFighting(user, "quit") | gate is IsInCombat |

## `internal/usercommands/rake.go`

FAM 1 RETAG 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 33 | FAM | (actions.CostRefusalText(res.Cost)) | refusal.Cost(user, res.Cost) | planned Cost helper |
| 38 | RETAG | You have no claws to rake with. |  | not clawed |

## `internal/usercommands/rally.go`

FAM 2 RETAG 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 22 | FAM | (actions.CostRefusalText(result.Cost)) | refusal.Cost(user, result.Cost) | planned Cost helper |
| 27 | FAM | <ansi fg="red">You can't rally while focused on your work. Finish or be interrupted first.</ansi> | refusal.WhileBusy(user, "rally") | hand-copied busyRefusalText; rally.go is in TestSpecialMoveFilesKeepOnlyNoticesOnSystem |
| 31 | RETAG | You're already rallied. Save it for when it matters. |  | already active |

## `internal/usercommands/read.go`

NOT 2

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 54 | NOT | You look at <ansi fg="item">%s</ansi>... |  | outcome |
| 65 | NOT | (the item's text or description) |  | item text |

## `internal/usercommands/remove.go`

FAM 3 NOT 2 RETAG 1 REWORD 2

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 32 | FAM | <ansi fg="red">You can't change equipment while focused on your work. Finish or be interrupted first.</ansi> | refusal.WhileBusy(user, "change equipment") | remove all, via busyRefusalText |
| 36 | REWORD | You can't seem to remove your <ansi fg="item">%s</ansi>... It's <ansi fg="red-bold">CURSED!</ansi> | user.Character.CursedRefusal(it), sent on CategoryRefusal: `Your %s is cursed and prevents you from removing it.` | rule 4: shouting caps "CURSED!"; spec section 1 routes curse refusals through the existing CursedRefusal (wear_slot.go:36), which wear already uses |
| 55 | FAM | <ansi fg="red">You can't change equipment while focused on your work. Finish or be interrupted first.</ansi> | refusal.WhileBusy(user, "change equipment") | remove one item, via busyRefusalText |
| 57 | FAM | You don't appear to be using a "%s". | refusal.NotWearing(user, rest) | equipment the player names; echo allowed |
| 59 | REWORD | You can't seem to remove your <ansi fg="item">%s</ansi>... It's <ansi fg="red-bold">CURSED!</ansi> | user.Character.CursedRefusal(result.Item), sent on CategoryRefusal: `Your %s is cursed and prevents you from removing it.` | same as :36 |
| 62 | NOT | It's <ansi fg="red-bold">CURSED</ansi> but luckily your <ansi fg="skillname">enchant</ansi> skill level allows you to remove it. |  | outcome note before the removal confirmation |
| 67 | NOT | You remove your <ansi fg="item">%s</ansi> and return it to your backpack. |  | confirmation |
| 81 | RETAG | You can't seem to remove your <ansi fg="item">%s</ansi>. |  | reached only when RemoveFromBody fails (server side, remove_equip.go:170); a Failed candidate if the owner prefers, but no err exists to log |

## `internal/usercommands/renameself.go`

NOT 3 RETAG 2 REWORD 2

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 23 | NOT | (help/rename template) |  | help |
| 28 | RETAG | That's already your name. |  | same name typed |
| 38 | RETAG | You renamed yourself recently. You can rename again on %s. |  | %s is a calendar date and clock time (2006-01-02 at 15:04), not a duration |
| 46 | REWORD | That name won't work: %s (err.Error() from users.ValidateActorName) | Task 3: as pet.go:39 | controller: as companion.go:122; agent: rule 3: same banned-pattern leak as pet.go:39; one fix at validate_actor_name.go:40 |
| 70 | NOT | Aborted. |  | confirmation of the player's "no" |
| 77 | REWORD | Rename failed: %s (err.Error(): "name was just claimed") | `Someone else just took that name.` | rule 3: Go error string and "failed"; RenameUser's only error is the name race, the player's lost race not a server fault |
| 89 | NOT | The world ripples briefly. You are now known as <ansi fg="username">%s</ansi>. |  | confirmation |

## `internal/usercommands/reply.go`

RETAG 2

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 18 | RETAG | No one has whispered to you recently. |  | nothing to reply to |
| 29 | RETAG | That person is no longer online. |  | last whisperer logged off |

## `internal/usercommands/report.go`

FAM 1 NARR 2 NOT 5 RETAG 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 38 | FAM | You are not in a party. | refusal.NotInParty(user) | NEW:NotInParty, see party.go:61 |
| 47 | NARR | <ansi fg="magenta">(party)</ansi> <ansi fg="username">%s</ansi> reports: %s | party channel: new CategoryParty (same as party chat, party.go:814) | sent to the other members |
| 52 | NOT | <ansi fg="magenta">(party)</ansi> You report: %s |  | own echo |
| 58 | NOT | You check yourself: %s |  | report |
| 73 | RETAG | They are no longer here. |  | target vanished between AimBySight and resolve; no echo |
| 83 | NOT | You check yourself: %s |  | report |
| 89 | NARR | <ansi fg="whisper"><ansi fg="username">%s</ansi> reports to you: %s</ansi> | CategoryWhisper | sent to the target player; styled and hidden as a whisper |
| 93 | NOT | <ansi fg="whisper">You report to <ansi fg="username">%s</ansi>: %s</ansi> |  | own echo; should follow :89 to CategoryWhisper if that moves |
| 101 | NOT | You report: %s |  | own echo; the room line already rides CategoryMobEmote |

## `internal/usercommands/salvage.go`

FAIL 1 FAM 4 RETAG 5 REWORD 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 26 | FAM | <ansi fg="command">salvage <item></ansi> - Break down an item for materials. | refusal.Prompt(user, "salvage", refusal.What, "salvage <item>") | empty argument; same shape as sell/throw/show prompts from slice 2 |
| 32 | FAM | <ansi fg="red">You're already busy working on something.</ansi> | refusal.WhileBusy(user, "salvage") | IsFree gate |
| 42 | REWORD | <ansi fg="red">You don't have "%s" and there's no corpse of that name here.</ansi> | <ansi fg="red">You aren't carrying that, and you don't see a corpse like that here.</ansi> | rule 1: echoes a typed name that may name a room corpse (found by looking); echo is also raw, untrimmed and unescaped |
| 64 | RETAG | <ansi fg="red">You need to remove that before you can salvage it.</ansi> |  | equipped item; own equipment |
| 94 | RETAG | <ansi fg="red">You can't find anything useful to salvage from that.</ansi> |  | refused before the activity starts |
| 124 | FAM | <ansi fg="red">You're already busy working on something.</ansi> | refusal.WhileBusy(user, "salvage") | TransitionToSalvaging refused: activity machine busy |
| 142 | RETAG | <ansi fg="red">You can't bring yourself to salvage that.</ansi> |  | player corpse |
| 148 | FAIL | <ansi fg="red">Something is wrong with that corpse.</ansi> | refusal.Failed(user, "salvage", fmt.Errorf("corpse mob %d has no spec", corpse.MobId), "userId", user.UserId) | mob spec missing is a data fault; nothing is logged today |
| 154 | RETAG | <ansi fg="red">There's nothing useful to recover here.</ansi> |  | corpse has no salvage returns |
| 161 | RETAG | That corpse still has loot on it. Pick it clean first. |  | classifier said other; it is a refusal |
| 184 | FAM | <ansi fg="red">You're already busy working on something.</ansi> | refusal.WhileBusy(user, "salvage") | corpse path, TransitionToSalvaging refused |

## `internal/usercommands/save.go`

NOT 2

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 12 | NOT | Saving... |  | confirmation |
| 14 | NOT | done. |  | confirmation |

## `internal/usercommands/sell.go`

FAM 3 NOT 4 RETAG 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 48 | FAM | Specify what you want to sell. (e.g. <ansi fg="command">sell all iron-ore</ansi>) | refusal.Prompt(user, "sell", refusal.What, "sell <item>") | controller: usage is sell <item>; agent: "sell all." with no item name |
| 61 | FAM | Specify what you want to sell. (e.g. <ansi fg="command">sell all iron-ore</ansi>) | refusal.Prompt(user, "sell", refusal.What, "sell <item>") | controller: as :48; agent: "sell all " with blank name |
| 72 | FAM | Specify what you want to sell. (e.g. <ansi fg="command">sell all iron-ore</ansi>) | refusal.Prompt(user, "sell", refusal.What, "sell <item>") | controller: as :48; agent: bare "sell all" |
| 90 | RETAG | There's no merchant here. |  | no echo |
| 101 | NOT | You sell a <ansi fg="itemname">%s</ansi> for <ansi fg="gold">%d gold</ansi>. |  | confirmation |
| 107 | NOT | You sell %d items for <ansi fg="gold">%d gold</ansi>. |  | confirmation |
| 112 | NOT | You sell %d <ansi fg="itemname">%s</ansi> for <ansi fg="gold">%d gold</ansi>. |  | confirmation |
| 117 | NOT | <ansi fg="yellow">Sold %d before the merchant ran out of gold.</ansi> |  | outcome report after a partial sale |

## `internal/usercommands/set.go`

NOT 83 RETAG 12 REWORD 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 75 | NOT | <ansi fg="yellow-bold">description:</ansi> |  | set status report |
| 76 | NOT | <ansi fg="yellow">%s</ansi> |  | set status report (description) |
| 77 | NOT |  |  | set status report spacer |
| 79 | NOT | <ansi fg="yellow-bold">ScreenReader:</ansi>  |  | set status report |
| 81 | NOT | <ansi fg="green">ON</ansi> |  | toggle state |
| 83 | NOT | <ansi fg="red">OFF</ansi> |  | toggle state |
| 85 | NOT |  |  | set status report spacer |
| 87 | NOT | <ansi fg="yellow-bold">charset:</ansi>  |  | set status report |
| 89 | NOT | <ansi fg="red">ASCII</ansi> (legacy client mode) |  | toggle state |
| 91 | NOT | <ansi fg="green">UTF-8</ansi> |  | toggle state |
| 93 | NOT |  |  | set status report spacer |
| 104 | NOT | <ansi fg="yellow-bold">prompt: </ansi>  |  | set status report |
| 105 | NOT | %s |  | current prompt |
| 106 | NOT |  |  | set status report spacer |
| 112 | NOT | <ansi fg="yellow-bold">fprompt:</ansi>  |  | set status report |
| 113 | NOT | %s |  | current fight prompt |
| 114 | NOT |  |  | set status report spacer |
| 120 | NOT | <ansi fg="yellow-bold">wimpy:</ansi>  |  | set status report |
| 121 | NOT | %d%% |  | setting value the player chose, in a report |
| 122 | NOT |  |  | set status report spacer |
| 124 | NOT | <ansi fg="yellow-bold">submission:</ansi>  |  | set status report |
| 125 | NOT | <ansi fg="green">%s</ansi> |  | set status report |
| 126 | NOT |  |  | set status report spacer |
| 128 | NOT | <ansi fg="yellow-bold">surrender:</ansi>  |  | set status report |
| 129 | NOT | <ansi fg="green">%s</ansi> |  | set status report |
| 130 | NOT |  |  | set status report spacer |
| 132 | NOT | <ansi fg="yellow-bold">combatverbosity:</ansi>  |  | set status report |
| 133 | NOT | <ansi fg="green">%s</ansi> |  | set status report |
| 134 | NOT |  |  | set status report spacer |
| 136 | NOT | See: <ansi fg="command">help set</ansi> |  | help pointer closing the report |
| 145 | NOT | <ansi fg="yellow-bold">%s:</ansi>  |  | set status report (bool setting label) |
| 146 | NOT | <ansi fg="green">ON</ansi> / <ansi fg="red">OFF</ansi> |  | toggle state |
| 147 | NOT |  |  | set status report spacer |
| 160 | NOT | Description set. Look at yourself to confirm. |  | confirmation |
| 177 | NOT | %s toggled <ansi fg="red">ON</ansi>. |  | toggle confirmation |
| 180 | NOT | %s toggled <ansi fg="red">OFF</ansi>. |  | toggle confirmation |
| 195 | NOT | ScreenReader mode toggled <ansi fg="red">OFF</ansi>. |  | toggle confirmation |
| 197 | NOT | ScreenReader mode toggled <ansi fg="red">ON</ansi>. |  | toggle confirmation |
| 217 | NOT | Your current prompt:\n |  | report |
| 218 | NOT | %s |  | current prompt |
| 219 | NOT | Type <ansi fg="command">help set-prompt</ansi> for more info on customizing prompts. |  | help pointer in a report |
| 238 | NOT | Prompt set. |  | confirmation |
| 252 | NOT | <ansi fg="yellow-bold">Fight Prompt:</ansi> |  | report |
| 254 | NOT |   <ansi fg="8">[system default - toggle-driven]</ansi> |  | report |
| 256 | NOT |   %s |  | report (custom fight prompt) |
| 258 | NOT |  |  | report spacer |
| 259 | NOT | <ansi fg="yellow">Toggle Settings</ansi> <ansi fg="8">(set fprompt tog <name> to change):</ansi> |  | report |
| 275 | NOT |   <ansi fg="magenta">%-14s</ansi> %s %s |  | toggle table row |
| 277 | NOT |  |  | report spacer |
| 278 | NOT | Type <ansi fg="command">help set-prompt</ansi> for more info on customizing prompts. |  | help pointer in a report |
| 302 | RETAG | Unknown toggle. Valid options: bars, pos, target, targethealth, targetpos, tank |  | 79 columns, fits |
| 317 | NOT | Fight prompt: %s <ansi fg="green">[ON]</ansi> |  | toggle confirmation |
| 319 | NOT | Fight prompt: %s <ansi fg="red">[OFF]</ansi> |  | toggle confirmation |
| 339 | NOT | fprompt set. |  | confirmation |
| 356 | NOT | Your current wimpy:\n |  | report |
| 357 | NOT | %d%% |  | setting value, in a report |
| 358 | NOT | Type <ansi fg="command">help wimpy</ansi> to learn about the wimpy setting. |  | help pointer in a report |
| 371 | NOT | wimpy set. |  | confirmation |
| 390 | NOT | Charset mode set to <ansi fg="red">ASCII</ansi>. |  | toggle confirmation |
| 391 | NOT | Box-drawing characters will be converted to ASCII equivalents. |  | toggle confirmation |
| 392 | NOT | Use <ansi fg="command">set charset</ansi> again to switch back to UTF-8. |  | toggle confirmation |
| 394 | NOT | Charset mode set to <ansi fg="green">UTF-8</ansi>. |  | toggle confirmation |
| 395 | NOT | Full Unicode box-drawing characters will be displayed. |  | toggle confirmation |
| 408 | NOT | Your submission policy: <ansi fg="command">%s</ansi> |  | report |
| 410 | NOT | Set with: <ansi fg="command">set submission &lt;mercy\|subdue\|cripple\|lethal&gt;</ansi> |  | usage hint inside a report (no-argument view) |
| 411 | NOT | See <ansi fg="command">help submission</ansi> for details. |  | help pointer in a report |
| 417 | RETAG | Unknown submission policy: <ansi fg="red">%s</ansi> |  | echoes the player's own typed option |
| 418 | RETAG | Valid policies: mercy, subdue, cripple, lethal. |  | second line of the 417 refusal; moves with it |
| 428 | NOT | <ansi fg="yellow-bold">WARNING:</ansi> Setting your submission policy to <ansi fg="red-bold">lethal</ansi> means your successful submissions will KILL opponents, players included, with full deprogression and corpse loot. |  | confirmation gate (run again to confirm) |
| 431 | NOT | If you're sure, run the command again to confirm. |  | confirmation gate |
| 441 | NOT | Submission policy set to <ansi fg="command">%s</ansi>. |  | confirmation |
| 447 | NOT | Your surrender policy: <ansi fg="command">%s</ansi> |  | report |
| 449 | NOT | Set with: <ansi fg="command">set surrender &lt;never\|always\|auto-tap-below &lt;N&gt;&gt;</ansi> |  | usage hint inside a report |
| 450 | NOT | See <ansi fg="command">help surrender</ansi> for details. |  | help pointer in a report |
| 458 | RETAG | Unknown surrender policy: <ansi fg="red">%s</ansi> |  | echoes the player's own typed option |
| 459 | RETAG | Valid policies: never, always, auto-tap-below &lt;1-100&gt;. |  | second line of the 458 refusal; the range is input syntax, not a balance number |
| 464 | NOT | Surrender policy set to <ansi fg="command">%s</ansi>. |  | confirmation |
| 474 | NOT | Your arrest policy: <ansi fg="command">%s</ansi> |  | report |
| 475 | NOT | Set with: <ansi fg="command">set arrest &lt;surrender\|resist&gt;</ansi> |  | usage hint inside a report |
| 476 | NOT | See <ansi fg="command">help arrest</ansi> for details. |  | help pointer in a report |
| 481 | RETAG | Unknown arrest policy: <ansi fg="red">%s</ansi> |  | echoes the player's own typed option |
| 482 | RETAG | Valid policies: surrender, resist. |  | second line of the 481 refusal |
| 486 | NOT | Arrest policy set to <ansi fg="command">%s</ansi>. |  | confirmation |
| 492 | NOT | Line width is currently %d. |  | report |
| 497 | RETAG | Line width must be a number between 40 and 240. |  | input range, not a balance number |
| 501 | NOT | Line width set to %d. |  | confirmation |
| 513 | NOT | Combat verbosity is currently <ansi fg="yellow-bold">%s</ansi>. |  | report |
| 515 | NOT | Options: <ansi fg="command">full</ansi> (everything), <ansi fg="command">medium</ansi> (hits only), <ansi fg="command">light</ansi> (round summaries). See <ansi fg="command">help combatverbosity</ansi>. |  | report (no-argument view) |
| 520 | RETAG | Combat verbosity must be <ansi fg="command">full</ansi>, <ansi fg="command">medium</ansi>, or <ansi fg="command">light</ansi>. |  | bad option |
| 528 | NOT | Combat verbosity set to <ansi fg="yellow-bold">%s</ansi>. |  | confirmation |
| 555 | RETAG | Invalid macro number supplied. |  | bad input |
| 570 | NOT | Macro <ansi fg="command">=%d</ansi> deleted. |  | confirmation |
| 575 | RETAG | Macros are limited to 10 commands. |  | input limit, not a balance number |
| 585 | RETAG | You cannot reference macros inside of a macro |  | bad input; missing full stop is taste, not a rule |
| 594 | REWORD | There was a problem setting your macro. | Your macro has no commands in it. | rule 3/4: reads as a server fault, but the only cause is the player's input (nothing but ";" separators) |
| 600 | NOT | Macro set. Type <ansi fg="command">=%d</ansi> or (if your terminal supports it) press <ansi fg="command">F%d</ansi> to use it. |  | confirmation |

## `internal/usercommands/setdesc.go`

NOT 6

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 17 | NOT | Your current description:\n%s |  | report |
| 18 | NOT |  |  | report spacer |
| 19 | NOT | To set a new description: <ansi fg="command">setdesc <your description></ansi> |  | help inside the no-argument report |
| 20 | NOT | To clear: <ansi fg="command">setdesc clear</ansi> |  | help inside the no-argument report |
| 26 | NOT | Your description has been cleared. |  | confirmation |
| 35 | NOT | Your description has been set to:\n%s |  | confirmation |

## `internal/usercommands/sethome.go`

NOT 5 RETAG 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 29 | NOT | <ansi fg="yellow-bold">Current home:</ansi> %s |  | report |
| 32 | NOT |  |  | report spacer |
| 33 | NOT | <ansi fg="yellow-bold">Available locations:</ansi> |  | report |
| 39 | NOT |   <ansi fg="command">sethome %s</ansi> - %s%s |  | report row |
| 47 | RETAG | <ansi fg="red">Unknown location "%s".</ansi> Type <ansi fg="command">sethome</ansi> to see options. |  | echoes the player's typed option (a known name) |
| 55 | NOT | <ansi fg="green">Home set to %s.</ansi> You will return here when you die. |  | confirmation |

## `internal/usercommands/share.go`

FAM 1 RETAG 3

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 20 | RETAG | You can only share in a party. |  | classifier said other; it is a refusal |
| 37 | RETAG | You can't share a negative amount of gold. |  | bad input |
| 42 | RETAG | You don't have that much gold to share. |  | not enough gold |
| 83 | FAM | You can share gold by typing <ansi fg="command">share [amt] gold</ansi>? | refusal.Prompt(user, "share", refusal.HowMuchGold, "share <amount> gold") | wrong or missing argument; today's line ends in a stray "?" |

## `internal/usercommands/shoot.go`

FAM 3 NARR 1 NOT 3 RETAG 13 REWORD 1 USAGE 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 65 | RETAG | You can't see well enough to aim. |  | actions.ShotBlindedLine; pre-fire sight gate, no echo |
| 67 | RETAG | It is too dark to aim. You need light, or eyes that do not need it. |  | actions.ShotTooDarkLine; pre-fire sight gate, no echo |
| 76 | RETAG | You can't shoot yourself. |  | reached only at full sight |
| 84 | RETAG | <ansi fg="username">%s</ansi> is in your party! |  | own party member, reached only at full sight |
| 93 | RETAG | %s (room.CanPvp error: "PVP is disabled." / "This is not a PVP area." / "You need more experience before engaging in PVP." / "Fighting is not allowed here.") |  | player-facing refusal text carried in an error, not a server fault |
| 98 | RETAG | %s (target room's CanPvp error, same strings as line 93) |  | player-facing refusal text carried in an error |
| 108 | FAM | %s (actions.CostRefusalText: "You are too spent to manage that right now." / "You cannot muster the resolve for that right now.") | refusal.Cost(user, result.Cost) | cost refusal |
| 114 | FAM | <ansi fg="red">You can't shoot while focused on your work. Finish or be interrupted first.</ansi> | refusal.WhileBusy(user, "shoot") | busyRefusalText written out by hand |
| 118 | RETAG | You don't have a ranged weapon equipped. |  | own equipment |
| 131 | REWORD | Your <ansi fg="itemname">%s</ansi> is empty. Check that you still have ammunition, and the breath to ready it. | Your <ansi fg="itemname">%s</ansi> is empty. You may be out of ammunition, or too winded. | rule 4: 82 visible columns before the weapon name is added; keeps the two causes the comment insists on |
| 135 | USAGE | Syntax: <ansi fg="command">shoot &lt;target&gt; [direction]</ansi> |  | syntax line (BadSyntax: no arguments) |
| 139 | RETAG | The <ansi fg="exit">%s</ansi> exit is locked. |  | exits may echo |
| 143 | FAM | Could not find your target. | refusal.NotHereCreature(user) | FireResult.NoTarget: typed name matched nothing, or the target is hidden; covers cross-room shots too, where "here" still reads acceptably |
| 147 | RETAG | You can't see well enough to aim. |  | actions.ShotBlindedLine (FireResult.Blinded) |
| 151 | RETAG | It is too dark to aim. You need light, or eyes that do not need it. |  | actions.ShotTooDarkLine (FireResult.TooDarkToAim) |
| 155 | RETAG | <ansi fg="mobname">%s</ansi> is your friend! |  | shooter's own charmed companion, past the full-sight gate |
| 159 | RETAG | You can't attack <ansi fg="mobname">%s</ansi>. |  | past the full-sight gate |
| 267 | NOT | That was the last of your <ansi fg="itemname">%s</ansi>. |  | post-shot chambering report |
| 274 | NOT | You reach for more <ansi fg="item">%s</ansi> and find none. Your <ansi fg="itemname">%s</ansi> is empty. |  | post-shot chambering outcome |
| 288 | RETAG | You cannot work another round into your <ansi fg="itemname">%s</ansi> just now. It stays empty. |  | controller: the reload refused; agent: post-shot chambering outcome (the shot already fired) |
| 491 | NOT | %s (combat.ChannelDefenceShortageText, to the player target) |  | defender's resource note during the shot, an outcome |
| 531 | NARR | Someone is attacking you. You cannot steady your aim, so the shot is weak. | the shooter's outcome line's category (shooterLine.Cat; CategoryHitRanged when that line is silent) | controller (slice 4 N2): plan Step 3: the shot's own outcome category; agent: aimedWhileEngagedText: outcome explanation after the shot |

## `internal/usercommands/show.go`

NARR 2 NOT 2

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 72 | NOT | You show the <ansi fg="item">%s</ansi> to <ansi fg="username">%s</ansi>. |  | confirmation |
| 78 | NARR | <ansi fg="username">%s</ansi> shows you their <ansi fg="item">%s</ansi>. | CategoryMobEmote | sent to the other player; the room line for the same act rides CategoryMobEmote |
| 83 | NARR | \n%s\n (the shown item's long description) | CategoryMobEmote | sent to the other player with line 78; keep the pair on one category |
| 97 | NOT | You show the <ansi fg="item">%s</ansi> to <ansi fg="mobname">%s</ansi>. |  | confirmation |

## `internal/usercommands/skill.cast.go`

FAM 3 RETAG 10 REWORD 3

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 33 | RETAG | <ansi fg="red">You can't cast. You are dead.</ansi> |  | dead caster gate |
| 42 | RETAG | <ansi fg="red">You have no spellcasting skill.</ansi> |  | no casting skill at all |
| 65 | REWORD | <ansi fg="red">No spell found for "%s". Use the spell name or an alias (e.g. <ansi fg="cyan-bold">mm</ansi>, <ansi fg="cyan-bold">heal</ansi>). Type <ansi fg="cyan-bold">spells</ansi> to list what you know.</ansi> | <ansi fg="red">There is no spell called "%s".</ansi> Try <ansi fg="command">spells</ansi>. | rule 4: about 100 visible columns plus the echo; echo of rest is raw (should go through the package echo trim) |
| 85 | RETAG | <ansi fg="red">You haven't learned the spell "%s".</ansi> |  | spell names may echo |
| 92 | RETAG | <ansi fg="red">You have no manifestation skill.</ansi> |  | school skill missing |
| 97 | RETAG | <ansi fg="red">You have no spellcasting skill.</ansi> |  | school skill missing |
| 113 | FAM | You are too busy to cast right now. | refusal.WhileBusy(user, "cast") | IsFree gate |
| 122 | RETAG | <ansi fg="red">You don't have the conviction to cast %s.</ansi> |  | no number shown |
| 142 | RETAG | <ansi fg="red">%s requires a %s in your inventory.</ansi> |  | classifier said other; component tag check before the cast starts |
| 163 | REWORD | <ansi fg="red">%s requires a %s in your inventory.</ansi> (fallback component name "item #%d") | <ansi fg="red">%s requires a %s in your inventory.</ansi> | rule 3: when the item spec has no name the line reads "item #123"; fall back to "component" instead of the id |
| 250 | RETAG | <ansi fg="red">There is nothing here for that spell to strike.</ansi> |  | area harm spell with no targets |
| 252 | FAM | <ansi fg="red">You need a target to cast that spell.</ansi> | refusal.NoTarget(user, "cast "+spellInfo.SpellId) | reads "You have no target. Try cast <spellid> <name>." |
| 305 | FAM | You're already busy with something else. | refusal.WhileBusy(user, "cast") | TransitionToCasting refused (defensive) |
| 322 | RETAG | Something about this place prevents you from recalling. |  | note: sent after the cast-start lines already narrated (12b), then the cast is undone |
| 337 | REWORD | You reach for the Veil, but there is no anchor to pull you. Set one first with <ansi fg="command">cast fold-anchor</ansi>. | You have no anchor to return to. Try <ansi fg="command">cast fold-anchor</ansi>. | rule 4: about 96 visible columns |
| 342 | RETAG | You are already standing on your anchor. |  | same note as 322 (sent after the cast-start lines) |

## `internal/usercommands/skill.disenchant.go`

FAM 1 NOT 2 RETAG 2

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 23 | FAM | <ansi fg="command">disenchant <item></ansi>: strip a Chrysalis enchantment from an item. | refusal.Prompt(user, "disenchant", refusal.What, "disenchant <item>") | empty argument; same shape as the slice 2 prompts |
| 29 | RETAG | <ansi fg="red">You need to be at an enchanting circle to strip an enchantment.</ansi> |  | wrong place |
| 46 | RETAG | <ansi fg="red">That item has no Chrysalis enchantment to strip.</ansi> |  | classifier said other; it is a refusal |
| 73 | NOT | <ansi fg="magenta">You pry the Chrysalis free. It comes away screaming, a soundless wail that reverberates through your bones. The item falls silent, stripped of its living power.</ansi> |  | outcome of the player's own act |
| 76 | NOT | <ansi fg="red">A sudden emptiness floods through you. Your body aches for the connection it has lost. The withdrawal will pass... in time.</ansi> |  | outcome of the player's own act |

## `internal/usercommands/skill.map.go`

FAIL 2 FAM 1 NOT 3

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 112 | NOT | The reach of your maps is %d rooms. |  | report |
| 117 | FAM | You can only create 1 map per round. | refusal.Cooldown(user, "draw another map") | rule 2: names a round count |
| 140 | NOT | %s (premade zone map template) |  | report |
| 153 | FAIL | No map found (or an error occured)" | refusal.Failed(user, "map", errors.New("no mapper for zone "+zone), "userId", user.UserId) | server fault; today's text has a typo and a stray quote; drop the mudlog.Error, Failed logs |
| 225 | FAIL | No map found (or an error occured)" | refusal.Failed(user, "map", err, "userId", user.UserId) | template error; drop the mudlog.Error, Failed logs |
| 229 | NOT | %s (rendered map) |  | report |

## `internal/usercommands/skill.skullduggery.plant.go`

RETAG 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 101 | RETAG | You can't plant items on other players. |  | resolved at full sight (AimBySight) |

## `internal/usercommands/skill.skullduggery.shadow.go`

FAM 1 RETAG 3

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 33 | RETAG | You aren't advanced enough at skullduggery for that. |  | spec counts 6 copies codebase-wide; only 1 in this group, so no NEW helper proposed here |
| 44 | RETAG | You aren't shadowing anyone. |  | shadow stop with nothing to stop |
| 70 | RETAG | You can't shadow yourself. |  | self target |
| 88 | FAM | You need to wait %s before shadowing again. | refusal.Cooldown(user, "shadow anyone") | rule 2 and 4: result.Reason is "%d rounds remaining", so it reads "wait 3 rounds remaining before shadowing again" |

## `internal/usercommands/skill.skullduggery.sneak.go`

FAM 3 NARR 1 NOT 2 RETAG 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 33 | FAM | You are busy with something else. | refusal.WhileBusy(user, "sneak") | IsFree gate |
| 43 | FAM | You need to wait %d more rounds before you can try that again. | refusal.Cooldown(user, "sneak") | rule 2: raw round count |
| 51 | FAM | %s (actions.CostRefusalText) | refusal.Cost(user, result.Cost) | cost refusal |
| 57 | RETAG | You're already hidden! |  | wrong state |
| 67 | NOT | You let the shroud fall away and slip into the shadows on your own. |  | outcome |
| 77 | NARR | You try to blend into the shadows but <ansi fg="mobname">%s</ansi> notices you. | CategoryMobEmote or CategoryEmote | controller (slice 4 N2): by the spotter's kind, as move.go's sneaking-arrival spotted line (slice 4 Task 5); the guard cannot see it (opens with You); agent: actions.SpottedLine: failed attempt outcome |
| 96 | NOT | You slip into the shadows. |  | outcome |

## `internal/usercommands/skill.skullduggery.steal.go`

RETAG 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 91 | RETAG | You can't steal from other players. |  | resolved at full sight (AimBySight) |

## `internal/usercommands/skill_move_defence.go`

NOT 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 81 | NOT | %s (combat.ChannelDefenceShortageText, to the defender) |  | defender's resource note during a move, an outcome |

## `internal/usercommands/skills.go`

NOT 3

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 37 | NOT | %s (character/skills template) |  | report |
| 40 | NOT | <ansi fg="yellow">Cooldown Tracking:</ansi> |  | report (skills extra) |
| 42 | NOT |  <ansi fg="yellow">%s</ansi>: <ansi fg="red">%d</ansi> |  | report row (skills extra) |

## `internal/usercommands/sort.go`

NOT 1 RETAG 2

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 23 | RETAG | You don't have a component bag or bandolier equipped. |  | own equipment |
| 38 | RETAG | No items found to sort. |  | nothing to do |
| 54 | NOT | <ansi fg="green">Sorted %s.</ansi> |  | confirmation |

## `internal/usercommands/spells.go`

NOT 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 139 | NOT | %s (spells table) |  | report |

## `internal/usercommands/stand.go`

NOT 1 RETAG 4 REWORD 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 33 | RETAG | You're already standing. |  | wrong state |
| 37 | RETAG | You're locked in a grapple. You'll need to break free first. |  | wrong state |
| 41 | RETAG | You can't stand up right now. |  | position gate |
| 82 | REWORD | You're too winded to get up. Catch your breath first.[ Your gear is holding %s of your stamina in reserve, so you have less to draw on than you might expect.] | You're too winded to get up. Catch your breath first. / second send when reserved: Your gear holds %s of your stamina in reserve. | rule 4: with the reservation clause the one line runs past 140 columns; split it and shorten the clause (band stays a word, no number) |
| 90 | RETAG | Something prevents you from standing. |  | server-fault path (Position transition error, logged by mudlog.Warn), but the player is not shown internal text; could become refusal.Failed if the plan wants every fault there |
| 102 | NOT | You struggle to your feet! |  | outcome |

## `internal/usercommands/start.go`

NOT 9

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 40 | NOT |  |  | onboarding spacer |
| 41 | NOT |   <ansi fg="magenta">*** Your form takes shape on the world of Gaius ***</ansi>%s |  | onboarding narration to the new player only |
| 61 | NOT | You will be known as <ansi fg="yellow-bold">%s</ansi>!%s |  | confirmation |
| 96 | NOT | %s<ansi fg="magenta-bold">One quick thing before you go.</ansi> Gaius is still being built, and your eyes are new... type <ansi fg="command">bug</ansi>... type <ansi fg="command">suggest</ansi>. We read every one.%s |  | onboarding message |
| 135 | NOT | %sNew to DOGMud but not to MUDs? Type <ansi fg="command">help dogmud</ansi> for what works differently here.%s |  | onboarding help pointer |
| 169 | NOT | <ansi fg="magenta">Suddenly, a vortex appears before you, drawing you in before you have any chance to react!</ansi>%s |  | onboarding narration seen only by the new player |
| 236 | NOT | <ansi fg="magenta">The grey takes you, gentle as sleep...</ansi>%s |  | onboarding narration seen only by the new player |
| 261 | NOT | <ansi fg="magenta">The pool is behind you before you ever saw it. You step out already changed, into a city that does not know your name.</ansi>%s |  | classifier said refusal?; it is onboarding narration (veteran route), not a refusal |
| 266 | NOT | New to Gaius after all? Type <ansi fg="command">tutorial</ansi> to be taken to the newcomers' pool. |  | onboarding help pointer |

## `internal/usercommands/stash.go`

NOT 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 36 | NOT | You stash the <ansi fg="itemname">%s</ansi>. To get it back, try <ansi fg="command">get %s from stash</ansi> |  | confirmation |

## `internal/usercommands/status.go`

NOT 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 14 | NOT | %s (character/status template) |  | report |

## `internal/usercommands/storage.go`

FAM 3 NOT 13 RETAG 5 REWORD 2 USAGE 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 25 | RETAG | You are not at a storage location. |  | wrong place (sent with a trailing CRLF) |
| 33 | REWORD | Maybe you meant to use the <ansi fg="command">put</ansi> command to <ansi fg="command">put</ansi> something into the <ansi fg="container">%s</ansi>? | Try <ansi fg="command">put <item> <container></ansi>. | controller: names no container; agent: rule 1: names a room container the player never typed, from room.Containers with no sight check (and a hidden container too); line also runs past 80 with the name |
| 54 | NOT | %s (character/storage template) |  | report |
| 67 | USAGE | Try <ansi fg="command">help storage</ansi> for more information about storage. |  | answer to bad syntax (not add/remove); usage pointer |
| 125 | NOT | You placed %d item(s) into storage. |  | confirmation |
| 161 | NOT | You placed the <ansi fg="itemname">%s</ansi> into storage. |  | confirmation |
| 163 | NOT | You placed %d <ansi fg="itemname">%s</ansi> into storage. |  | confirmation |
| 189 | NOT | You only had %d to deposit. |  | partial outcome report |
| 204 | NOT | You placed the <ansi fg="itemname">%s</ansi> into storage. |  | confirmation |
| 206 | NOT | You placed %d <ansi fg="itemname">%s</ansi> into storage. |  | confirmation |
| 219 | RETAG | You can't carry the rest. |  | carry refusal (remove all) |
| 226 | NOT | You retrieved %d item(s) from storage. |  | confirmation |
| 247 | RETAG | You can't carry the rest. |  | carry refusal (remove all <name>) |
| 255 | FAM | You don't have a %s in storage. | refusal.NotInStorage(user, itemName) | NEW helper (see 269, 284); echo allowed (the player's own storage), today the echo is raw and "a %s" breaks on vowels |
| 257 | NOT | You removed the <ansi fg="itemname">%s</ansi> from storage. |  | confirmation |
| 259 | NOT | You retrieved %d <ansi fg="itemname">%s</ansi> from storage. |  | confirmation |
| 269 | FAM | You don't have that item in storage. | refusal.NotInStorage(user, "") | NEW:NotInStorage "You don't have '<typed>' in storage. Try storage." (empty typed reads "that"); slot number past the end |
| 282 | NOT | You can only retrieve %d from storage. |  | partial outcome report |
| 284 | FAM | You don't have a %s in storage. | refusal.NotInStorage(user, itemName) | NEW helper (see 269) |
| 290 | RETAG | You can't carry that! |  | carry refusal |
| 292 | RETAG | You can't carry the rest. |  | carry refusal (remove N) |
| 299 | NOT | You retrieved %d <ansi fg="itemname">%s</ansi> from storage. |  | confirmation |
| 366 | REWORD | The keepers will not take the <ansi fg="itemname">%s</ansi>: it was reported stolen, and not long ago. | The keepers won't take the <ansi fg="itemname">%s</ansi>: it was reported stolen. | controller: wording; agent: rule 4: 74 visible columns before the item name, so most names run past 80 |
| 407 | NOT | You removed the <ansi fg="itemname">%s</ansi> from storage. |  | confirmation |

## `internal/usercommands/stow.go`

NOT 2 RETAG 3

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 18 | RETAG | There is nowhere to stow anything here. |  | wrong place |
| 46 | RETAG | Storage is full. |  | storage.go says "Your storage is full." on CategoryRefusal; wording left alone |
| 48 | RETAG | You have no crafting components to stow. |  | nothing to do |
| 50 | NOT | You stow %d component(s); storage filled up before the rest. |  | partial outcome report |
| 52 | NOT | You stow %d crafting component(s). |  | confirmation |

## `internal/usercommands/suggest.go`

FAIL 3 NOT 1 USAGE 2

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 22 | USAGE | <ansi fg="yellow">Usage: <ansi fg="cyan-bold">suggest <your idea></ansi> |  | empty argument |
| 23 | USAGE | <ansi fg="yellow">Use this command to share ideas and suggestions for improving the game.</ansi> |  | second line of the usage block |
| 29 | FAIL | <ansi fg="red">Could not save suggestion. Please notify an admin.</ansi> | refusal.Failed(user, "suggestion", err, "userId", user.UserId) | MkdirAll error is dropped today, nothing logged |
| 36 | FAIL | <ansi fg="red">Could not save suggestion. Please notify an admin.</ansi> | refusal.Failed(user, "suggestion", err, "userId", user.UserId) | OpenFile error dropped today |
| 51 | FAIL | <ansi fg="red">Could not save suggestion. Please notify an admin.</ansi> | refusal.Failed(user, "suggestion", err, "userId", user.UserId) | WriteString error dropped today |
| 55 | NOT | <ansi fg="green">Suggestion submitted. Thank you for your feedback!</ansi> |  | confirmation |

## `internal/usercommands/suicide.go`

RETAG 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 19 | RETAG | You're already dead! |  | wrong state |

## `internal/usercommands/talk.go`

REWORD 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 195 | REWORD | You are carrying too much to take the <ansi fg="itemname">%s</ansi>. Put something down, then ask again. | You can't carry the <ansi fg="itemname">%s</ansi>. Drop something, then ask again. | rule 4: 75 visible columns before the item name; dialogue item refused before it transfers |

## `internal/usercommands/target.go`

NARR 1 NOT 5 RETAG 9

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 45 | RETAG | You're not in combat. Use <ansi fg="command">attack</ansi> to initiate combat. |  | wrong state, with a command hint |
| 53 | RETAG | You can't switch targets right now. |  | casting or disengaging |
| 79 | RETAG | You can't target yourself! |  | self target |
| 94 | RETAG | You're already targeting them! |  | player target already engaged |
| 99 | RETAG | You're already targeting them! |  | mob target already engaged |
| 117 | RETAG | <ansi fg="mobname">%s</ansi> is someone's companion! |  | past AimBySight full sight, and HideNames by sight |
| 122 | RETAG | You can't attack <ansi fg="mobname">%s</ansi>. |  | past AimBySight full sight, and HideNames by sight |
| 139 | RETAG | %s (room.CanPvp error text) |  | player-facing refusal text carried in an error |
| 146 | RETAG | <ansi fg="username">%s</ansi> is in your party! |  | own party member |
| 182 | NOT | You turn your attention to <ansi fg="mobname">%s</ansi>! |  | confirmation |
| 188 | NOT | You turn your attention to <ansi fg="username">%s</ansi>! |  | confirmation |
| 225 | NOT | You shift your focus to <ansi fg="mobname">%s</ansi>! |  | outcome (successful switch roll) |
| 236 | NOT | You shift your focus to <ansi fg="username">%s</ansi>! |  | outcome (successful switch roll) |
| 243 | NARR | <ansi fg="username">%s</ansi> shifts focus to you! | CategoryMobEmote | sent to the targeted player; the room line for the same switch rides CategoryMobEmote; check TestNarrationSitesMatchViewpointAudit fingerprint when moving |
| 257 | NOT | You try to reposition but can't break away from your current opponent! |  | failed switch roll, an outcome |

## `internal/usercommands/taunt.go`

FAM 2 NOT 1 RETAG 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 31 | FAM | %s (actions.CostRefusalText) | refusal.Cost(user, result.Cost) | cost refusal |
| 37 | FAM | <ansi fg="red">You can't taunt while focused on your work. Finish or be interrupted first.</ansi> | refusal.WhileBusy(user, "taunt") | safety net after the staging pre-reject |
| 42 | RETAG | You cannot make a sound. |  | silenced |
| 160 | NOT | %s (combat.ChannelDefenceShortageText, to the defender) |  | defender's resource note, an outcome |

## `internal/usercommands/throttle.go`

FAM 1 RETAG 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 45 | FAM | %s (actions.CostRefusalText) | refusal.Cost(user, res.Cost) | cost refusal |
| 50 | RETAG | You have no fangs to throttle with. |  | missing body part |

## `internal/usercommands/throw.go`

FAM 1 NARR 1 RETAG 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 220 | RETAG | <ansi fg="itemname">%s</ansi> isn't something you can throw. |  | own carried item |
| 246 | FAM | %s (actions.CostRefusalText) | refusal.Cost(user, cost) | cost refusal (admitThrowCost result) |
| 276 | NARR | <ansi fg="red-bold">You pull out the <ansi fg="itemname">%s</ansi> and it crumbles apart in your hand. It's gone bad!</ansi> | CategoryHitRanged | the spec's named example (throw.go:275, now 276); outcome of the throw (item consumed); the throw's other lines ride CategoryHitRanged |

## `internal/usercommands/title.go`

NOT 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 130 | NOT | %s (character/title template) |  | report |

## `internal/usercommands/trip.go`

FAM 2 RETAG 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 38 | FAM | %s (actions.CostRefusalText) | refusal.Cost(user, res.Cost) | cost refusal |
| 44 | FAM | <ansi fg="red">You can't trip someone while focused on your work. Finish or be interrupted first.</ansi> | refusal.WhileBusy(user, "trip someone") | safety net after the staging pre-reject (CraftingVerb "trip someone") |
| 58 | RETAG | %s is already on the ground. |  | target staged by stageSpecialMoveTarget (sight-gated) |

## `internal/usercommands/tutorial.go`

NOT 1 RETAG 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 23 | RETAG | The newcomers' coulee is for those just arriving. You are past that now. |  | classifier said other; it is a refusal |
| 28 | NOT | A familiar pull takes you back toward the glowing pool... |  | outcome (teleport) |

## `internal/usercommands/unban.go`

NOT 4 USAGE 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 20 | USAGE | <ansi fg="yellow">Usage: unban <name>  \|  unban ip <ip></ansi> |  | staff command |
| 28 | NOT | <ansi fg="alert-5">Unban FAILED to save:</ansi> %s is STILL banned. %s |  | staff-only moderation output; the error text is shown on purpose |
| 31 | NOT | <ansi fg="green">IP unbanned:</ansi> %s |  | staff confirmation |
| 36 | NOT | <ansi fg="alert-5">Unban FAILED to save:</ansi> <ansi fg="username">%s</ansi> is STILL banned. %s |  | staff-only moderation output; the error text is shown on purpose |
| 39 | NOT | <ansi fg="green"><ansi fg="username">%s</ansi> has been unbanned.</ansi> |  | staff confirmation |

## `internal/usercommands/unlock.go`

FAM 1 NOT 4 RETAG 4

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 40 | RETAG | That's not locked. |  | classifier said other; it is a refusal |
| 59 | NOT | You use a key to unlock the <ansi fg="container">%s</ansi>. |  | confirmation |
| 81 | NOT | You use your <ansi fg="item">%s</ansi> to unlock the <ansi fg="container">%s</ansi>, and add it to your key ring for the future. |  | confirmation |
| 86 | RETAG | You do not have the key for that. Maybe you could <ansi fg="command">picklock</ansi> the lock. |  | 68 columns |
| 96 | RETAG | That's not locked. |  | exit path |
| 115 | NOT | You use a key to unlock the <ansi fg="exit">%s</ansi> lock. |  | confirmation |
| 137 | NOT | You use your <ansi fg="item">%s</ansi> to unlock the <ansi fg="exit">%s</ansi> exit, and add it to your key ring for the future. |  | confirmation |
| 142 | RETAG | You do not have the key for that. Maybe you could <ansi fg="command">picklock</ansi> the lock. |  | exit path |
| 149 | FAM | There is no such exit or container. | refusal.NotHereThing(user) | typed name matched no exit or container; one of the "isn't here" variants |

## `internal/usercommands/use.go`

NOT 5 RETAG 5 REWORD 2

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 35 | RETAG |  |  | spacer line of the 36 refusal; moves with it |
| 36 | REWORD | The <ansi fg="container">%s</ansi> is locked. | That is locked. | rule 1: echoes a room container's name, resolved by FindContainerByName with no sight check |
| 37 | RETAG |  |  | spacer line of the 36 refusal; moves with it |
| 44 | RETAG |  |  | spacer line of the 45 refusal; moves with it |
| 45 | REWORD | The <ansi fg="container">%s</ansi> seems to be missing something. | That seems to be missing something. | rule 1: echoes a room container's name with no sight check |
| 46 | RETAG |  |  | spacer line of the 45 refusal; moves with it |
| 63 | NOT |  |  | spacer of the 64 outcome |
| 64 | NOT | The <ansi fg="container">%s</ansi> produces a <ansi fg="itemname">%s</ansi>! |  | outcome |
| 65 | NOT |  |  | spacer of the 64 outcome |
| 85 | RETAG | You can't use <ansi fg="itemname">%s</ansi>. |  | own carried item |
| 92 | NOT | You use the <ansi fg="itemname">%s</ansi>. |  | confirmation |
| 103 | NOT | %s (item spec OnUseUserText) |  | authored use outcome for the player |

## `internal/usercommands/usercommands.go`

FAM 1 NARR 1 NOT 1 RETAG 3

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 414 | NARR | <ansi fg="cyan">You lose your concentration as you flee!</ansi> | CategorySpellDisruption | the cast breaking is a spell disruption; the room line beside it rides CategorySpellDisruption |
| 430 | FAM | <ansi fg="cyan">You are holding <ansi fg="cyan-bold">%d/%d</ansi> folds. Type <ansi fg="cyan-bold">cancel</ansi> to stop.</ansi> | refusal.WhileBusy(user, "do that") | controller: the casting gate is the busy situation; cancel is its way out; agent: rule 2: shows raw fold counts in a refusal (command blocked while casting); hint moved to the command style |
| 466 | NOT | <ansi fg="8">You are no longer AFK.</ansi> |  | state notice |
| 472 | RETAG | <ansi fg="alert-5">You must change your password before doing anything else.</ansi> |  | command gate |
| 473 | RETAG | <ansi fg="yellow">Type <ansi fg="yellow-bold">password</ansi> to set a new password.</ansi> |  | second line of the 472 refusal |
| 485 | RETAG | You can't do that while you're dead. Hold on; you'll be pulled back to safety. |  | dead command gate |

## `internal/usercommands/warcry.go`

FAM 2 RETAG 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 22 | FAM | %s (actions.CostRefusalText) | refusal.Cost(user, result.Cost) | cost refusal |
| 27 | FAM | <ansi fg="red">You can't muster a warcry while focused on your work. Finish or be interrupted first.</ansi> | refusal.WhileBusy(user, "muster a warcry") | busyRefusalText written out by hand |
| 32 | RETAG | Your warcry still echoes. You can't shout it louder. |  | already active |

## `internal/usercommands/whisper.go`

FAM 1 RETAG 1

| Line | Ruling | Today | Proposal | Note |
|---|---|---|---|---|
| 26 | FAM | You need to specify a message. | refusal.Prompt(user, "whisper", refusal.What, "whisper <name> <message>") | name given, no message |
| 34 | RETAG | You can't find anyone by that name. |  | global lookup by character name (not sight), no echo |
