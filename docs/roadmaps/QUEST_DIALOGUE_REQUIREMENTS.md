# Quest and Dialogue Arc: Requirements and Done-When

> The one-page target for epic #375. Every spec, plan and review in the arc checks against
> this page. How we get there lives in `QUEST_DIALOGUE_ROADMAP.md`.
> Set by the owner 2026-10-20. Changing a line here is an owner decision.

## The goal

Talking to an NPC is one command and a numbered menu. Every quest is a closed loop of small,
hinted steps that starts and ends at an NPC, and a new player is walked from one quest giver
to the next.

## What the player sees

1. An NPC with something new shows a marker before their name: `[!]` new quest,
   `[?]` ready to hand in, `[*]` new info.
2. `talk <npc>` shows a numbered menu, each line led by its marker.
3. Pick a number. Info: the NPC answers, and the room hears both sides. Quest: the pitch,
   the objective, the reward, then Accept or Leave.
4. Come back to hand in. Finishing offers the next quest in the chain.
5. Once taken or heard, a topic stops marking but can always be heard again.
6. Abandon or focus a quest by command or by right-click in the web client quest panel.

## Rules

1. Every quest starts and ends at an NPC questgiver. No exceptions.
2. Every step is one action, or at most "go here, then do one action". 40 small steps beat
   5 tangled ones.
3. Every step has a hint.
4. Quests are picked up one at a time. Nothing grants a batch.
5. Related quests chain: hand-in offers the next one, often a bridge quest to the next giver.
   New areas hold the player's hand.
6. Ineligible quests are hidden. A breadcrumb line telling the player to come back later is
   strongly encouraged.
7. One command, `talk`. `ask` points the player to `talk`.
8. One code path for quest topics and info topics.
9. Marker priority on a name: `[?]`, then `[!]`, then `[*]`. The symbol carries the meaning,
   never the color alone.
10. `[*]` lights only for topics the writer flags. Small talk never marks.
11. Heard topics are saved by stable key, never by menu position. Quest state comes from quest
    tokens.
12. Abandon keeps progress. Re-taking resumes where the player left off.
13. Repeatable quests have a frequency and a repeat limit.
14. Quest steps progress on real actions (cast, explore, acquire, talk, observe, use), never on
    a typed noun the player has to guess.
15. Minimap guidance points at a room, an NPC, a zone or an area. "Destination unknown" is
    rare and logs a warning.
16. Pinnacle recipes are sold, not quested. Info that cannot be a closed loop is a topic, not
    a quest.
17. LLM dialogue (forks only; DOGMud keeps it off) uses the same topic structure and behaves
    like written dialogue.

## The arc is done when

- [ ] `talk` opens the menu for every NPC with dialogue; `ask` only redirects.
- [ ] Every numbered menu in the game, player or admin, uses the one menu primitive.
- [ ] Markers show on names and menu lines and clear correctly across logout and restart.
- [ ] The boot validator refuses a quest that does not start and end at an NPC, a step with no
      hint, and a step that is more than one action.
- [ ] No quest is granted except by accepting it from an NPC menu.
- [ ] Every chain offers its next quest at hand-in.
- [ ] Zero typed-noun `room_interact` triggers remain.
- [ ] Every quest step has a minimap target; any "destination unknown" is logged and listed.
- [ ] Abandon and focus work by command and in the web client; abandon cannot reset progress.
- [ ] Repeatable quests enforce both frequency and limit.
- [ ] All dialogue files are ported to keyed, labelled topics with `[*]` decided.
- [ ] All quests are rewritten to atomic, hinted, action-triggered steps.
- [ ] Zero em or en dashes in dialogue, quests and conversations, and the dash guards no
      longer exempt those paths.
- [ ] Pinnacle recipes 79 to 87 are sold; the Pothole Coulee portal quest is an info topic.
- [ ] The web builder cross-links quests, rooms, zones, items and mobs, and teaches the topic
      fields with mouseover hints.
- [ ] Every zone pass has passed its adversarial playtest.
- [ ] Players are taught the system: the Pothole Coulee opening explains the markers and the
      `talk` menu in play, and the `talk`, `ask` and `quests` help files
      (`templates/help/talk.template`, `ask.template`, `quests.template`) describe it.
- [ ] Every package the arc touches has an up-to-date `context.md`, and new packages ship one.
- [ ] Sub-issues of #375 are closed: #290, #315, #588, #613, #616, #625.

### Tests and guards (belt and suspenders)

Every guard is proven able to fail: each ships with a fixture that breaks it and a test that
sees it break.

- [ ] **One validator, three doors.** Boot, the web builder's save and a CI sweep all call the
      same quest and dialogue validators. No rule lives only in the builder or only at boot.
- [ ] **Shipped-content sweep.** A boot smoke test runs every validator over every shipped
      quest and dialogue file and fails on any error.
- [ ] **Every quest is completable.** A headless walkthrough test drives each quest from its
      giver's offer through every step's trigger to hand-in, and proves each required item has
      a source (drop, shop, or a giver hands it over).
- [ ] **Malformed-fixture suite.** One broken fixture per rule (no hint, two actions in a step,
      no NPC start, trigger grant, duplicate key, bare-scalar list, dangling reference, chain
      to a missing quest). Boot refuses each one and the builder refuses to save each one.
- [ ] **Stable-id manifests.** Committed manifests of topic keys and quest step ids. CI fails
      when a shipped key or step id is renamed or removed without a migration, because saved
      characters point at them.
- [ ] **Persistence round trips.** Heard topics, quest progress, abandon state, repeat counts
      and cooldowns survive save, load, logout and restart.
- [ ] **Exploit tests.** Abandon then re-take cannot repeat a reward; a repeatable quest stops
      at its limit and respects its cooldown; giving an item twice cannot complete twice.
- [ ] **Marker tests.** Each of `[!]`, `[?]`, `[*]` appears and clears in every state change,
      including abandon, ineligibility and hidden or dim NPCs.
- [ ] **Code guards.** Tests that fail if a numbered-menu parse appears outside the menu
      package, if `ask` grows topic handling again, or if `room_interact` returns.
- [ ] **Dash guards** cover dialogue, quests and conversations with no exemptions.
- [ ] **Builder round trip.** Everything the builder can save loads unchanged at boot.

## Not in this arc

Web client visual compaction (Malia's web client arc). Kept as they are: progression toasts,
step hints, minimap arrows, and the reward types (gold, stats, skills, items, spell and recipe
discovery).
