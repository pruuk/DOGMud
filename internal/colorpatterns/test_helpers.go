package colorpatterns

// SeedPatternsForTest adds numeric colour patterns for a test without
// reading color-patterns.yaml. The returned func removes them again.
func SeedPatternsForTest(patterns map[string][]int) func() {
	prev := map[string][]int{}
	for name := range patterns {
		if v, ok := numericPatterns[name]; ok {
			prev[name] = v
		}
		numericPatterns[name] = patterns[name]
	}
	return func() {
		for name := range patterns {
			if v, ok := prev[name]; ok {
				numericPatterns[name] = v
			} else {
				delete(numericPatterns, name)
			}
		}
	}
}
