# Ranged: One Verb, One Action

**Status:** design approved 2026-09-07, not yet planned.

**Goal:** firing a ranged weapon becomes a single deliberate action. `fire`
resolves the shot, chambers the next round, and burns one special-move cooldown.
The standalone player `reload` retires, taking with it a cooldown collision that
punished the natural way to set up an ambush.

**Owner framing, 2026-09-07:** *"If we're sneaking, the shot is a surprise strike
attack with appropriate bonuses, then reload happens automatically, same cooldown
burned, stealth lost. If the shooter is not stealthed, shot gets fired, reload
happens automatically, cooldown still gets burned."* And: *"mob and player needs
to have parity. This can't be just a player only fix."*

---

## Facts verified against source, 2026-09-07

Read from the files at spec-writing time. Where this table and any older note
disagree, this table is right.

| Fact | Evidence |
|---|---|
| `fire` is ALREADY an alias of `shoot`; nothing needs inventing to make the word work | `_datafiles/world/dogmud/keywords.yaml:318` -> `shoot: ['fire']` |
| Player and mob both route through one seam, so parity is structural, not a second implementation | `internal/usercommands/shoot.go:74` and `internal/mobcommands/shoot.go:22` both call `actions.ExecuteFire`; `internal/usercommands/reload.go:23` and `internal/mobcommands/reload.go:17` both call `actions.ExecuteReload` |
| An ORDINARY shot does NOT burn the special-move cooldown today. **Reload does.** | `combat_fire.go:89-95` doc comment; the only `TryCooldown` in fire is at `:277`, gated on `surpriseShot` |
| The surprise shot burns it, and is downgraded to an ordinary shot when already claimed | `combat_fire.go:277-282`, setting `result.SurpriseOnCooldown` |
| Reload burns the same timer, and **gates on it being ready** | `combat_reload.go:86` (read-only gate) and `:133` (`TryCooldown`, with a full rollback path if it fails) |
| 🔴 **Therefore: after an ambush you CANNOT reload for `SpecialMoveCooldown` rounds.** The ambush claims the timer; reload then refuses on it. Never written down before | derived from the two rows above; `SpecialMoveCooldown: 4` at `config.yaml:747` |
| Firing unloads the weapon **even on a miss**, so every shot costs two commands | `combat_fire.go:89-95` |
| Costs are separate and both real: shoot 2 stamina, reload 1 | `costs.ActionShoot` / `costs.ActionReload` (`internal/costs/action.go:33-34`); `ShootBaseStaminaCost: 2`, `ReloadBaseStaminaCost: 1` (`config.yaml:812-813`) |
| **Three** quests key on these commands, not one | `50-first_shot.yaml:82` (`command: reload`) and `:95` (`command: shoot`); `51-across_the_canyon.yaml:80`; `59-range_practice.yaml:54` |
| The quest notify hardcodes the string, so it does not follow what the player typed | `usercommands/shoot.go:207-211` sends `Command: "shoot"` |
| Quest 50 exists TO TEACH the reload loop. Its dialogue says so | `50-first_shot.yaml:13` (*"teach the ranged loop ... reload it ... shoot"*), `:70` (*"always the same: shoot, then reload, shoot, then reload"*) |
| Every ranged weapon shares one subtype, so per-weapon flavour can only key on ammo | `items.IsRangedWeapon` = `Type == Weapon && Subtype == Shooting` (`items.go:207-213`); all 8 ranged weapons are `subtype: shooting` |
| There are exactly **three** ammo tags across 8 weapons | `arrows` (Training Bow, Hunting Bow, Ironhorn Warbow), `bolts` (Hand Crossbow, Arbalest), `shot` (Sling, Primitive Pistol, Relic Sidearm) |
| A shooting message file already exists with an established token vocabulary | `_datafiles/world/dogmud/combat-messages/shooting.yaml` (`{itemname}`, `{source}`, `{target}`, ...) |
| The archer tree only DECIDES whether to reload; it does not implement it | `internal/behaviortree/actions_archer.go` -> `unloadedRangedWeapon`, `hasMatchingAmmo`, both mirroring `ExecuteReload` |
| `reload` is overloaded: the same verb is the admin data-file reload, split by role permission | `usercommands/reload.go:34-64` handles `reload items` / `biomes` / `translations` / `mapcache` / `help` |

---

## The problem

Two defects that look separate and are not.

**1. Every shot costs two commands.** Firing unloads the weapon, so the loop is
`shoot`, `reload`, `shoot`, `reload`. On the AI port, where a round admits three
commands, a single shot consumes two thirds of the budget.

**2. Reload and the ambush fight over one timer.** Reload burns the special-move
cooldown, and the surprise shot needs that same timer. The code already says so:

> *"reload burns the same shared special-move timer the opener needs, so the
> natural reload-sneak-shoot order denies the ambush whenever the reload was
> recent."* (`usercommands/shoot.go:239-243`)

So the intuitive sequence, reload then sneak then shoot, denies the ambush the
player was setting up. And the reverse holds too, which nobody had written down:
**an ambush claims the timer, and reload refuses on it, so a successful surprise
shot leaves you unable to reload for four rounds.**

These are the same problem. Folding reload into firing removes the separate
timer-claiming step, so neither collision has anywhere left to happen.

## The design

### One action

`fire <target> [direction]` resolves the shot, then chambers the next round as
part of the same action, and burns the special-move cooldown **once**.

- **Sneaking:** resolves as a surprise strike with its existing bonuses, stealth
  drops, cooldown burned.
- **Not sneaking:** ordinary shot, cooldown burned.

Cadence is preserved rather than loosened. Today a shot-plus-reload cycle already
costs one special-move burn (spent by the reload); it still costs one. What
changes is that it costs one *command* instead of two, and that the burn can no
longer land on the wrong side of an ambush.

`Item.Loaded` stays. It remains the difference between a freshly equipped weapon
and a chambered one, and it is how "no ammo" surfaces: if firing cannot chamber a
replacement, the shot still resolves and the weapon is left empty, so the next
`fire` reports the empty weapon rather than silently failing.

**Stamina:** the folded action admits both existing costs, 2 for the shot and 1
for the chambering, for 3 total. This is deliberately the same total a shot and a
reload cost today; folding is a command-count change, not a stamina discount. Both
`costs.ActionShoot` and `costs.ActionReload` therefore survive.

### Where it lands

In **`actions.ExecuteFire`**, the seam both sides already call. Player and mob
parity comes from putting it there rather than from writing it twice.

`actions.ExecuteReload` stops being a player-facing action and becomes an internal
step of firing. Its careful ammo-bundle handling, follow-the-identity revalidation
after cost admission, `BundleEmptied`, and the rollback path all move with it;
none of that is being rewritten.

The archer behaviour tree simplifies. `unloadedRangedWeapon` and `hasMatchingAmmo`
exist to decide whether the mob should spend a turn reloading; with reload folded
in, the tree only needs to know whether the mob has ammo at all.

### Command surface

- **`fire` becomes the registered command** in both `internal/usercommands` and
  `internal/mobcommands`. `shoot` becomes its alias, so existing habits, macros
  and mob YAML keep working.
- **The player half of `reload` retires.** Typing it points at `fire`.
- **`reload items` / `biomes` / `translations` / `mapcache` stays**, admin-only.
  Retiring the player half un-overloads the verb as a side effect: one command
  stops doing two unrelated jobs separated by a role check.

### Content

**Quest 50 First Shot** is rewritten around `fire`. The `50-reload` beat goes
away; Iden teaches the one verb that now exists. The quest keeps two beats so its
shape is unchanged, and the second beat teaches the ambush: fire once openly, then
sneak and fire again to feel the bonus and lose stealth. That turns a mechanical
tutorial into one that teaches the part worth knowing.

**Quests 51 and 59** need their `command:` triggers moved to `fire`.

**The notify string moves with them.** `usercommands/shoot.go:210` hardcodes
`Command: "shoot"`, which is why 51 and 59 would not have broken on a rename by
themselves. Leaving it saying `"shoot"` while the command is `fire` is exactly the
silent drift this codebase keeps getting bitten by, so it becomes `"fire"` and all
three quest files move in the same commit.

**Help:** `shoot.template` becomes the `fire` page, `reload.template` retires its
player half, and `ranged-combat.template` is rewritten to describe one action.
`keywords.yaml` gains the alias flip.

### Messaging

The recovery folds into the shot line so the two read as one motion, keyed on
ammo tag, in the existing `combat-messages/shooting.yaml`:

- **arrows** -- nocking the next shaft
- **bolts** -- cranking and seating the next bolt
- **shot** -- whipping the next stone into the cradle

⚠️ **`shot` covers both the Sling and the Primitive Pistol / Relic Sidearm**,
which read very differently. Ammo tag is the only discriminator available, since
all eight ranged weapons share `subtype: shooting`. The `shot` line is therefore
written neutrally enough to fit both rather than adding a per-item override field
for two items. If sling-versus-firearm flavour is wanted later, that is a new
ItemSpec field and its own change.

Player-facing copy follows the house rules: no raw numbers, no em or en dashes,
wrapped under 80 visible columns.

## Testing

The regression that matters most, stated as one test: **firing from stealth
produces the ambush, leaves the weapon loaded, and burns the cooldown exactly
once.** That single assertion covers the fold, the parity of the cooldown rule,
and the collision that motivated the work.

Also required:

- **Mob parity** through `MobActor`: a mob firing gets the same fold, the same
  single cooldown burn, and the same auto-chamber.
- **Out of ammo:** the shot resolves, the weapon is left empty, the next `fire`
  reports it. `BundleEmptied` still fires when the last round is spent.
- **The retired collision, as a named regression:** reloading can no longer deny
  an ambush, because there is no separate reload. Assert the ambush succeeds in
  the sequence that used to deny it.
- **The reverse collision:** after an ambush, the shooter is not locked out of a
  chambered weapon for four rounds.
- **Quest triggers:** 50, 51 and 59 each still fire their beat, guarding the
  notify string against drifting from the command name.
- Every new test must be **proven capable of failing** by sabotaging the fix and
  confirming it goes red naming the right thing.

**The content playtest gate applies** (help pages, quest dialogue, new combat
messaging). Note the recorded harness constraints: shops sleep on NPC schedules,
and a fresh character cannot reach the Pothole Coulee firing range inside a 25
minute budget at three commands per round. The goals file should seed a stocked
character at the range rather than walk one there.

## Out of scope

- **Retiring `Item.Loaded`.** It still distinguishes a fresh weapon from a
  chambered one and carries the out-of-ammo signal.
- **Per-weapon flavour beyond the three ammo tags.** Needs a new ItemSpec field.
- **Moving the admin data-file reload to its own verb.** Retiring the player half
  already removes the ambiguity; renaming the admin side is a separate cleanup.
- **Touching throw.** It shares the special-move cooldown and is deliberately
  unchanged here.
- **Retuning any ranged damage knob.** `SurpriseRangedStrikeMultiplier` (0.5) and
  `RangedUnengagedDamageMultiplier` (2.75) keep their shipped values; this change
  is about the command loop, not the numbers.

## Done when

1. `fire <target> [direction]` resolves a shot and chambers the next round in one
   action, for players and mobs alike, through `actions.ExecuteFire`.
2. Exactly one special-move cooldown is burned per shot, sneaking or not.
3. The player `reload` command is gone; the admin one still works.
4. Reload can no longer deny an ambush, and an ambush can no longer strand an
   empty weapon, both pinned by tests.
5. Quests 50, 51 and 59 fire their beats, and the notify string matches the
   command name.
6. Help pages describe one verb and one action.
7. Boot clean, full suite green, and the content playtest gate run against a
   seeded character at the range.
