package templates

import (
	"strconv"
	"testing"
)

// The spec asks plan 3a to measure what trying narrow forms costs before it
// merges (section 2, "Measuring limits"). Run on one CPU, as production has:
//
//	go test ./internal/templates/ -run '^$' -bench BenchmarkTableAt -cpu 1 -benchmem
//
// At 80 the base table renders once. At 30 the base, the 40-column form and
// the 30-column form each render: the worst case a player-requested screen
// pays.
func benchTable() TemplateTable {
	rows := make([][]string, 0, 20)
	for i := 0; i < 20; i++ {
		rows = append(rows, []string{"spell" + strconv.Itoa(i), "Aurora of the Slumbering Deep", "group", "12"})
	}
	return GetTable("Spells", []string{"SpellId", "Name", "Target", "Cost"}, rows)
}

func BenchmarkTableAt80(b *testing.B) {
	data := benchTable()
	for i := 0; i < b.N; i++ {
		if _, err := process("tables/generic", data, templateConfig{}, 80); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTableAt30(b *testing.B) {
	data := benchTable()
	for i := 0; i < b.N; i++ {
		if _, err := process("tables/generic", data, templateConfig{}, 30); err != nil {
			b.Fatal(err)
		}
	}
}
