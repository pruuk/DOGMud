# PR #208 review: ai companions, correctness lens

Blind reviewer `ai_companions:correctness`, run 2026-10-05 against PR head `3ce674451`. Findings were then checked by independent verifiers (three for critical and high, one otherwise). Severity is the verifiers' median. Back to the [synthesized report](../README.md).

## Reviewer summary

Correctness review of the AI companion overhaul (Waystone Hollow waiters, roster/parting, finds/bauble seam, combat styles, mob search/salvage seams) at the PR head. I found three real issues. (1) When a waiting companion accepts a player, her spoken reply is silently dropped: her lines are queued as delayed mob commands, and then recruit destroys that mob instance in the same call. (2) The new rule that she sets out with her starting kit on every bond, together with handOver withholding kit by bare ItemId, does two things: it lets the kit be farmed through give or sell, then part, then re-court, and it can destroy a player's own enchanted or upgraded copy of the same item instead of her plain kit copy. (3) A sibling parting path frees her into the roster without handing the owner's gear back. One minor consent-rule slip: tendOwner writes the owner's name into her mind without the mayRemember gate. Things I checked and found sound: roster persistence (onSave and detach save it, so a restart does not reset LastSeen), room mob persistence (room mobs are not saved, so no duplicate waiters on boot), MobDeath of bonded companions (comp.Items is cleared, so a fallen sendBack cannot duplicate), the seq guards on in-flight interview and parting replies, Release of tickets (same pattern as runtime.go), the `#id`/`@id` resolution used by archerTargetRef and cast targets, that the Hollow room and mob template references resolve to real files, and the module config clamps.

## Coverage

Read in full at the PR head: modules/aicompanion/hollow.go (lines 1 to ~950, which covers the waiter lifecycle, consent door, show, dispatch and apply interview, join and recruit), roster.go, parting.go, finds.go, combat_style.go, and the relevant parts of commands.go (bondTo, unbond, cmdPart, revoke), meeting.go (bondRecord, consent, leave, hintHollow), runtime.go (onNewRound, sync, detach, keepFrom), aicompanion.go (init, onLoad, onSave, getMind) and config.go (diff). Also read the engine diffs: internal/companionai/companionai.go and search.go, internal/mobcommands/search.go and salvage.go, internal/actions/search.go and search_bauble.go (companion delivery), internal/mobs/mobs.go (HollowGroup and IsEssential), internal/usercommands/show.go, internal/behaviortree/conditions_player.go (multiple_foes) and actions_archer.go, and internal/rooms/roomdetails.go. Verified that the Hollow rooms 6880-6885 and the mob templates 9800 and 9805-9809 with their companion_* archetypes exist. Not read in depth: hollow.go below ~950 (prompt building and visitorDossier), the bulk of combat.go outside the tendOwner, spell and opener sites, cooking.go apart from equipStartingKit, decision.go, prompt.go, the profile YAML bodies, the help templates, relay.js, and all tests. The consent and security lenses (sign-as-consent semantics, RequirePlayerKey routing) were only touched where they produced a correctness bug.

## Findings (4)

<a id="f027"></a>
### F027 [medium] A waiting companion's reply is lost on every successful join: recruit destroys the mob her speech was queued on

`modules/aicompanion/hollow.go:670` · status **confirmed** · reported as medium

applyInterview calls hollowSpeak first (line ~670). That queues each line as mob.Command(kind+text, delay) with delay >= 0.5s, so each line becomes an events.Input carrying a future ReadyTurn and the waiter's MobInstanceId. Then, for verdict `join`, considerJoining calls recruit, and recruit calls dismissWaiting(p). dismissWaiting synchronously runs r.RemoveMob and mobs.DestroyInstance on that same instance. When the queued inputs come due, world.processMobInput finds no mob, logs 'Mob not found', and returns. So none of what she said in her acceptance reply ever reaches the room. hollowSpeak has already written those lines into her mind as `said`, so she believes she said them.

**Failure scenario.** A player with their own key courts Mara, who answers with verdict `offer`. The player says "yes, let's go". The model replies with verdict `join` and speech ["Then let's not waste the light."]. The room sees only "Mara gathers up her things and falls in beside X." Her line is never shown, and the server log gets one 'Mob not found ... processMobInput()' error per queued line. Later her working memory says she told them "Then let's not waste the light."

**Suggested fix.** On a join, send her lines to the room directly instead of queueing them on the mob that is about to be destroyed. Alternatively, defer dismissWaiting and bondTo by the total speech delay, or re-issue the lines on the new bonded instance that bondTo creates.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
hollow.go applyInterview: `m.hollowSpeak(w, mob, mind, u, d.Speech, rt)` runs before `case \`join\`: m.considerJoining(...)`. hollowSpeak: `mob.Command(l.Kind+\` \`+util.EscapeAnsiTags(piece), delay)` with `delay := 0.5`. recruit: `m.dismissWaiting(p)`, then dismissWaiting: `r.RemoveMob(mob.InstanceId)` and `mobs.DestroyInstance(mob.InstanceId)`. mobs.go Command: `events.AddToQueue(events.Input{MobInstanceId: m.InstanceId, ..., ReadyTurn: m.lastCommandTurn})`. world.go:1082: `mob := mobs.GetInstance(mobInstanceId); if mob == nil { ... mudlog.Error("Mob not found" ...); return }`.
```

- **confirmed** (medium): The claim holds as the code is written. applyInterview runs under util.LockMud on the interview goroutine (hollow.go:646-649). It calls hollowSpeak first (line 700). Unless the visitor is muted, hollowSpeak queues every line through mob.Command(..., delay) with delay >= 0.5s (lines 818-832). mobs.Command (internal/mobs/mobs.go:960-989) only adds an events.Input to the queue, tagged with the waiter mob's InstanceId and a future ReadyTurn. Nothing runs at that point. Then the `join` verdict reaches considerJoining (line 742). Once its guards pass, considerJoining calls recruit (line 774), and recruit calls dismissWaiting(p) first (line 795). dismissWaiting runs r.RemoveMob and then mobs.DestroyInstance on that same waiter instance (hollow.go:295-299). DestroyInstance (mobs.go:871-875) only deletes the instance from the map. It does not mark the instance in recentlyDied. bondTo then spawns a brand-new instance with NewMobByIdFresh (commands.go:255), so the queued inputs still point at the destroyed instance id. When they come due, processMobInput (world.go:1079-1088) gets nil from GetInstance, RecentlyDied is false, so it logs "Mob not found" and returns. The acceptance speech never reaches the room, even though hollowSpeak already wrote it into mind as `said` (line 840). This is a real lost-output defect at the key recruitment moment, plus log noise. It does not corrupt any data, so medium is a fair rating.

</details>

<a id="f028"></a>
### F028 [medium] Starting kit is re-issued on every bond and withheld by bare ItemId: kit can be farmed, and a player's upgraded copy of the same item can be destroyed

`modules/aicompanion/roster.go:386` · status **confirmed** · reported as medium

This PR changed bondTo so equipStartingKit runs on every bond (commands.go ~297). The baseline ran it only when the player had never met her (`rec == nil || !rec.Met`). handOver is supposed to withhold exactly one of each kit ItemId when she leaves, so that setting out again cannot multiply the kit. It matches on ItemId alone, though, and walks Items (the pack) before Equipment. That causes two problems. (a) Duplication: the owner can have her `give` a kit item to them (give-to-owner is allowed and kit items are not protected), or have her sell it, in which case the gold comes back to the owner in handOver. At parting nothing is withheld because she no longer carries it, and the next bond hands her a fresh kit. (b) Data loss: items.Item carries per-instance state (Enchantments, EnchantTier, Quality, Affixed, Uncursed, MakerName, Wear, Adjectives). If the owner gave her a second, better instance of the same ItemId, it sits in her pack and is visited before her worn kit copy, so the player's instance is the one silently dropped. The plain kit copy goes back to the player.

**Failure scenario.** Data loss: Mara's kit is iron dagger 10009, worn. The owner gives her an enchanted, pristine iron dagger (ItemId 10009) to carry. Later the owner sends her away (companion-part twice). handOver iterates gear = Items then Equipment. hers[10009]=1 is consumed by the enchanted dagger in her pack, which is never stored or dropped and simply vanishes. The worn plain dagger goes into the owner's pack. Farming: Liesl's kit is staff 10020 plus healing draught 30001. The owner says "give me your draught", sends her away, and wins her back in the Hollow. She arrives with a new draught, and the cycle repeats.

**Suggested fix.** Tag the kit instances that equipStartingKit creates, for example a tempDataStore or field marker, or track their UUIDs on the controller while she is live. Withhold only those exact instances. Alternatively, keep a per-profile kit-outstanding count in the roster and re-issue only what she actually took back.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
commands.go bondTo: `// Her own few things, every time she sets out with someone ... equipStartingKit(mob, p)` (baseline: `if rec := m.bonds.Users[owner.UserId]; rec == nil || !rec.Met { equipStartingKit(mob, p) }`). roster.go sendBack: `gear = append(gear, mob.Character.Items...); gear = append(gear, mob.Character.Equipment.GetAllItems()...)`. handOver: `for _, id := range p.StartingItems { hers[id]++ } ... if hers[it.ItemId] > 0 { hers[it.ItemId]--; continue }`. actions.go: `if a.Verb == \`give\` && a.To != \`owner\` { return Refused }`, so give to the owner is allowed. The `sell` verb is in the owner-prompted set at actions.go:150.
```

- **confirmed** (medium): Both halves reproduce from the code as written.  (a) The kit can be farmed. The PR removed the baseline's first-meeting gate (`if rec == nil || !rec.Met { equipStartingKit }`), and bondTo now calls equipStartingKit on every bond. handOver withholds kit ids only from the gear she still holds when she leaves. The `give` action branch checks only that the item is carried and that the recipient is the owner; it never calls canPartWithItem. Even sell's canPartWithItem check would not stop it, because Protected is filled only by gifts (listeners.go:292), never by the kit. So Liesl's carried healing draught (30001) can be handed to the owner. At parting nothing is withheld, and the next bond issues a fresh one. She can also sell it, and handOver returns her gold to the owner.  (b) Gear can be lost. sendBack builds gear as Items first, then Equipment. handOver decrements hers[ItemId] on the first match by ItemId alone and `continue`s: that item is never stored or dropped, and the mob's Items and Equipment were already cleared. A gifted dagger with id 10009 in Mara's pack is consumed as "hers" before her worn kit dagger. The gifted instance, with its enchantments and quality, vanishes, and the plain kit dagger goes to the owner. The gift's protection (protectItem/protectInstance) is not consulted by handOver.  Severity stays medium. The farmable items are cheap (a dagger, a staff, a draught), but silently destroying a player's possibly upgraded item is real data loss. Both behaviors are new in this PR: handOver/roster.go does not exist at c696c117a, and the every-bond kit replaces the first-meeting-only gate.

</details>

<a id="f088"></a>
### F088 [low] tendOwner writes the owner's name into her mind without the mayRemember consent gate

`modules/aicompanion/combat_style.go:71` · status **confirmed** · reported as low

The module's rule (meeting.go mayRemember) is that nothing naming anyone is written into a mind before the owner has agreed, or while they have turned it off. The reason given is that the mind is what gets sent to the model. tendOwner adds `You mended <owner name> in the thick of it.` unconditionally. Other combat lines that name people are gated with `if m.mayRemember(c)` (combat.go ~941, ~987). A holder who ran `companion-ai off` (Refused) still has a controller and a fighting companion, so this line accumulates in RecentLines and is sent if they later turn consent back on.

**Failure scenario.** The owner of a healer companion (Liesl, mend_below set) types `companion-ai off`, then fights while wounded. Each mend writes their character name into her working memory. They later run `companion-ai on`, and the next prompt includes lines naming them that were written while consent was withdrawn.

**Existing mechanism.** meeting.go mayRemember

**Suggested fix.** Wrap the addLine in `if m.mayRemember(c)`, as the other named combat lines are.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
combat_style.go tendOwner: `c.mind.addLine(Line{Kind: \`event\`, Text: \`You mended \` + u.Character.Name + \` in the thick of it.\`}, m.cfg.WorkingMemoryLines)` with no mayRemember check. meeting.go: `func (m *AICompanionModule) mayRemember(c *controller) bool { return m.consented(c.ownerUserId) }` and its doc: 'nothing naming anyone is written down before her owner has agreed (or while they have turned it off)'.
```

- **confirmed** (low): The claim reproduces from the code as written. tendOwner (new in this PR, combat_style.go:69) adds the line "You mended <owner name> in the thick of it." with no mayRemember check. Its only caller is combat.go:554, inside `if ownerHere`, and the only things guarding it are the fight, profile, cast-readiness and health checks. None of them involve consent. mayRemember (meeting.go:281) is just consented(ownerUserId). Its doc comment says nothing that names anyone is written down before the owner agrees or while they have turned it off, because writing first and gating the send later would send it the moment they agreed. The other lines that name people, both in this PR and before it (combat.go:941, 987; listeners.go:282, 371, 502, 555, 614; travel.go:194, 293, 306; autonomy.go:653; runtime.go:317), are all wrapped in mayRemember, so this line breaks the module's rule. `companion-ai off` (commands.go:803) only sets Consented=false and Refused=true; it keeps the controller and the mind. `companion-ai on` (commands.go:791) clears c.pending but does not clear RecentLines, so the lines written while consent was off stay in her memory, and the module's own comment says that memory is what gets sent. The scenario is reachable in shipped content: liesl.yaml:151 sets mend_below: 60. Severity stays low. The leaked text is only the owner's own character name, it goes to the provider the owner has just agreed to, and the fix is a one-line mayRemember guard (the cast itself does not need gating).

</details>

<a id="f089"></a>
### F089 [low] companion-part with no controller frees her to the roster but destroys the owner's gear instead of handing it over

`modules/aicompanion/commands.go:568` · status **confirmed** · reported as low

Every other path that ends a bond in this PR goes through sendBack, which hands her pack, worn gear and gold to the owner (handOver) and keeps her progress. The no-controller branch of cmdPart is a sibling path. This PR changed it to also call m.unclaim(held, partSentAway), so she is freed back to the Hollow, but it still calls only unbond. unbond calls DestroyInstance on the mob together with everything she carries, and does no handOver or keepProgress. The comment in sendBack says gear is always handed back to the person she leaves, and this branch breaks that.

**Failure scenario.** A player logs in. Before the first sync creates their controller, or while the controller is missing for another reason, they type `companion-part`. Her pack, which holds gear and gold the player gave her, is destroyed along with the mob, and she reappears in the Hollow. Her progress since the last keepFrom snapshot is also lost to the roster.

**Existing mechanism.** roster.go sendBack (the PR's own end-of-bond path with handOver and keepProgress)

**Suggested fix.** When held != nil, route this branch through m.sendBack(user, departure, m.holdsHer(held,user), partSentAway), as revoke and leave do. Keep the bare unbond only for a record whose profile is gone.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
commands.go cmdPart: `c, ok := m.ctrls[user.UserId]; if !ok { _, held := m.bondedCompanionOf(user); if name, err := m.unbond(user, ...); err == nil { if held != nil && m.holdsHer(held, user) { m.unclaim(held, partSentAway) } ...`. unbond: `mobs.DestroyInstance(mob.InstanceId)` with no item transfer. roster.go sendBack: `m.handOver(owner, roomId, p, gear, gold)`.
```

- **confirmed** (low): The defect is in the code as written. In cmdPart's no-controller branch (commands.go:567-579), when no controller exists the PR now looks up the held profile, calls unbond, and then unclaim(held, partSentAway). unbond (commands.go:345-385) calls mobs.DestroyInstance on the live mob and splices the CompanionInfo record out of owner.Character.Companions. It never reads mob.Character.Items, Equipment or Gold, or comp.Items, comp.Equipment or comp.Gold. sendBack (roster.go:324-379) is the path every other ending uses (commands.go:336 release, meeting.go:188, roster.go:312 reclaimFrom, roster.go:627). It collects the gear first, then calls handOver and keepProgress before unclaim. So in this branch the companion is freed to the Hollow and her gear and gold are destroyed, which contradicts the sendBack and handOver contract.  The branch is reachable with a loaded profile (held != nil). Controllers are created only in sync (runtime.go:137) and deleted in detach (runtime.go:433). A user whose bonded record has a loaded profile but who has no controller yet hits this branch. That happens in the gap after login before the next sync round, or after a detach before sync re-attaches. Even with no live mob (InstanceId 0 before sync respawns her), gear kept on the CompanionInfo record (comp.Items, comp.Equipment, comp.Gold, which sendBack's else branch hands over) is dropped with the record. keepProgress is also skipped, so the roster's Kept snapshot is not refreshed.  The window is narrow (a command typed before the first sync round) and the comment says the branch is meant for an orphan bond. So low severity stands. The fix is to route the held != nil case through sendBack(user, departure, true, partSentAway) and keep bare unbond only when no profile is loaded.

</details>
