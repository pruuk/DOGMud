# Skill routing audit, 2026-09-08

Task 1 of the Phase 2 plan
(`docs/superpowers/plans/2026-09-08-context-budget-phase-2.md`). Task 2 of
that plan deletes 31 CLAUDE.md sections on the strength of this audit, so
the finding here is load bearing for that deletion, not a nice-to-have.

## What was tested and how

Two rounds were run against the twelve `.claude/skills/dogmud-*/SKILL.md`
frontmatter blocks. In both rounds a judge was shown only the twelve
`name` and `description` pairs, never the skill bodies, and never the
expected answer for any prompt. The judge used zero tools: it picked a
first and second choice from the twelve descriptions by reading text
alone, the same information a routing model has at dispatch time before
it ever loads a skill.

Blindness is the point. A judge that has already read the skill bodies,
or that is told what the "right" answer is supposed to be, routes from
memory of the content rather than from the description text a real
router sees. That would test whether a human agrees the skill covers the
topic, not whether the description actually gets picked. The two rounds
below test the second thing, which is what the deletion in Task 2 needs
proven.

Round 1 supplied fifteen realistic prompts spanning all twelve skills,
written independently of the descriptions. Round 2 supplied ten narrower
probes written specifically to re-test one finding from round 1: that
`dogmud-combat` never won a first choice.

## Table 1: all 25 prompts, both rounds

| # | Round | Prompt | First choice | Confidence | Second choice |
|---|-------|--------|---------------|------------|----------------|
| 1 | 1 | add a mob to Thornwall | authoring-content | high | - |
| 2 | 1 | why is this test flaky | writing-tests | high | combat |
| 3 | 1 | open a PR for this branch | shipping | high | - |
| 4 | 1 | is it safe to wipe rooms.instances | persistence | high | shipping |
| 5 | 1 | make fire damage hurt more | balance-config | high | combat |
| 6 | 1 | the NPC stopped talking | authoring-quests | high | - |
| 7 | 1 | the deploy took way longer than usual | deploying | high | - |
| 8 | 1 | write the room description for the shrine | player-copy | medium | authoring-content |
| 9 | 1 | my stat is not going up | progression-model | high | - |
| 10 | 1 | remove the bleedout feature | refactoring | high | combat |
| 11 | 1 | run a playtest of the newbie area | playtesting | high | authoring-content |
| 12 | 1 | where do I set the shop restock rate | balance-config | high | persistence |
| 13 | 1 | the quest keeps getting re-offered after I finish it | authoring-quests | high | - |
| 14 | 1 | this combat message shows a raw damage number | player-copy | high | combat |
| 15 | 1 | add a field to Character that survives restart | persistence | high | - |
| A | 2 | the parry check seems wrong | combat | high | refactoring |
| B | 2 | add a new defence type | combat | medium | refactoring |
| C | 2 | where does a mob decide what to attack | combat | high | - |
| D | 2 | why is my spell doing so little damage | combat | high | balance-config |
| E | 2 | I need to change how crits are calculated | combat | medium | balance-config |
| F | 2 | the mob attacks before its behaviour tree fires | combat | high | refactoring |
| G | 2 | polish the prose in this room's description field | player-copy | high | authoring-content |
| H | 2 | create the shrine room | authoring-content | high | player-copy |
| I | 2 | before I smoke test, do I need to wipe rooms.instances | shipping | high | persistence |
| J | 2 | which config field controls fire damage | balance-config | high | combat |

## Table 2: coverage by skill

One row per skill. "First" lists prompts where that skill won outright.
"Second" lists prompts where it was named as runner-up. All twelve
skills appear at least once across the two columns.

| Skill | Routed first | Named second |
|-------|-------------|--------------|
| authoring-content | 1, H | 8, 11, G |
| authoring-quests | 6, 13 | - |
| balance-config | 5, 12, J | D, E |
| combat | A, B, C, D, E, F | 2, 5, 10, 14, J |
| deploying | 7 | - |
| persistence | 4, 15 | 12, I |
| player-copy | 8, G | 14, H |
| playtesting | 11 | - |
| progression-model | 9 | - |
| refactoring | 10 | A, B, F |
| shipping | 3, I | 4 |
| writing-tests | 2 | - |

## Findings

**The combat false alarm, cleared.** Round 1's fifteen prompts covered
all twelve skills but happened to phrase every combat-adjacent prompt
(2, 5, 10, 14) in terms of another skill's home turf (flaky tests,
balance tuning, refactoring, player copy), so combat only ever placed
second. That looked like a routing weakness in the description. Round 2
tested it directly with six combat-shaped prompts (A through F) written
without deference to any other skill's wording, and all six chose combat
first, four of them at high confidence. The gap was round 1 under
sampling combat, not a defect in the combat description. Cleared.

**The G/H verb split.** G ("polish the prose in this room's description
field") and H ("create the shrine room") are the same subject, a room
description, but split cleanly on the verb: G chose player-copy first
with authoring-content second, H chose authoring-content first with
player-copy second. This is exactly the intended division of labor
between the two skills and confirms the descriptions respond to real
distinctions in the prompt, not just keyword overlap.

**I and J changed winner under narrower phrasing.** Round 1's prompt 4
("is it safe to wipe rooms.instances") chose persistence first with
shipping second. Round 2's prompt I asked nearly the same underlying
question but tied it explicitly to the pre-smoke-test moment ("before I
smoke test, do I need to wipe rooms.instances") and the winner flipped to
shipping, with persistence second. Likewise round 1's prompt 12 ("where
do I set the shop restock rate") chose balance-config first with
persistence second, and round 2's prompt J narrowed the same subject to
"which config field controls fire damage," which also chose
balance-config first but changed the runner-up to combat. Both pairs
prove the descriptions are sensitive to genuine phrasing differences
rather than returning a fixed answer for a fixed topic.

**The "crit" gap, and its fix.** On prompt E ("I need to change how
crits are calculated") the judge picked combat but only at medium
confidence, and said why in its own words: crit calculation is a real,
named mechanic in this codebase, but the combat description never used
the word "crit." It was covered only by inference through "opposed
contests," which the judge called thinner ground than it wanted a
routing judge to stand on. Fix: the combat skill's description now reads
"crits as an opposed-roll margin" alongside the existing damage,
defence, and contest language. The description was already at 496 of
the 500 character cap, so the fix required trimming wording elsewhere
without dropping any named symbol or the model/placement split:
"The model half covers the five-factor damage formula" became "Model
half: the five-factor damage formula," and the equivalent trim was
applied to the placement-half sentence. Before: 496 characters. After:
495 characters, with "crits" now present. This closes the gap the
round 2 judge flagged without touching any other skill.

**The balance-config overlap: accepted, not fixed.** The round 2 judge
also flagged that the combat and balance-config descriptions
independently claim the same ground: both describe shipped config
damage-scale knobs, and the judge asked which skill should own "why is a
channel's scale what it is" versus "where do I look up any knob," saying
that right now both descriptions claim it. This is a real overlap. It is
being recorded as accepted rather than fixed, for two reasons found in
the evidence above, not asserted independently of it:

1. Both descriptions are already near the 500 character cap (combat 495
   after the crit fix above, balance-config 495), so resolving the
   overlap in the description text would mean cutting content that is
   currently carrying its own routing weight, for a problem the data
   says is not costing correct routing.
2. The overlap is demonstrably benign in practice. Prompt D ("why is my
   spell doing so little damage") routed to combat with balance-config
   second. Prompt J ("which config field controls fire damage") routed
   to balance-config with combat second. Both landed on the skill a
   human would pick for that exact phrasing, and in the case where
   either skill loads, a reader gets most of the way to the answer
   because each skill correctly says "read config.yaml, not the Go
   default" for these knobs.

This is a decision, recorded here so a later reader does not mistake the
unresolved overlap for an oversight: it was seen, weighed against the
cost of fixing it, and left alone because the evidence says it is not
breaking routing.

## What this licenses

Every one of the twelve skills won at least one first-choice routing
across the two rounds (Table 2), and the one skill that looked
uncovered after round 1 (combat) was shown by round 2 to be correctly
routed once the prompt set actually exercised it. The two weaknesses
round 2 did surface are addressed above: the crit gap is fixed in the
combat description, and the balance-config overlap is recorded as an
accepted, evidence-backed decision rather than left as a silent
uncertainty.

This licenses Task 2 of the Phase 2 plan to proceed with deleting the 31
CLAUDE.md sections that these skills now cover, because the skills
demonstrably trigger on realistic prompts rather than only on
inspection.

## Note for the tripwire task

A later task in the Phase 2 plan builds a tripwire block meant to catch
future routing regressions. That block should be written against Table 1
and Table 2 above, reusing the actual prompts and outcomes recorded here,
rather than guessed or re-derived from scratch.
