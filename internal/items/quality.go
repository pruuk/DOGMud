package items

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/configs"
)

// Quality is the grade of a gathered or processed material instance: a
// skinned pelt, a chopped log, a cut of meat, and later anything crafted from
// them. It lives on the item INSTANCE (Item.Quality), never on the spec, so
// two wolf pelts from the same template can be crude and pristine.
//
// QualityNone (0) means "ungraded" and is what every item that predates
// grading carries. It is not a grade: it prints nothing, sells at 1.0x and
// stacks only with other ungraded items. Do not treat it as "standard".
//
// There is no "ruined" grade. A ruined result is the absence of an item; the
// gathering code simply produces nothing.
type Quality int

const (
	QualityNone     Quality = 0
	QualityCrude    Quality = 1
	QualityStandard Quality = 2
	QualityFine     Quality = 3
	QualitySuperb   Quality = 4
	QualityPristine Quality = 5

	QualityMin = QualityCrude
	QualityMax = QualityPristine
)

var qualityNames = map[Quality]string{
	QualityCrude:    `crude`,
	QualityStandard: `standard`,
	QualityFine:     `fine`,
	QualitySuperb:   `superb`,
	QualityPristine: `pristine`,
}

// String returns the lowercase grade word ("fine"), or "" for QualityNone and
// for anything out of range.
func (q Quality) String() string {
	return qualityNames[q]
}

// Valid reports whether q is a real grade (crude through pristine).
func (q Quality) Valid() bool {
	return q >= QualityMin && q <= QualityMax
}

// Clamp bounds q to crude..pristine. QualityNone is returned unchanged, so a
// caller can clamp an ungraded item without accidentally grading it.
func (q Quality) Clamp() Quality {
	if q == QualityNone {
		return q
	}
	if q < QualityMin {
		return QualityMin
	}
	if q > QualityMax {
		return QualityMax
	}
	return q
}

// ParseQuality maps a grade word back to its Quality. Unknown words return
// QualityNone, false.
func ParseQuality(word string) (Quality, bool) {
	for q, name := range qualityNames {
		if name == word {
			return q, true
		}
	}
	return QualityNone, false
}

// QualityValueMultiplier is the sell-value multiplier for a grade, read from
// Balance (QualityValue*). Ungraded and out-of-range grades return 1.0, so
// pricing for every pre-existing item is unchanged.
func QualityValueMultiplier(q Quality) float64 {
	b := configs.GetBalanceConfig()
	switch q {
	case QualityCrude:
		return float64(b.QualityValueCrude)
	case QualityStandard:
		return float64(b.QualityValueStandard)
	case QualityFine:
		return float64(b.QualityValueFine)
	case QualitySuperb:
		return float64(b.QualityValueSuperb)
	case QualityPristine:
		return float64(b.QualityValuePristine)
	}
	return 1.0
}

// qualitySuffix is the display-name suffix for a graded item. Standard is
// deliberately silent: it is the expected result, and printing "(standard)" on
// every pelt would bury the grades that matter. Crude, fine, superb and
// pristine are shown.
func (q Quality) qualitySuffix() string {
	if !q.Valid() || q == QualityStandard {
		return ``
	}
	return fmt.Sprintf(` <ansi fg="item-quality">(%s)</ansi>`, q.String())
}
