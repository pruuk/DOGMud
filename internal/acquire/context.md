# acquire: the one way an item is acquired

## What it is

The acquire seam (#375 phase 3a). Every route that puts an item into a
character's carried inventory as a gain (get, buy, loot, give, a quest
reward, a craft, a theft, and so on) calls this package instead of
`Character.StoreItem` directly. The seam stores the item and, only when the
store lands, queues exactly one `events.ItemOwnership{Gained: true, Reason}`.
That event is what fires quest `item_gain` triggers and item `QuestToken`
grants (`hooks.CheckItemQuests`), refreshes the GMCP inventory, and so on, so
a route that skipped it used to be invisible to quests.

It is a leaf package (imports only `characters`, `events`, `items`, `users`)
rather than part of `internal/actions` because `actions` imports
`internal/questengine`, and the quest bridge's `give_item`
(`questengine.GameBridge.GiveItem`) has to acquire too.

## Surface

- `type AcquireReason string`: how the item arrived. It rides on
  `events.ItemOwnership.Reason`; losses carry `""`. Constants, one per route:
  `AcquireGet` (`get`), `AcquireGive` (`give`), `AcquireBuy` (`buy`),
  `AcquireLoot` (`loot`), `AcquireForage` (`forage`), `AcquireHarvest`
  (`harvest`, no route uses it yet), `AcquireSteal` (`steal`),
  `AcquireSearch` (`search_find`), `AcquireStorage` (`storage`),
  `AcquireVault` (`vault`), `AcquireAuction` (`auction`), `AcquireTopic`
  (`topic_give`), `AcquireCraft` (`craft`), `AcquireSalvage` (`salvage`),
  `AcquireReward` (`reward`), `AcquireQuestGive` (`quest_give`),
  `AcquireTreeGive` (`tree_give`), `AcquireMail` (`mail`),
  `AcquireShieldSwap` (`shield_swap`), `AcquirePlant` (`plant`). An
  `item_gain` trigger's `reason:` filter names these words (#375 phase 7).
  The loader's list is `quests.TriggerGainReasons`, since `quests` cannot
  import this package; `TestGainReasonsMatchTheAcquireSeam` in
  `internal/guards` fails if the two differ, so a new reason goes in both, and
  in `GAIN_REASONS` in the quest editor (`quests.js`).
- `AcquireItem(user *users.UserRecord, itm items.Item, reason AcquireReason)
  bool`: for a player. Nil user returns false.
- `AcquireItemForChar(char *characters.Character, userId int, mobInstanceId
  int, itm items.Item, reason AcquireReason) bool`: for a route holding a bare
  character, or an `actions.Actor` (pass `GetUserId()` and
  `GetMobInstanceId()`). It serves mob receivers too; the event keeps
  whichever id the caller passed. It is also the one place a copy is marked
  quest-given (#940, owner ruling 2026-11-22): for `AcquireTopic` and
  `AcquireQuestGive` it sets `items.Item.QuestGiven` on the copy before
  storing it, so the stored copy and the gain event both carry the mark. Such
  a copy fades away whenever it leaves its holder
  (`actions.VanishQuestGiven`). `AcquireReward` is not marked: a reward is
  paid once and no NPC replaces it. So is the item a hand-in topic
  (`kind: handin`) gives: the dialogue path acquires it with `AcquireReward`
  (`PlayerState.GiveRewardItem`), while an offer or info topic's item uses
  `AcquireTopic` and is marked.
- `MarksQuestGiven(reason AcquireReason) bool`: whether `AcquireItemForChar`
  marks an item acquired for that reason.
- `AcquireOrDrop(char, userId, mobInstanceId, itm, reason) bool` and
  `AcquireItemOrDrop(user, itm, reason) bool` (#696): for an item that already
  exists for the receiver (made, bought, foraged, salvaged, earned, given by a
  tree). When it is too much to carry it falls in the receiver's room
  (`Room.AddItem`, as a drop does) and a player reads `TooMuchToCarryLine`;
  no gain event, since it was never held. A quest hand-out reason marks the
  copy here too (#940), so one that falls is the same quest gear it would be
  in hand.

## Using it correctly

- A false return means `StoreItem` refused (carried weight past twice carry
  capacity, or an invalid item). Nothing was stored and nothing was queued;
  the caller keeps its own refusal handling (a message, a rollback, a drop to
  the floor). **No route may ignore it** (#696: a crafted dagger vanished
  after its materials were spent): a route with nothing to roll back uses
  `AcquireOrDrop`. `internal/guards/acquire_result_guard_test.go` fails on a
  bare or `_ =` call to `AcquireItem` or `AcquireItemForChar`. A plant checks
  the target takes the item before the thief lets go, and the thief keeps it
  otherwise.
- Do not also queue an `ItemOwnership{Gained: true}` at the call site: the
  seam already did, and a second one fires quest triggers twice.
- Do not hand-fire an item's `QuestToken` after acquiring it:
  `CheckItemQuests` grants it from the gain event
  (`hooks/quest_reward_acquire_test.go` pins exactly once).
- If a later step can undo the store (the guild vault's `TakeItem` failing),
  queue the matching `ItemOwnership{Gained: false}` when you undo it.
- Moves that are not acquisitions (unequip back to the pack, a rollback to the
  giver, mob-internal inventory shuffles) still call `StoreItem`; phase 3a
  Task 3 adds a guard with an allowlist of them.

## Callers

`actions/transfer.go` (`TransferItemToBackpack` with `AcquireGet`;
`TransferItemBetweenChars` with the caller's reason: `give` from
`GiveItemToChar`, `tree_give` from the behaviour tree's `return_item`), and
the routes in buy, craft, forage, plant, salvage, search_bauble and steal
(`internal/actions`), `usercommands` get, loot, storage, guild, mail and
craft, `hooks` quest rewards, timed craft and the offhand-break replacement,
`behaviortree` give_item and give_item_multiple, `conversation` topic give,
`questengine` bridge give_item, `modules/auctions`, and
`modules/aicompanion` loot.

## Tests

`acquire_test.go` (the seam itself), `acquire_or_drop_test.go` (an item too
much to carry falls, #696), `quest_given_test.go` (only the two quest
hand-out reasons mark, a reward does not, #940), `actions/transfer_gain_test.go` (the
transfer helpers), and one test per formerly silent route:
`questengine/bridge_acquire_test.go`, `hooks/quest_reward_acquire_test.go`,
`hooks/shield_swap_acquire_test.go`,
`behaviortree/actions_quest_acquire_test.go`,
`usercommands/acquire_routes_test.go`, `actions/acquire_routes_test.go`.
