package items

import (
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v2"
)

// The record layout (#370, M6 rows 26-27): each split holds tier -> list of
// records, and each record carries every role's line for ONE blow. The
// retired role-first layout paired separate per-role lists only by position,
// so a reorder in one list silently narrated different blows to different
// audiences; it is now rejected at load. Records make that unrepresentable:
// moving a record moves all of its roles. The decoder fills the same per-role
// SkillTieredMessages the rest of the package has always used, so PoolFor and
// Render are unchanged.

var (
	tierNames     = []string{"beginner", "expert", "master"}
	togetherRoles = []string{"actor", "actee", "observer"}
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
// pools keyed by role. The retired role -> tier -> list layout is rejected.
func decodeTieredRecords(unmarshal func(interface{}) error, roles []string) (pools map[string]SkillTieredMessages, err error) {
	var probe map[string]interface{}
	if err := unmarshal(&probe); err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(probe))
	for k := range probe {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	sawRole, sawTier := false, false
	for _, k := range keys {
		switch {
		case isOneOf(k, roles):
			sawRole = true
		case isOneOf(k, tierNames):
			sawTier = true
		default:
			return nil, fmt.Errorf("unknown key %q", k)
		}
	}
	if sawRole && sawTier {
		return nil, fmt.Errorf("mixes role keys and tier keys")
	}
	if sawRole {
		return nil, fmt.Errorf("role-first layout is retired; write one record per blow")
	}

	var tiers map[string][]map[string]string
	if err := unmarshal(&tiers); err != nil {
		return nil, err
	}
	// A map decode silently keeps the last of a repeated key, so a second
	// keys-only pass finds duplicates. Values stay exact strings from `tiers`.
	var slices map[string][]yaml.MapSlice
	if err := unmarshal(&slices); err != nil {
		return nil, err
	}
	pools = make(map[string]SkillTieredMessages, len(roles))
	for _, tier := range tierNames {
		for i, rec := range tiers[tier] {
			if i < len(slices[tier]) {
				seen := make(map[string]bool, len(slices[tier][i]))
				for _, item := range slices[tier][i] {
					k := fmt.Sprint(item.Key)
					if seen[k] {
						return nil, fmt.Errorf("%s record %d: duplicate key %q", tier, i, k)
					}
					seen[k] = true
				}
			}
			recKeys := make([]string, 0, len(rec))
			for k := range rec {
				recKeys = append(recKeys, k)
			}
			sort.Strings(recKeys)
			for _, k := range recKeys {
				if !isOneOf(k, roles) {
					return nil, fmt.Errorf("%s record %d: unknown key %q", tier, i, k)
				}
			}
			for _, role := range roles {
				line, ok := rec[role]
				if !ok {
					return nil, fmt.Errorf("%s record %d: missing role %q", tier, i, role)
				}
				if strings.TrimSpace(line) == "" {
					return nil, fmt.Errorf("%s record %d: empty %q line", tier, i, role)
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
	return pools, nil
}

// UnmarshalYAML accepts only the record layout.
func (m *TogetherMessages) UnmarshalYAML(unmarshal func(interface{}) error) error {
	pools, err := decodeTieredRecords(unmarshal, togetherRoles)
	if err != nil {
		return fmt.Errorf("together: %w", err)
	}
	m.ToAttacker, m.ToDefender, m.ToRoom = pools["actor"], pools["actee"], pools["observer"]
	return nil
}

// UnmarshalYAML accepts only `together`. A `separate` block (a blow whose
// participants stand in different rooms) can never render: round combat
// releases a target in another room before the wait round or the swing
// (hooks.resolveCombatTarget), so it is rejected at load rather than shipped
// as dead copy (owner ruling C, spec 2026-10-16 #541 section 3).
func (a *AttackOptions) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var probe map[string]interface{}
	if err := unmarshal(&probe); err != nil {
		return err
	}
	keys := make([]string, 0, len(probe))
	for k := range probe {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if k != "together" {
			return fmt.Errorf("unknown key %q (only together; separate is retired)", k)
		}
	}
	var raw struct {
		Together TogetherMessages `yaml:"together"`
	}
	if err := unmarshal(&raw); err != nil {
		return err
	}
	a.Together = raw.Together
	return nil
}

// attackOptionsCapture stores the unmarshal func yaml.v2 hands it, so each
// intensity can be decoded later, in sorted order, from the ORIGINAL node. That
// keeps strict decoding, exact scalar text and true line numbers, none of which
// survive a marshal and re-parse round trip.
type attackOptionsCapture struct {
	unmarshal func(interface{}) error
}

func (c *attackOptionsCapture) UnmarshalYAML(unmarshal func(interface{}) error) error {
	c.unmarshal = unmarshal
	return nil
}

// UnmarshalYAML decodes each intensity on its own so a record error names the
// intensity it came from; the file loader adds the path.
func (a *AttackTypes) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var raw map[Intensity]*attackOptionsCapture
	if err := unmarshal(&raw); err != nil {
		return err
	}
	names := make([]string, 0, len(raw))
	for intensity := range raw {
		names = append(names, string(intensity))
	}
	sort.Strings(names)

	out := make(AttackTypes, len(raw))
	for _, name := range names {
		intensity := Intensity(name)
		var opts AttackOptions
		if c := raw[intensity]; c != nil && c.unmarshal != nil {
			if err := c.unmarshal(&opts); err != nil {
				return fmt.Errorf("option[`%s`]: %w", intensity, err)
			}
		}
		out[intensity] = opts
	}
	*a = out
	return nil
}
