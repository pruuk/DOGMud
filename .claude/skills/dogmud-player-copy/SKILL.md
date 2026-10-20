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

Player-facing copy follows the global no-dash rule in `~/.claude/CLAUDE.md`
("Writing"): no em or en dashes, `&mdash;`/`&ndash;` in web copy included;
hyphens in compound words are fine.

## Menu language

Numbered menus (NPC conversations, builder lists, confirms) are drawn by
`internal/menu`. Its full reference is `internal/menu/context.md`. The rules
for the text in them:

1. An info label is what the player says, in the player's voice, usually a
   question. A quest entry's label is the quest's title, and its `Detail` line
   is what the player says: a new quest offers help ("Do you want help settling
   the feud at the well?"), a hand-in reports the result ("I've settled the
   feud at the well.").
2. Never name the NPC in the third person and never use narrator voice in a
   label or a player line.
3. One line each, at most 50 characters for a label and 70 for a `Detail` line.
4. ESL-clear: plain words, no idioms, no em or en dashes, no raw numbers for
   stats or rewards.
5. Specific, not generic. No two labels in one menu read alike.
6. Leave lines may be in character ("Goodbye.") and default to "Leave".
7. Confirms are two entries; for actions that cannot be undone, the entries
   name the action.
8. Speak in the NPC's manner where it fits: gruff and plain, educated and
   pretentious.
9. Validators enforce the checkable rules (length, no dashes, no duplicate
   labels in one menu, no "Ask:" / "Task:" prefix): `menu.LintLabels`.

## Sources

- [[feedback_esl_clear_language_player_copy]]
- [[feedback_helpfile_loadout_vs_reaction_advice]]
- `~/.claude/CLAUDE.md` "Writing" (the dash rule; its memory file was retired
  2026-10-15 once the rule went global)
