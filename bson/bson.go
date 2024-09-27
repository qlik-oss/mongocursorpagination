package bson

import (
	"errors"
	"fmt"
)

// GenerateCursorQuery generates and returns a cursor range query
func GenerateCursorQuery(paginatedFields []string, comparisonOps []string, cursorFieldValues []interface{}) (map[string]interface{}, error) {
	var query map[string]interface{}

	if len(paginatedFields) != len(cursorFieldValues) {
		return nil, errors.New("wrong number of cursor field values specified")
	}

	if len(comparisonOps) != len(cursorFieldValues) {
		return nil, errors.New("wrong number of comparison operators specified")
	}

	for i := range comparisonOps {
		if comparisonOps[i] != "$lt" && comparisonOps[i] != "$gt" {
			return nil, errors.New("invalid comparison operator specified: only $lt and $gt are allowed")
		}
	}

	if len(paginatedFields) > 1 {
		if len(paginatedFields) == 2 {
			rangeOp := fmt.Sprintf("%se", comparisonOps[0])
			query = map[string]interface{}{"$or": []map[string]interface{}{
				{paginatedFields[0]: map[string]interface{}{comparisonOps[0]: cursorFieldValues[0]}},
				{"$and": []map[string]interface{}{
					{paginatedFields[0]: map[string]interface{}{rangeOp: cursorFieldValues[0]}},
					{"_id": map[string]interface{}{comparisonOps[0]: cursorFieldValues[1]}},
				}},
			}}
		} else {
			conditions := make([]map[string]interface{}, len(paginatedFields)-1)
			for i := 0; i < len(paginatedFields)-1; i++ {
				rangeOp := fmt.Sprintf("%se", comparisonOps[i])
				conditions[i] = map[string]interface{}{"$or": []map[string]interface{}{
					{paginatedFields[i]: map[string]interface{}{comparisonOps[i]: cursorFieldValues[i]}},
					{"$and": []map[string]interface{}{
						{paginatedFields[i]: map[string]interface{}{rangeOp: cursorFieldValues[i]}},
						{"_id": map[string]interface{}{comparisonOps[i]: cursorFieldValues[len(cursorFieldValues)-1]}},
					}},
				}}
			}
			query = map[string]interface{}{"$and": conditions}
		}
	} else {
		query = map[string]interface{}{"_id": map[string]interface{}{comparisonOps[0]: cursorFieldValues[0]}}
	}
	return query, nil
}
