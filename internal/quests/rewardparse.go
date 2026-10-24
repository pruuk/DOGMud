package quests

import (
	"strconv"
	"strings"
)

// SkillGrant is one parsed skill reward: a skill tag and the level it trains to.
type SkillGrant struct {
	Skill string
	Level int
}

// StatGrant is one parsed stat reward: a lowercase stat name and the amount added.
type StatGrant struct {
	Stat   string
	Amount int
}

// ItemGrant is one parsed item-stockpile reward: an item id and a quantity.
type ItemGrant struct {
	ItemId int
	Qty    int
}

// ParseSkillGrants parses a skillinfo reward, "skill:level[,skill:level]".
// Malformed entries (no colon, unparseable level, empty name) are skipped.
func ParseSkillGrants(skillInfo string) []SkillGrant {
	if skillInfo == `` {
		return nil
	}
	var grants []SkillGrant
	for _, entry := range strings.Split(skillInfo, `,`) {
		details := strings.Split(strings.TrimSpace(entry), `:`)
		if len(details) <= 1 {
			continue
		}
		name := strings.ToLower(strings.TrimSpace(details[0]))
		level, err := strconv.Atoi(strings.TrimSpace(details[1]))
		if err != nil || name == `` {
			continue
		}
		grants = append(grants, SkillGrant{Skill: name, Level: level})
	}
	return grants
}

// ParseStatGrants parses a stat_info reward, "stat:amount[,stat:amount]".
// Malformed entries (no colon, unparseable amount, empty name) are skipped.
func ParseStatGrants(statInfo string) []StatGrant {
	if statInfo == `` {
		return nil
	}
	var grants []StatGrant
	for _, entry := range strings.Split(statInfo, `,`) {
		details := strings.Split(strings.TrimSpace(entry), `:`)
		if len(details) <= 1 {
			continue
		}
		name := strings.ToLower(strings.TrimSpace(details[0]))
		amount, err := strconv.Atoi(strings.TrimSpace(details[1]))
		if err != nil || name == `` {
			continue
		}
		grants = append(grants, StatGrant{Stat: name, Amount: amount})
	}
	return grants
}

// ParseRecipeGrants parses a recipe_info reward, a comma-separated list of
// recipe ids. Empty entries are skipped.
func ParseRecipeGrants(recipeInfo string) []string {
	if recipeInfo == `` {
		return nil
	}
	var ids []string
	for _, entry := range strings.Split(recipeInfo, `,`) {
		if id := strings.TrimSpace(entry); id != `` {
			ids = append(ids, id)
		}
	}
	return ids
}

// ParseItemGrants parses an item_info reward, "itemid[:qty][,itemid[:qty]]".
// A bare id means one. Unparseable ids and non-positive quantities are skipped.
func ParseItemGrants(itemInfo string) []ItemGrant {
	if itemInfo == `` {
		return nil
	}
	var grants []ItemGrant
	for _, entry := range strings.Split(itemInfo, `,`) {
		entry = strings.TrimSpace(entry)
		if entry == `` {
			continue
		}
		details := strings.Split(entry, `:`)
		itemId, err := strconv.Atoi(strings.TrimSpace(details[0]))
		if err != nil || itemId <= 0 {
			continue
		}
		qty := 1
		if len(details) > 1 {
			q, qErr := strconv.Atoi(strings.TrimSpace(details[1]))
			if qErr != nil || q <= 0 {
				continue
			}
			qty = q
		}
		grants = append(grants, ItemGrant{ItemId: itemId, Qty: qty})
	}
	return grants
}
