package characters

import "testing"

func TestRecordItemRead(t *testing.T) {
	c := &Character{}
	if c.HasReadItem(40508) {
		t.Fatal("a new character has read nothing")
	}
	c.RecordItemRead(40508)
	c.RecordItemRead(40508)
	if !c.HasReadItem(40508) {
		t.Fatal("the item should be recorded")
	}
	if got := len(c.ItemsRead); got != 1 {
		t.Fatalf("a re-read counts once, got %d", got)
	}
	if got := c.CountItemsRead([]int{40508, 40509}); got != 1 {
		t.Fatalf("CountItemsRead = %d, want 1", got)
	}
}
