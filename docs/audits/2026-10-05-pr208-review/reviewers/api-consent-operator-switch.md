# PR #208 review: api consent, operator switch lens

Blind reviewer `api_consent:operator_switch`, run 2026-10-05 against PR head `3ce674451`. Findings were then checked by independent verifiers (three for critical and high, one otherwise). Severity is the verifiers' median. Back to the [synthesized report](../README.md).

## Reviewer summary

Baseline (c696c117a): there is no global API master switch. Model use was controlled by four things. (1) Each feature has its own Enabled. Baubles shipped `Enabled: false`, and modules/baubles/config.go:124 says it defaults to FALSE "until an operator switches it on". (2) A server key, read from config or from OPENAI_API_KEY. (3) One shared DailyTokenBudget with per-feature shares, plus breakers. (4) Per player, the companion asked an explicit "i agree" / "i decline" question, and a player's own key was lent to another feature only through an opt-in key-page box ("finds", unticked by default).

The PR keeps the server-key budget and breaker plumbing intact. The new lively features never reserve server budget (spendServer=false), the companion's RequirePlayerKey defaults true in Go and in config, and route() only uses the server key when RequirePlayerKey is false.

The consent model, however, is inverted in four ways:
- Four new model features (npcidle, roomlife, lookdetail, rift room generation) are ON by default in Go and in shipped config, and players are opted in by default.
- Keys players saved before this PR are silently opted into all four, with no checkbox shown on unlock.
- Companion consent is now inferred: from any unaddressed `say` or `show` in a Hollow room, and from an admin `grant` (bondTo, then consentByCompanionship).
- The lively features send player-authored text (housing room descriptions, companion nicknames, player names inside "X's Homunculus") labelled CarriesNoPlayerData through another player's key, which bypasses the consent door.

There is a de facto master switch: turning off the companion module or PlayerKeys disables every lively feature through playerKeysOffered(). Per-feature Enabled, though, is read only at load.

## Coverage

Read at the PR head (3ce674451):
- apiframework: relay.go and budget.go diffs, Reserve with spendServer=false, the baseline config.apiframework.go and the baseline context.md.
- internal/lively/lively.go in full.
- Configs: modules/npcidle/config.go; roomlife and lookdetail config Enabled defaults; modules/roomlife/roomlife.go; the rifts gen.go generation path and its data-overlay config.
- Request builders for npcidle, lookdetail and roomlife (npcidle fully; the others for description and mob-name fields). lookdetail/match.go phrase rules.
- aicompanion: relay.js and relay-setup.html diffs; relayfor.go; the relayTable lending code in tiers.go; hollow.go consent, sign and dispatch paths; meeting.go diff; roster.go load and consentHolders; commands.go grant and bondTo; finds.go.
- Baubles: the generate.go diff, the search_bauble.go companion path, baseline baubles config defaults, and the housing overlay's description override.

Not read in depth:
- The prompt files of npcidle, roomlife and lookdetail.
- modules/lookdetail/lookdetail.go's cache logic beyond onNewRound.
- The rest of modules/rifts/rifts.go (portal logic, not API).
- aicompanion parting.go beyond the askParting gate, and combat_style.go, cooking.go and other non-API companion changes.
- tiers.go route() was only seen at its RequirePlayerKey line (278).
- Whether moderation calls to a custom (non-OpenAI) BaseURL could incur cost.

Nothing was run.

## Findings (6)

<a id="f007"></a>
### F007 [high] Four new model-calling features ship ON in Go defaults and config.yaml, with players opted in by default; this inverts the baubles opt-in convention

`modules/npcidle/config.go:56` · status **confirmed** · reported as high

npcidle, roomlife and lookdetail default `Enabled` to true via `lively.Bool(get(`Enabled`), true)`, and rifts defaults `GenerateEnabled` to true (modules/rifts/gen.go:239). Shipped _datafiles/config.yaml sets npcidle/roomlife/lookdetail `Enabled: true`, and modules/rifts/files/data-overlays/config.yaml sets `GenerateEnabled: true`. So no operator action is needed: every server that runs the companion key relay starts lending players' keys to these features on upgrade. The player-side box also starts ticked (relay-setup.html `<input id="lively" type="checkbox" checked>`). The existing model feature, baubles, is the opposite on both axes: config.go:124 says 'Enabled defaults to FALSE: nothing is named, and no room text goes to any model, until an operator switches it on', and the finds box starts unticked. Two further effects arrive without operator opt-in. Each lively output triggers a server-key moderation call (lively.Moderate) whenever a server key exists, including one that only came from OPENAI_API_KEY in the environment. Rift generation also writes model output permanently into world data (gen-*.yaml).

**Failure scenario.** An operator who enabled the companion and PlayerKeys for companion chat upgrades. Without changing config, every player with a relay key now has NPC idles, ambient events, look details and permanent rift rooms generated on their key. The server key, if present, starts sending each output to the moderation endpoint. Turning these off means finding four separate switches, two of them in a module overlay file.

**Existing mechanism.** The modules/baubles convention: Enabled defaults false in Go and in config.yaml, and the player permission (finds) starts unticked.

**Suggested fix.** Default every lively feature's Enabled (and rifts GenerateEnabled) to false in Go and in shipped config, and leave the lively box unticked, matching baubles and finds. Consider one APIFramework-level switch for lent player keys so an operator can stop all of them in one place.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
modules/npcidle/config.go:56 `Enabled: lively.Bool(get(`Enabled`), true),` (also roomlife/config.go:54, lookdetail/config.go:49, rifts/gen.go:239 GenerateEnabled true)
config.yaml: `npcidle:\n    Enabled: true`, `roomlife:\n    Enabled: true`, `lookdetail:\n    Enabled: true`
rifts data-overlays/config.yaml `GenerateEnabled: true`
baseline modules/baubles/config.go:124 'Enabled defaults to FALSE ... until an operator switches it on'; config.yaml baubles `Enabled: false`
```

- **confirmed** (high): Every cited fact checks out in the PR head. npcidle, roomlife and lookdetail all default Enabled to true in Go, and rifts defaults GenerateEnabled to true. The shipped config.yaml and the rifts overlay also set all four on. Baubles is the opposite: its Go default is false, config.yaml has `Enabled: false`, and the finds box starts unticked. So the inversion of the baubles convention is real on both the operator axis and the player axis.  Two things guard spending, and both are permissive by default. On the server, tiers.go only lends a key for lively purposes when the owner's relay reports lively=true. In the browser, relay.js constrainBody only forwards LIVELY_SCHEMAS requests when settings.lively is true.  The client side makes the problem worse than the claim states. The checkbox starts ticked, and openSealed treats a key sealed before this PR as ticked too: the code returns `lively: s.lively !== false`, with the comment "A key sealed before the lively box existed has it ticked, as a new one does". A player who stored a key for companion chat before the upgrade is opted in without ever seeing the box.  lively.Moderate sends each output to the server key whenever `apiframework.Server().HasKey()` is true and ModerateOutput is on, which is the default. The server key can come from OPENAI_API_KEY in the environment (settings.go DefaultKeyEnv). The rifts overlay confirms that generated rooms are saved permanently as gen-*.yaml.  What limits the damage: spending is on the player's own key only and is capped per day per user. The opt-in-by-default choice is also documented on purpose in comments. Even so, it reproduces exactly as described from the code as written.

- **confirmed** (high): I found no guard that prevents this. The only gates are the player's own relay key and the "lively" flag (lively.Ask returns ErrNoRelay when r.Model fails). That flag starts ticked on the setup page. relay.js also defaults it to true for keys sealed before this PR, because it reads `lively: s.lively !== false` and a missing field counts as ticked. The comment there says so: "A key sealed before the lively box existed has it ticked, as a new one does." So players who agreed only to companion chat, or to finds, before this PR are opted in to all four features on upgrade without seeing the new box. On the operator side, the Go defaults are true, shipped config.yaml sets npcidle, roomlife and lookdetail to Enabled: true, and the rifts overlay sets GenerateEnabled: true. Baseline baubles is the opposite: Enabled false in Go and in config.yaml, and the finds box starts unticked and is checked with `finds === true`. One partial mitigation: spend goes on the player's key, not the server budget. Moderation uses the server key only when HasKey() and only through a free endpoint (Moderate says "The check is free and reserves nothing"), so that cost point is weak. Still, opting players in silently through the legacy-key default, plus permanent gen-*.yaml world writes, keeps this at high.

- **confirmed** (medium): Every fact in the claim checks out. npcidle, roomlife and lookdetail set `Enabled` to `lively.Bool(get(`Enabled`), true)`. Rifts sets `GenerateEnabled` the same way, true. The shipped config.yaml has all three modules at `Enabled: true`, and the rifts overlay ships enabled. The `lively` box in relay-setup.html starts checked. Baubles does the opposite on both counts: Go default false, config `Enabled: false`, and the finds box starts unchecked. So the claim is right that this breaks the baubles opt-in convention.  The upgrade path is actually worse than the claim says. relay.js (around lines 183-185) reads a key saved before the lively box existed as `lively: s.lively !== false`, and the comment says "A key sealed before the lively box existed has it ticked". A player who saved a key only for companion chat is therefore opted in on upgrade without ever seeing the new box.  Some of the impact is overstated, though: 1. Nothing runs without an explicit player act. The player has to put their own key into the relay, and the box sits on that setup page, so the player can untick it. 2. The four Go comments show the default was a deliberate choice, stated in the code, not an oversight. 3. The moderation call adds little risk. `lively.Moderate` sends only generated text under `apiframework.CarriesNoPlayerData`, it is skipped when the server has no key, and its comment says the check is free. 4. "Four separate switches" is fair, but the player side has a single kill switch: the one lively box covers every lively purpose, enforced by `LIVELY_SCHEMAS` in `constrainBody`.  What remains real is a consent-convention inversion: opt-out instead of opt-in, plus silent opt-in for keys saved before the upgrade. Rift output written permanently into world data makes it matter more. That is a medium design or consent issue, not a high-severity defect.

</details>

<a id="f058"></a>
### F058 [medium] Lively features send player-authored text (housing room descriptions, companion nicknames, player names in homunculus names) as CarriesNoPlayerData through another player's key

`internal/lookdetail/request.go:43` · status **confirmed** · reported as medium

lookdetail, npcidle and roomlife all build their prompts from room.GetDescription() and from the names of mobs in the room, and lively.Ask always sends them as apiframework.CarriesNoPlayerData. That bypasses the consent door, which apiframework reserves for authored game text. In this same PR the housing overlay replaces r.Description with the owner-written `house.Descriptions[r.RoomId]` (internal/housing/overlay.go:88-89, set by a redecorating voucher). Mob names can also be player text: a pet nickname (usercommands/companion.go:138, `renameNick + " the " + BaseName`) and `ch.Name + "'s Homunculus"` (hooks/chrysifier_homunculus.go:148), which names a player even though npcidle/request.go:44 claims players are 'never named'. None of the three request builders excludes private/housing rooms.

**Failure scenario.** A lodging owner writes a personal description of their room. A guest with a lively-ticked key runs `look at <a word from it>` (lookdetail) or idles there (roomlife), and the owner's text goes to the guest's provider. The owner never agreed to this. lookdetail then caches the derived detail and shows it to everyone. A player's nickname for their pet, or their character name inside a homunculus's name, reaches another player's provider in the same way.

**Existing mechanism.** apiframework's Carries classification ('Only authored game text may be sent as CarriesNoPlayerData') and rooms.IsPrivateRoom, which this PR already uses to keep baubles out of housing (search_bauble.go baubleRoomAllowed).

**Suggested fix.** Skip rooms.IsPrivateRoom rooms in all three lively features (as baubleRoomAllowed does), and send mobs by template name (spec name), not by live Character.Name, or exclude companion and homunculus mobs.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
internal/housing/overlay.go:88 `if text, ok := house.Descriptions[r.RoomId]; ok && text != `` { r.Description = text }`
internal/lookdetail/request.go:43, internal/npcidle/request.go:64, internal/roomlife/request.go:49 `Description: plain(room.GetDescription(), ...)`
lively.go Ask: `r.Send(ctx, userId, body, apiframework.CarriesNoPlayerData)`
grep for private|housing in internal/{roomlife,lookdetail,npcidle} and their modules: no matches
```

- **confirmed** (medium): The claim holds up against the code. In this PR, house.Descriptions is free text the owner writes (use_items.go useRedecorate: the player types a paragraph, or answers the prompt, and it is stored as next.Descriptions[room.RoomId] = text). overlay.go:88-89 then copies that text into r.Description. Nothing in internal/lookdetail, internal/roomlife, internal/npcidle or their modules checks for private or housing rooms.  The lookdetail path reproduces directly. look.go:545 calls lookdetail.TryLook. TryLook matches the phrase against room.GetDescription(), which simply returns r.Description. It then reserves the looker's key and calls Snapshot, which puts the owner's text into Request.Description. The module's Generate calls lively.Ask, and lively.go:156 hard-codes apiframework.CarriesNoPlayerData. transport.go says that class is for "authored game text and nothing of any player's", and admitted() lets that class through with no Admit hook.  A guest is an account recorded on the house (guests.go), so a guest with the lively setting ticked can stand in the owner's room and send the owner's text to the guest's provider. The owner never consented. The result is cached under cacheKey(roomId, description, shown) and served to later lookers.  The secondary points also hold: - npcidle/request.go puts mob.Character.Name into the prompt. - companion.go:131-137 sets mob.Character.Name to the player-chosen nickname. - chrysifier_homunculus.go:148 builds the name from ch.Name + "'s Homunculus". - Both contradict npcidle's doc comment that players are "never named" and that "nothing a player wrote is in it".  The roomlife path also needs the room to have idle messages; I did not confirm that housing rooms have any. That path is less certain, but lookdetail alone carries the claim. Medium severity fits: the leak needs an invited guest whose key has the lively setting ticked, and the text goes to that guest's own provider, not to a public channel.

</details>

<a id="f059"></a>
### F059 [medium] Admin `aicompanion grant` (and roster load) records model consent for a player who never agreed or saw the sign

`modules/aicompanion/commands.go:313` · status **confirmed** · reported as medium

bondTo() unconditionally calls m.consentByCompanionship(owner.UserId), which sets rec.Consented=true unless the player has explicitly Refused. grant() (commands.go:206-226) is an admin command that calls bondTo for any online player, so the admin's action becomes the player's consent. consentHolders(), run from every loadRoster, does the same for every roster owner. The old explicit 'i agree' / 'i decline' flow (answerConsent, consentQuestion) was deleted. As a result there is no path where an admin-granted player is told what is sent before it is sent.

**Failure scenario.** An admin runs `aicompanion grant Bob mara` for Bob, who never visited the Hollow or saw the sign. Bob is now Consented. If the server runs RequirePlayerKey: false, Bob's next `say` to Mara is sent to OpenAI on the server key, with nothing ever having told him it would be. If RequirePlayerKey is true, the same happens the moment Bob sets up a relay key for any reason, finds included.

**Existing mechanism.** The baseline explicit consent ledger (bondRecord.Consented set only by answerConsent 'i agree' or `companion-ai on`).

**Suggested fix.** Do not infer consent in bondTo. Record it only in the Hollow path (hollowMayTalk, after the sign) and in `companion-ai on`. For admin grants, show the sign text to the player and require an explicit action.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
commands.go:310-313 `// Travelling with her is their agreement (consentByCompanionship) ... m.consentByCompanionship(owner.UserId)`
meeting.go:99-110 consentByCompanionship sets `rec.Consented = true` unless Consented||Refused
roster.go:107-118 consentHolders loops every roster Owner
meeting.go diff removes answerConsent/consentQuestion
```

- **confirmed** (medium): I could not refute this. The code at the PR head works the way the claim says. bondTo (modules/aicompanion/commands.go:313) calls m.consentByCompanionship(owner.UserId) with no conditions. That function (meeting.go:99-110) sets rec.Consented=true unless the record is already Consented or Refused. It never checks SignRead and never tells the player anything. The admin grant() at commands.go:206-226 reaches bondTo for any online player. The only message is the arrival line sent to the room; nothing tells the owner what will be sent or where. consentHolders() (roster.go:107-118), which runs on roster load, does the same for every roster Owner. Saving calls syncConsent (meeting.go:328), which copies Consented into the consentLedger. That ledger is what admit/doorFor/sendRelay in openai.go check, so the model door opens for this player. Every feature gate (listeners.go, conversation.go, corememory.go, finds.go, parting.go, reflect.go, romance.go) reads m.consented(), which now returns true. The baseline c696c117a behaved differently. Bonding through the meeting path set AskedAt and sent consentQuestion, and Consented became true only through answerConsent ('i agree') or 'companion-ai on' (baseline commands.go:719). So in the baseline an admin grant left the player unconsented. The PR deletes the 'i agree' flow, and the doc comment on consentByCompanionship states this is deliberate ("It also covers a companion an admin granted"). A deliberate design still swaps the player's explicit word for an admin's action, which is the consent concern being claimed. Mitigation: the shipped config has RequireConsent: true and RequirePlayerKey: true (_datafiles/config.yaml:2765, 2781). Under that config nothing is sent until the player sets up their own key through the web client's Companion key button ("Run your companion on your own model key"). That act itself suggests a model is involved, so the claim's 'RequirePlayerKey: false' scenario does not apply to the shipped config. Its second scenario does still reproduce: a key set up for any reason opens the door with no notice and no agreement given. I keep the severity at medium because this is a real regression from explicit consent, though the shipped key requirement limits the exposure.

</details>

<a id="f060"></a>
### F060 [medium] Keys saved before the PR are silently opted into every lively feature; the unlock view never shows the box

`modules/aicompanion/relayweb/relay.js:185` · status **confirmed** · reported as high

unseal() returns `lively: s.lively !== false`, so a key sealed before the lively field existed decodes as lively=true. A player with a remembered key only ever sees the #unlock view (relay-setup.html), which has a passphrase field and no lively checkbox. unlock() then posts settings with `lively: opened.lively !== false` (line 495), and the frame stores `lively: data.lively !== false` (line 426). The Go side records it through readyFor(userId, model, finds, lively). From then on their key is lent to npcidle, roomlife, lookdetail and rift generation (apiframework.IsLively, relayOwner.allows). The comment at relay.js:36-40 says outright that 'a key saved before the box existed has it ticked'. The relay-setup warning text still talks only about the companion and passers-by.

**Failure scenario.** A player remembered their key before this PR so it could run their companion, with finds deliberately unticked. After the deploy they log in and type their passphrase in the unlock popup. Without ever seeing the new option, their key now pays for up to 20000 tokens/day each of npcidle, roomlife and lookdetail, plus 40000 for rift rooms. That is roughly 100k tokens/day of their own money for features they never agreed to.

**Existing mechanism.** The `finds` permission (PurposeFinds), which decodes as `finds === true`: absent means no. The Go side also treats an absent `lively` as no (relay.go comment: 'absent (an older page) means no'), but the JS always fills it in as true.

**Suggested fix.** Treat lively like finds. Use `=== true` everywhere (unseal, settings, popup-state), leave the box unticked in relay-setup.html, and show the permission boxes on the unlock view, or force a one-time re-confirm when a sealed blob has no `lively` field.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
relay.js:185 `return { endpoint: s.endpoint, key: s.key, model: s.model, finds: s.finds === true, lively: s.lively !== false };`
relay.js:495 `model: opened.model, finds: opened.finds === true, lively: opened.lively !== false, sealed: sealed, remember: true }`
relay-setup.html #unlock div: only unlockpass, Unlock/Forget/Not now.
relay.js comment: "Unlike finds it starts ticked, and a key saved before the box existed has it ticked"
```

- **confirmed** (medium): The code does what the claim says. The baseline relay.js (c696c117a:146) sealed only {endpoint, key, model, finds}, so every key remembered before the PR has no lively field. In the PR, unseal() (relay.js:185) decodes a missing field as true (`s.lively !== false`). unlock() (relay.js:495) then posts lively true. applySettings (relay.js:426) stores it as true. The Go handler (relay.go:259-266) passes r.Lively into readyFor, and relayOwner.allows (tiers.go:136-137) lends the key to every apiframework.IsLively purpose: npcidle, roomlife, lookdetail and rifts. The #unlock view in relay-setup.html (lines 54-60) has only a passphrase field and Unlock/Forget/Not now buttons, with no lively checkbox. So a returning player who only unlocks never sees the option.  The Go comment at relay.go:254-258 says "absent (an older page) means no", but the JS always sends it filled in as true, so that protection never applies to an old saved key. This is not an accident. The JS comments at relay.js:36-40 and :183-184 say so on purpose, and docs/PATCH_NOTES.md:368-371 announces that the box "is ticked for you; untick it". That is an announced opt-out. It is not hidden, and each feature has its own daily token cap (npcidle DailyTokensPerUser 20000 in config.yaml). Those two points lower the severity from high. Still, it spends a player's own money on features they never saw, with the only notice in patch notes. It is also out of step with how finds is handled (missing means no). Given the owner's consent focus, it is a real consent defect at medium.

- **confirmed** (high): I found nothing in the code that stops this. The baseline unseal (c696c117a, relay.js:165) returned only {endpoint, key, model, finds}, so every key saved before the PR has no lively field. The PR's unseal (relay.js:185) turns an absent field into true (`s.lively !== false`), and the code comment says this is on purpose. When a player has a saved key and no live settings, view() (relay.js:330-333) returns 'unlock'. The #unlock div in relay-setup.html holds only a passphrase field and the Unlock / Forget it / Not now buttons, with no lively checkbox and no warning text. unlock() (relay.js:495) and applySettings (relay.js:426) carry lively=true through. The glue (companion-relay-glue.js:188) then sends Companion.Relay.Ready with lively:true, and relay.go:266 calls readyFor(userId, model, r.Finds, r.Lively). From there tiers.go relayOwner.allows lends the key to every apiframework.IsLively purpose. The Go comment "absent (an older page) means no" covers only an older page that sends no field. It does not cover an older stored blob, which the new JS fills in as true. Nothing on the game side (glue, in-game panel, help text) tells the player the lively choice is on. Even the setup view's warning text mentions only the companion and passers-by. The only limits are spending ones: per-feature daily token caps (e.g. npcidle DailyTokensPerUser) and the frame's per-minute caps. They bound the cost but do not give consent. Starting new keys ticked is a design choice the owner may accept. Silently opting in keys saved before the option existed, through a view that never shows it, is a real consent gap. The claim's cost figure (about 100k tokens a day) depends on the config caps, which I did not total exactly. That does not change the verdict.

- **confirmed** (medium): The mechanism is real. The baseline relay.js (c696c117a) has no "lively" field at all, so every blob sealed before the PR lacks it. unseal() returns `lively: s.lively !== false` (relay.js:185), and the comment just above it says this is on purpose ("A key sealed before the lively box existed has it ticked"). unlock() posts `lively: opened.lively !== false` (line ~495). applySettings stores `lively: data.lively !== false`. The #unlock div in relay-setup.html has only the passphrase field and the Unlock / Forget it / Not now buttons, with no lively checkbox. The Go side (modules/aicompanion/relay.go:254-266 into tiers.go readyFor/allows) then lends the key to every apiframework.IsLively purpose: npcidle, roomlife, lookdetail and rifts. So a pre-PR remembered key does get silently opted in at the next unlock, and nothing in the unlock flow shows it. The asymmetry with `finds` (`=== true`, absent means no) is also accurate.  The severity is overstated, though: 1. It is a documented, deliberate opt-out default, not an accident. The code comment says so, and docs/PATCH_NOTES.md:367-371 tells players the new box "is ticked for you; untick it to keep your key for your companion alone". 2. Once the key is unlocked, view() returns 'setup', where the lively box is visible and reflects the setting. So the player can untick it, though only by reopening the popup. 3. "Roughly 100k tokens/day" is the sum of per-feature daily caps (20000 x3 from config.yaml plus the 40000 rifts default in modules/rifts/gen.go:224). It is a ceiling, not an expected spend. Each feature is also gated by Chance (npcidle 5%, roomlife 10%), MinSecondsPerPlayer spacing, the player being present and able to see, and look misses.  It is still a real consent gap: a player who chose finds-off before the PR never gets an in-flow prompt about a new spending permission on their own key. That is medium, not high.

</details>

<a id="f128"></a>
### F128 [low] Hollow consent is inferred from any unaddressed `say` while alone, or from `show`, and the triggering utterance itself is sent

`modules/aicompanion/hollow.go:337` · status **confirmed** · reported as medium

hollowMayTalk sets rec.Consented=true and returns true for any player whose SignRead is set, and the caller then pushes the same text to the model. hearInHollow treats any `say` as addressed when the speaker is alone with her: isAddressed(text, name, true, others, RespondWhenAlone) returns `whenAlone && speakerIsOwner && otherPlayersPresent == 0`, and the call passes speakerIsOwner=true. handleShow (`show <item>`) also consents. SignRead is set merely by the sign text being pushed to the player on entering HollowSignRoom (tendSign, which calls showSign), with no check that they read it, saw it in the dark, or were scrolling. Baseline required the literal 'i agree' inside a 10-minute window.

**Failure scenario.** A player passes room 6880 (the sign text scrolls by), walks into Mara's room and types `say hmm, dead end`, or `show sword` to see what happens. The player is now permanently Consented, `hmm, dead end` goes to the provider, and every later companion conversation is sent too.

**Existing mechanism.** The baseline answerConsent literal-phrase flow and `companion-ai on`.

**Suggested fix.** Require an explicit action (`companion-ai on`, or a literal agreement phrase after the sign) before the first Hollow interview. At minimum, do not count unaddressed speech or `show` as consent, and do not send the utterance that grants it.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
hollow.go:326-339 `if !rec.SignRead { m.showSign(u); return false }; rec.Consented = true; m.saveBonds()`
hollow.go:460 `addressed := isAddressed(text, w.profile.Name, true, otherPlayersPresent(room, u.UserId, mob), m.cfg.RespondWhenAlone)`
prompt.go:639 `return whenAlone && speakerIsOwner && otherPlayersPresent == 0`
hollow.go:497 handleShow -> hollowMayTalk
```

- **confirmed** (low): The claim describes the code accurately. hollowMayTalk (hollow.go:318-341) sets rec.Consented=true, saves, and returns true for any unconsented, unrefused player whose SignRead is set. The callers then push the triggering text: hearInHollow pushes a 'said' stim, askInHollow pushes an 'asked' stim, and handleShow notes the shown item. hearInHollow passes speakerIsOwner=true to isAddressed. With RespondWhenAlone on (the Go default and config.yaml:2790), isAddressed (prompt.go:635-640) returns true for any say made when no other player is present. tendSign marks SignRead the moment the board text is pushed to a player in HollowSignRoom (6880). It does not check darkness, scrolling, or whether the player read it. The baseline (c696c117a) had no hollow.go; consent there came from the literal "i agree" answer in a window, as its tests show. The PR replaced that on purpose (meeting.go:37: "There is no typed 'i agree' any more").  So the scenario reproduces as written. Severity is lower than medium for three reasons: 1. It is a documented design choice, not an accident. The board text states "Speaking to them is your agreement to that", and so do the hollowMayTalk and meeting.go comments. 2. RequirePlayerKey ships true (config.yaml:2781, Go default true). hollowMayTalk returns false before any consent logic unless the player has a live key relay of their own (hasOwnKey). Under shipped config, nothing is ever sent to the server's key, and a player can only reach this path after an earlier deliberate step: supplying their own key through the web client Companion key button. The text goes to that player's own provider. 3. Consent can be revoked with "companion-ai off".  What remains is a weaker-consent UX concern. A casual unaddressed `say` or `show` while alone becomes durable consent. The sign may have scrolled past unread, and the triggering line itself is sent. That is fair to flag, but it is not a medium-severity bypass under default config.

</details>

<a id="f129"></a>
### F129 [low] Lively Enabled is read only at load; `server set ...Enabled false` does not stop calls until restart

`modules/roomlife/roomlife.go:67` · status **confirmed** · reported as low

configure() reads Enabled once at onLoad. onNewRound returns early when the snapshot is disabled and otherwise refreshes only Chance, MinSecondsPerPlayer and DailyTokensPerUser, never Enabled. Reserve() and Chance() check the stale snapshot. npcidle (npcidle.go:71) and lookdetail (lookdetail.go:68) work the same way. Baubles also reads Enabled only at load, so this matches existing code. But an operator turning these on-by-default features off at runtime gets no effect, and the config comments mark only the other knobs as 'live'.

**Failure scenario.** An operator alarmed by spend complaints runs `server set Modules.lookdetail.Enabled false`. Calls keep going on players' keys until the next restart. Only setting Chance or DailyTokensPerUser to 0 actually stops roomlife or npcidle live, and lookdetail has no Chance knob.

**Existing mechanism.** The baubles module behaves the same way (Enabled read at load).

**Suggested fix.** Re-read Enabled in onNewRound and install or remove the generator when it changes, or document in config.yaml that Enabled needs a restart.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
roomlife.go:67-79 onNewRound: `if !m.snapshot().Enabled { return }` then refreshes only chance/minSeconds/dailyTokens; Reserve: `return cfg.Enabled && m.turns.Reserve(...)` on the snapshot
```

- **confirmed** (low): The claim holds as written. In roomlife, npcidle and lookdetail, Enabled is read only in onLoad through configure(buildConfig(m.get)). Each onNewRound returns early when the snapshot says disabled, and otherwise refreshes only Chance, MinSecondsPerPlayer and DailyTokensPerUser; lookdetail refreshes only the last two. Reserve() and Chance() gate on the stale snapshot's Enabled. configure() is the only place that installs or removes the generator (roomlife.SetGenerator(m) or nil). The plugin framework has no config-change callback: PluginCallbacks offers only SetOnLoad, SetOnSave, SetIACHandler and SetOnNetConnect. So a runtime `server set Modules.<x>.Enabled false` cannot reach the module until restart. One caveat keeps severity at low. Every call runs on the consenting player's own key, behind their per-player 'Make the world livelier' tick and daily allowance, so players can still stop spend themselves. Setting DailyTokensPerUser to 0 (and Chance to 0 where it exists) is a working live stop for operators. The flaw is real but has workarounds.

</details>
