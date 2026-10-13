package main

import (
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/fileloader"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/species"
)

// secondPersonWord matches the reader being addressed. "you're" matches too:
// the apostrophe is a word boundary.
var secondPersonWord = regexp.MustCompile(`(?i)\b(you|your|yours|yourself|yourselves)\b`)

// secondPersonEmoteAllowlist names the deliberate exceptions, keyed
// "<list> | <owner file key> | <emote text>". Each needs a reason. It is empty:
// every shipped case was reworded to "its foe" in #515, including the Ascended
// species line the issue floated as a room-wide address.
var secondPersonEmoteAllowlist = map[string]string{}

// TestMobFightEmotesDoNotAddressEveryoneAsYou is the #515 guard. A mob's
// `emote` command goes to everyone in the room through actions.SendSeen, so a
// fight emote reading "kicks at your legs" tells two onlookers who are not in
// the fight that they are being kicked. The engine has no target-aware emote
// form (an emote is one fixed string; only `sayto` and its kin resolve a
// target), so combatcommands and angrycommands, which fire on a mob that is
// fighting a specific opponent, must speak of "its foe" in the third person.
//
// idlecommands are not covered: an idle emote has no opponent, it is room
// ambience that every reader may take personally ("watches you from a crack").
func TestMobFightEmotesDoNotAddressEveryoneAsYou(t *testing.T) {
	mudlog.SetupLogger(nil, `LOW`, ``, false)

	configs.SetConfigForTest(t, configs.GetConfig())
	if err := configs.ReloadConfig(); err != nil {
		t.Fatalf("ReloadConfig: %v", err)
	}

	base := configs.GetFilePathsConfig().DataFiles.String()

	mobPath := base + `/mobs`
	templates, err := fileloader.LoadAllFlatFiles[int, *mobs.Mob](mobPath)
	if err != nil {
		t.Fatalf("loading mob templates from %s: %v", mobPath, err)
	}

	speciesPath := base + `/species`
	allSpecies, err := fileloader.LoadAllFlatFiles[int, *species.Species](speciesPath)
	if err != nil {
		t.Fatalf("loading species from %s: %v", speciesPath, err)
	}

	checked := 0
	check := func(list, owner string, commands []string) {
		for _, c := range commands {
			for _, part := range strings.Split(c, `;`) {
				part = strings.TrimSpace(part)
				if part != `emote` && !strings.HasPrefix(part, `emote `) {
					continue
				}
				checked++
				if !secondPersonWord.MatchString(part) {
					continue
				}
				key := list + ` | ` + owner + ` | ` + part
				if _, ok := secondPersonEmoteAllowlist[key]; ok {
					continue
				}
				t.Errorf("%s of %s addresses every onlooker as you: %q "+
					"(an emote reaches the whole room; write the opponent as \"its foe\")",
					list, owner, part)
			}
		}
	}

	for id, m := range templates {
		owner := m.Character.Name + ` (mob ` + strconv.Itoa(id) + `)`
		check(`combatcommands`, owner, m.CombatCommands)
		check(`angrycommands`, owner, m.AngryCommands)
	}
	for id, s := range allSpecies {
		check(`angrycommands`, s.Name+` (species `+strconv.Itoa(id)+`)`, s.AngryCommands)
	}

	if checked < 50 {
		t.Fatalf("only %d fight emotes were inspected; the loader or field names drifted and the guard is blind", checked)
	}
}
