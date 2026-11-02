# #375 Phase 7 Pothole Coulee, Plan 5: Close (Playtest and Wrap) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Prove the rebuilt Pothole Coulee works for a real newcomer, file every finding, and close the zone pass.

**Architecture:** One six-seat `playtestrun scenario` on master after plans 1 to 4 have merged: two coaching-free newcomers, a hub seat, a finisher seat, a Shadows seat and a consume seat. Findings become GitHub issues; the pass closes only when the playtest has run and its issues are filed.

**Tech Stack:** `cmd/playtestrun` (Docker env), the external `mudagent` harness (`../gomud-playtest-harness`), YAML scenario and goals files under `tools/playtest/`.

**Spec:** docs/superpowers/specs/2026-10-30-quest-arc-phase7-pothole-coulee-design.md

**Branch / worktree:** `chore/375-p7-coulee-close`, worktree `C:/tmp/dogmud-375-p7-close` off master **after plan 4 has merged**.

---

## Facts verified against source (master `8432192f1`, 2026-10-30)

| Fact | Source |
|---|---|
| Scenario command: `playtestrun scenario --checkout PATH --scenario PATH [--wall-clock 45m] [--force]`; `status` and `stop` take `--checkout PATH --run ID` | `internal/playtestrun/context.md:61-64` |
| Scenario shape: `name`, `mode: party`, `summary`, `on_actor_stop`, `budgets.wall_clock`, `requires.max_connections`, `roster[] {id, personality, goals}`, `group_goals[] {id, do, verify}` | `tools/playtest/scenarios/375-p5-quest-log.yaml` |
| Seat goals shape: `ephemeral: {profile, start_room, overlays: {set_quest_tokens, grant_items}, budgets}` then `goals:`; a newcomer seat uses `ephemeral.creation_flow: true` with a `creation_rationale` | `tools/playtest/goals/scenarios/375-p5-quest-log/hunt.yaml`; `tools/playtest/goals/newbie-naive.yaml` |
| `set_quest_tokens` gives no items an offer would hand over; add them with `grant_items` | memory, 2026-10-28 slice 1 lessons |
| Goals must describe the step the tokens leave the seat ON | same |
| `playtestrun stop` exits 0 and leaves the container; remove it with `docker rm -f` | `dogmud-playtesting` skill |
| Seat subagents sharing one scratchpad must name helper scripts after their actor id | memory, 2026-10-28 slice 3 lessons |
| Seats leave `tail.exe` bridge processes; stop yours by PID | memory, 2026-10-26 lessons |
| Testers observe only; findings go to GitHub issues (`bug`, `from-playtest`, an `area:*` label); exploit-class defects go to the owner privately | `dogmud-playtesting` skill |
| Toke's fallback room is 5203, Esk's 5215 | `quests/88-tokes_road.yaml`, `quests/89-esks_road.yaml` |

## Values that come from plans 2 to 4 (derive at execution, do not guess)

Seat goals below need step ids and room ids that plans 2 to 4 create. Before Task 1, read them
from the merged files and write them into the goals:

```bash
cd C:/tmp/dogmud-375-p7-close
grep -E '^(89|9[0-9]|10[01])-' _datafiles/world/dogmud/quests/step-ids.txt
grep -n -E 'start_room|room:' _datafiles/world/dogmud/quests/9[7-9]-*.yaml _datafiles/world/dogmud/quests/10[01]-*.yaml | head -40
grep -h -E '^\s*name:' _datafiles/world/dogmud/mobs/pothole_coulee/*.yaml | sort
```

Record in the plan's execution notes: the cutpurse's mob id, name and room; the lamplighter's
name; the last step id of quest 100 before its hand-in; the item ids of the hooded lantern,
lockpicks, practice token, Herbal Tea, a ration and Spiced Wine as shipped after plan 1.

---

### Task 1: Scenario and seat goals

Suggested executor: Opus (the newcomer goals must stay coaching-free).

**Files:**
- Create: `tools/playtest/scenarios/375-p7-coulee.yaml`
- Create: `tools/playtest/goals/scenarios/375-p7-coulee/naive-a.yaml`
- Create: `tools/playtest/goals/scenarios/375-p7-coulee/naive-b.yaml`
- Create: `tools/playtest/goals/scenarios/375-p7-coulee/hub.yaml`
- Create: `tools/playtest/goals/scenarios/375-p7-coulee/finisher.yaml`
- Create: `tools/playtest/goals/scenarios/375-p7-coulee/shadows.yaml`
- Create: `tools/playtest/goals/scenarios/375-p7-coulee/consume.yaml`

- [ ] **Step 1: Write the scenario**

```yaml
# #375 phase 7, Pothole Coulee pass: the whole newcomer flow. Two newcomers
# play with no coaching from landing on. Four seeded seats check the hub, the
# end of the Coulee (Esk's Road and the repeatable flavour lines), the new
# Shadows line, and quaff / eat / drink.
name: 375-p7-coulee
mode: party
summary: >-
  A newcomer is walked from Dewey to the rite, Toke and a first trade without
  coaching; Toke starts any line; each line ends with Report to Toke; all eight
  done sends the player to Esk; the Shadows line teaches light, sneak, steal,
  plant and lockpicking.
on_actor_stop: continue
budgets:
  wall_clock: 60m
requires:
  max_connections: 20
roster:
  - id: naive-a
    personality: feel-tester
    goals: goals/scenarios/375-p7-coulee/naive-a.yaml
  - id: naive-b
    personality: feel-tester
    goals: goals/scenarios/375-p7-coulee/naive-b.yaml
  - id: hub
    personality: bug-finder
    goals: goals/scenarios/375-p7-coulee/hub.yaml
  - id: finisher
    personality: bug-finder
    goals: goals/scenarios/375-p7-coulee/finisher.yaml
  - id: shadows
    personality: bug-finder
    goals: goals/scenarios/375-p7-coulee/shadows.yaml
  - id: consume
    personality: bug-finder
    goals: goals/scenarios/375-p7-coulee/consume.yaml

group_goals:
  - id: walked-not-lost
    do: The newcomer seats play from character creation with no route hints.
    verify: Each reaches Toke and finishes at least one line's first quest, or reports exactly where it got lost and what the screen said.
  - id: loop-closes
    do: Seeded seats take, finish and hand in quests across the hub, a Shadows line run and the Esk ending.
    verify: Every quest starts and ends at an NPC menu; every line ends with Report to Toke; Toke offers Esk's Road only after all eight.
```

- [ ] **Step 2: Write the two newcomer seats (coaching-free; do not add routes or commands)**

`naive-a.yaml`:

```yaml
# NEWCOMER seat A. Deliberately minimal: the test is whether the game itself
# carries a brand-new player from landing to a first trade. Do not add route
# hints or command lists. Quote game lines verbatim when you report a problem.
goals:
  - >-
    You have never played a MUD before. Create a brand-new character and play
    whatever the game puts in front of you, relying only on what the game
    itself tells you. If you feel finished, or you cannot tell what to do
    next, say so and stop: being lost is itself a finding.

ephemeral:
  creation_flow: true
  creation_rationale: >
    Brand-new character required: this seat grades whether the rebuilt
    antechamber and Coulee opening teach a first-time player unaided.
  budgets:
    wall_clock: 50m
```

`naive-b.yaml`:

```yaml
# NEWCOMER seat B. Same rule as seat A: no routes, no commands. This seat is
# told only that it likes to learn many things, so it should try more than
# one of the trades the game offers. Quote game lines verbatim.
goals:
  - >-
    You have never played a MUD before, and you like to learn a little of
    everything. Create a brand-new character and play whatever the game puts
    in front of you, relying only on what the game itself tells you. When the
    game offers you a choice of things to learn, try more than one. If you
    cannot tell what to do next, say so and stop.

ephemeral:
  creation_flow: true
  creation_rationale: >
    Brand-new character required: grades the hub choice for a player new to
    MUDs.
  budgets:
    wall_clock: 50m
```

- [ ] **Step 3: Write the hub seat**

```yaml
# HUB seat: Find Your Footing is done, so Crier Toke should offer the start of
# every trade. You start in Toke's room. Quote every game line VERBATIM.
ephemeral:
  profile: early
  start_room: 5203
  overlays:
    set_quest_tokens: ["30-end", "88-end", "31-end"]
  budgets:
    wall_clock: 40m

goals:
  - >-
    Type `look` and quote how Crier Toke's name is shown, including any mark
    before it. Type `talk toke` and quote the whole menu.
  - >-
    Take the fighting trade's first quest from Toke's menu. Type `quests` and
    quote the entry. Type `talk toke` again and say whether that quest is
    still offered.
  - >-
    Take a second trade from Toke. Type `quests` and say whether both are
    listed. Type `hint` and quote it.
  - >-
    Follow the fighting quest's hint to the trainer it names and finish that
    quest. Quote every progress line and the hand-in. Then type `talk` to the
    trainer and quote the menu: say whether it offers you the next quest and
    where it sends you.
```

- [ ] **Step 4: Write the finisher seat** (fill `<cutpurse>`, `<cutpurse room>` and quest 100's
  last-step token from the derivation step above)

```yaml
# FINISHER seat: seven trades are finished and reported to Toke; the Shadows
# trade's finale is handed in, so the cutpurse should offer Report to Toke.
# Quote every game line VERBATIM.
ephemeral:
  profile: early
  start_room: <cutpurse room>
  overlays:
    set_quest_tokens: ["30-end", "88-end", "31-end", "90-end", "91-end", "92-end", "93-end", "94-end", "95-end", "96-end", "100-end"]
  budgets:
    wall_clock: 40m

goals:
  - >-
    Type `talk <cutpurse>` and quote the menu. Take Report to Toke.
  - >-
    Go to Crier Toke (follow the quest's hint) and hand it in. Quote the
    hand-in. Type `talk toke` and say whether Esk's Road is offered; take it.
  - >-
    Leave Toke's room and come back in eight times, waiting a few seconds each
    time. Quote every line Toke says to you on arrival.
  - >-
    Follow Esk's Road's hint to Warden Esk and hand it in. Quote it. Then type
    `talk esk` and quote the menu. Leave and re-enter Esk's room eight times
    and quote anything he says on arrival. Do NOT walk through the arch.
```

- [ ] **Step 5: Write the Shadows seat**

```yaml
# SHADOWS seat: Find Your Footing is done. Take the Shadows trade from Toke and
# play it to the end. The mine is dark. Quote every game line VERBATIM.
ephemeral:
  profile: early
  start_room: 5203
  overlays:
    set_quest_tokens: ["30-end", "88-end", "31-end"]
  budgets:
    wall_clock: 50m

goals:
  - >-
    Type `talk toke` and take the trade about light and shadows. Follow each
    hint the game gives you, one step at a time, through every quest that
    trade gives you, to its last hand-in. Quote every progress line, hint and
    hand-in.
  - >-
    Along the way, open every menu line marked [*] on the two teachers and say
    in one line what each taught you.
  - >-
    Report anything that did not happen the way the hint said, and any time
    you could not see.
```

- [ ] **Step 6: Write the consume seat** (fill item ids from the derivation step)

```yaml
# CONSUME seat: you carry a healing salve, Herbal Tea, Spiced Wine and a
# ration. Quote every game line VERBATIM.
ephemeral:
  profile: early
  start_room: 5203
  overlays:
    grant_items: [<healing salve id>, <herbal tea id>, <spiced wine id>, <ration id>]
  budgets:
    wall_clock: 20m

goals:
  - >-
    Type `drink salve` and quote the reply. Then `quaff salve` and quote it.
  - >-
    Type `drink tea` and quote it. Type `eat ration` and quote it. Type
    `drink wine` and quote it.
  - >-
    Type `help quaff`, `help drink` and `help eat` and quote the first lines
    of each.
```

- [ ] **Step 7: Commit**

```bash
git add tools/playtest/scenarios/375-p7-coulee.yaml tools/playtest/goals/scenarios/375-p7-coulee/
git commit -F - <<'EOF'
test(playtest): #375 phase 7 Pothole Coulee scenario

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

### Task 2: Run the scenario

Suggested executor: the controller (it drives one subagent per seat).

**Files:** none

- [ ] **Step 1: Check the harness is present**

Run: `ls ../gomud-playtest-harness/mudagent.exe`
Expected: the file is listed. If not, restore it as the `dogmud-playtesting` skill says.

- [ ] **Step 2: Start the env**

```bash
cd C:/tmp/dogmud-375-p7-close
go build -o playtestrun.exe ./cmd/playtestrun
./playtestrun.exe scenario --checkout C:/tmp/dogmud-375-p7-close --scenario tools/playtest/scenarios/375-p7-coulee.yaml --wall-clock 60m
```

Expected: a run id and six actor bridge folders under `tools/playtest/.run/<run_id>/actors/`.

- [ ] **Step 3: Drive the seats** per `.claude/commands/playtest-scenario.md`: one subagent per
  seat, each told its actor id, to name any helper script after that id, to pace commands a
  couple of seconds apart, and to observe only.

- [ ] **Step 4: Stop and clean up**

```bash
./playtestrun.exe stop --checkout C:/tmp/dogmud-375-p7-close --run <run_id>
docker ps -a | grep <run_id>
docker rm -f <container id>
docker ps -a | grep <run_id>
rm playtestrun.exe
```

Expected: the last `docker ps` prints nothing. Stop any `tail.exe` the seats started, by PID only.

### Task 3: Triage and file findings

Suggested executor: Opus.

- [ ] **Step 1:** Collect each seat's report from `tools/playtest/reports/` (gitignored: extract now).
- [ ] **Step 2:** Recheck each defect against master. Search open issues first:
  `gh issue list --repo pruuk/DOGMud --search "<words>"`.
- [ ] **Step 3:** Fix in this branch anything that is a defect in this pass's own work (a hint
  that names the wrong command, a broken gate, a missing hand-in). Each fix gets its test or
  boot check and its own commit. Rerun only the affected seat.
- [ ] **Step 4:** File the rest: `gh issue create --repo pruuk/DOGMud --label bug --label
  from-playtest --label area:content` (group copy nits into one checklist issue; add
  `owner-decision` where a ruling is needed). An exploit-class defect goes to the owner, not a
  public issue.
- [ ] **Step 5:** Commit any scenario fixture fixes.

### Task 4: Close the pass

Suggested executor: the controller.

- [ ] **Step 1: Confirm the backlog is clear for the zone**

Run: `grep -E '^[a-z-]+ (28|29|3[0-9]|4[0-9]|5[0-9]|76|88|89|9[0-9]|10[01]) ' _datafiles/world/dogmud/quests/loop-backlog.txt`
Expected: no output (grep exits 1; run it on its own, not in an `&&` chain).

- [ ] **Step 2:** Run the gate from `dogmud-shipping` on the branch: gofmt, build, `go test
  ./...`, then `DOGMUD_BOOT_SMOKE=1 go test -v -run 'TestSmoke_|TestQuestDoor|TestDestinationDoor|TestQuestLoop' .`
  (the root loop tests SKIP without the variable; expect `--- PASS` lines), lint 0 issues, boot check.
- [ ] **Step 3:** Update `docs/roadmaps/QUEST_DIALOGUE_ROADMAP.md` phase 7: Pothole Coulee done,
  with the playtest run id and the issues filed.
- [ ] **Step 4:** Push and open the PR:
  `gh pr create --repo pruuk/DOGMud --base master --head chore/375-p7-coulee-close` with a body
  that lists the run id and filed issues and says "Closes #315, closes #689, closes #690,
  closes #694, closes #695, closes #697" only for issues whose every item is fixed; for any
  issue with an item left, write "Part of #N" instead.
- [ ] **Step 5:** After merge: remove the worktree (`git worktree remove C:/tmp/dogmud-375-p7-close`),
  confirm each closed issue closed, and record harness lessons (not findings) in memory.
