package usecase

import (
	"encoding/json"
	"fmt"
	"reflect"

	"sysscope/internal/domain"
)

// CompareUseCase performs a field-level diff of two reports.
type CompareUseCase struct{}

// NewCompareUseCase creates a new CompareUseCase.
func NewCompareUseCase() *CompareUseCase {
	return &CompareUseCase{}
}

// Compare returns a list of differences between two reports.
func (uc *CompareUseCase) Compare(a, b *domain.Report) []domain.CompareResult {
	// We serialise to map[string]interface{} for a deep comparison.
	aJSON, _ := json.Marshal(a)
	bJSON, _ := json.Marshal(b)

	var aMap, bMap map[string]interface{}
	_ = json.Unmarshal(aJSON, &aMap)
	_ = json.Unmarshal(bJSON, &bMap)

	var results []domain.CompareResult
	compareMaps("", aMap, bMap, &results)
	return results
}

func compareMaps(prefix string, a, b map[string]interface{}, results *[]domain.CompareResult) {
	for key, av := range a {
		fullKey := key
		if prefix != "" {
			fullKey = prefix + "." + key
		}
		bv, exists := b[key]
		if !exists {
			*results = append(*results, domain.CompareResult{
				Field:        fullKey,
				Report1Value: formatVal(av),
				Report2Value: "<missing>",
				Diff:         "removed",
			})
			continue
		}
		compareValues(fullKey, av, bv, results)
	}
	for key, bv := range b {
		fullKey := key
		if prefix != "" {
			fullKey = prefix + "." + key
		}
		if _, exists := a[key]; !exists {
			*results = append(*results, domain.CompareResult{
				Field:        fullKey,
				Report1Value: "<missing>",
				Report2Value: formatVal(bv),
				Diff:         "added",
			})
		}
	}
}

func compareValues(field string, a, b interface{}, results *[]domain.CompareResult) {
	// If both are maps, recurse
	am, aIsMap := a.(map[string]interface{})
	bm, bIsMap := b.(map[string]interface{})
	if aIsMap && bIsMap {
		compareMaps(field, am, bm, results)
		return
	}

	// If both are slices, compare length and content
	if reflect.DeepEqual(a, b) {
		return // no difference
	}

	// Scalar difference
	*results = append(*results, domain.CompareResult{
		Field:        field,
		Report1Value: formatVal(a),
		Report2Value: formatVal(b),
		Diff:         fmt.Sprintf("%v → %v", formatVal(a), formatVal(b)),
	})
}

func formatVal(v interface{}) string {
	if v == nil {
		return "<nil>"
	}
	return fmt.Sprintf("%v", v)
}
