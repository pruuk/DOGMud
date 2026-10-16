# Messaging M7 slice 4: admin and staff ledger

Every `messaging.CategorySystem` send in an admin-only command, a staff-only
subcommand of a player command, or a file the slice 3 ledger skipped
(`internal/usercommands/admin.*.go`), ruled one by one. Inventory: the slice 3
classifier with its admin skip removed, at master `ae94920b7` (2026-10-16), 667
rows. Rulings: four read-only agents read each site; the controller reconciled the
counts (667 rows, keys identical to the inventory) and set the final category per
the plan's owner rulings. Line numbers are as of `ae94920b7`; match by text once a
file has been edited.

Plan: `docs/superpowers/plans/2026-10-16-messaging-m7-slice4-tags.md`. The narration
and usage rows outside these files are in the slice 3 ledger,
`docs/superpowers/audits/messaging-m7-refusal-ledger.md` (NARR and USAGE rows).

| Ruling | Rows | Meaning |
|---|---|---|
| ADMIN | 338 | shown to the staff member who typed it |
| REFUSAL | 218 | a refusal; staff ones ride `CategoryAdmin` (ruling R2), player-reachable ones `CategoryRefusal` |
| USAGE | 88 | a usage line or block |
| NOTICE | 10 | sent to a different player |
| ROOM | 1 | sent to the room |
| SHARED | 1 | a helper with admin and player callers |
| NOT | 11 | stays on `CategorySystem` (player-reachable output) |
| total | 667 | |

Final category: Admin 552, Refusal 4, System 11, Usage 88, other 12.

Columns: line, ruling, what the recipient reads (args as `%s`), final category,
the agent's proposal, the note.

## `internal/actions/bauble_admin.go`

ADMIN 1

| Line | Ruling | Today | Category | Proposal | Note |
|---|---|---|---|---|---|
| 38 | ADMIN | async result line of bauble regen, sent to the admin who asked (tellBaubleAdmin) | Admin |  | only reaches the admin who ran bauble regen |

## `internal/usercommands/admin.ai.go`

ADMIN 5 REFUSAL 1 USAGE 1

| Line | Ruling | Today | Category | Proposal | Note |
|---|---|---|---|---|---|
| 23 | USAGE | Usage: ai-flag <username> - Toggle AI flag on a user account. | Usage |  |  |
| 42 | REFUSAL | Could not find user. | Admin |  |  |
| 49 | ADMIN | <username> (<char>) has been flagged as AI. | Admin |  | flagged player is not told |
| 51 | ADMIN | <username> (<char>) AI flag has been removed. | Admin |  | flagged player is not told |
| 81 | ADMIN |   <username> (<char>) [flagged] [AI port] | Admin |  | ai-list row |
| 86 | ADMIN | No AI accounts currently online. | Admin |  | empty report of ai-list, not a failure |
| 88 | ADMIN | %d AI account(s) online. | Admin |  |  |

## `internal/usercommands/admin.badcommands.go`

ADMIN 1

| Line | Ruling | Today | Category | Proposal | Note |
|---|---|---|---|---|---|
| 38 | ADMIN | Bad Commands table (command, count) | Admin |  |  |

## `internal/usercommands/admin.bauble.go`

ADMIN 17 REFUSAL 14 USAGE 4

| Line | Ruling | Today | Category | Proposal | Note |
|---|---|---|---|---|---|
| 91 | USAGE | usage block from template admincommands/help/command.bauble (baubleUsage) | Usage |  |  |
| 94 | USAGE | Usage: block listing every bauble subcommand (baubleUsage fallback), multi-line | Usage |  |  |
| 118 | REFUSAL | Tier must be one of: cheap, average, rare. | Admin |  |  |
| 129 | ADMIN | You conjure a %s bauble from this room. It will arrive in your pack shortly, %s. | Admin |  |  |
| 184 | ADMIN | bauble status report (naming, corpus, search settings, sweep) | Admin |  |  |
| 255 | ADMIN | bauble show: one record in full | Admin |  |  |
| 290 | REFUSAL | The bauble catalog is empty. | Admin |  | bauble list with nothing to list (also when a filter matches nothing) |
| 302 | ADMIN | bauble list: sales totals and newest records | Admin |  |  |
| 311 | ADMIN | Bauble roll windows for room %d and its features reset. The next search starts a fresh one. | Admin |  |  |
| 353 | ADMIN | bauble window report for this room | Admin |  |  |
| 363 | REFUSAL | Which bauble? Give an id (B0000012) or the name of one in your pack or on the floor. | Admin |  | missing argument; could equally be USAGE, no wording change either way |
| 371 | REFUSAL | No bauble record %s. | Admin |  |  |
| 385 | REFUSAL | No bauble called "%s" in your pack or on the floor here. | Admin |  |  |
| 425 | ADMIN | bauble stats: catalog summary | Admin |  |  |
| 446 | USAGE | Usage: bauble edit <bauble> <field> <text>. Fields: %s. | Usage |  |  |
| 455 | REFUSAL | Not changed: %s. | Admin |  | raw error to staff, stays |
| 459 | ADMIN | Its old text left the fallback corpus (promoted entries removed: %d). | Admin |  |  |
| 462 | ADMIN | <red>%s.</red> (ErrCorpusCleanup warning, edit itself succeeded) | Admin |  | warning beside a success, so ADMIN not REFUSAL |
| 464 | ADMIN | Bauble %s is now <name> (keyword %s, %s, %d gold, %.1f lb). Every copy in the world shows the change. | Admin |  |  |
| 476 | REFUSAL | No model is set up (see "bauble status"), so there is nothing to regenerate with. | Admin |  |  |
| 480 | REFUSAL | Could not regenerate %s: %s. | Admin |  |  |
| 483 | ADMIN | Asking the model to rename %s (%s). You will be told when it answers. | Admin |  |  |
| 495 | REFUSAL | raw err.Error() from baubles.Retire | Admin |  | raw error to staff, stays |
| 498 | ADMIN | <red>%s.</red> (ErrCorpusCleanup warning, retire itself succeeded) | Admin |  | warning beside a success |
| 500 | ADMIN | Bauble %s (%s) is retired: it now shows as a plain Trinket everywhere. "bauble restore %s" undoes it. | Admin |  |  |
| 504 | REFUSAL | raw err.Error() from baubles.Restore | Admin |  | raw error to staff, stays |
| 508 | ADMIN | Bauble %s shows as <name> again. | Admin |  |  |
| 520 | REFUSAL | No prompt renderer is installed (the baubles module is not built in). | Admin |  |  |
| 529 | ADMIN | bauble prompt: the prompt that would name the record | Admin |  |  |
| 549 | REFUSAL | Not promoted: %s. | Admin |  |  |
| 552 | ADMIN | Bauble %s (%s) is now in the fallback corpus under %s. Finds with no model may use its text. | Admin |  |  |
| 560 | USAGE | Usage: bauble corpus list [key] \| remove <key> <name or record id> \| reload \| export | Usage |  |  |
| 609 | ADMIN | shared send of baubleCorpus buffer b: mixed content (remove usage line, "Not removed: %s.", "Removed %s ...", reload report, default usage) | Admin |  | one send site, several kinds; to be exact the code needs a category per branch (remove usage and default usage USAGE, Not removed REFUSAL, rest ADMIN) |
| 668 | REFUSAL | No promoted entries to export. | Admin |  |  |
| 673 | REFUSAL | Could not export: %s. | Admin |  |  |

## `internal/usercommands/admin.bounty.go`

ADMIN 3 NOT 3 REFUSAL 7 USAGE 4

| Line | Ruling | Today | Category | Proposal | Note |
|---|---|---|---|---|---|
| 43 | REFUSAL | That subcommand is admin-only. | Refusal |  | shown to a non-admin player who typed bounty declare/withdraw/prune-expired; bounty is NOT AdminOnly in userCommands (false at usercommands.go:132) |
| 67 | USAGE | admin usage block from template admincommands/help/command.bounty (bountyAdminUsage) | Usage |  | shown only when HasRolePermission bounty |
| 70 | USAGE | Usage: block, admin bounty subcommands (bountyAdminUsage fallback), multi-line | Usage |  |  |
| 84 | USAGE | player usage block from template help/bounty (bountyUserUsage) | Usage |  | shown to ordinary players |
| 87 | USAGE | Usage: block, bounty list/show for players (bountyUserUsage fallback), multi-line | Usage |  | shown to ordinary players |
| 131 | NOT | No bounties. | System |  | bountyAdminList is reached by ordinary players (list/show open to all), so not staff-only; rides plain System (or Refusal if the owner wants empty-list as refusal) |
| 149 | NOT | bounty list table | System |  | player-reachable, not staff-only output |
| 164 | REFUSAL | Bad bounty id %q | Refusal |  | bountyAdminShow also reached by ordinary players |
| 169 | REFUSAL | No bounty with id %d | Refusal |  | bountyAdminShow also reached by ordinary players |
| 188 | NOT | Bounty #%d detail block | System |  | player-reachable, not staff-only output |
| 199 | REFUSAL | Bad issuer spec %q (use type:id) | Admin |  |  |
| 204 | REFUSAL | Bad target spec %q (use type:id) | Admin |  |  |
| 244 | REFUSAL | Declare failed: %v | Admin |  | raw error to staff, stays |
| 247 | ADMIN | Declared bounty #%d. | Admin |  |  |
| 258 | REFUSAL | Bad bounty id %q | Admin |  |  |
| 262 | ADMIN | Withdrew bounty #%d. | Admin |  |  |
| 268 | ADMIN | Pruned %d expired bounty rows. | Admin |  |  |

## `internal/usercommands/admin.build.go`

ADMIN 3 REFUSAL 8 USAGE 1

| Line | Ruling | Today | Category | Proposal | Note |
|---|---|---|---|---|---|
| 31 | USAGE | help block from template admincommands/help/command.build | Usage |  |  |
| 40 | REFUSAL | raw err.Error() from rooms.CreateZone | Admin |  | raw error to staff, stays |
| 42 | ADMIN | Zone %s created. | Admin |  |  |
| 45 | REFUSAL | raw err.Error() from rooms.MoveToRoom | Admin |  | raw error to staff, stays |
| 47 | ADMIN | Moved to room %d. | Admin |  |  |
| 75 | REFUSAL | raw err.Error() from mapper.AdjustExitName (exit name) | Admin |  | raw error to staff, stays |
| 89 | REFUSAL | raw err.Error() from mapper.AdjustExitName (return exit name) | Admin |  | raw error to staff, stays |
| 102 | REFUSAL | No map found (or an error occured)" | Admin |  | stray trailing quote in the literal, wording stays |
| 115 | REFUSAL | raw err.Error() from rooms.BuildRoom | Admin |  | raw error to staff, stays |
| 116 | REFUSAL | Error building room %s. | Admin |  |  |
| 135 | REFUSAL | raw err.Error() from rooms.MoveToRoom | Admin |  | raw error to staff, stays |
| 137 | ADMIN | Moved to room %d. | Admin |  |  |

## `internal/usercommands/admin.caravan.go`

ADMIN 2 REFUSAL 3 USAGE 2

| Line | Ruling | Today | Category | Proposal | Note |
|---|---|---|---|---|---|
| 32 | USAGE | Usage: caravan reset [instanceId] | Usage |  |  |
| 44 | REFUSAL | Invalid instance ID %q — must be an integer. | Admin |  | contains an em dash in the literal; staff output keeps its words |
| 51 | ADMIN | Caravan leader #%d reset to ThornwallDwell. | Admin |  |  |
| 56 | REFUSAL | Instance #%d not found or is not a caravan leader. | Admin |  |  |
| 67 | REFUSAL | No active caravan leaders found. | Admin |  | global reset with nothing to reset |
| 71 | ADMIN | Reset %d caravan leader(s) to ThornwallDwell. | Admin |  |  |
| 79 | USAGE | Usage: caravan reset [instanceId] | Usage |  |  |

## `internal/usercommands/admin.cartcheck.go`

ADMIN 2 REFUSAL 1

| Line | Ruling | Today | Category | Proposal | Note |
|---|---|---|---|---|---|
| 32 | REFUSAL | Zone "%s" not found. | Admin |  |  |
| 65 | ADMIN | Cartesian consistency: no findings. All checked zones are clean. | Admin |  | a clean result, not a failure |
| 71 | ADMIN | cartcheck findings report plus "%d error(s), %d warning(s)." | Admin |  |  |

## `internal/usercommands/admin.combatstats.go`

ADMIN 7 REFUSAL 7 USAGE 2

| Line | Ruling | Today | Category | Proposal | Note |
|---|---|---|---|---|---|
| 25 | USAGE | help block from template admincommands/help/command.combatstats (no args) | Usage |  |  |
| 48 | USAGE | help block from template admincommands/help/command.combatstats (unknown subcommand) | Usage |  |  |
| 68 | REFUSAL | No combat events found for attack type "%s". | Admin |  |  |
| 71 | REFUSAL | No combat events in the buffer. | Admin |  | nothing to show |
| 117 | ADMIN | Combat Analytics Dashboard table | Admin |  |  |
| 126 | REFUSAL | No combat events in the buffer. | Admin |  | nothing to show |
| 158 | ADMIN | Attack Types table | Admin |  |  |
| 167 | REFUSAL | No combat events in the buffer. | Admin |  | nothing to show |
| 194 | ADMIN | Combat Matchups table | Admin |  |  |
| 203 | REFUSAL | No combat events in the buffer. | Admin |  | nothing to show |
| 233 | ADMIN | Defense Breakdown table | Admin |  |  |
| 242 | REFUSAL | No combat events in the buffer. | Admin |  | nothing to show |
| 270 | ADMIN | Position Hit Rates table | Admin |  |  |
| 278 | ADMIN | Combat analytics buffer cleared. %s events removed. | Admin |  |  |
| 289 | REFUSAL | No combat events to export. | Admin |  |  |
| 294 | ADMIN | Flushed %s events to analytics log. | Admin |  |  |

## `internal/usercommands/admin.command.go`

USAGE 1

| Line | Ruling | Today | Category | Proposal | Note |
|---|---|---|---|---|---|
| 34 | USAGE | help block from template admincommands/help/command.command (lists mob commands) | Usage |  | command is AdminOnly (usercommands.go:93); sent when fewer than 2 args |

## `internal/usercommands/admin.copyover.go`

ADMIN 1 REFUSAL 1

| Line | Ruling | Today | Category | Proposal | Note |
|---|---|---|---|---|---|
| 16 | ADMIN | Initiating copyover... | Admin |  |  |
| 19 | REFUSAL | Copyover failed: %s | Admin |  | raw error to staff, stays |

## `internal/usercommands/admin.crime.go`

ADMIN 4 REFUSAL 7 USAGE 2

| Line | Ruling | Today | Category | Proposal | Note |
|---|---|---|---|---|---|
| 62 | USAGE | usage block from template admincommands/help/command.crime (crimeShowUsage) | Usage |  |  |
| 65 | USAGE | Usage: block, crime subcommands (crimeShowUsage fallback), multi-line | Usage |  |  |
| 81 | REFUSAL | Unknown faction: %s | Admin |  |  |
| 86 | REFUSAL | No crimes for %s. | Admin |  | empty list |
| 108 | ADMIN | crime list table for a faction | Admin |  |  |
| 128 | REFUSAL | No such player: %s | Admin |  |  |
| 142 | REFUSAL | No crimes recorded for %s. | Admin |  | empty list |
| 165 | ADMIN | crime show table for a player | Admin |  |  |
| 189 | REFUSAL | Bad crime id %q: %v | Admin |  |  |
| 194 | REFUSAL | Unknown faction: %s | Admin |  |  |
| 198 | ADMIN | Resolved %s crime %d: %s | Admin |  |  |
| 209 | REFUSAL | Unknown faction: %s | Admin |  |  |
| 213 | ADMIN | Resolved %d stale crime(s) for %s. | Admin |  |  |

## `internal/usercommands/admin.deafen.go`

ADMIN 2 REFUSAL 2 USAGE 2

| Line | Ruling | Today | Category | Proposal | Note |
|---|---|---|---|---|---|
| 22 | USAGE | help block from template admincommands/help/command.deafen (deafen, no args) | Usage |  |  |
| 28 | REFUSAL | Could not find user. | Admin |  |  |
| 33 | ADMIN | <username> (<char>) has been DEAFENED | Admin |  | goes to the staff member only; the deafened player gets no message from this code |
| 42 | USAGE | help block from template admincommands/help/command.deafen (undeafen, no args) | Usage |  |  |
| 48 | REFUSAL | Could not find user. | Admin |  |  |
| 53 | ADMIN | <username> (<char>) has been UNDEAFENED | Admin |  | goes to the staff member only |

## `internal/usercommands/admin.devtool.go`

ADMIN 6 REFUSAL 7 USAGE 1

| Line | Ruling | Today | Category | Proposal | Note |
|---|---|---|---|---|---|
| 44 | REFUSAL | Error: %s | Admin |  | raw error to staff, stays |
| 47 | ADMIN | devtool check zone consistency report | Admin |  |  |
| 48 | ADMIN | Total issues: %d | Admin |  |  |
| 59 | REFUSAL | Width and height must be integers. | Admin |  | followed by the help template (line 145) |
| 66 | REFUSAL | Error: %s | Admin |  | raw error to staff, stays |
| 69 | ADMIN | Created %d rooms in zone %q (room IDs %d-%d). | Admin |  |  |
| 82 | REFUSAL | Error parsing first arg: %s | Admin |  |  |
| 88 | REFUSAL | Error parsing third arg: %s | Admin |  |  |
| 92 | REFUSAL | Error: %s | Admin |  | raw error to staff, stays |
| 95 | ADMIN | Linked room %d (%s) %s <-> room %d (%s). | Admin |  |  |
| 107 | ADMIN | devtool json result | Admin |  |  |
| 122 | ADMIN | Moon phases report (multi-line, stat effects) | Admin |  |  |
| 135 | REFUSAL | Unknown subcommand: %q | Admin |  | followed by the help template |
| 145 | USAGE | showDevtoolHelp: help block from template admincommands/help/command.devtool | Usage |  | helper called only from Devtool (AdminOnly), all callers are usage |

## `internal/usercommands/admin.fact.go`

ADMIN 10 REFUSAL 7 USAGE 3

| Line | Ruling | Today | Category | Proposal | Note |
|---|---|---|---|---|---|
| 59 | USAGE | usage block from template admincommands/help/command.fact (factUsage) | Usage |  |  |
| 62 | USAGE | Usage: block, fact subcommands (factUsage fallback), multi-line | Usage |  |  |
| 91 | REFUSAL | No facts. | Admin |  | empty list |
| 119 | ADMIN | fact list table | Admin |  |  |
| 130 | REFUSAL | No fact with id %q | Admin |  |  |
| 150 | ADMIN | fact show detail block | Admin |  |  |
| 158 | USAGE | Missing -- separator before description. / Usage: fact declare <factId> [opts] -- <description> | Usage |  | two-line message, second line is the usage line |
| 213 | REFUSAL | Declare failed: %v | Admin |  | raw error to staff, stays |
| 216 | ADMIN | Declared fact %q. | Admin |  |  |
| 226 | ADMIN | Withdrew fact %q. | Admin |  |  |
| 236 | ADMIN | Expired fact %q. | Admin |  |  |
| 242 | ADMIN | Pruned %d expired fact(s). | Admin |  |  |
| 253 | REFUSAL | Bad mob id %q | Admin |  |  |
| 266 | ADMIN | fact awareness report for a mob | Admin |  |  |
| 277 | REFUSAL | Bad mob id %q | Admin |  |  |
| 289 | ADMIN | Taught mob %d the fact %q (source: %s). | Admin |  |  |
| 300 | REFUSAL | Bad mob id %q | Admin |  |  |
| 304 | ADMIN | Mob %d forgot fact %q. | Admin |  |  |
| 315 | REFUSAL | Bad mob id %q | Admin |  |  |
| 319 | ADMIN | Mob %d forgot all facts and events. | Admin |  |  |

## `internal/usercommands/admin.faction.go`

ADMIN 8 REFUSAL 7 USAGE 2

| Line | Ruling | Today | Category | Proposal | Note |
|---|---|---|---|---|---|
| 59 | USAGE | help template admincommands/help/command.faction (usage block, called by factionShowUsage) | Usage |  |  |
| 62 | USAGE | "Usage:" block listing the faction subcommands (inline fallback, factionShowUsage) | Usage |  | one SendText call carries the whole block |
| 76 | ADMIN | No factions loaded. | Admin |  | empty-list report to staff |
| 93 | ADMIN | faction list table: Loaded factions, ID/Name/Default/Allies-Enemies | Admin |  |  |
| 112 | REFUSAL | No such player: %s | Admin |  |  |
| 130 | ADMIN | No factions hold a non-default rep for %s. | Admin |  | empty-result report, not a failure |
| 144 | ADMIN | faction show <player> table: Faction/Rep/Tier | Admin |  |  |
| 150 | REFUSAL | Unknown faction: %s | Admin |  |  |
| 155 | REFUSAL | No such player: %s | Admin |  |  |
| 159 | ADMIN | %s -> %s: rep=%d, tier=%s | Admin |  |  |
| 183 | REFUSAL | Unknown faction: %s | Admin |  |  |
| 188 | REFUSAL | No such player: %s | Admin |  |  |
| 196 | REFUSAL | Bad rep %q: %v | Admin |  | bad number typed by staff; raw parse error shown on purpose |
| 200 | ADMIN | Set %s -> %s = %d | Admin |  | confirmation; target player is told nothing |
| 204 | REFUSAL | Bad delta %q: %v | Admin |  |  |
| 208 | ADMIN | Bumped %s -> %s by %d (now %d) | Admin |  |  |
| 213 | ADMIN | Reset %s -> %s to default %d | Admin |  |  |

## `internal/usercommands/admin.goal.go`

ADMIN 11 REFUSAL 15 USAGE 2

| Line | Ruling | Today | Category | Proposal | Note |
|---|---|---|---|---|---|
| 69 | USAGE | help template admincommands/help/command.goal (usage block, goalShowUsage) | Usage |  |  |
| 72 | USAGE | "Usage:" block listing the goal subcommands plus the mob-ident line (goalShowUsage inline fallback) | Usage |  |  |
| 111 | REFUSAL | Unknown mob: %s | Admin |  | goalList |
| 116 | ADMIN | %s (%d) has no goals. | Admin |  | empty-list report |
| 128 | ADMIN | goal list table: Goals for X (id), ID/Type/Prio/Params | Admin |  |  |
| 139 | REFUSAL | Unknown mob: %s | Admin |  | goalShow |
| 163 | ADMIN | goal show detail block (type, priority, created_at, params) | Admin |  |  |
| 167 | REFUSAL | No goal %s on %s (%d). | Admin |  | goalShow, goal id not found |
| 179 | REFUSAL | Unknown mob: %s | Admin |  | goalAdd |
| 185 | REFUSAL | Bad priority %q: %v | Admin |  |  |
| 192 | REFUSAL | Bad param %q (expected key=value) | Admin |  |  |
| 206 | REFUSAL | Blocked by goal %s (type=%s, priority=%d). | Admin |  | goal add refused by a conflict |
| 212 | REFUSAL | Add failed: %v | Admin |  | raw error shown to staff on purpose |
| 220 | ADMIN | Added goal %s (type=%s, priority=%d)[ - displaced goals: %s]. | Admin |  | success confirmation, built as msg+".\r\n" |
| 231 | REFUSAL | Unknown mob: %s | Admin |  | goalRemove |
| 236 | REFUSAL | No goal %s on %s (%d). | Admin |  | goalRemove |
| 241 | REFUSAL | Remove failed: %v | Admin |  | raw error shown to staff on purpose |
| 244 | ADMIN | Removed goal %s from %s (%d). | Admin |  |  |
| 256 | REFUSAL | Unknown mob: %s | Admin |  | goalClear |
| 260 | REFUSAL | Clear failed: %v | Admin |  | raw error shown to staff on purpose |
| 263 | ADMIN | Cleared all goals from %s (%d). | Admin |  |  |
| 276 | REFUSAL | Unknown mob: %s | Admin |  | goalCurrent |
| 282 | ADMIN | Current goal for %s (mob %d): none (0 goals on file) | Admin |  | status report |
| 288 | ADMIN | Current goal for %s (mob %d): none (%d goal(s) on file; selection has not landed on one) | Admin |  | status report |
| 296 | ADMIN | Current goal for %s (mob %d): %s %s priority=%d | Admin |  |  |
| 310 | REFUSAL | Unknown mob: %s | Admin |  | goalScores |
| 316 | ADMIN | Score breakdown for %s (mob %d): no goals on file. | Admin |  | empty-result report |
| 356 | ADMIN | goal scores table: Score breakdown, Archetype, per-goal weights | Admin |  |  |

## `internal/usercommands/admin.item.go`

ADMIN 33 REFUSAL 6 USAGE 1

| Line | Ruling | Today | Category | Proposal | Note |
|---|---|---|---|---|---|
| 33 | USAGE | help template admincommands/help/command.item (shown when item has no args) | Usage |  |  |
| 41 | REFUSAL | you do not have item.create permission | Admin |  | staff role lacks the permission |
| 52 | REFUSAL | you do not have item.create permission | Admin |  | item spawn path (checks item.spawn, text names item.create); wording not for this slice |
| 63 | REFUSAL | you do not have item.create permission | Admin |  | item list path (checks item.spawn, text names item.create) |
| 109 | ADMIN | (blank line before the item list) | Admin |  |  |
| 112 | ADMIN | item list: DynamicList of item ids and names | Admin |  |  |
| 113 | ADMIN | (blank line after the item list) | Admin |  |  |
| 127 | ADMIN | You wave your hands and %s appears. | Admin |  | item_Spawn, staff only; bystanders get a CategoryMobEmote line at 130, not in this list |
| 140 | REFUSAL | Item %s not found. | Admin |  | item_Spawn |
| 161 | ADMIN | (blank line at start of item create wizard) | Admin |  |  |
| 162 | ADMIN | Lets get a little info first. | Admin |  | item create wizard intro |
| 196 | ADMIN | numbered list of item types shown while the wizard asks Type | Admin |  | prompt help, not a refusal |
| 217 | ADMIN | numbered list of item types, re-shown after RejectResponse (unrecognised type) | Admin |  | sits next to a rejection but is the option list itself |
| 303 | ADMIN | Aborting... | Admin |  | wizard cancelled (empty room id) |
| 320 | ADMIN | Aborting... | Admin |  | wizard cancelled (empty exit name) |
| 345 | ADMIN | numbered list of item subtypes while the wizard asks Subtype | Admin |  |  |
| 350 | ADMIN | Aborting... | Admin |  | wizard cancelled (empty subtype) |
| 372 | ADMIN | numbered list of item subtypes, re-shown after RejectResponse | Admin |  |  |
| 420 | REFUSAL | Invalid quest token | Admin |  | wizard answer rejected |
| 459 | ADMIN |   Name:   <name> | Admin |  | wizard confirmation preview |
| 460 | ADMIN |   Desc:   <description> | Admin |  | wizard confirmation preview |
| 461 | ADMIN |   Type:   <type> | Admin |  | wizard confirmation preview |
| 464 | ADMIN |   KeyId:  <key lock id> | Admin |  | wizard confirmation preview |
| 467 | ADMIN |   Damage: <dice> | Admin |  | wizard confirmation preview |
| 468 | ADMIN |   DmgMult: <x.xx> | Admin |  | wizard confirmation preview |
| 471 | ADMIN |   Physical: <n>% | Admin |  | wizard confirmation preview |
| 472 | ADMIN |   Magical: <n>% | Admin |  | wizard confirmation preview |
| 473 | ADMIN |   Conviction: <n>% | Admin |  | wizard confirmation preview |
| 477 | ADMIN |   Uses: <n> | Admin |  | wizard confirmation preview |
| 481 | ADMIN |   Value: <n> | Admin |  | wizard confirmation preview |
| 485 | ADMIN |   Quest Token: <token> | Admin |  | wizard confirmation preview |
| 488 | ADMIN |   SubType: <subtype> | Admin |  | wizard confirmation preview |
| 496 | ADMIN | Aborting... | Admin |  | wizard declined at the final confirm |
| 504 | REFUSAL | Error: <err> | Admin |  | CreateNewItemFile failed; raw error shown to staff on purpose |
| 510 | ADMIN | (blank line) | Admin |  | ITEM CREATED block |
| 511 | ADMIN |   ITEM CREATED | Admin |  |  |
| 512 | ADMIN | (blank line) | Admin |  |  |
| 513 | ADMIN |   File Path: <path> | Admin |  |  |
| 514 | ADMIN | (blank line) | Admin |  |  |
| 515 | ADMIN |   note: Try item spawn <name> to test it. | Admin |  |  |

## `internal/usercommands/admin.knowledge.go`

ADMIN 7 REFUSAL 7 USAGE 2

| Line | Ruling | Today | Category | Proposal | Note |
|---|---|---|---|---|---|
| 46 | USAGE | help template admincommands/help/command.knowledge (knowledgeUsage) | Usage |  |  |
| 49 | USAGE | "Usage:" block listing the knowledge subcommands (knowledgeUsage inline fallback) | Usage |  |  |
| 125 | REFUSAL | Unknown mob: %s | Admin |  | knowledgeShow |
| 133 | ADMIN | No knowledge records for mob %d. | Admin |  | empty-result report |
| 153 | ADMIN | knowledge show <mob> table: Subject/Met/Name/LastSeen | Admin |  |  |
| 160 | REFUSAL | Unknown subject | Admin |  |  |
| 165 | REFUSAL | No record: mob %d <-> %s:%d | Admin |  | specific record lookup failed |
| 185 | ADMIN | knowledge subject detail block (HasMet, LastSeen, crimes) | Admin |  |  |
| 196 | REFUSAL | Unknown mob: %s | Admin |  | knowledgeFrequented |
| 205 | REFUSAL | No such player: %s | Admin |  |  |
| 216 | ADMIN | No observations recorded. | Admin |  | empty-result report |
| 224 | ADMIN | Top %d rooms for mob %d watching %s: list | Admin |  |  |
| 235 | REFUSAL | Unknown mob: %s | Admin |  | knowledgeForget |
| 244 | REFUSAL | No such player: %s | Admin |  |  |
| 250 | ADMIN | Forgot record: mob %d <-> %s | Admin |  |  |
| 254 | ADMIN | Forgot fact %q: mob %d <-> %s | Admin |  |  |

## `internal/usercommands/admin.locate.go`

ADMIN 3 NOTICE 1 REFUSAL 1 USAGE 1

| Line | Ruling | Today | Category | Proposal | Note |
|---|---|---|---|---|---|
| 25 | USAGE | help template admincommands/help/command.locate (locate with no argument) | Usage |  |  |
| 37 | ADMIN | %s is in room #%d - %s [%s] | Admin |  | staff report of the located player's room; TSV class guess "refusal" was wrong |
| 41 | NOTICE | You get the feeling someone is looking for you... | Warning | CategoryWarning | sent to the located player (locateUser), not the staff member; deliberate hint to the target. If the owner prefers a world-event reading, a narration category would also fit; Warning chosen because the staff action is aimed at that player |
| 154 | ADMIN | Matches for "%s" [Page n/m] mob table (full page flush) | Admin |  |  |
| 167 | ADMIN | Matches for "%s" [Page n/m] mob table (final flush) | Admin |  |  |
| 173 | REFUSAL | No user or mob found with the name %s | Admin |  |  |

## `internal/usercommands/admin.mob.go`

ADMIN 14 REFUSAL 8 USAGE 4

| Line | Ruling | Today | Category | Proposal | Note |
|---|---|---|---|---|---|
| 34 | USAGE | help template admincommands/help/command.mob (mob with no args) | Usage |  |  |
| 42 | REFUSAL | you do not have mob.spawn permission | Admin |  | mob spawn path |
| 53 | REFUSAL | you do not have mob.spawn permission | Admin |  | mob list path |
| 64 | REFUSAL | you do not have mob.spawn permission | Admin |  | mob heal path |
| 75 | REFUSAL | you do not have mob.spawn permission | Admin |  | mob schedule path |
| 106 | REFUSAL | No mobs in this room to heal. | Admin |  | mob_Heal did nothing |
| 109 | ADMIN | Healed %d mob(s) in the room to full. | Admin |  |  |
| 116 | USAGE | Usage: mob heal [instId] - omit instId to heal every mob in the room. | Usage |  |  |
| 122 | REFUSAL | No mob instance with id %d. | Admin |  | mob_Heal |
| 130 | ADMIN | Healed %s (instance %d) to full. | Admin |  |  |
| 170 | ADMIN | (blank line before the mob list) | Admin |  |  |
| 173 | ADMIN | mob list: DynamicList of mob ids and names | Admin |  |  |
| 174 | ADMIN | (blank line after the mob list) | Admin |  |  |
| 186 | ADMIN | Somewhere in the realm, a loot goblin appears! | Admin |  | loot goblin forced spawn; sent only to the staff member (no broadcast), so it is a staff confirmation |
| 204 | ADMIN | You wave your hands and %s appears. | Admin |  | mob_Spawn; bystanders get CategoryMobEmote at 207, not in this list |
| 216 | REFUSAL | Mob %s not found. | Admin |  | mob_Spawn |
| 239 | USAGE | Usage: mob schedule [instId] | Usage |  | no argument |
| 245 | USAGE | Usage: mob schedule [instId] | Usage |  | bad instId |
| 251 | REFUSAL | No mob instance with id %d. | Admin |  | mob_Schedule |
| 257 | ADMIN | Vitals for %s (instance %d): stamina / action points readout | Admin |  |  |
| 260 | ADMIN | %s (mob %d, instance %d) has no schedule. | Admin |  | status report |
| 267 | ADMIN | %s references unknown schedule %q. | Admin |  | data fault report to staff |
| 275 | ADMIN | Schedule %q has no segment for hour %d (coverage bug). | Admin |  | data fault report to staff |
| 310 | ADMIN | Schedule for %s (mob %d, instance %d): schedule debug block | Admin |  |  |
| 321 | ADMIN |   (patrol_id %q does not resolve) | Admin |  | data fault report to staff |
| 350 | ADMIN | Patrol state: debug block | Admin |  |  |

## `internal/usercommands/admin.modify.go`

ADMIN 1 NOTICE 1 REFUSAL 6 USAGE 1

| Line | Ruling | Today | Category | Proposal | Note |
|---|---|---|---|---|---|
| 28 | USAGE | help template admincommands/help/command.modify (fewer than 3 args) | Usage |  |  |
| 36 | REFUSAL | you do not have modify.role permission | Admin |  |  |
| 47 | REFUSAL | Invalid permission type. | Admin |  |  |
| 58 | REFUSAL | Admin permissions cannot be removed this way. | Admin |  | online-user branch |
| 63 | REFUSAL | That permission is already set for this user. | Admin |  | online-user branch |
| 74 | NOTICE | Your role has been set to: %s | Warning | CategoryWarning | sent to the affected user u (not the staff member) when online; a moderation-style notice aimed at that player |
| 85 | REFUSAL | Admin permissions cannot be removed this way. | Admin |  | offline-user branch |
| 90 | REFUSAL | That permission is already set for this user. | Admin |  | offline-user branch |
| 109 | ADMIN | Role changed for user %s (Character name: %s). | Admin |  |  |

## `internal/usercommands/admin.mudmail.go`

ADMIN 6 REFUSAL 2 USAGE 1

| Line | Ruling | Today | Category | Proposal | Note |
|---|---|---|---|---|---|
| 27 | USAGE | help template admincommands/help/command.mudmail | Usage |  | dead code: inside a /* */ block comment at lines 22-30, never runs. Retag harmlessly or leave; a count-based guard would still see the reference |
| 35 | ADMIN | Starting a new mud mail... | Admin |  | wizard intro |
| 51 | REFUSAL | Some name must be provided. | Admin |  | wizard answer rejected |
| 95 | REFUSAL | Could not find item: %s | Admin |  | wizard answer rejected |
| 108 | ADMIN | mail/message template preview of the draft message | Admin |  |  |
| 116 | ADMIN | Okay! Cancelling mass mail. | Admin |  |  |
| 132 | ADMIN | (blank line) | Admin |  | Message SENT block |
| 133 | ADMIN | Message SENT! | Admin |  | recipients get the mail through u.Command("inbox check"), not through this line |
| 134 | ADMIN | (blank line) | Admin |  |  |

## `internal/usercommands/admin.mute.go`

ADMIN 2 REFUSAL 2 USAGE 2

| Line | Ruling | Today | Category | Proposal | Note |
|---|---|---|---|---|---|
| 22 | USAGE | help template admincommands/help/command.mute (mute with no argument) | Usage |  |  |
| 28 | REFUSAL | Could not find user. | Admin |  | mute |
| 33 | ADMIN | %s (%s) has been MUTED | Admin |  | staff confirmation; the muted player is sent nothing |
| 42 | USAGE | help template admincommands/help/command.mute (unmute with no argument) | Usage |  |  |
| 48 | REFUSAL | Could not find user. | Admin |  | unmute |
| 53 | ADMIN | %s (%s) has been UNMUTED | Admin |  | staff confirmation; the player is sent nothing |

## `internal/usercommands/admin.opinion.go`

ADMIN 6 REFUSAL 7 USAGE 2

| Line | Ruling | Today | Category | Proposal | Note |
|---|---|---|---|---|---|
| 58 | USAGE | help template admincommands/help/command.opinion (opinionShowUsage) | Usage |  |  |
| 61 | USAGE | "Usage:" block listing the opinion subcommands (opinionShowUsage inline fallback) | Usage |  |  |
| 91 | REFUSAL | No such player: %s | Admin |  | opinionShowAll |
| 97 | ADMIN | No NPCs hold an opinion of %s. | Admin |  | empty-result report |
| 117 | ADMIN | opinion show <player> table: Mob/Score/Tier/Last bump | Admin |  |  |
| 124 | REFUSAL | Unknown mob: %s | Admin |  | opinionShowOne |
| 129 | REFUSAL | No such player: %s | Admin |  | opinionShowOne |
| 133 | ADMIN | %s (%d) -> %s: score=%d, tier=%s | Admin |  |  |
| 154 | REFUSAL | Unknown mob: %s | Admin |  | opinionMutate |
| 169 | REFUSAL | No such player: %s | Admin |  | opinionMutate |
| 177 | REFUSAL | Bad score %q: %v | Admin |  |  |
| 181 | ADMIN | Set %s (%d) -> %s = %d | Admin |  |  |
| 185 | REFUSAL | Bad delta %q: %v | Admin |  |  |
| 189 | ADMIN | Bumped %s (%d) -> %s by %d (now %d) | Admin |  |  |
| 194 | ADMIN | Reset %s (%d) -> %s to default %d | Admin |  |  |

## `internal/usercommands/admin.paz.go`

ADMIN 3 NOTICE 1 REFUSAL 1

| Line | Ruling | Today | Category | Proposal | Note |
|---|---|---|---|---|---|
| 27 | REFUSAL | Paz target not found. | Admin |  |  |
| 33 | ADMIN | You illuminate %s with a beam of light! | Admin |  | target is a mob; staff-only line, bystanders get CategoryMobEmote at 34 |
| 43 | ADMIN | You illuminate %s with a beam of light! | Admin |  | target is a player; staff-only line |
| 45 | NOTICE | %s illuminates you with a beam of light! | MobEmote | CategoryMobEmote | sent to the healed player u; same event the room sees at 44 on CategoryMobEmote, so ride that category |
| 58 | ADMIN | You paz yourself with a beam of light! | Admin |  | bystanders get CategoryMobEmote at 59 |

## `internal/usercommands/admin.prepare.go`

ADMIN 1 USAGE 1

| Line | Ruling | Today | Category | Proposal | Note |
|---|---|---|---|---|---|
| 19 | USAGE | help template admincommands/help/command.prepare (prepare with no argument) | Usage |  |  |
| 29 | ADMIN | All rooms have been Prepare()'ed | Admin |  |  |

## `internal/usercommands/admin.questdebug.go`

ADMIN 2 REFUSAL 1 USAGE 2

| Line | Ruling | Today | Category | Proposal | Note |
|---|---|---|---|---|---|
| 23 | USAGE | questdebug <player> - Enable verbose quest logging for a player. | Usage |  | first line of the inline usage block |
| 24 | USAGE | questdebug <player> off - Disable quest debug for a player. | Usage |  | second line of the inline usage block |
| 34 | REFUSAL | Player "%s" not found or not online. | Admin |  |  |
| 40 | ADMIN | Quest debug disabled for %s. | Admin |  | target is told nothing |
| 44 | ADMIN | Quest debug enabled for %s. All quest evaluations will log at verbose. | Admin |  | target is told nothing |

## `internal/usercommands/admin.questtoken.go`

ADMIN 6 USAGE 2

| Line | Ruling | Today | Category | Proposal | Note |
|---|---|---|---|---|---|
| 29 | USAGE | help template admincommands/help/command.questtoken (usage block, no args) | Usage |  |  |
| 61 | ADMIN | "Quest Tokens" table of the staff member's own quest progress | Admin |  |  |
| 83 | ADMIN | "Quest Tokens" table of all quests | Admin |  |  |
| 101 | ADMIN | "Quest Flags" table of the staff member's own flags | Admin |  |  |
| 106 | USAGE | Usage: questtoken flag <key> [value] | Usage |  |  |
| 110 | ADMIN | Flag %q is not set. | Admin |  | query result for the staff member's own flag |
| 112 | ADMIN | Flag %q = %q | Admin |  |  |
| 116 | ADMIN | Set flag %q = %q | Admin |  | confirmation |

## `internal/usercommands/admin.redescribe.go`

ADMIN 1 USAGE 1

| Line | Ruling | Today | Category | Proposal | Note |
|---|---|---|---|---|---|
| 29 | USAGE | help template admincommands/help/command.redescribe (usage block) | Usage |  |  |
| 51 | ADMIN | You chant softly and wave your hand over the %s. Success! It now has a new description! | Admin |  | confirmation to the staff member; the room line right after is CategoryMobEmote, not in this batch |

## `internal/usercommands/admin.relationship.go`

ADMIN 8 REFUSAL 7 USAGE 2

| Line | Ruling | Today | Category | Proposal | Note |
|---|---|---|---|---|---|
| 48 | USAGE | help template admincommands/help/command.relationship (usage block, relationshipUsage) | Usage |  |  |
| 51 | USAGE | Usage: block (relationship show/between/add/remove/list lines) in relationshipUsage fallback | Usage |  | one SendText, multi-line block |
| 70 | REFUSAL | Bad mob id %q | Admin |  |  |
| 75 | ADMIN | No relationships for mob %d. | Admin |  | empty query result, not a refusal |
| 95 | ADMIN | Relationships for mob %d (%s): then one line per relation | Admin |  | report |
| 106 | REFUSAL | Bad mob id %q | Admin |  |  |
| 111 | REFUSAL | Bad mob id %q | Admin |  |  |
| 117 | ADMIN | No relationship between mob %d and mob %d. | Admin |  | empty query result |
| 136 | ADMIN | %d (%s) <-> %d (%s): then relation lines | Admin |  | report |
| 147 | REFUSAL | Bad mob id %q | Admin |  |  |
| 152 | REFUSAL | Bad mob id %q | Admin |  |  |
| 161 | ADMIN | Added: %d -> %d (%s). | Admin |  | confirmation |
| 172 | REFUSAL | Bad mob id %q | Admin |  |  |
| 177 | REFUSAL | Bad mob id %q | Admin |  |  |
| 182 | ADMIN | Removed: %d -> %d (%s). | Admin |  | confirmation |
| 189 | ADMIN | Graph is empty. | Admin |  | empty query result |
| 207 | ADMIN | Relationship graph (%d edges): then edge lines | Admin |  | report |

## `internal/usercommands/admin.reload.go`

ADMIN 5 REFUSAL 4 USAGE 1

| Line | Ruling | Today | Category | Proposal | Note |
|---|---|---|---|---|---|
| 36 | REFUSAL | You do not need to reload. Firing readies the next round by itself. See help fire. | Refusal |  | shown to a NON-staff player (HasRolePermission false); reload is dispatchable by anyone on purpose. Must ride CategoryRefusal, not Admin. |
| 44 | REFUSAL | Items reload failed. The items loaded before are still in use. | Admin |  | staff, command failed |
| 46 | ADMIN | %s (escaped raw error text from items.LoadDataFilesE) | Admin |  | raw error shown to staff on purpose |
| 49 | ADMIN | Items reloaded. | Admin |  | confirmation |
| 53 | ADMIN | Biomes reloaded. | Admin |  | confirmation |
| 58 | REFUSAL | Translations reload failed. | Admin |  | staff, command failed |
| 60 | ADMIN | Translations reloaded. | Admin |  | confirmation |
| 65 | ADMIN | Mapper cache cleared. Next 'map' command will rebuild from current room data. | Admin |  | confirmation |
| 69 | USAGE | help template admincommands/help/command.reload (usage block, reload help) | Usage |  |  |
| 72 | REFUSAL | Unknown reload command. See reload help. | Admin |  | staff, unknown subcommand |

## `internal/usercommands/admin.rename.go`

ADMIN 1 USAGE 1

| Line | Ruling | Today | Category | Proposal | Note |
|---|---|---|---|---|---|
| 27 | USAGE | help template admincommands/help/command.renameitem (usage block) | Usage |  |  |
| 53 | ADMIN | You chant softly and wave your hand over the %s. Success! It's now a %s | Admin |  | confirmation to the staff member; the room line after it is CategoryMobEmote, not in this batch |

## `internal/usercommands/admin.room.dispatcher.go`

ADMIN 25 REFUSAL 18 USAGE 4

| Line | Ruling | Today | Category | Proposal | Note |
|---|---|---|---|---|---|
| 20 | REFUSAL | you do not have room.edit permission | Admin |  | staff-role denial |
| 26 | REFUSAL | you do not have room.edit.container permission | Admin |  |  |
| 34 | REFUSAL | you do not have room.edit.exits permission | Admin |  |  |
| 42 | REFUSAL | you do not have room.edit.mutators permission | Admin |  |  |
| 48 | USAGE | edit WHAT? Try: | Usage |  | first line of a usage block (49 to 51 follow) |
| 49 | USAGE |     room edit containers | Usage |  | usage block line |
| 50 | USAGE |     room edit exits | Usage |  | usage block line |
| 51 | USAGE |     room edit mutators | Usage |  | usage block line |
| 59 | REFUSAL | you do not have room.noun permission | Admin |  |  |
| 72 | ADMIN | Noun Added: | Admin |  |  |
| 73 | ADMIN | <noun> - <description> | Admin |  | confirmation echo |
| 82 | ADMIN | Noun deleted. | Admin |  | confirmation |
| 84 | REFUSAL | Noun not found. | Admin |  |  |
| 89 | ADMIN | Room Nouns: | Admin |  | report header |
| 91 | ADMIN | <noun> - <description> | Admin |  | report row |
| 99 | REFUSAL | you do not have room.copy permission | Admin |  |  |
| 110 | ADMIN | Spawn info copied/overwritten. | Admin |  | confirmation |
| 119 | ADMIN | IdleMessages copied/overwritten. | Admin |  | confirmation |
| 128 | ADMIN | Mutators copied/overwritten. | Admin |  | confirmation |
| 138 | REFUSAL | you do not have room.info permission | Admin |  |  |
| 151 | REFUSAL | Room %d not found. | Admin |  |  |
| 161 | ADMIN | admincommands/ingame/roominfo template (room summary) | Admin |  | report |
| 168 | REFUSAL | you do not have room.exit permission | Admin |  |  |
| 186 | REFUSAL | Exit %s does not exist. | Admin |  |  |
| 194 | ADMIN | Exit %s already exists (overwriting). | Admin |  | warning, command goes on to do the work |
| 199 | ADMIN | Exit %s renamed to %s. | Admin |  | confirmation |
| 207 | REFUSAL | room %d not found | Admin |  | raw error text shown to staff on purpose |
| 212 | ADMIN | Exit %s added. | Admin |  | confirmation |
| 219 | REFUSAL | you do not have room.exit permission | Admin |  |  |
| 229 | ADMIN | Exit %s secrecy REMOVED. | Admin |  | confirmation |
| 234 | ADMIN | Exit %s secrecy ADDED. | Admin |  | confirmation |
| 237 | REFUSAL | Exit %s not found. | Admin |  |  |
| 245 | REFUSAL | you do not have room.set permission | Admin |  |  |
| 258 | ADMIN | Mutators: | Admin |  | report header |
| 260 | ADMIN |   None. | Admin |  | report row |
| 263 | ADMIN |   <mutator id> | Admin |  | report row |
| 265 | ADMIN | (blank line) | Admin |  |  |
| 267 | ADMIN | (blank line) | Admin |  |  |
| 269 | REFUSAL | <id> is an invalid mutator id. | Admin |  |  |
| 270 | ADMIN |   Here is a list of valid mutator id's: | Admin |  | continues the refusal at 269 with a report list |
| 272 | ADMIN |     <mutator id> | Admin |  | report row |
| 275 | ADMIN | Mutator <id> Removed. | Admin |  | confirmation |
| 277 | ADMIN | Mutator <id> Added. | Admin |  | confirmation |
| 279 | ADMIN | (blank line) | Admin |  |  |
| 320 | REFUSAL | %s (raw error from rooms.MoveToZone) | Admin |  | raw error shown to staff on purpose |
| 326 | REFUSAL | Invalid property provided to room set. | Admin |  |  |
| 330 | ADMIN | Room %s set to %s. | Admin |  | confirmation |

## `internal/usercommands/admin.room.exits.go`

ADMIN 12 REFUSAL 4

| Line | Ruling | Today | Category | Proposal | Note |
|---|---|---|---|---|---|
| 66 | ADMIN | tables/numbered-list of the room's exits (prompt choice list) | Admin |  |  |
| 93 | REFUSAL | Invalid option selected. | Admin |  |  |
| 94 | ADMIN | Aborting... | Admin |  | follows the refusal at 93 |
| 111 | ADMIN | (blank line) | Admin |  |  |
| 112 | ADMIN | %s deleted from the room. | Admin |  | staff confirmation (TSV class "narration" is wrong; only the editing staff member receives it) |
| 113 | ADMIN | (blank line) | Admin |  |  |
| 139 | REFUSAL | Invalid new name selected. | Admin |  |  |
| 140 | ADMIN | Aborting... | Admin |  | follows the refusal at 139 |
| 149 | REFUSAL | An exit with that name already exists! | Admin |  |  |
| 172 | REFUSAL | Invalid RoomId provided. | Admin |  |  |
| 238 | ADMIN | (blank line) | Admin |  |  |
| 242 | ADMIN | To Create Key - LockId: %s | Admin |  | report |
| 248 | ADMIN | To pick lock - Sequence: %s | Admin |  | report |
| 251 | ADMIN | (blank line) | Admin |  |  |
| 252 | ADMIN | Changes saved. | Admin |  | confirmation |
| 253 | ADMIN | (blank line) | Admin |  |  |

## `internal/usercommands/admin.room.go`

ADMIN 28 REFUSAL 15 USAGE 1

| Line | Ruling | Today | Category | Proposal | Note |
|---|---|---|---|---|---|
| 43 | USAGE | help template admincommands/help/command.room (usage block, no args) | Usage |  |  |
| 56 | REFUSAL | room %d did not load from its template (raw error text) | Admin |  | raw error shown to staff on purpose |
| 71 | REFUSAL | Invalid room command: secretexit | Admin |  | missing argument; could equally ride CategoryUsage, but it is not a Usage: line |
| 77 | REFUSAL | Invalid room command: copy | Admin |  | missing argument |
| 83 | REFUSAL | Invalid room command: exit | Admin |  | missing argument |
| 89 | REFUSAL | Invalid room command: set | Admin |  | missing argument |
| 94 | REFUSAL | Invalid room command: %s | Admin |  | unknown subcommand |
| 148 | ADMIN | tables/numbered-list of the room's containers (prompt choice list) | Admin |  |  |
| 175 | REFUSAL | Invalid option selected. | Admin |  |  |
| 176 | ADMIN | Aborting... | Admin |  | follows the refusal at 175 |
| 193 | ADMIN | (blank line) | Admin |  |  |
| 194 | ADMIN | %s deleted from the room. | Admin |  | staff confirmation (TSV class "narration" is wrong) |
| 195 | ADMIN | (blank line) | Admin |  |  |
| 221 | REFUSAL | Invalid new name selected. | Admin |  |  |
| 222 | ADMIN | Aborting... | Admin |  | follows the refusal at 221 |
| 231 | REFUSAL | A container with that name already exists! | Admin |  |  |
| 276 | ADMIN | (blank line) | Admin |  |  |
| 280 | ADMIN | To Create Key - LockId: %s | Admin |  | report |
| 286 | ADMIN | To pick lock - Sequence: %s | Admin |  | report |
| 289 | ADMIN | (blank line) | Admin |  |  |
| 290 | ADMIN | Changes saved. | Admin |  | confirmation |
| 291 | ADMIN | (blank line) | Admin |  |  |
| 302 | ADMIN | (blank line) | Admin |  | recipe display helper room_Edit_Containers_SendRecipes |
| 303 | ADMIN | Current Recipe for %d (%s): | Admin |  | recipe display |
| 317 | ADMIN | recipe ingredient row [xN] id (name) | Admin |  | recipe display |
| 320 | ADMIN | (blank line) | Admin |  |  |
| 348 | REFUSAL | Difficulty must between 2 and 32, inclusive. | Admin |  | editLockAndTrap is shared by room edit exits and containers only, both staff-only |
| 413 | ADMIN | tables/numbered-list-doubled of trap conditions (prompt choice list) | Admin |  |  |
| 440 | REFUSAL | Invalid selection. | Admin |  |  |
| 444 | ADMIN | tables/numbered-list-doubled of trap conditions (re-shown after refusal) | Admin |  |  |
| 475 | ADMIN | tables/numbered-list-doubled of trap conditions with marks | Admin |  |  |
| 535 | REFUSAL | Invalid selection. | Admin |  |  |
| 587 | ADMIN | tables/numbered-list of recipe options (prompt choice list) | Admin |  |  |
| 617 | REFUSAL | Invalid itemId. | Admin |  |  |
| 646 | ADMIN | (blank line) | Admin |  |  |
| 647 | ADMIN | Positive numbers add items, negative numbers remove items. | Admin |  | prompt hint |
| 665 | REFUSAL | Invalid ItemId provided. | Admin |  |  |
| 771 | ADMIN | tables/numbered-list-doubled of mutators (prompt choice list) | Admin |  |  |
| 796 | REFUSAL | Invalid selection. | Admin |  |  |
| 800 | ADMIN | tables/numbered-list-doubled of mutators (re-shown after refusal) | Admin |  |  |
| 831 | ADMIN | tables/numbered-list-doubled of mutators with marks | Admin |  |  |
| 845 | ADMIN | (blank line) | Admin |  |  |
| 846 | ADMIN | Changes saved. | Admin |  | confirmation |
| 847 | ADMIN | (blank line) | Admin |  |  |

## `internal/usercommands/admin.server.go`

ADMIN 21 REFUSAL 5 USAGE 1

| Line | Ruling | Today | Category | Proposal | Note |
|---|---|---|---|---|---|
| 38 | USAGE | help block admincommands/help/command.server (bare "server") | Usage |  |  |
| 53 | ADMIN | blank spacer line before the "server set" config listing | Admin |  |  |
| 83 | ADMIN | blank spacer between config key groups | Admin |  |  |
| 91 | ADMIN | config key: value line in the "server set" listing | Admin |  |  |
| 101 | ADMIN | Time set to %s | Admin |  |  |
| 106 | ADMIN | Time set to %s | Admin |  |  |
| 114 | REFUSAL | config change error: %s=%s (%s) | Admin |  | raw error text shown to staff on purpose |
| 118 | ADMIN | config changed: %s=%s | Admin |  |  |
| 125 | ADMIN | ansi aliases reloaded | Admin |  |  |
| 146 | ADMIN | blank spacer in "server stats" | Admin |  |  |
| 147 | ADMIN | IP/Port: %s | Admin |  |  |
| 148 | ADMIN | blank spacer in "server stats" | Admin |  |  |
| 182 | ADMIN | Timer Stats table | Admin |  |  |
| 296 | ADMIN | Specific Memory table | Admin |  |  |
| 308 | ADMIN | blank spacer opening the "server config" menu | Admin |  |  |
| 311 | ADMIN | numbered config menu | Admin |  |  |
| 324 | REFUSAL | This config value is locked. You must edit the config file directly. | Admin |  |  |
| 339 | ADMIN | blank spacer after a config edit | Admin |  |  |
| 340 | ADMIN | %s has been set to: %s | Admin |  |  |
| 341 | ADMIN | blank spacer after a config edit | Admin |  |  |
| 344 | REFUSAL | raw configs.SetVal error text | Admin |  | raw error shown to staff on purpose |
| 355 | ADMIN | Quitting... | Admin |  |  |
| 367 | REFUSAL | This config value is locked. You must edit the config file directly. | Admin |  |  |
| 387 | REFUSAL | This config value is locked. You must edit the config file directly. | Admin |  |  |
| 404 | ADMIN | blank spacer before the menu path header | Admin |  |  |
| 405 | ADMIN |    [%s] menu path header | Admin |  |  |
| 409 | ADMIN | numbered config submenu | Admin |  |  |

## `internal/usercommands/admin.setcondition.go`

ADMIN 3 REFUSAL 6 USAGE 1

| Line | Ruling | Today | Category | Proposal | Note |
|---|---|---|---|---|---|
| 90 | ADMIN | Search Results table of conditions | Admin |  |  |
| 137 | REFUSAL | conditionId must be an integer > 0. | Admin |  |  |
| 154 | REFUSAL | Condition %d (%s) stacks and can only be applied by whatever move or proc grants it, not this command. | Admin |  | player branch; command did not apply it |
| 157 | ADMIN | Condition %d (%s) applied to %s. | Admin |  | the target player gets no line from this send; the staff member is the recipient |
| 161 | REFUSAL | Condition %d not found. | Admin |  |  |
| 175 | REFUSAL | Condition %d (%s) stacks and can only be applied by whatever move or proc grants it, not this command. | Admin |  | mob branch |
| 178 | ADMIN | Condition %d (%s) applied to %s. | Admin |  | mob target |
| 182 | REFUSAL | Condition %d not found. | Admin |  | mob branch |
| 192 | REFUSAL | target not found. | Admin |  | fallthrough also prints the help block (row 196) |
| 196 | USAGE | help block admincommands/help/command.setcondition | Usage |  |  |

## `internal/usercommands/admin.setmotd.go`

ADMIN 3 REFUSAL 1 USAGE 2

| Line | Ruling | Today | Category | Proposal | Note |
|---|---|---|---|---|---|
| 21 | USAGE | Usage: setmotd <message> - Set the message of the day. | Usage |  |  |
| 22 | USAGE | Use motd to view the current message. | Usage |  | second line of the usage block |
| 29 | REFUSAL | Error setting MOTD: %s | Admin |  |  |
| 33 | ADMIN | Message of the day has been updated. | Admin |  |  |
| 34 | ADMIN | blank spacer | Admin |  |  |
| 35 | ADMIN | New MOTD: %s | Admin |  |  |

## `internal/usercommands/admin.skillset.go`

ADMIN 2 NOTICE 2 REFUSAL 1 USAGE 6

| Line | Ruling | Today | Category | Proposal | Note |
|---|---|---|---|---|---|
| 34 | USAGE | help block admincommands/help/command.skillset (fewer than 2 args) | Usage |  |  |
| 36 | USAGE | Skill Names: | Usage |  | header of the skill list printed under the usage block |
| 38 | USAGE |   <skill name> per skill | Usage |  | list of valid skill names, part of the usage block |
| 54 | USAGE | help block admincommands/help/command.skillset (target given, too few args) | Usage |  | duplicate of row 34 |
| 56 | USAGE | Skill Names: | Usage |  | duplicate of row 36 |
| 58 | USAGE |   <skill name> per skill | Usage |  | duplicate of row 38 |
| 69 | NOTICE | Your "%s" skill level has been set to %d. | Admin when the target is the typist, else Warning | CategoryAdmin when the target is the typist; otherwise stays a player-facing notice (no matching narration category; CategorySystem or CategoryWarning, owner to pick) | sent to targetUser, which is another player when a name is given (then the staff member only gets "done."); one send serves both cases, so branch on targetUser.UserId == user.UserId. Looped once per skill for "all" |
| 73 | ADMIN | done. | Admin |  |  |
| 86 | NOTICE | Your "%s" skill level has been set to %d. | Admin when the target is the typist, else Warning | CategoryAdmin when the target is the typist; otherwise a player-facing notice as row 69 | same shape as row 69 for a single skill |
| 89 | ADMIN | done. | Admin |  |  |
| 92 | REFUSAL | Skill "%s" not found. | Admin |  | sent to targetUser, not user: when staff name another player, that player reads the refusal and staff see nothing. Existing defect; should go to user |

## `internal/usercommands/admin.spawn.go`

ADMIN 3 USAGE 1

| Line | Ruling | Today | Category | Proposal | Note |
|---|---|---|---|---|---|
| 27 | USAGE | help block admincommands/help/command.spawn | Usage |  |  |
| 50 | ADMIN | You wave your hands and %s appears. | Admin |  | staff member's own action line; room gets CategoryMobEmote separately |
| 76 | ADMIN | You wave your hands and %d gold appears. | Admin |  | same |
| 89 | ADMIN | You wave your hands around pathetically. | Admin |  | fallback when the spawn args matched nothing; room gets CategoryMobEmote |

## `internal/usercommands/admin.spell.go`

ADMIN 1 REFUSAL 1 USAGE 1

| Line | Ruling | Today | Category | Proposal | Note |
|---|---|---|---|---|---|
| 28 | USAGE | help block admincommands/help/command.spell | Usage |  |  |
| 36 | REFUSAL | you do not have spell.list permission | Admin |  |  |
| 73 | ADMIN | numbered spell list | Admin |  |  |

## `internal/usercommands/admin.syslogs.go`

ADMIN 2 USAGE 1

| Line | Ruling | Today | Category | Proposal | Note |
|---|---|---|---|---|---|
| 22 | USAGE | help block admincommands/help/command.syslogs | Usage |  |  |
| 31 | ADMIN | Log follow disabled. | Admin |  |  |
| 37 | ADMIN | Log follow enabled. Use syslogs off to turn it off. | Admin |  |  |

## `internal/usercommands/admin.teleport.go`

ADMIN 1 NOTICE 1 REFUSAL 6 ROOM 1 USAGE 1

| Line | Ruling | Today | Category | Proposal | Note |
|---|---|---|---|---|---|
| 32 | USAGE | help block admincommands/help/command.teleport | Usage |  |  |
| 55 | REFUSAL | you do not have teleport.direction permission | Admin |  |  |
| 63 | REFUSAL | No map found (or an error occured)" | Admin |  |  |
| 75 | REFUSAL | you do not have teleport.direction permission | Admin |  | text names teleport.direction though the check is teleport.playername (existing wrong text, not touched) |
| 86 | REFUSAL | you do not have teleport.direction permission | Admin |  | text names teleport.direction though the check is teleport.roomid (existing wrong text, not touched) |
| 100 | REFUSAL | raw rooms.MoveToRoom error text | Admin |  | raw error shown to staff on purpose |
| 104 | ADMIN | Moved to room %d. | Admin |  | shown to the typist even when the typist teleported another player (targetUser) |
| 109 | ROOM | %s appears in a flash of light! | MobEmote | CategoryMobEmote | arrival line to the destination room (sight path). The sibling party-member line at 135 already uses CategoryMobEmote, so this one is the inconsistent one |
| 134 | NOTICE | Moved to room %d. | MobEmote, reworded (plan table C) | CategoryMobEmote or a movement narration category; the room id must not reach a non-staff party member | sent to partyUser, a party member of the teleported leader (non-staff), followed by Look. Leaks the raw room id to a non-staff player |
| 158 | REFUSAL | Invalid teleport command: %s (No RoomId, direction, or character name match) | Admin |  |  |

## `internal/usercommands/admin.zap.go`

ADMIN 4 NOTICE 2 REFUSAL 3

| Line | Ruling | Today | Category | Proposal | Note |
|---|---|---|---|---|---|
| 28 | REFUSAL | Zap target not found. | Admin |  |  |
| 34 | ADMIN | You zap %s with a bolt of lightning! | Admin |  | staff member's own action (mob target) |
| 44 | ADMIN | You zap %s with a bolt of lightning! | Admin |  | staff member's own action (player target) |
| 46 | NOTICE | %s zaps you with a bolt of lightning! | MobEmote | CategoryMobEmote | sent to the zapped player u, a different player; matches the room line CategoryMobEmote used for the same event at line 45 |
| 67 | REFUSAL | Zap Mob not found. | Admin |  |  |
| 70 | ADMIN | You zap %s with a bolt of lightning! | Admin |  | in-combat path, mob |
| 79 | REFUSAL | Zap User not found. | Admin |  |  |
| 82 | ADMIN | You zap %s with a bolt of lightning! | Admin |  | in-combat path, player |
| 90 | NOTICE | %s zaps you with a bolt of lightning! | MobEmote | CategoryMobEmote | sent to the zapped player u, in-combat path |

## `internal/usercommands/admin.zone.go`

ADMIN 11 REFUSAL 5 USAGE 1

| Line | Ruling | Today | Category | Proposal | Note |
|---|---|---|---|---|---|
| 35 | USAGE | help block admincommands/help/command.zone | Usage |  |  |
| 50 | REFUSAL | Couldn't find zone info for %s | Admin |  |  |
| 56 | ADMIN | blank spacer in "zone info" | Admin |  |  |
| 57 | ADMIN | Zone Config for: %s | Admin |  |  |
| 58 | ADMIN | Root Room Id: %d | Admin |  |  |
| 60 | ADMIN | blank spacer in "zone info" | Admin |  |  |
| 67 | REFUSAL | Not enough arguments provided. | Admin |  | argument-count error, not a usage line |
| 78 | REFUSAL | Autoscaling has been removed. Use per-mob statpool values instead. | Admin |  | command did not do what was asked |
| 91 | REFUSAL | Could not find zone config. | Admin |  |  |
| 141 | ADMIN | numbered mutator list (zone edit prompt) | Admin |  |  |
| 170 | REFUSAL | Invalid selection. | Admin |  |  |
| 174 | ADMIN | numbered mutator list re-shown after Invalid selection | Admin |  |  |
| 205 | ADMIN | numbered mutator list after toggle | Admin |  |  |
| 224 | ADMIN |    Note: Music file path must be relative to: %s | Admin |  | editing prompt hint |
| 249 | ADMIN | blank spacer | Admin |  |  |
| 250 | ADMIN | Changes saved. | Admin |  |  |
| 251 | ADMIN | blank spacer | Admin |  |  |

## `internal/usercommands/ban.go`

ADMIN 3 NOTICE 1 REFUSAL 4 USAGE 1

| Line | Ruling | Today | Category | Proposal | Note |
|---|---|---|---|---|---|
| 24 | USAGE | Usage: ban <name> [reason]  \|  ban ip <name\|ip> [reason] | Usage |  |  |
| 40 | REFUSAL | No online player named "%s" (and "%s" is not an IP). | Admin |  |  |
| 45 | REFUSAL | Could not read that player's connection. | Admin |  |  |
| 59 | REFUSAL | IP ban FAILED to save: %s. The ban is NOT in effect. %s | Admin |  | raw error shown to staff on purpose |
| 62 | ADMIN | IP banned: %s | Admin |  |  |
| 84 | REFUSAL | Ban FAILED to save: %s is NOT banned. %s | Admin |  | raw error shown to staff on purpose |
| 90 | NOTICE | You have been banned. %s | Warning | CategoryWarning | sent to the banned player (target) just before the kick |
| 93 | ADMIN | Note: no account named "%s" is currently known. The name is banned and will apply if it is ever registered. | Admin |  | staff-facing note on success path, not a refusal |
| 96 | ADMIN | %s has been banned. Reason: %s | Admin |  |  |

## `internal/usercommands/boot.go`

ADMIN 1 NOTICE 1 REFUSAL 1 USAGE 1

| Line | Ruling | Today | Category | Proposal | Note |
|---|---|---|---|---|---|
| 21 | USAGE | Usage: boot <name> [reason] | Usage |  |  |
| 32 | REFUSAL | No online player named "%s". | Admin |  |  |
| 37 | NOTICE | You have been disconnected by staff. %s | Warning | CategoryWarning | sent to the booted player (target) just before the kick |
| 41 | ADMIN | %s has been booted. | Admin |  |  |

## `internal/usercommands/petitions.go`

ADMIN 10 REFUSAL 3 USAGE 2

| Line | Ruling | Today | Category | Proposal | Note |
|---|---|---|---|---|---|
| 26 | ADMIN | No open petitions. | Admin |  | empty-queue report, not a failure |
| 29 | ADMIN | %d open petition(s): | Admin |  |  |
| 31 | ADMIN |   #%d <reporter>: <snippet> | Admin |  |  |
| 38 | ADMIN | No petitions on record. | Admin |  | empty-queue report, not a failure |
| 42 | ADMIN |   #%d [status] <reporter>: <snippet> | Admin |  |  |
| 48 | USAGE | Usage: petitions resolve <id> [note] | Usage |  |  |
| 53 | REFUSAL | That is not a valid petition id. | Admin |  |  |
| 58 | REFUSAL | raw moderation.Resolve error text (in red) | Admin |  | raw error shown to staff on purpose |
| 61 | ADMIN | Petition #%d marked resolved. | Admin |  |  |
| 67 | USAGE | Usage: petitions \| petitions all \| petitions <id> \| petitions resolve <id> [note] | Usage |  |  |
| 72 | REFUSAL | No petition with that id. | Admin |  |  |
| 75 | ADMIN | Petition #%d [status] | Admin |  |  |
| 76 | ADMIN |   From: %s  When: %s  Room: %d (%s) | Admin |  |  |
| 77 | ADMIN |   Message: %s | Admin |  |  |
| 79 | ADMIN |   Resolved by %s: %s | Admin |  |  |

## `internal/usercommands/unban.go`

ADMIN 2 REFUSAL 2 USAGE 1

| Line | Ruling | Today | Category | Proposal | Note |
|---|---|---|---|---|---|
| 20 | USAGE | Usage: unban <name>  \|  unban ip <ip> | Usage |  |  |
| 28 | REFUSAL | Unban FAILED to save: %s is STILL banned. %s | Admin |  | raw error shown to staff on purpose |
| 31 | ADMIN | IP unbanned: %s | Admin |  |  |
| 36 | REFUSAL | Unban FAILED to save: %s is STILL banned. %s | Admin |  | raw error shown to staff on purpose |
| 39 | ADMIN | %s has been unbanned. | Admin |  |  |

## `modules/aicompanion/commands.go`

ADMIN 10 NOT 8 USAGE 8

| Line | Ruling | Today | Category | Proposal | Note |
|---|---|---|---|---|---|
| 54 | USAGE | aiCompanionUsage block (grant with too few args) | Usage |  | const lines 23-34, multi-line |
| 57 | ADMIN | result string of m.grant: either a refusal (No profile / not online / already has a bonded companion) or "Bonded %s to %s." | Admin |  | one send carries both outcomes; to ride CategoryRefusal on failure the helper must return a bool or category. Staff output otherwise |
| 60 | USAGE | aiCompanionUsage block (revoke with too few args) | Usage |  |  |
| 63 | ADMIN | result string of m.revoke: refusal (not online / has no bonded companion) or "Removed %s from %s. Their memories are kept on disk." | Admin |  | mixed outcomes in one string, as row 57 |
| 66 | USAGE | aiCompanionUsage block (mind with too few args) | Usage |  |  |
| 69 | ADMIN | describeMind report, or "%s is not online." / "%s has no active companion." | Admin |  | mixed outcomes in one string, as row 57 |
| 72 | USAGE | aiCompanionUsage block (trace/prompt with too few args) | Usage |  |  |
| 75 | ADMIN | describeDebug trace or last prompt, or a not-online / no-companion / "No decisions yet" line | Admin |  | mixed outcomes in one string, as row 57 |
| 81 | ADMIN | Circuit breaker reset; model calls resume. | Admin |  |  |
| 84 | USAGE | aiCompanionUsage block (pause/resume with too few args) | Usage |  |  |
| 87 | ADMIN | result string of m.setPaused: refusal (not online / no active companion) or "%s is paused." / "%s is resumed." | Admin |  | mixed outcomes in one string, as row 57 |
| 89 | USAGE | aiCompanionUsage block (unknown subcommand) | Usage |  |  |
| 174 | ADMIN | cmdStatus report (multi-line) | Admin |  |  |
| 179 | ADMIN | No companion profiles are loaded. | Admin |  | empty-list report for "aicompanion profiles" |
| 192 | ADMIN | cmdProfiles list (id: name (mob N)) | Admin |  |  |
| 436 | ADMIN | cmdModels report (multi-line) | Admin |  |  |
| 506 | NOT | Your companion has been reset. Their memories are untouched. | System |  | companion-unstick is a player command (RegisterCommand, admin false, aicompanion.go:585), not staff output; a plain confirmation to the player. Outside the admin scope; leave on CategorySystem unless the spec gives players a category |
| 524 | NOT | %s has gone their own way. | System |  | companion-part, player command (aicompanion.go:586); confirmation to the owner |
| 531 | NOT | confirmPart message (varies) | System |  | companion-part, player command; message built by confirmPart, not read here |
| 549 | NOT | courtStep message (varies) | System |  | companion-court, player command (aicompanion.go:587); manual WrapAnsi(msg,80) is redundant once System never wraps |
| 564 | NOT | setBoundary message (varies) | System |  | companion-boundary, player command (aicompanion.go:588); manual WrapAnsi as row 549 |
| 627 | USAGE | Usage: companion-ask <who> about <what> | Usage |  | player command companion-ask; usage line applies to any command |
| 688 | NOT | %s will %s | System |  | companion-stay, player command (aicompanion.go:590); confirmation to the owner |
| 691 | NOT | %s will %s (companion-stay close\|normal\|free) | System |  | companion-stay status with a trailing hint; player command. The parenthesised hint is usage-like but part of a status sentence |
| 694 | USAGE | Usage: companion-stay close\|normal\|free | Usage |  | player command |
| 732 | NOT | companion-ai tell() lines (varied, wrapped at 80) | System |  | cmdAI helper closure; companion-ai is a player command (aicompanion.go:591); not admin. Content is consent/status text, not read line by line here |

## `modules/playtest/commands.go`

ADMIN 4 REFUSAL 2 USAGE 1

| Line | Ruling | Today | Category | Proposal | Note |
|---|---|---|---|---|---|
| 22 | USAGE | Usage: ai-flag <username> [on\|off] | Usage |  |  |
| 32 | REFUSAL | No such account: %s | Admin |  |  |
| 37 | REFUSAL | Failed to save: <err> | Admin |  | raw error shown to staff on purpose |
| 40 | ADMIN | %s IsAI = %v | Admin |  | confirmation of a successful change (TSV class guess "refusal" was wrong) |
| 59 | ADMIN | No AI-flagged accounts. | Admin |  | empty-list report |
| 62 | ADMIN | AI-flagged accounts: | Admin |  |  |
| 64 | ADMIN |   <username> (online\|offline) per account | Admin |  |  |

## `modules/weather/weather.go`

SHARED 1

| Line | Ruling | Today | Category | Proposal | Note |
|---|---|---|---|---|---|
| 147 | SHARED | sendLine(user, text) used by every weather line | per caller (plan Task 5) | sendLine takes a category argument. Admin callers (weather_commands.go, subcommands zones, fronts, spawn, clear, graph, rebuild, status, seasons and their helpers lines 56-258) pass CategoryAdmin; adminUsage (line 66) and "Usage: weather spawn ..." (line 135) pass CategoryUsage; "simulation not running", "zone not in the graph", "spawn failed", "graph rebuild failed" pass CategoryRefusal. Player callers (printLocalWeather, weather_commands.go lines 74-94, reached by bare "weather" or by a non-permitted user) keep CategorySystem | the single user.SendText in the module; the command is registered non-admin (weather.go:54) and gated inside by HasRolePermission. Leak note: printLocalWeather line 93 shows every player the internal front id and intensity/effective numbers |

## `internal/usercommands/party.go`

ADMIN 10 REFUSAL 2 USAGE 3

| Line | Ruling | Today | Category | Proposal | Note |
|---|---|---|---|---|---|
| 862 | USAGE | Usage: party admin list-npc | Usage |  | first line of two-line usage block; staff-only subcommand (party.debug) |
| 863 | USAGE |        party admin show <id> | Usage |  | second line of the usage block |
| 894 | ADMIN | No active NPC parties. | Admin |  | empty-list report |
| 897 | ADMIN | NPC Parties: | Admin |  |  |
| 899 | ADMIN |   #%-3d Leader: ... Members: ... HomeRoom: ... HelpRoom: ... per party | Admin |  |  |
| 906 | USAGE | Usage: party admin show <id> | Usage |  |  |
| 911 | REFUSAL | Invalid party id: %s | Admin |  |  |
| 923 | REFUSAL | No party with id %d. | Admin |  |  |
| 936 | ADMIN | Party #%d | Admin |  |  |
| 944 | ADMIN |   Leader: %s (player %d, room %d) | Admin |  |  |
| 951 | ADMIN |   Leader: %s (mob %d, room %d) | Admin |  |  |
| 959 | ADMIN |   Leader: (none) | Admin |  |  |
| 962 | ADMIN |   Members: | Admin |  |  |
| 973 | ADMIN |     %s (player %d, room %d)%s | Admin |  |  |
| 978 | ADMIN |     %s (mob %d, room %d)%s | Admin |  |  |
