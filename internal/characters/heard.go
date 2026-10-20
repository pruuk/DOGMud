package characters

import "sort"

// HasHeard reports whether the character has heard a talk topic, by its
// saved key "<mob template id>.<key>" (dialogue.HeardKey).
func (c *Character) HasHeard(key string) bool {
	i := sort.SearchStrings(c.HeardTopics, key)
	return i < len(c.HeardTopics) && c.HeardTopics[i] == key
}

// MarkHeard records a heard info topic, keeping the list sorted and unique.
func (c *Character) MarkHeard(key string) {
	i := sort.SearchStrings(c.HeardTopics, key)
	if i < len(c.HeardTopics) && c.HeardTopics[i] == key {
		return
	}
	c.HeardTopics = append(c.HeardTopics, "")
	copy(c.HeardTopics[i+1:], c.HeardTopics[i:])
	c.HeardTopics[i] = key
}

// RenameHeardTopics applies the topic-key manifest's renames (old -> new)
// so a renamed topic stays heard. Run at load.
func (c *Character) RenameHeardTopics(renames map[string]string) {
	if len(renames) == 0 || len(c.HeardTopics) == 0 {
		return
	}
	keys := c.HeardTopics
	c.HeardTopics = nil
	for _, k := range keys {
		if n, ok := renames[k]; ok {
			k = n
		}
		c.MarkHeard(k)
	}
}
