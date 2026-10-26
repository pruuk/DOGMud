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
  `AcquireShieldSwap` (`shield_swap`), `AcquirePlant` (`plant`).
- `AcquireItem(user *users.UserRecord, itm items.Item, reason AcquireReason)
  bool`: for a player. Nil user returns false.
- `AcquireItemForChar(char *characters.Character, userId int, mobInstanceId
  int, itm items.Item, reason AcquireReason) bool`: for a route holding a bare
  character, or an `actions.Actor` (pass `GetUserId()` and
  `GetMobInstanceId()`). It serves mob receivers too; the event keeps
  whichever id the caller passed.

## Using it correctly

- A false return means `StoreItem` refused (carried weight past twice carry
  capacity, or an invalid item). Nothing was stored and nothing was queued;
  the caller keeps its own refusal handling (a message, a rollback, a drop to
  the floor).
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

`acquire_test.go` (the seam itself), `actions/transfer_gain_test.go` (the
transfer helpers), and one test per formerly silent route:
`questengine/bridge_acquire_test.go`, `hooks/quest_reward_acquire_test.go`,
`hooks/shield_swap_acquire_test.go`,
`behaviortree/actions_quest_acquire_test.go`,
`usercommands/acquire_routes_test.go`, `actions/acquire_routes_test.go`.
