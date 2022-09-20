package bson

import (
	"errors"
	"fmt"
)

// GenerateCursorQuery generates and returns a cursor range query
func GenerateCursorQuery(shouldSecondarySortOnID bool, paginatedField string, comparisonOp string, cursorFieldValues []interface{}) (map[string]interface{}, error) {
	var query map[string]interface{}
	if (shouldSecondarySortOnID && len(cursorFieldValues) != 2) ||
		(!shouldSecondarySortOnID && len(cursorFieldValues) != 1) {
		return nil, errors.New("wrong number of cursor field values specified")
	}

	rangeOp := fmt.Sprintf("%se", comparisonOp)

	if shouldSecondarySortOnID {
		query = map[string]interface{}{"$or": []map[string]interface{}{
			{paginatedField: map[string]interface{}{comparisonOp: cursorFieldValues[0]}},
			{"$and": []map[string]interface{}{
				{paginatedField: map[string]interface{}{rangeOp: cursorFieldValues[0]}},
				{"_id": map[string]interface{}{comparisonOp: cursorFieldValues[1]}},
			}},
		}}
	} else {
		query = map[string]interface{}{paginatedField: map[string]interface{}{comparisonOp: cursorFieldValues[0]}}
	}
	return query, nil
}
