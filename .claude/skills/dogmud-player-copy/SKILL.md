---
name: dogmud-player-copy
description: Use when writing any text a player will read - room descriptions, combat and spell messages, help files, patch notes, NPC lines, MOTD. Covers the 80-character hard wrap, the rule against showing raw numbers for damage, healing, armor or durations, ESL-clear phrasing, and framing combat advice as pre-combat loadout rather than in-the-moment reaction.
---

This is the small stuff that gets skipped in a hurry: raw numbers leaking
into combat text, lines that wrap ugly in a fixed-width client, idioms an
ESL player cannot parse, and combat advice that quietly assumes a mid-fight
reaction the engine will not let the player take.

## Never show a raw number

This is the rule most often broken, and the one that leaks internal balance
values to players, so it leads.

Never display raw numeric values (damage, healing, armor points, round
counts, etc.) directly to the player in combat or spell messages. Use
descriptive language instead:

- **Damage**: use `combat.GetDamageDescription(amount, targetMaxHP)` →
  "light wounds", "serious wounds", etc.
- **Healing**: use `combat.GetHealDescription(amount, targetMaxHP)` →
  "light mending", "moderate restoration", etc.
- **Durations / other numbers**: describe the effect, not the mechanics
  ("A barrier forms around you." not "A barrier forms for 10 rounds.")
- **Armor / stat bonuses**: describe the feel ("bolsters your defenses" not
  "+33 armor")

Displaying raw numbers breaks immersion and leaks internal balance values to
players. The exception is the `status` command's stat sheet: that is a
deliberate mechanical display.

## Wrap at 80

All player-visible text (descriptions, help files, templates,
ANSI-formatted tables) must wrap at **80 characters per line**. MUD clients
render in fixed-width columns: long lines get cut off or wrap uglily. When
writing multi-line `description:` fields, room descriptions, or help
templates, hard-wrap prose at ~78-80 chars.

## ESL-clear

Avoid opaque English idioms in player-facing and announcement copy (README,
listing blurbs, MOTD, splash text, helpfiles). Part of the audience is non-
native English speakers, including the project's recurring playtester, and
an idiom whose meaning cannot be composed from its words ("has teeth", "cut
to the chase", "on the nose") reads as noise to them. Prefer vivid-but-
literal phrasing ("belief changes the world") over opaque idioms; the
"belief has teeth" opener was rewritten to "belief matters" for exactly this
reason. In-game NPC dialogue may still use idioms deliberately as character
voice. [[feedback_esl_clear_language_player_copy]]

## Framing combat advice

When a helpfile gives "X is a counter to Y" tactical advice for a
grapple, position, or control-axis mechanic, frame it as loadout planning
the player does before combat, not a reaction during it: "carry a dagger
in your offhand" rather than "swap to a dagger when grappled." DOGMud
explicitly disallows weapon swaps mid-fight, so reactive-sounding advice
describes a command the player cannot actually issue and will read as
broken. Players prepare for threats; they do not react to them in real
time. [[feedback_helpfile_loadout_vs_reaction_advice]]

## No em or en dashes

Player-facing copy also follows the project's no-dash preference: see
[[feedback_no_em_dashes_in_prose]] for the rule and rationale.

## Sources

- [[feedback_esl_clear_language_player_copy]]
- [[feedback_helpfile_loadout_vs_reaction_advice]]
- [[feedback_no_em_dashes_in_prose]] (preference, cited above, not folded)
