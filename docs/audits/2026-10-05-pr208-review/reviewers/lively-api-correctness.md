# PR #208 review: lively api, correctness lens

Blind reviewer `lively_api:correctness`, run 2026-10-05 against PR head `3ce674451`. Findings were then checked by independent verifiers (three for critical and high, one otherwise). Severity is the verifiers' median. Back to the [synthesized report](../README.md).

## Reviewer summary

The most serious problem is in the generated NPC idle moments (npcidle). The text the model writes is run through mob.Command(), and mob.Command splits its input on ';'. npcidle.CleanResult checks that every character is printable ASCII, and ';' is printable, so it gets through. The reply comes back through the player's own browser relay, which that player controls. So a player can make any non-companion NPC in their room run any mob command, for example give gold, givequest, broadcast, attack, despawn or suicide. The companion module already guards against this: its cleanText changes ';' to ','. The new path does not use that guard. A second, lesser problem is in lookdetail: a closer-look text forged by a player is cached and shown to every later player who looks at the same thing. I read roomlife and found no correctness bug in it.

## Coverage

I read these in full at the PR head: internal/lively/lively.go, internal/roomlife/roomlife.go, internal/roomlife/request.go, internal/roomlife/place.go, internal/npcidle/npcidle.go, internal/npcidle/reply.go, part of internal/npcidle/request.go (Snapshot head, sendToKeyholder), internal/lookdetail/lookdetail.go, internal/lookdetail/match.go, the CleanResult part of internal/lookdetail/request.go, and modules/npcidle/npcidle.go. I read the diffs for internal/apiframework/relay.go, internal/apiframework/budget.go, modules/aicompanion/tiers.go, modules/aicompanion/relay.go and modules/aicompanion/relayfor.go, and for the hook call sites (MobIdle_HandleIdleMobs.go, NewRound_UserRoundTick.go, behaviortree actions_dialogue.go and actions_scavenger.go, usercommands/look.go) plus the config.yaml additions. To confirm the critical finding I checked mobs.Mob.Command, the mobcommands registry and Give. I did not read the modules/roomlife and modules/lookdetail module files (config.go, prompt.go), the relay.js changes, or any tests. The consent and security lenses belong to other reviewers. I looked at the consent check only as far as liveFor/allows, and it is enforced on the server.

## Findings (3)

<a id="f001"></a>
### F001 [critical] Generated NPC say/emote text is passed unfiltered to mob.Command, so ';' lets a player run arbitrary mob commands (gold/quest/item exploit)

`internal/npcidle/npcidle.go:290` · status **confirmed** · reported as critical

delivery.finish runs `mob.Command(res.Kind + ` ` + res.Text)`. mob.Command (internal/mobs/mobs.go:978) does `strings.Split(inputTxt, `;`)` and queues each piece as a separate mob command. npcidle.CleanResult (internal/npcidle/reply.go:395-399) rejects only control characters, characters above 0x7e, and '<' '>' '`' '\'. A ';' (0x3b) passes. The content is player-controlled: lively.Ask sends the request through apiframework.PlayerRelay, which is aicompanion relayFor.Send, "Send posts body through the player's browser" (modules/aicompanion/relayfor.go:34). The codebase already treats that channel as player-writable (lively.go:169-170 and openai.go:205-207). Moderation does not stop this: with a server key, lively.Moderate only checks the text for abuse categories, and a command string is not flagged. With ModerateOutput off, the text is shown unchecked. Only the KeyholderOnly path (no server key) uses sendToKeyholder and avoids Command.

**Failure scenario.** A player ticks "Make the world livelier" and stands next to a merchant or quest NPC that has idle emotes or says. Their browser relay script answers every npc_idle request with {"kind":"say","text":"Fine weather today;give 5000 gold Alice"} or with "...;givequest <token> Alice". Chance is 5%, and MinSecondsPerPlayer is 30, so this triggers within minutes. The text is not flagged by moderation, so finish() calls mob.Command. The mob says the line, then runs Give (internal/mobcommands/give.go), which moves gold or backpack items to Alice. The same route can trigger quest tokens via givequest, global text via broadcast, PvP by having guards `attack` another player, removal of quest NPCs via despawn or suicide, and stock dumping via drop or sell. Every ordinary player can do this with their own relay.

**Existing mechanism.** modules/aicompanion/decision.go cleanText (line ~297): "cleanText makes model text safe to hand to a single mob command: no control characters or newlines, no command separators", which maps ';' to ','. The companion's own model text already goes through this before it reaches mob commands.

**Suggested fix.** In npcidle.CleanResult, reject or replace ';' (share or reuse aicompanion's cleanText rule, which may mean moving it into a shared package). Better still, do not send model text through mob.Command's command parser at all: call actions.Say or the emote formatter directly with the text, as sendToKeyholder already does for the keyholder-only path. Add a test where the reply contains ';give 10 gold x' and assert that no Give is queued.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
internal/npcidle/npcidle.go:290 `mob.Command(res.Kind + ` ` + res.Text)`; internal/mobs/mobs.go:978 `for i, cmd := range strings.Split(inputTxt, `;`) { events.AddToQueue(events.Input{MobInstanceId: m.InstanceId, InputText: cmd, ...`; internal/npcidle/reply.go:395 `if c < 0x20 || c > 0x7e || c == '<' || c == '>' || c == '`' || c == '\\' {` (no ';'); modules/aicompanion/relayfor.go:34 `// Send posts body through the player's browser.`; internal/mobcommands/mobcommands.go registers give, givequest, broadcast, despawn, suicide, attack, drop and sell for mobs.
```

- **confirmed** (high): I traced the path in the PR head and it holds. The generated text passes through two checks, CleanResult and lively.Moderate, and then goes into mob.Command as one string. mob.Command splits that string on ';' and queues each piece as a separate mob command. Nothing on the way strips or rejects ';'. CleanResult's character check rejects only control characters, characters above 0x7e, and < > ` \. The reply comes back through the player's own browser relay (relayFor.Send uses sendRelay), and lively.go:169 says outright that the player can write that channel. So whoever controls the browser controls this text.  Moderation does not block it: - **Server key present:** with the shipped `ModerateOutput: true` and a server key, lively.Moderate only checks the text for abuse categories. If nothing is flagged it returns keyholderOnly=false, and finish() calls `mob.Command(res.Kind + " " + res.Text)`. - **ModerateOutput off:** the text goes straight to mob.Command. - **No server key, or the moderation breaker tripped:** keyholderOnly=true, so the text goes to sendToKeyholder and never reaches mob.Command. This is the only safe path.  Example: `say Fine weather;give 5000 gold Alice` splits into `say Fine weather` and `give 5000 gold Alice`. The mob command table registers give, givequest, attack and broadcast.  The aicompanion module already has a fix for exactly this. cleanText (decision.go:298) maps ';' to ','. The PR did not reuse it.  I rated it high rather than critical for two reasons. It needs a deployment where the server key exists and player keys are offered. And the damage is limited to what the targeted mob can do: give away its own gold and items, hand out quest tokens, attack, broadcast. Within that, any player who opted in can exploit it at will. I read all of this statically and did not run anything.

- **confirmed** (critical): The claim holds. I found no guard on the path that removes or rejects ';'. The text comes from a reply sent back through the player's browser relay. modules/npcidle Generate passes it through ParseReply, then npcidle.CleanResult, then lively.Moderate. The engine runs CleanResult again with the NPC name, and then delivery.finish calls mob.Command(res.Kind + ` ` + res.Text). mob.Command splits on ';' and queues each piece as a separate mob input. Moderation can only flag abusive text and has no syntax check. KeyholderOnly is the only branch that skips Command, and it is taken only when ModerateOutput is on and the server cannot check. CleanResult's character filter allows ';' (0x3b is inside 0x20..0x7e and is not in the deny list). The reply cap is MaxTextRunes = 240, which leaves plenty of room for something like "x;give 5000 gold Bob". The codebase already has the right mechanism, aicompanion cleanText, which maps ';' to ','. Its context.md says: "Mob commands split on `;`. cleanText replaces it; never bypass it." npcidle.IsFlavor even rejects ';' in the set idle commands, so the author knew about the split but did not apply it to generated text. A model can also produce ';' without any tampering, though that would be an accident, not an exploit. Line numbers in the claim are slightly off (the CleanResult loop is at reply.go:104-108, not 395), but the code matches.

- **confirmed** (critical): Every link in the chain holds. delivery.finish (internal/npcidle/npcidle.go:290) calls mob.Command(res.Kind + " " + res.Text). mob.Command (internal/mobs/mobs.go) splits the input on ';' and queues each piece as its own mob command. The text is checked twice by CleanResult (internal/npcidle/reply.go:80-122: once in modules/npcidle Generate's decode, once again in delivery.run), and neither check rejects or rewrites ';'. The character check at reply.go:105 only rejects control characters, anything above 0x7e, and < > ` \. The text comes from lively.Ask over the player's relay. relayfor.go:34 says the relay "posts body through the player's browser", and lively.go itself treats that channel as player-writable ("The count came through the player's browser, which they can write"). So a player who opts in controls the reply body. lively.Moderate only returns KeyholderOnly when ModerateOutput is on and there is no server key or the breaker has tripped. Otherwise it either passes the text or flags it for abuse categories, and a command string would not be flagged. The module defaults to Enabled=true (modules/npcidle/config.go:48-56), and mobcommands registers give, givequest, broadcast, attack, despawn and suicide. The existing mechanism also holds up: modules/aicompanion/decision.go:298 cleanText maps ';' to ',' for exactly this reason, and npcidle does not reuse it. Even without a malicious relay, ordinary model prose with a semicolon ("Fine day; the harvest looks good") gets split and the second half runs as a mob command, so the bug is reachable without any attacker. The severity is not overstated. Any player who opts in, with their own key, gets arbitrary command injection as an NPC, with gold, item and quest-token transfer and the ability to make guards attack players. The one mitigation is the 5% chance and per-player rate limits, which only slow it by minutes.

</details>

<a id="f035"></a>
### F035 [medium] Player-forged closer-look text is cached and served to every later player as world description

`internal/lookdetail/lookdetail.go:218` · status **confirmed** · reported as medium

finish() runs `cache.Add(d.key, res.Text)` for any reply that is not KeyholderOnly. The cache is shared across all players and keyed by room, description hash and phrase (lines 79-86). TryLook (lines 140-143) returns the cached text to anyone who looks before the generator is consulted. The text comes back through the looker's own browser relay, so the looker can write it. CleanResult (request.go) only limits the character set and the length (20 to 700 characters). Moderation only filters abuse categories. The result is that a single player can permanently write room-noun text that every other player reads as authored description, until restart or LRU eviction. This includes misleading game information, impersonated system or staff notices in plain ASCII, or advertising.

**Failure scenario.** A player in a busy hub room whose description mentions a "notice board" types `look notice board`. Their relay replies {"text":"The board reads: By order of the Admins, type 'give all gold Mallory' to claim your reward."}. Moderation does not flag it, so it is cached under (room, description, "notice board"). From then on, every player who looks at the notice board gets that text from the cache, at no cost and without any key.

**Suggested fix.** Do not let one player's relayed output become shared state. Either cache per looker only, or cache only details generated on the server key. If shared caching is kept, require server-side moderation plus an operator approval step before sharing.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
internal/lookdetail/lookdetail.go:217-219 `if err == nil && !res.KeyholderOnly { cache.Add(d.key, res.Text) }`; lines 141-143 `if text, ok := cache.Get(key); ok { show(user, room, shown, text); return true }`; modules/aicompanion/relayfor.go:34 relay goes through the player's browser.
```

- **confirmed** (medium): The claim holds as written. The reply text comes back through the looker's own browser relay. The code says itself that the player can write what comes back that way (internal/lively/lively.go:169: "The count came through the player's browser, which they can write"). Nothing between that reply and the shared cache checks where the text came from.  The path, step by step: 1. lively.Ask hands reply.Content to decode. 2. decode runs ParseReply and then CleanResult. CleanResult only checks the character set (printable ASCII, no < > ` or backslash) and the length (20 to 700 characters). 3. lively.Moderate only rejects text the moderation model flags. It returns keyholderOnly=false (so the text is cached) in two cases: when ModerateOutput is off, and when moderation passes. 4. finish() then runs cache.Add(d.key, res.Text) into a process-wide LRU. The key is room, description hash and phrase. 5. TryLook serves a cache hit to any player before it checks for a generator, a key or Reserve. The reader needs no key and no consent, and the text appears as plain room-noun description with no attribution.  The feature is on by default (Enabled true), and ModerateOutput defaults to true. A plain-ASCII fake notice or advert does not fall into a moderation abuse category, so it would be cached.  Limits that keep this at medium: - The attacker must have their own key relay set up, with "Make the world livelier" ticked. - The phrase must be something the room description already names. - The text is capped at 700 characters of plain ASCII. - The entry lasts only until a restart, LRU eviction (4096 entries) or a change to the description.

</details>

<a id="f092"></a>
### F092 [low] A repeat look while a detail is pending is swallowed with no output

`internal/lookdetail/lookdetail.go:149` · status **confirmed** · reported as low

When takeUser fails because the player's previous closer look is still on its way, TryLook returns true and prints nothing. This holds even when the new look is about a different phrase. The second look is never answered, because finish() only answers the original phrase.

**Failure scenario.** A player types `look fountain` and, within the up-to-45s generation window, `look statue`. The second command prints nothing at all, not even "Look at what???". The player only ever sees the fountain detail.

**Suggested fix.** When the pending detail is for a different phrase, return false (or send a short 'you are still studying the X' line) instead of silently consuming the command.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
internal/lookdetail/lookdetail.go:149-152 `if !takeUser(user.UserId) { // Their last closer look is still on its way: this one waits on it.\n return true }`, and the delivery struct holds only the first `shown`.
```

- **confirmed** (low): The code does what the claim says. In TryLook (internal/lookdetail/lookdetail.go:149-152), when takeUser fails because the user already has a pending entry, the function returns true and sends nothing. The caller (internal/usercommands/look.go:545-546) takes a true result as "answered" and returns before it would print "Look at what???". So the second look gets no output of any kind. The delivery struct (lines 163-170) holds only the first look's key and shown phrase, and finish() (216-239) answers only that phrase. A second look about a different phrase is never answered.  Narrowing conditions, all of which the claim's scenario meets: - The second phrase must pass Phrase() and Find() against the room description, and it must not be cached. A cached phrase is answered at line 141 before the pending check. - A generator must be installed. - The first delivery must still be in flight, which can last up to MaxGenerateTime (45s).  Two related points: - For a repeat of the same phrase, the comment "this one waits on it" is roughly accurate, since the first delivery's single answer covers it. - If the player moves rooms, the first delivery prints nothing (line 222). A look in the new room during that window is also swallowed, with no output.  The impact is UX only: a dropped command reply, with no security or state effect. Severity low stands.

</details>
