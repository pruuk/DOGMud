# Follow-up slices D and E: Purge Affliction reaches a companion, and a buff flag must be a real flag

The last two slices of the messaging M3 item 5a follow-ups (order ruled
2026-09-11: A, F, B, C, D+E). Owner rulings: Purge Affliction "should be able
to target others OR self" (09-11); buff flag names get a guard "as a SEPARATE
follow-up" (09-11); `poison-immunity` is WIRED, not dropped, and Savant's
Infusion becomes significantly more expensive than Essence of Growth (09-12).

## Facts verified against source (2026-09-12, master `327822986`)

| Fact | Where |
|---|---|
| The purge dispatch handles a player target or falls back to self; `TargetMobInstanceIds` is never consulted, so a cast at a charmed companion purges the CASTER | `internal/hooks/spell_resolution.go:272-281` |
| `resolvePurgeAffliction(user, target *users.UserRecord)` narrates through `SendTrio` with `spellAudience`, then cancels `buffs.Poison` buffs and removes `ConditionPoisoned` on the target | `internal/hooks/spell_purgeaffliction.go:14-46` |
| Other help spells already resolve `TargetMobInstanceIds[0]` for a companion target | `internal/hooks/spell_resolution.go:217-218` |
| The engine declares 33 `Flag` constants; the dogmud buff files use exactly ONE the engine does not know: `poison-immunity` on `64-stone_stomach.yaml` (item 30046, description "Nothing can turn your stomach now") | `internal/buffs/buffspec.go:28-92`, exact comm of yaml flags vs constants |
| Buff flags are not validated at load: `LoadDataFiles` asserts only canonical names; `BuffSpec.Validate` checks tick fields only | `internal/buffs/buffspec.go:265-283, 220-245` |
| Precedent for a boot guard that panics naming the offender: `species.ValidateSpeciesBuffIds` wired in `main.go:1694` | slice F |
| Poison arrives two ways: a buff carrying the `poison` flag (`Buffs.AddBuff` / `AddBuffScaled`, the primitives every path reaches) and `Character.AddCondition(ConditionPoisoned, ...)` (two spell sites) | `internal/buffs/buffspec.go:54`, `internal/buffs/buffs.go:186-260`, `internal/characters/conditions.go:89`, `spell_resolution.go:612,1626` |
| Essence of Growth (30053) and Savant's Infusion (30054) both `value: 60`; the mutagenic pair is 80 / 100 | the four item files |
| **NO dogmud buff carries the `poison` flag.** 39 Venom, 40 Spore Toxin and 78 Toxic Cloud are health tick buffs with no `flags:` at all, so `CancelBuffsWithFlag(buffs.Poison)` in Purge Affliction and Cleansing Wave cancels nothing, and the poisoned adjective never shows | CRLF-tolerant grep over the buff files; `spell_purgeaffliction.go:42`, `spell_resolution.go:995`, `characters/description.go:166` |

## Defects

1. **Purge Affliction ignores a mob target.** A caster who names a poisoned
   companion purges themselves and reads "You purge the afflictions from your
   body." Found by the 5a playtest.
2. **A misspelled or unknown buff flag loads silently and does nothing.** The
   Cat's Eye Draught shipped broken this way (`night-vision` for
   `nightvision`, fixed in 5a); `poison-immunity` is the one still in the data.
3. **Stone Stomach promises poison immunity and delivers only a dexterity
   penalty.**
4. **Savant's Infusion is stronger than Essence of Growth at the same price**
   (since #125).
5. **The three toxin buffs carry no `poison` flag**, so the two purge spells
   are inert against them and the poisoned adjective never appears. Found
   while designing the immunity, which would otherwise have refused nothing.

## Design

**Slice D.** The dispatch takes the first mob target when no player target is
named. `resolvePurgeAffliction` becomes target-shape agnostic: it takes the
caster and a target `actions.Actor`-like pair (character plus an optional
player recipient), builds its `messaging.Audience` directly (the pattern of
`sendCritEffectTrio`: a nil Actee recipient for a mob, `ActeeName` set either
way, names hidden per reader by the seam), renders a mob's name with
`mobDisplayName`, and applies the same cancel and remove to whichever
character was named. Self-cast is unchanged. Slice A's admission rules already
refuse a targeted cast without sight, so nothing new is needed there.

**Slice E, the guard.** `buffs.AllFlags` lists every declared `Flag`.
`LoadDataFiles` panics on any spec whose flags include a value not in the
list, naming the buff id, name and flag, the same shape as the species guard.
A root test walks `_datafiles/world/dogmud/buffs/*.yaml` and fails on the
same condition, so a merge cannot ship one. A companion unit test parses the
`Flag` constants out of `buffspec.go` and asserts `AllFlags` names every one,
so a new constant cannot be forgotten. Flag names are compared exactly; the
loader does not normalise, and the guard's message says so.

**Slice E, poison immunity.** New `PoisonImmunity Flag = "poison-immunity"`.
At the two primitives, `Buffs.AddBuff` and `Buffs.AddBuffScaled`, an incoming
spec carrying `poison` is refused (returns false, nothing added) when the
holder already has `poison-immunity`; `Character.AddCondition` refuses
`ConditionPoisoned` the same way. Refusal is silent: the buff's own start line
("Nothing could turn your stomach now") already told the player. Stone Stomach
keeps its file as is, now meaning what it says. **The three toxins (39, 40,
78) gain `flags: [poison]`**, which is what makes purge, cleansing wave, the
adjective and the new immunity all real at once.

**Price.** Savant's Infusion `value: 60` becomes `100`, the Chrysalis
Catalyst tier, since its multiplier is the stronger of the learning pair.

## Out of scope

- Normalising flag spellings at load (the owner asked for a guard, not
  leniency; a typo should fail loudly).
- Any purge target beyond players and mobs in the room (items, self-only
  variants).

## Gates

Unit tests for the dispatch, the hook with a mob target, the flag guard
(proven red by a sabotaged flag), the immunity at both primitives; the root
guard; full suite; gofmt; lint; the isolated boot check; and a playtest lane:
poison a charmed companion and purge it by name, then drink Stone Stomach and
take a poisoned hit.
