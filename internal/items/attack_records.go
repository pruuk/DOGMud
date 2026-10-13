package items

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v2"
)

// The record layout (#370, M6 rows 26-27): each split holds tier -> list of
// records, and each record carries every role's line for ONE blow. The old
// role-first layout paired separate per-role lists only by position, so a
// reorder in one list silently narrated different blows to different
// audiences. Records make that unrepresentable: moving a record moves all of
// its roles. The decoder fills the same per-role SkillTieredMessages the rest
// of the package has always used, so PoolFor and Render are unchanged.

var (
	tierNames     = []string{"beginner", "expert", "master"}
	togetherRoles = []string{"actor", "actee", "observer"}
	separateRoles = []string{"actor", "actee", "observer", "remote_observer"}
)

func isOneOf(s string, set []string) bool {
	for _, x := range set {
		if x == s {
			return true
		}
	}
	return false
}

// decodeTieredRecords reads one split in the record layout and returns its
// pools keyed by role. roleFirst is true when the node is instead the legacy
// role -> tier -> list layout, which the caller decodes itself.
func decodeTieredRecords(unmarshal func(interface{}) error, roles []string) (pools map[string]SkillTieredMessages, roleFirst bool, err error) {
	var probe map[string]interface{}
	if err := unmarshal(&probe); err != nil {
		return nil, false, err
	}
	sawRole, sawTier := false, false
	for k := range probe {
		switch {
		case isOneOf(k, separateRoles):
			sawRole = true
		case isOneOf(k, tierNames):
			sawTier = true
		default:
			return nil, false, fmt.Errorf("unknown key %q", k)
		}
	}
	if sawRole && sawTier {
		return nil, false, fmt.Errorf("mixes role keys and tier keys")
	}
	if sawRole {
		return nil, true, nil
	}

	var tiers map[string][]map[string]string
	if err := unmarshal(&tiers); err != nil {
		return nil, false, err
	}
	pools = make(map[string]SkillTieredMessages, len(roles))
	for _, tier := range tierNames {
		for i, rec := range tiers[tier] {
			for k := range rec {
				if !isOneOf(k, roles) {
					return nil, false, fmt.Errorf("%s record %d: unknown key %q", tier, i, k)
				}
			}
			for _, role := range roles {
				line, ok := rec[role]
				if !ok {
					return nil, false, fmt.Errorf("%s record %d: missing role %q", tier, i, role)
				}
				if strings.TrimSpace(line) == "" {
					return nil, false, fmt.Errorf("%s record %d: empty %q line", tier, i, role)
				}
				p := pools[role]
				switch tier {
				case "beginner":
					p.Beginner = append(p.Beginner, ItemMessage(line))
				case "expert":
					p.Expert = append(p.Expert, ItemMessage(line))
				case "master":
					p.Master = append(p.Master, ItemMessage(line))
				}
				pools[role] = p
			}
		}
	}
	return pools, false, nil
}

// UnmarshalYAML accepts the record layout and, until the last batch is
// converted, the legacy role-first layout.
func (m *TogetherMessages) UnmarshalYAML(unmarshal func(interface{}) error) error {
	pools, roleFirst, err := decodeTieredRecords(unmarshal, togetherRoles)
	if err != nil {
		return fmt.Errorf("together: %w", err)
	}
	if roleFirst {
		type plain TogetherMessages
		return unmarshal((*plain)(m))
	}
	m.ToAttacker, m.ToDefender, m.ToRoom = pools["actor"], pools["actee"], pools["observer"]
	return nil
}

// UnmarshalYAML accepts the record layout and, until the last batch is
// converted, the legacy role-first layout.
func (m *SeparateMessages) UnmarshalYAML(unmarshal func(interface{}) error) error {
	pools, roleFirst, err := decodeTieredRecords(unmarshal, separateRoles)
	if err != nil {
		return fmt.Errorf("separate: %w", err)
	}
	if roleFirst {
		type plain SeparateMessages
		return unmarshal((*plain)(m))
	}
	m.ToAttacker, m.ToDefender = pools["actor"], pools["actee"]
	m.ToAttackerRoom, m.ToDefenderRoom = pools["observer"], pools["remote_observer"]
	return nil
}

// UnmarshalYAML decodes each intensity on its own so a record error names the
// intensity it came from; the file loader adds the path.
func (a *AttackTypes) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var raw map[Intensity]interface{}
	if err := unmarshal(&raw); err != nil {
		return err
	}
	out := make(AttackTypes, len(raw))
	for intensity, node := range raw {
		b, err := yaml.Marshal(node)
		if err != nil {
			return fmt.Errorf("option[`%s`]: %w", intensity, err)
		}
		var opts AttackOptions
		if err := yaml.Unmarshal(b, &opts); err != nil {
			return fmt.Errorf("option[`%s`]: %w", intensity, err)
		}
		out[intensity] = opts
	}
	*a = out
	return nil
}
