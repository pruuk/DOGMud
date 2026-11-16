package mapper

type mapRender struct {
	Render [][]rune
	legend map[rune]string // Symbol=>Name map of any notes for the legend
}

func newMapRender(mapWidth int, mapHeight int) mapRender {

	ret := mapRender{
		Render: make([][]rune, mapHeight),
		legend: make(map[rune]string, 3),
	}

	for y := range mapHeight {
		ret.Render[y] = make([]rune, mapWidth)
		for x := range mapWidth {
			ret.Render[y][x] = ' '
		}
	}

	return ret
}

func (m *mapRender) GetLegend(overrides map[rune]string) map[rune]string {

	ret := map[rune]string{}

	for r, name := range m.legend {
		if overrides != nil {
			if oName, ok := overrides[r]; ok {
				ret[r] = oName
			} else {
				ret[r] = name
			}
		} else {
			ret[r] = name
		}

	}

	return ret
}
