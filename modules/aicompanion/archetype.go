package aicompanion

import (
	"fmt"
	"sort"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/skills"
)

// Archetype and skills (F8.1 to F8.9). An archetype is a set of weights
// over DOGMud's real skills, not a class and not a bonus. It shapes what
// catches the companion's eye, which gear it favours, and how it talks
// about itself. Skill levels only change through the game's own
// skill-by-use progression; the companion is told its levels in the same
// words the game's skill display uses, never as numbers.

// knownSkills is every skill tag the game defines. Profiles are validated
// against it at load, so a misspelt skill is an error, never a silent no-op.
var knownSkills = map[skills.SkillTag]bool{
	skills.WeaponCombat:  true,
	skills.UnarmedCombat: true,
	skills.RangedCombat:  true,
	skills.Spellcasting:  true,
	skills.Rhetoric:      true,
	skills.Skullduggery:  true,
	skills.Search:        true,
	skills.Bartering:     true,
	skills.Blacksmithing: true,
	skills.Alchemy:       true,
	skills.Tailoring:     true,
	skills.Cooking:       true,
	skills.Jewelcrafting: true,
	skills.Enchanting:    true,
	skills.Salvage:       true,
	skills.Woodwork:      true,
	skills.Manifestation: true,
}

// Archetype is the authored craft of a companion.
type Archetype struct {
	Primary   string             `yaml:"primary"`   // e.g. archer
	Secondary string             `yaml:"secondary"` // e.g. scout
	Skills    map[string]float64 `yaml:"skills"`    // skill tag -> preference 0..1
	Gear      []string           `yaml:"gear"`      // words in item names it favours: bow, leather
	Avoid     []string           `yaml:"avoid"`     // words in item names it will not use: plate, greatsword
}

// validate checks every skill against the game's list.
func (a *Archetype) validate() error {
	for tag, w := range a.Skills {
		if !knownSkills[skills.SkillTag(tag)] {
			return fmt.Errorf(`unknown skill %q in archetype`, tag)
		}
		if w < 0 || w > 1 {
			return fmt.Errorf(`skill %q weight %v must be between 0 and 1`, tag, w)
		}
	}
	return nil
}

// favouredSkills returns the archetype's skills, strongest preference first.
func (a *Archetype) favouredSkills() []string {
	tags := make([]string, 0, len(a.Skills))
	for tag := range a.Skills {
		tags = append(tags, tag)
	}
	sort.Slice(tags, func(i, j int) bool {
		if a.Skills[tags[i]] != a.Skills[tags[j]] {
			return a.Skills[tags[i]] > a.Skills[tags[j]]
		}
		return tags[i] < tags[j]
	})
	return tags
}

// gearFit scores how well an item name suits the archetype: positive for
// favoured gear, strongly negative for gear it avoids.
func (a *Archetype) gearFit(name string) float64 {
	l := strings.ToLower(name)
	for _, w := range a.Avoid {
		if w != `` && strings.Contains(l, strings.ToLower(w)) {
			return -0.5
		}
	}
	for _, w := range a.Gear {
		if w != `` && strings.Contains(l, strings.ToLower(w)) {
			return 0.35
		}
	}
	return 0
}

// skillWords describes the companion's favoured skills with the game's rank
// words (novice, apprentice, journeyman...).
func skillWords(mob *mobs.Mob, a *Archetype) []string {
	var out []string
	for _, tag := range a.favouredSkills() {
		level := mob.Character.GetSkillLevel(skills.SkillTag(tag))
		out = append(out, fmt.Sprintf(`%s: %s`, strings.ReplaceAll(tag, `-`, ` `), skills.GetSkillRankDescription(level)))
	}
	return out
}

// skillSnapshot records the rank word of each favoured skill, so a change of
// rank during a session can be noticed (F8.7).
func skillSnapshot(mob *mobs.Mob, a *Archetype) map[string]string {
	out := map[string]string{}
	for tag := range a.Skills {
		out[tag] = skills.GetSkillRankDescription(mob.Character.GetSkillLevel(skills.SkillTag(tag)))
	}
	return out
}

// skillGains compares two snapshots and describes any rank that changed.
func skillGains(before map[string]string, after map[string]string) []string {
	var out []string
	for tag, now := range after {
		if was, ok := before[tag]; ok && was != now {
			out = append(out, fmt.Sprintf(`your %s has grown from %s to %s`, strings.ReplaceAll(tag, `-`, ` `), was, now))
		}
	}
	sort.Strings(out)
	return out
}
