package characters

// RecordItemRead notes that this character has read a readable item. Reading
// it again changes nothing, and the record outlives the item.
func (c *Character) RecordItemRead(itemId int) {
	if c.ItemsRead == nil {
		c.ItemsRead = make(map[int]bool)
	}
	c.ItemsRead[itemId] = true
}

// HasReadItem reports whether this character has read the item.
func (c *Character) HasReadItem(itemId int) bool {
	return c.ItemsRead[itemId]
}

// CountItemsRead reports how many of the given items this character has read.
func (c *Character) CountItemsRead(itemIds []int) int {
	n := 0
	for _, id := range itemIds {
		if c.ItemsRead[id] {
			n++
		}
	}
	return n
}
