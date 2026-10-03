# aicompanion Module Context

## Purpose

`modules/aicompanion` drives **bonded companions**
(`characters.CompanionBonded`): persistent companions that talk, remember and
change mood through a language model (OpenAI chat completions with a strict
JSON schema). They ride on the existing companion system, so they appear with
their owner at login, leave at logout, follow, fight with the normal mob
combat AI, and progress skills by use exactly like any other companion.

The model never touches game state. The only things it can cause are
ordinary `say` and `emote` mob commands (phase 1). Later phases add more
ordinary mob commands; the rule stays the same.

Roadmap and phase plan: `docs/aicompanion/`.

## Files

- **aicompanion.go**: module registration, `controller`, the mind cache
  (`getMind`), config/profile loading, save callback, the per-owner and passer-by
  counters. The key, endpoint, server budget and breaker are
  `internal/apiframework`'s (`apiKey`, `baseURL` read `apiframework.Server()`,
  the snapshot `onNewRound` and `onLoad` refresh with `RefreshServer`;
  `endpoint` is a test seam). Budget and breaker calls go through `fw()`,
  the shared `apiframework.Books` on a server; in tests (`isolateBooks`, set
  by `frameworkForTests`) each module gets its own, so a call one test left
  in flight never lands in another's figures. `onLoad` lends the relay to the framework
  (`apiframework.SetRelay(relayFor{m})`), logs a refused custom endpoint,
  and warns with the names of settings still read from the old
  `Modules.aicompanion` place (`ServerSettings.Legacy`).
- **runtime.go**: the round loop (`sync`): attach/detach, session greeting,
  farewell during `quit`, fall and recovery, initiative after quiet spells,
  mood decay; `dispatch`, `applyResult` (speech, memory, facts, promises,
  bounded opinion change), the decision trace, the fallback path.
- **listeners.go**: `say`, emotes (`events.Emote`), gifts (`GiftAccepted`,
  and `GoldGiven` for coin, `onGoldGiven`, which names the real giver),
  attacks (`PlayerAttackedMob`), healing (`events.Healed`) and the `ask`
  hook; an owner speaking to the companion interrupts an errand. They record what the
  companion perceived and queue stimuli; they never call the model. The
  per-companion halves (`hearSaid`, `seeEmote`, `hearAsked`) write
  nothing into the mind before consent while still queueing the stimulus,
  so dispatch answers with set lines. The same holds for every deed with a person's
  name in it (`mayRemember`, meeting.go): a gift, gold, an attack,
  healing, a witnessed crime, being called back, an errand interrupted,
  a party change, the first meeting (`firstMet`; agreeing later writes it
  then, `keepFirstMeeting`), a fall, a fight's end, her owner falling,
  the walk back and reaching them, parting ways, asking to leave, and a
  core memory (`recordCore`); the stimulus is still queued and the
  owner's own rules (a gift's warmth, an attack's cost) still apply. Anything a passer-by aims at her (speech to
  her, `ask`, an emote, a gift, healing) is heard and remembered as usual
  but queued only if `strangerMayAsk` passes (their daily allowance,
  then what passers-by together may spend of this owner's companion,
  then a per-companion cooldown that trying spends, passed to the
  cooldown as `N real seconds`, which the period parser rounds up to
  whole rounds), called once
  per thing, just before the push. Their stimuli carry `AskerUserId`.
  With strangers off she still hears them and answers with set lines,
  paced by the same cooldown, but `strangerMayPrompt` stops any call they
  would prompt: dispatch falls back, and a talk with passers-by alone is
  not summed up. Strangers are off when the owner said
  `companion-ai strangers off` (`bondRecord.StrangersOff`), on when they
  said `on` (`StrangersOn`), and otherwise off on the owner's own key and
  on for the server's (`strangersOffOn`, by route; `strangersOff` asks
  for the route live). dispatch and the summary ask again with the route
  the call really carries, after `applyRoute`.
- **scene.go**: `buildScene`, the structured, locally scored picture of what
  the companion can see and carry, with the refs ("t2", "p1", "w1") the
  model uses in actions; item and NPC classification; novelty from the
  interaction memory.
- **actions.go**: `performAction` validates a proposed verb and ref against
  the scene and the live world, applies the loot arrangement, answers
  `look_at` and `consider` itself (the same information a player gets), or
  issues one ordinary mob command; `verifyPending` judges the outcome from
  what changed. A `get` of a household's bauble is refused before any
  command is issued ("it belongs to the household here"):
  `actions.GetItemFromFloor` would refuse it anyway, silently, and she
  would keep trying. A `remove` of a cursed item is refused the same way
  (slice 5a): `actions.CursedHolds(&mob.Character, t.Item)` is asked before
  the command is issued, and a holding curse returns `actionOutcome{Refused:
  "it will not come off"}` rather than issuing a `remove` the shared body
  would refuse anyway.
- **autonomy.go**: `perceive` (runs each round: settles the last action,
  notices new rooms and new things, offers quiet moments to act),
  `handleIdle` (owns the idle tick: first aid, idle gestures), impressions of
  NPCs and places. `noticeGold` matches purse growth against the
  `GoldGiven` events (either may come first, so unnamed growth waits a
  round); what no event names is credited to her owner only when the
  owner is with her and no other player is, and never while she is about
  her own business. `receiveGold` credits a named giver: only the owner's
  coin warms her, a passer-by's is paced and paid like their other gifts.
- **worldmap.go**: the companion's own map (`Mind.Map`): rooms it has
  stood in, exits it has walked, a clearly marked guess at the way back,
  features and people seen, danger; the Dijkstra route finder over that map
  only; nearby places; place search; hearsay and its confirmation.
- **travel.go**: trips. `go_to`, `explore` and the automatic return walk a
  route one ordinary `go <exit>` at a time, waiting for each arrival,
  recording blocked exits, re-planning from wherever the companion ends up,
  and giving up cleanly. Movement parity 4b: `advanceTravel` quotes each step
  with `stepQuote` (`actions.QuoteMobStep(mob, exitName)`, a variable so
  tests can pin it) before issuing it. Too tired to pay: the companion
  pauses, adds one mind line (`tooTiredLine`, "You are too tired to go on,
  and stop to catch your breath.") the first time she rests on a trip (once
  per trip, not once per rest, so a companion whose steps outrun her regen
  does not fill working memory with it), and re-quotes
  next round without starting the step-timeout clock. A quote that says
  `Never` (she could not pay even fully rested) ends the trip through
  `endTooWeak`, which calls `endTravel` like any failed trip, with its mind
  line and no failure charged to the exit. Before this
  slice a step that didn't complete because it couldn't be paid for ran out
  the `stepTimeoutRounds` timer like a genuinely blocked exit and counted an
  `er.Fails++` against it, which could reach the 3-strike threshold
  (`worldmap.go`) and teach the pathfinder to avoid a perfectly good exit for
  no reason but exhaustion.
- **archetype.go**: archetype validation against the game's skills, gear
  fit, skill rank words and rank-change detection.
- **inventory.go**: supplies and shortfalls, carrying load, protected items.
- **economy.go**: `browse` (priced as `list` prices), shop and price
  memory, the money rules for buying. Since baubles slice D, `browseShops`
  also appends a living shop's listed secondhand shelf after its stock
  (`ShopInventory.ListedIndexes(shops.ShelfNow())`, shelf order, never a
  held entry) as `ware` rows with `Secondhand` set, each named with
  `items.Item.ModelName()` so no player-written bauble text reaches the
  model. `describeListing` shows those rows with no item reference, and
  `rememberShop` skips them, because a shelf row shares its `ItemId` with
  regular stock (every bauble is item 900). So she can see shelf goods but
  cannot buy them.
- **loot.go**: the module's own mob commands `companion-loot` (owner's loot
  rights only) and `companion-takeout` (unhidden, unlocked containers,
  never a merchant's chest: `actions.IsMerchantChest`, since taking from
  one is watched theft).
- **cooking.go**: `craftableHere(mob, p, room)` lists what she knows, has the
  makings for, and has the place for. Slice 5a added a fourth gate ahead of
  the recipe walk: `cannotSee(mob, room)` (perception.go, `sightOf !=
  SightFull`) empties the list in the dark, so autonomy never proposes a
  craft that would not start and the prompt never lists a recipe she cannot
  see to make. Before 5a this function had no sight test at all.
  `seedRecipes` teaches every recipe of her `crafts` her skill rank has
  reached. Idle work at a trade: `tradeDrive` (her archetype's preference
  for the skill), `bestCraft` (the trade that means most, then the highest
  `skill_minimum`), `tradeAtHand` (a calling, drive at least
  `callingDrive` 0.5, starts on her idle tick ahead of the pastime roll, at
  most once every `IdleTradeSeconds`, with her drive as the chance) and
  `startCraft` (the ordinary `craft` command). `craftableHere` mirrors all
  of `actions.InitiateCraft`'s gates (skill minimum, own components, no
  enchanting), and the command is `craftCommand`, by NAME: the engine finds
  a recipe by name, never by id, so `craft grilled-meat` found nothing and
  every companion craft quietly failed before this (`craftsAs` keeps out a
  recipe whose name would resolve to another). `craftableElsewhere` lists
what she has the makings for but not the station (prompt: RecipesAway),
and `recordRoom` puts a room's station on her map (`stationWords`), so
the model can say where it could be made and `go_to` there. Idle pastime
weights are per profile (`Profile.Pastimes`, `pastimeWeight`,
`pastimeDefaults`; 0 is never). Tests: `trade_test.go`.
- **harm.go**: `harmAllowed` and `areaHarmAllowed`, the one gate on
  everything she starts: the engine's own player harm rules
  (`mobs.CheckPlayerHarm` for a creature; `(*Room).CanPvp` plus the party
  check for a person) asked with her OWNER as the one acting. The party
  check is the module's own, from `attack`, `shoot` and the special moves:
  `actions/cast.go` makes none for a player target, so she is stricter
  with spells than a player is. Who an area harm spell lands on is the
  engine's to say when it resolves (`internal/hooks/mob_area_harm.go`,
  `mobAreaHarmTargets`), since the room may have changed while it folded:
  it spares whoever her owner could not harm. So `areaHarmAllowed`
  refuses only a spell that would land on nobody, or on someone she will
  not fight (`refusesToFight`) who is not already fighting.
- **goals.go**: goals, checks against live state, restock goals, the
  session agenda, and the model's goal proposals.
- **finds.go**: what she turns up and butchers. `baubleSearchFor` (the
  `companionai.BaubleSearchFor` seam): `CompanionBaubleChance` percent of
  her searches also roll for a bauble on her owner's behalf, only for an
  owner online, agreed, with a model to use and (with RequirePlayerKey)
  their own key; `onSearched` (what the search turned up: kept for
  `verifyPending` and put to her as a `searched` stimulus, once per room
  for the same finds and counted against `NoticeCallsPerDay` through
  `takeNotice`); `onBaubleFound` (a find worked free, by its model-safe
  name, into her pack or left on the ground when she is overloaded: a
  `found` stimulus under the same cap, and a snapshot to the owner's
  record when pocketed); `butcherable`, `butcherHere`, `salvageHits`,
  `salvageCommand`: the `salvage` verb (owner-driven, like `loot`, only a
  body her owner could loot that is picked clean, aimed at that body) and
  the `butcher` pastime (game only: groups animal and rodent, per profile,
  Hal and Mara; under ask-first, never under leave-it). The idle `salvage`
  pastime is aimed the same way now: never at a body her owner could not
  loot, and never at the second of two alike killed in the same round,
  since the engine's `salvage <mob>:<round>` takes the first
  (`salvageHits`). Two failed tries in a room and both pastimes leave
  bodies there alone (`lastInteraction` fails). Tests:
  `finds_test.go`; engine side `internal/actions/companion_bauble_test.go`.
- **combat_style.go**: what makes each companion fight differently on top
  of the engine's scripted AI. The blow by blow is the mob template's
  `behavior_archetype`, one `companion_*` tree each in
  `_datafiles/world/dogmud/behaviors/archetypes` (archer, guardian,
  healer, skirmisher, battlemage, brawler; no flee, no idle, no packmate
  events: those are the module's). This file adds what no tree can see,
  her owner: `tendOwner` (profile `mend_below`, `ward_owner`: a healer
  mends and wards the person she travels with, since tree healing only
  reaches packmates), `surpriseOpener` (profile `opener: surprise`: from
  hiding, the first blow is an ordinary `attack #id` that the engine makes
  a surprise attack), `fightCastCommand`/`resolveFightSpell` (the model's
  `combat.spell` and `spell_at`, cast once, harm only where `mayStrike`
  allows, never at her owner, none while holding back), `castReady`,
  `canSneak` (the `sneak` verb, for a profile training skullduggery or
  opening from hiding) and `combatRule` (the prompt's fighting paragraph,
  written per profile from `approach`, `moves` and the above).
  Tests: `combat_style_test.go`; the trees are tested in
  `internal/behaviortree/companion_archetypes_test.go`.
- **combat.go**: the fight from the companion's side: tracking, the
  model's plan (stance, target, flee point, style, move, spell), local reflexes at most
  once a round, hold-back with the owner's auto-assist restored, authored
  battle lines (silent while her owner is muted), and the summary
  afterwards. Every plan is her owner's to pay for, whoever started the
  fight: defending her owner is the owner's concern, PvP is off on the
  live server and companions are harm-protected. `requestPlan` puts the
  `fight` stimulus at the front of the queue and `promptedBy` gives
  `fight` and `fight_over` to the owner, so a plan is never batched with
  a passer-by's words, never billed to them, and never refused because
  strangers are off.
- **meeting.go**: the persisted bond state (`bondRecord`, with `Notice`,
  a line kept for a player's next login, `HollowHinted` and `SignRead`), the one-time
  hint to a new character (`onCharacterCreated` marks them in `newcomers`,
  `hintHollow` tells them in the Hollow's own zone), parting ways (`leave`
  sends her back to the Hollow through `sendBack`), and consent:
  `consented` and `consentLedger`, the copy of who has
  agreed that the model door reads off the mud lock (rebuilt by
  `syncConsent` on every bond load and save). Nobody is handed a companion
  any more: there is no auto-bond.
- **roster.go**: one of each companion on the server. `rosterState` (the
  `roster` plugin file) holds, per profile, who has her (`Owner`, 0 while
  she waits in the Hollow), since when, when they were last seen together,
  and what she has learned (`keptProgress`: skills, trained stats, spells,
  mutations, never items, so the bauble sweep has nothing new to walk).
  `claim` (refused while someone else has her, and one per account),
  `unclaim` (marks her `returning`), `keepProgress`, `seeOwner`,
  `releaseLapsed` (owner away `ReleaseAfterDays`; run at load and every 600
  rounds; leaves a `Notice`), `reclaimFrom` (at login, a record the roster
  does not give this player loses her, without carrying their progress:
  the move to one of each and every release use it), `sendBack` (ends a
  bond, keeps her progress when it was theirs to lose, hands her gear to the
  owner through `handOver` minus her own starting kit, frees the claim,
  resets the courtship), `abandons` (trust and affection both at or below
  `AbandonBelow`), `progressOfMob`/`progressOfInfo`/`applyProgress` (mirror
  the progression half of `internal/hooks`' `snapshotCompanionProgression`
  and `applyCompanionState`; keep them in step), `learnStartingSpells`, and
  the admin `rosterLines` and `release`.
- **hollow.go**: the Waystone Hollow. A free companion waits in her own
  room (`HollowProfile.RoomId`) as a plain, uncharmed mob (`waiter`), made
  non-combatant, charm- and attack-immune and essential
  (`mobs.HollowGroup`) by `spawnWaiting`; `tendHollow` (every round, from
  `onNewRound`) keeps the Hollow in step with the roster (`ensureWaiting`,
  `dismissWaiting`) and dispatches. A visitor's say (`hearInHollow`), `ask`
  (`askInHollow`), `show <item> <her>` (`handleShow`, the
  `companionai.RouteShow` seam: model-safe text only) and
  pass `hollowMayTalk`: no key of their own (with `RequirePlayerKey`) is
  no answer and no notice. Consent is the board at the Hollow's mouth:
  `tendSign` shows it (`showSign`, the words from `signText`, the `board`
  noun of `HollowSignRoom`, falling back to `hollowSignText`) to each
  player the first time they stand in that room (`bondRecord.SignRead`);
  speaking to a traveller after it records `Consented`; one who reached a
  traveller without passing it is shown it then, and that utterance is not
  sent. Having a companion is consent as well: `bondTo` calls
  `consentByCompanionship`, and `consentHolders` (from `loadRoster`)
  consents every roster owner, so admin grants and companions from before
  the board are covered. The one thing neither overrides is
  `companion-ai off` (`Refused`). `applyInterview` re-checks consent
  before it records anything from an interview that comes back late. There is no recruit command. `dispatchHollow` builds one
  `companion_interview` call (`buildInterviewMessages`, `visitorDossier`:
  what anyone can see of them, their strongest skill, kills, quests seen
  through, faction standing, skill in what she cares about), routed and
  paid by the VISITOR, and carried in the mind she would have of them
  (`mind-<visitor>-<mob>`, `Mind.Courtship`). `applyInterview` speaks
  (`hollowSpeak`), remembers, bounds her opinion change
  (`hollowEnvelopes`), counts a rise (`Courtship.Gains`), and acts on the
  verdict: `offer` (she has asked them, in her own words, whether they would
  like her company) stands (`Courtship.OfferedAt`, `offerStanding`,
  `offerStandsFor`) only with a rise counted and `wouldTravelWith`; `join`
  (their yes) reaches `considerJoining`, which requires a standing offer, a
  rise in an EARLIER moment and `wouldTravelWith` before `recruit` (which
  goes through `bondTo`), and otherwise has her hesitate with an emote and
  no system line; `not_them` withdraws an offer. The dossier
  (`visitorDossier`) adds plain hand-made gear, `fearedKills` (creatures
  with a stat pool of `fearedStatPool` or more), killings of people, every
  skill practised, up to `maxDossierQuests` quests seen through and
  `maxDossierFactions` standings.
- **tools.go**: the read-only questions the model may ask the game before
  answering (look closer, size up, wares, recall, find a place), answered
  under the mud lock from player-visible information only. For a call
  through her owner's own browser (`answerTools`' `relay`), a closer look
  at another player is `describePerson` without `full`: how they are and
  what kind, never their description or gear.
- **models.go**: model tiers (fast, main, deep) and their routing, the
  server key's breakers as the companion sees them (`breakerOpen` is
  `Blocked(ConsumerCompanion)`, a peek; `breakerResult` hands the call's
  ticket back with its outcome, or releases it for a refused door, a
  resting breaker (`errServerResting`) or a cancelled call). Her own
  consumer breaker counts every failure of hers, exactly as her breaker
  always did; the provider breaker, shared with baubles, counts only
  provider-wide failures, so a bauble model the provider refuses never
  pauses her. `ownerBudgetLeft` peeks at the ledger's own allowance
  (`DimCompanionOwner`, no reservation); the budget-state file is the
  companion's backup of its allowances (`loadBudget`, `restoreBudget`,
  `saveBudget`, `budgetStateToSave`); per-tier metrics, decision traces.
- **tiers.go**: who pays for a call. `route` picks the owner's own key
  through their browser (`routeRelay`, tier 2, only when
  `playerKeysOffered`: `PlayerKeys` on and `validRelayOrigin`), else the
  server's key (`routeServer`, tier 3), else nothing (`routeNone`, set
  lines). `applyRoute` stamps the route on every `modelCall` at build
  time; `reserveRoute`, `settleRoute` and `routeResult` read that one
  route, so a call settles once, to the ledger it was held against, and
  its outcome reaches the breaker of whoever paid (`relayTable` keeps a
  per-owner breaker; the global one is the server key's alone, kept by
  `apiframework`).
  `reserveRoute` builds each call's per-user allowances
  (`allowanceCharges`) and holds them with the server budget in one
  `apiframework.Reserve`; it returns a `hold` (route, the ledger's own
  `Hold` when it reserved anything, and, on a refusal, the ledger's
  error) that `settleRoute` takes back through `apiframework.Settle`. A
  hold from an earlier day gives nothing back to the owner's or
  passer-by's allowances; the ledger floors every counter at 0 itself.
  `validRelayOrigin` reads `WebDomain` through `gameHostname`, the same
  reading `gameOrigin` gives the relay page.
- **conversation.go**: talk gathered into one conversation per exchange
  (`noteConversation`, which also notes whether her owner spoke and how
  many turns each passer-by took), held mid-talk notes, and `closeConversation`,
  which sums the whole talk up as one memory on the fast tier
  (`summariseConversation`) or keeps the best note when there is no model
  or nobody's allowance to pay. A talk with passers-by alone is paid for
  by the one who said the most (`conversation.payer`, a tie to the lower
  user id); one her owner took part in is the owner's. An owner whose own
  key was live this session (`relaySeen`) but whose relay is down now (at
  logout) has the talk kept (`deferSummary`, at most
  `maxDeferredSummaries` per owner, the oldest kept as its best note) and
  summed up through their relay once they are back
  (`startDueSummaries`, beside `startDueReflection`), never on the
  server's key.
- **reflect.go**: the private end-of-session reflection (summary,
  conclusions, facts), run in the background after logout
  (`detachReflection`). An owner whose own key was live this session
  (`controller.relaySeen`) cannot be reached at logout, so their
  reflection is copied (`deferReflection`, one per owner, the newest) and
  started by the round tick (`startDueReflection` in `sync`, under the
  lock) once they are online with their relay up (`dueReflection`), never
  on the server's key and never from the connection goroutine. The launch
  asks for consent again.
- **memory.go**: retrieval (importance, recency, relevance by words, place
  and people) and forgetting.
- **opinion.go**: `Opinion`, per-trigger envelopes, `boundDelta`
  (envelope, personality sensitivity, diminishing returns on praise),
  the audited `applyOpinion`, and `opinionWords` (the only form the model
  ever sees).
- **openai.go**: `callModel`, the blocking call (goroutine only), with a
  per-call strict JSON schema. The wire types are `apiframework`'s
  (`chatMessage` and friends are aliases); the body is
  `apiframework.Chat.Body()`, the server send is `apiframework.Post` with
  `doorFor` as its `Admit`, and spend is `apiframework.Charged`. Every request passes `admit`, the one door,
  which refuses (`errNoConsent`) any request whose `OwnerUserId` has not
  agreed, or is 0; only `listModels` (key, no player data) is exempt, as
  `carriesNoPlayerData`. There are two ways out and both call `admit`
  first: `send` (HTTP, the server's key) and `sendRelay` (the owner's
  browser, always guarded). Both transports decode through one
  `decodeChatResponse`, which keeps at most `maxToolCallsPerReply` (4)
  questions of a reply. A relay call waits `RelayTimeoutSeconds`, not the
  tier's timeout, since it includes the browser's round trip. A relayed
  error status keeps none of its body in the error (it is the provider's
  text about the owner's own account); the server key's keeps a snippet.
  `callModelOnce` is where spend is counted, once for every caller
  (`exchangeOnce` does the transport): a request that left (`Sent`) but
  reported no usage (a timeout, a dropped connection, a call cancelled
  after it was sent) counts its prompt estimate plus its MaxTokens
  (`Estimated`), since it may have been billed a whole answer; one the
  door refused, that found no browser, that could not connect, or that
  was cancelled before it left counts nothing; a count relayed through a
  player's browser is held between nothing and prompt plus completion,
  and `settleRoute` charges a passer-by at most their reservation. A
  cancelled decision reaches no breaker (not even as a success, which
  would reset the failure count). The background calls (reflection,
  summary, core memory) settle first thing in their apply function, and
  their recover handler settles when the apply was never reached.
- **relay.go**: the relay transport (tier 2). `pendingRelays.do` sends
  `Companion.Relay.Request {id, body, deadlineMs?}` (a random 128-bit hex
  id, the chat completions body, and the time left on the call from
  `relayDeadlineMs`; nothing else) and waits for
  `Companion.Relay.Response {id, status, body}`; `deliver` accepts a reply
  only from the owner it went to, for a pending id, once (refusals are
  counted in `dropped`). A reply over 1 MiB (`errRelayTooLarge`) or that
  `looksLikeAKey` (`errRelayKeyShaped`, logged once a minute without the
  text) is a failed call. `onRelayInbound` (installed as
  `companionai.SetRelayInbound` while the module is on) handles `Ready`
  (model name checked by `relayModelOK`: at most 100 printable,
  space-free characters, not key-shaped), `Gone` and `Response`, and does
  nothing while `playerKeysOffered` is false. `relayGone` (on `Gone` and
  on `PlayerDespawn`) takes the relay down and fails pending calls at once
  (`errRelayGone`). Unsent, gone, timed-out (`errRelayTimeout`),
  key-shaped and oversized failures are never retried (`relayFinal`).
  A relay that went away, a key-shaped reply and a cancelled call are not
  the provider failing: `routeResult` counts none of them against the
  owner's breaker. `keyShaped` matches an `sk-` key, "authorization:
  bearer", or "bearer" followed by `sk-` or a token-length run, not the
  words alone (a standard-bearer is not a key). The
  first failure it does count in a relay session tells the owner once, in
  plain words (`noticeFallback`, `relayOwner.noticeSent`, reset by
  `ready`).
- **relaypage.go**, **relayweb/relay.html**, **relayweb/relay-setup.html**,
  **relayweb/relay.js**: the key relay origin, the only place a player's
  key exists. All three files are embedded. `installRelayPage` (from
  `onLoad`) puts `serveRelayPage` on `companionai.SetRelayPage` while
  `playerKeysOffered`, else removes it. The engine runs it OUTERMOST
  (`internal/web.relayFirst` wraps the mux of both servers), and
  `serveRelayPage` claims EVERY request whose host is the `RelayOrigin`
  host (port and case ignored): `/companion-relay.html` (the frame),
  `/companion-relay-setup.html` (the key window) and `/companion-relay.js`
  are served (GET and HEAD only), every other path (`/`, `/ws`, `/admin/`,
  `/build`, ...) is a 404, so no game page, websocket or admin route ever
  runs on the key's origin. `gameOrigin` turns `FilePaths.WebDomain` into
  `https://host[:port]` (a pasted scheme or path is dropped; anything not
  a plain host gives "" and nothing is served); it is the frame's only
  `frame-ancestors` source in `relayCSP` (no inline or eval script, inline
  styles only) and is HTML-escaped into the frame's `game-origin` meta tag
  (`renderRelayHTML`). The key window gets `frame-ancestors 'none'`: it is
  a top-level window, opened by the FRAME (`window.open` from a click
  inside the frame, so the frame is its `opener` and the game page never
  holds it), and its address bar is the player's proof of where the key
  is going. The key is typed only there; the frame has no input. The
  window hands the frame `{type:'settings', endpoint, key, model, sealed,
  remember}` by `postMessage` to its opener at its own origin, and the
  frame owns the storage, so no storage or BroadcastChannel is shared
  between the two and browser storage partitioning (a cross-site relay
  framed by the game) cannot break the handoff. Neither page has a
  `<form>` or a password field: the key and passphrase inputs are
  `type="text"` with `autocomplete="off"`, masked by
  `-webkit-text-security` where the browser has it (Firefox does not; the
  window says so there). `relay.js` is UMD: node tests require its pure
  parts (`relayOne`, `seal`/`unseal`, `createRelay`, `createSetup`,
  `acceptMessage`), the browser runs `boot`, which picks the page by
  `<body data-page>`. The frame posts only to the stored endpoint
  (`redirect: 'error'`, no credentials, no referrer), returns `{id,
  status, body}` with no body on an error status and status 0 for a reply
  over 60 KiB or one echoing the key, allows at most 2 requests in flight,
  30 a minute and 40000 summed completion tokens a minute (excess is
  status 0 without a fetch), rewrites every body through `constrainBody`
  (stored model forced, token caps at 4000, `n` 1 and no streaming, one of
  the five companion schema names, at most 256 KiB), aborts the fetch at
  the server's deadline (`fetchTimeout`, never past 90 s), binds the key
  window to the account it was opened for,, accepts messages
  only from its parent at the game origin and from the window it opened at
  its own origin, and posts only to those. A remembered key is
  PBKDF2-SHA256 (600000) into AES-GCM with the account's storage key as
  additional data, blob `v: 1`, sealed in the window and stored by the
  frame. Tests: `relaypage_test.go`, `tools/jstest/companion-relay.test.js`.
- **decision.go**: the decision schema, `parseDecision`,
  `sanitizeDecision` and `cleanText`, which enforce everything the schema
  cannot.
- **prompt.go**: `buildMessages` and pure helpers (`isAddressed`,
  `humanizeElapsed`); `writeIdentity`, WHO YOU ARE, shared with the
  Hollow's prompt. Player text only ever appears quoted in the user
  message. Trust-gated backstory is withheld from the model until earned.
  `relaySafeLines` and `relaySafeStimuli` are what a prompt carries when
  the call goes through her owner's own browser, where the owner can read
  it: speech she only overheard from someone other than her owner is left
  out, and a look at another player uses its plain form (`Line.Plain`,
  `stimulus.Plain`, set by `lookAt`). dispatch, the reflection launch and
  `recordCore` build their messages after `applyRoute`, so they know the
  route; a conversation summary carries only what was said to her.
- **perception.go**: `describeSituation`, what the companion can currently
  perceive, as words (no numbers, no hidden creatures, no secret exits, no
  hidden containers, no container contents). Every item name and
  description told to the model goes through `items.Item.ModelName` and
  `ModelDescription`: text a player's own key wrote, moderated or not,
  never reaches a model prompt (spec S3).
- **mind.go**: `Mind` (schema 2: memories, facts, promises, summaries,
  opinion and its audit log), migration from schema 1, save/load. Every
  writer of text into her mind (`addLine`, `addMemory`, `addFact`,
  `addPromise`, `addOwnPhrase`, `addHearsay`, `addCore`) and
  `noteConversation` keep at most `maxStoredRunes` (300) runes
  (`capRunes`), since player text arrives whole and goes out in every
  prompt.
- **profile.go**, **profiles/*.yaml**: authored companion definitions,
  embedded in the binary: mara, corvel, liesl, tobin, isaura, hal. Beyond
  who they are: `specialty` (a WHO YOU ARE line), `starting_spells`
  (checked against the game's spells at load, `spellsValid`), `ammo_word`
  (what combat calls their ammunition), the fighting style in `combat`
  (`stance` they go in with, the `moves` that suit them, an explicit empty
  list for none, `mend_below`, `ward_owner`, `opener`, and `approach`, a
  sentence for the model; see combat_style.go), and `hollow` (`HollowProfile`: their
  room, what wins them over, what puts them off, how they wait, how they
  come back). Duplicate ids, mob ids and Hollow rooms are refused at load.
- **commands.go**: the admin-only `aicompanion` command, including
  `aicompanion mind <character>` and a status line per companion with its
  `tier=relay|server|none`; the player's `companion-ai` (consent, the
  `strangers on|off` toggle, and which tier is answering, `tierWords`).
  Its lines are wrapped at 80 columns before sending, since the system
  category is never wrapped for the reader.
- **primer.txt**: the common-knowledge world primer given to the model.
- **relayfor.go**: `relayFor`, the `apiframework.Relay` the module lends to
  other features, per purpose. It answers for `apiframework.PurposeFinds`
  only for an owner whose key page has "Also name things I find while
  searching" ticked (`relayOwner.finds`), and for every lively purpose
  (`apiframework.IsLively`, such as `PurposeNPCIdle`, townsfolk idle
  moments) only for one who left "Make the world livelier" ticked
  (`relayOwner.lively`, `relayOwner.allows`; the box starts ticked, and the
  Ready message carries it as `lively`, `relayTable.readyFor`). It sends
  through `sendRelay` with the request's own `Carries`, and feeds that
  purpose's own breaker (`Result(userId, purpose, err)`,
  `relayTable.purposeResult`, `relayOwner.lent`, a `lentBreaker` per
  purpose, each lively feature its own), never the one she runs on nor any
  other purpose's: a request the owner's provider will not serve pauses
  that feature on that key, nothing else. Tests: `relayfor_test.go`.
- **config.go**: every setting and its default (`buildConfig`). The API key,
  base URL, custom endpoint switch and daily token budget are no longer here:
  they are the `APIFramework` section's. Left in the old place (a server's
  config.yaml from before), they are still read, with the same meanings and
  defaults as before, and a warning. `BreakerErrors` and `BreakerSeconds` stay
  here for each player's relay breaker, and are also the server key's when
  `APIFramework` sets none. The module
  ships no data-overlay: a plugin overlay overwrites `_datafiles/config.yaml`
  rather than filling in behind it. See docs/aicompanion/settings.md.

## Locking

- Listeners, the ask handler and admin commands run under the mud lock and
  must never take it.
- `dispatch` builds the whole request under the lock, then starts a goroutine
  that holds no game pointers. That goroutine takes `util.LockMud()` only
  around `applyResult`.
- Reflections run after the controller is gone. They find their Mind through
  the `minds` cache, which keeps one pointer per mind for the life of the
  process, so a reflection and a new session can never hold two copies.
- `controller.seq` is bumped on every dispatch, detach, fall and pause. A
  reply whose seq no longer matches is dropped.
- `Mind` is only touched under the lock. There is no other mutex.

## Persistence

One mind file per owner and companion, through the plugin store
(`WriteStruct`, durable and autosave-queued): identifier
`mind-<ownerUserId>-<mobId>`. A visitor courting a companion in the Hollow
has one too, the same file, so it carries over if she joins them. The
roster is `roster` (saved on every claim and release, at logout and at
each autosave); a missing roster frees every companion. Each roster entry
also keeps `OwnerName` and `Partings`: when she leaves someone (`unclaim`,
every departure path), `partWith` (parting.go) keeps ONE sentence of them
by account, wipes the rest of her mind of them (`forgetOwner`: memories,
core memories, facts, promises, romance, courtship, keepsakes; Opinion and
her world knowledge stay), and asks the model on that player's own route
for the sentence in her own voice (`askParting`, `companion_parting`;
`fallbackParting`, which does not name them, stands until it answers or
if it never does). `forgetOwner` also puts `Autonomy` back to normal, drops
`AssistToRestore`, and bumps `Mind.Wipes`; a core memory, reflection or
conversation summary in flight when she left carries the count it started
with and is thrown away on return if it differs (`applyCore`,
`applyReflection`, `applyConversationSummary`), and `partWith` drops any
deferred reflection or summary for that mind. One account, one holder:
`holdsHer` matches the roster's `OwnerName` as well as the account, so
another character on the same account cannot field her (`claim` refuses,
`sync` reclaims), and a roster claim whose character has no record of her
is released (`sync`, `cmdPart`). The Hollow interview shows it to her when that player
comes to talk (`interviewInput.Parting`); `bondTo` deletes it when they
win her back (`forgetParting`). Tests: `parting_test.go`. The server key's day total is in
`apiframework`'s ledger (`<DataFiles>/apiframework/budget.yaml`); the
per-owner and passer-by allowances are the ledger's too (`companion.owner`,
`companion.stranger`, `companion.strangersfor`). The module's own budget
file keeps its calls and notices and a backup of those allowances
(`budgetStateToSave`, from `Allowances`); every boot hands it to
`restoreBudget`, which seeds the ledger (`SeedTokens`, `SeedAllowances`)
only on the first boot after the move or after a quarantined budget.yaml.
A corrupt file is quarantined by
`ReadIntoStruct` and a fresh mind is used. Mechanical state (items, gold,
skills, health) is never in the mind file; it lives on the owner's
`CompanionInfo` and on the live mob.

## Model tiers

- With no model configured, `modelChooser` picks per tier from
  `modelPreferences`, using the API's model list when it could be read
  (`probeModels` at load), and moves on from any model the API refuses
  (`modelRefused`). `effortFor` adapts reasoning effort to each family.

- `tierFor` picks fast, main or deep from what triggered the call;
  `Config.settingsFor` resolves the model, token limit, timeout and
  reasoning effort, falling back to `Model`.
- Parsing and optional moderation happen on the model goroutine; the result
  carries the parsed decision into `applyResult`.
- `aicompanion prompt <character>` shows the exact last request. See
  `docs/aicompanion/testing-and-prompts.md` for what each section is for.

## Combat rules

- The engine's combat AI does the fighting; the module never blocks it.
- `combatTick` runs from `onCombatRound`, registered `events.First` on
  `NewRound`, ahead of the engine's combat hook, for each controller the
  previous round's `sync` found standing and not sneaking
  (`controller.standRound`). Her `companion_*` tree runs inside that hook
  and claims the shared special move cooldown, so a reflex chosen after it
  would always be dropped.
- A spell's `[m]` ref is its place in her whole spellbook
  (`spellsReady`), not in the affordable list, so it does not drift when
  her conviction changes between the prompt and the answer.
- `move: unchanged` (the default for a plan that does not say) keeps the
  move already planned; `none` drops it. `realMoves()` is the list of real
  moves, used by `movesFor` and profile validation.
- Plans come from the model in the background; reflexes run every round
  with `CombatReactionRounds` between moves, using only `attack #id`,
  `fire #id`, `taunt`, a special move, `cast <spell> [@id|#id]`, `flee`,
  `drink` and `aid @id`. Order: flee, potion, tend owner (mend, ward),
  protect, the model's move (only one in `movesFor(profile)`), the
  model's spell, the chosen target, an archer's first shot from an empty
  chamber (later shots are her tree's `try_fire`), out of ammunition.
- Each companion's template names its own `companion_*` archetype,
  `aiprofile`, `specialmovechance` and submission/surrender policies; the
  template's `skills:` are the trade they arrive with (`applyProgress`
  keeps the higher of that and what she has learned since), and
  `seedRecipes` teaches every recipe her skill has reached, so a seasoned
  cook knows more dishes.
- The run reflex fires only when `actions.FleeGate` returns `FleeOK`, so a
  companion knocked down, grappled, rooted or frenzied does not speak its
  flee line or set `Fled` for a flee the engine would refuse. A flee
  already under way (`FleeRefuseAlready`) ends the reflex for that round.
- She harms only what her owner could harm (`harmAllowed`): the `attack`
  verb, a harmful `cast`, the plan's chosen target
  (`applyCombatProposal`), the reflex strike at whatever is hurting the
  owner, and a special move at her current foe all pass it (`mayStrike`,
  `mayStrikeCurrent`). The engine gates harm by a PLAYER actor and never
  a mob one, so without this a bonded companion could reach what her owner
  may not. There is no self-defence exception for what she starts, because
  a player gets none: a protected creature that turns on her is fought by
  the engine's round. The one allowance is the player's own: a player in a
  fight may use a special move on their foe whoever it is
  (`actions.StageMeleeTarget` stages no target checks in combat), so
  `mayStrikeCurrent` allows a move at a foe that is fighting her
  (`foeFightingHer`), never at her owner. Ordinary companions are not gated; making
  "a mob acting for a player is gated as that player" engine-wide is a
  separate call.
- `refusesToFight` is personality only: shopkeepers and the profile's
  `refuse` words. What nobody may attack is the engine's to say.
- A profile can go into a fight holding back (`combat.stance: hold_back`,
  Liesl). If auto-assist already sent her at a foe that is not fighting
  her, `setHoldBack` ends that engagement (`EndAggro`); a foe on her she
  still answers. `combatTick` repeats that check every round, because the
  engine's retargeting pulls her back in when a foe falls.
- Holding back switches the owner's `AutoAssist` off for that fight only.
  The previous value is kept in `Mind.AssistToRestore` and put back when
  the fight ends, when the companion falls, when the session ends, or at
  the next attach.

## Safety rules

- The bond ends only through `companion-part` (the owner's command), never
  on the model's word: `leave` sets a request that lapses after
  `LeaveConfirmSeconds`. The exception is `collapsed()`.
- A romance (romance.go) moves only at the owner's command
  (`companion-court`, `courtStep`) and only after consent: before it,
  `courtStep` answers that she is a plain companion for now and
  `companion-ai on` changes that, and `tendRomance` and `noteMilestone`
  count, feel and raise nothing.
- Every dispatch captures `controller.worldRev`; `answerTools` and
  `applyResult` drop everything if it has changed.
- `controller.cancelInFlight` cancels the HTTP call on logout, pause, reset
  and death. Budgets are reserved at dispatch in one check-and-hold step
  (`reserveRoute`; `hold.fw` is the ledger's own hold) and settled on
  return against the same payer and route (`settleRoute`), exactly once.
  On the server's key a logical call takes one breaker ticket
  (`callModel`: `apiframework.Allow` just before its first send) and keeps
  it across its retry and tool rounds
  (`modelCall.ticket`, `modelResult.Ticket`); `routeResult` hands it back.
  Each of the five model goroutines also releases it last (`ticketOut`), a
  no-op once recorded, so a panic cannot hold a half-open breaker's probe.
  A call held back while another probes (`errServerResting`) falls back to
  set lines and is no error: not counted in the tier stats, `errorsToday`
  or `lastErr`. A call a passer-by prompted (`strangerBehind`) is held
  against their `StrangerDailyTokens`, what passers-by together may spend
  of that owner's companion (`StrangerTokensPerOwner`; `allowanceCharges`,
  reserved on the ledger by `reserveRoute`) and, on
  the server's key, the server budget, never the owner's allowance, and
  runs with no tool rounds so its worst case fits. On the owner's own key
  (tier 2) nothing of the server's is held: only the passer-by caps. `modelReadyFor(owner, asker)`
  routes by the owner even when a passer-by asks.
- `nextBatch` gives each decision one prompter: everything the owner did
  or asked (`ownerDeeds`: words, emote, gift, healing, an attack,
  `companion-ask`, greeting, farewell, first meeting, a fight, an errand
  the owner sent her on) and each passer-by's are decided separately,
  with the world's stimuli going to the first. The follow-up to a look
  keeps the payer of the decision that looked (`lookedFollowUp`). The owner-only
  verbs (`ownerPrompted`) and "ask first" (`ownerAskedNow`) are refused
  whenever any passer-by's stimulus is in the batch, whoever else spoke.
- What she says is her owner's to answer for, on every tier: a muted owner
  (`UserRecord.Muted`) silences her say, emote and `sayto` alike
  (`spokenLines`, in `speak` and the `sayto` action), and her authored
  battle lines, idle gestures and thinking gestures too
  (`ownerSilenced`). On the owner's own
  key nothing moderates her words, so each line she says that way is
  logged at Info against the owner (`speechLogLine`, `logSpeech`).
- Item commands use `itemRef` (`!<itemId>:<uuid>`), never a display name.
- Sight is `messaging.ParticipantSight`, the engine's own rule.

## Acting rules

- The model never writes a command. It picks a verb from a closed list and a
  ref from the scene it was shown; code builds the command.
- A `cast` is owner-driven when the spell harms (`SpellData.IsHarm`, the
  engine's answer): `castHarm` refuses it in any batch a passer-by
  prompted, and aims it only at a creature or person `harmAllowed` passes
  and, like `attack`, not at anyone on her refusal list who is not
  already fighting; one that lands on the room must pass
  `areaHarmAllowed` (see harm.go above). A helpful cast (a mending, a ward) stays her own judgement.
- One non-perception action at a time: a new one is refused until the last
  one's outcome has been judged (two rounds after issue).
- `look_at` and `consider` are answered by the module from what a player
  would read, and earn exactly one follow-up decision so the companion can
  react. A follow-up never earns another.
- Loot arrangement (`ask_first` by default): with `ask_first`, `get` is
  allowed only in a decision triggered by the owner speaking to the
  companion; `leave_it` refuses all pickups; `take_freely` allows them.
  The arrangement changes only when the model reports a new agreement.
- Travel: the companion only knows rooms it has stood in. `mapper.GetPath`
  is never used, because it searches the whole world. Errands start only
  from beside the owner, who is not fighting; the owner moving pulls the
  companion back through the engine's companion transport, which asks
  `holdFollow` per companion; only the bonded companion is ever held (a
  sneaking owner, or her walking in on foot), never another companion of
  the same owner. After
  `ErrandLingerRounds` apart it walks back by its own map; after
  `RescueRounds` (never fewer than `LostRounds`) with no known way it
  rejoins through `companionai.Rejoin`.
- Exit names that are numbers or `home` are never walked: the mob `go`
  command would treat them as a teleport to a room id or as its own
  pathing.
- Buying and selling go through the ordinary mob `buy` and `sell`; a
  purchase must pass `purchaseCheck` (purse, reserve, size for the
  personality, unless a need or the owner asked). Protected items (gifts
  from the owner) cannot be sold, dropped, stored or given to anyone else.
- Checkable goals are completed only by `checkGoals`, never by the model.
- Not possible: repair (no item condition in the game), banking, skill
  training, trading item for item with the owner.

## Opinion rules

- The model proposes a change; code applies at most the envelope of what
  triggered it (talk: 2 per dimension; an emote: 3; a gift: up to 5
  affection; an attack by the owner: up to 15, and never a gain).
- Only stimuli caused by the owner can move the owner opinion.
- Three warm conversational gains within an hour and further warm words
  stop counting until deeds follow.
- Rules that apply whatever the model says: an owner attack costs 5 trust
  and 5 affection; a gift from the owner adds 1 affection (three a day at
  most); a promise the owner keeps adds 4 trust, one they break costs 6;
  a session of ten minutes or more adds a point of trust and affection
  while they are still modest.
- Every change is recorded in `OpinionLog` with its trigger and reason.

## Adding a companion

1. Author a mob template (see `_datafiles/world/dogmud/mobs/summons/9800-mara_venn.yaml`).
   It must carry **no items, equipment or gold**: a bonded companion that dies
   loses its gear to its corpse and respawns from the template, so template
   gear would be free gear on every death. Give it a `behavior_archetype`
   of its own (a `companion_*` tree with no flee, idle or packmate events),
   an `aiprofile`, and `skills:` for the trade it arrives with.
2. Author a room of their own in the Waystone Hollow (Pothole Coulee,
   6880 onward), furnished to their taste; no two companions share one.
3. Author `profiles/<id>.yaml` with that `mob_id` and `hollow.room_id`.
4. Rebuild. `aicompanion profiles` lists what loaded, `aicompanion claims`
   shows them waiting; `aicompanion grant <character> <id>` bonds a free
   one to an online player.

## Gotchas

- **One of each.** A companion is claimed for one account at a time
  (roster.go). Never field one without `claim`: `sync` takes back, at the
  next round, any bonded companion the roster does not give that player.
- **Key-only speech.** With `RequirePlayerKey` (the default) `route` never
  returns `routeServer`, `fallback` says nothing to a player without a key
  of their own, and a passer-by without one prompts nothing
  (`strangerMayPrompt`). Tests of the server-key path set it false
  (`consentModule`).
- **Waiting companions are not controllers.** They are in `waiting`, not
  `ctrls`; `isBonded` is false for them, and every listener checks
  `waiterForInstance` or `waiterInRoom` first.

- **Bonded companions cannot be dismissed or renamed by command**, cost no
  Conviction reserve, and recover from death (see
  `internal/hooks/companion_bonded.go`).
- **Switched off, the module leaves bonded records alone.** Login still
  fields a bonded companion from the owner's record (the engine's
  `respawnCompanions` does not filter by source type), and nothing drives
  it. The engine's `dismiss` asks `companionai.DrivesBonded(mobId)` for
  that one companion, which is `drivesBonded` (on, and a profile for its
  mob template) while the module is on and false while it is off, so the
  owner can part with an undriven one peacefully. Nothing is deleted at
  boot, so switching the module back on picks the bond up again. A
  companion her owner DISMISSED while the module was off does not come
  back when it is switched on: the bond record still says `Met`, which
  stops `considerMeeting`, exactly as `companion-part` is for good while
  it is on. There is no player path to a new companion after either; an
  admin can `aicompanion grant` one.
- **Mob commands split on `;`.** `cleanText` replaces it; never bypass it.
- **All model text is escaped** with `util.EscapeAnsiTags` before it reaches
  a mob command.
- **The companion list shows the source type**, which is why the stored value
  is `bonded`, not `ai`.
- **The farewell rides the `quit` meditation.** `quit` applies condition 0
  and the owner leaves when it expires; `sync` sees the condition and
  dispatches a goodbye immediately. A dropped connection gets no farewell.
- **Three listeners are registered even when the module is off.** The
  engine raises `events.Emote`, `events.Healed` and `events.GoldGiven`
  for this module alone, so `onLoad` registers `onEmote`, `onHealed` and
  `onGoldGiven` before the `Enabled` check (all return at once while
  off); otherwise every emote, every heal cast at a creature and every
  coin given to one is logged as "no listener for event". The events
  package has no notion of an event that may go unheard.
- **Config is read through the plugin config bag**, so a mistyped key is a
  silent default. `aicompanion status` shows what is actually in effect.

## Dependencies

From `internal/` (read from `go list -f '{{.Imports}}'`, 2026-09-25):
`actions`, `apiframework`, `characters`, `combatvocab`, `companionai`, `conditions`,
`configs`, `crafting`, `events`, `factions`, `gametime`, `items`,
`justice`, `messaging`, `mobcommands`, `mobs`, `mudlog`, `parties`,
`plugins`, `quests`, `rooms`, `shops`, `skills`, `species`, `spells`,
`targeting`, `usercommands`, `users`, `util`, `worldevents`. No other
module is imported: the relay reaches `modules/gmcp` and `internal/web`
only through the `companionai` seams.

Outside the repo: `gopkg.in/yaml.v3`. Standard library of note:
`net/http`, `net/url`, `crypto/rand` (relay ids), `embed` (the relay
page), `html`.

## Opinion

A bonded companion's feelings live in `Mind.Opinion` (three axes, audited,
no decay: only the owner's own deeds move it, and only the owner can bring
it back up). The engine's per-template `internal/opinions` score is
deliberately not moved for bonded companions (`companionai.IsBondedCompanion`
guards `actions.aggression` and the gift seeder), because that store is keyed
by mob template and every owner of the same profile would share one slot.
So `admin.opinion` does not describe a bonded companion; `aicompanion mind
<character>` does.
