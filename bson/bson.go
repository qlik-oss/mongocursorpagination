package bson

import (
	"errors"
	"reflect"
	"strings"
)

// GenerateCursorQuery generates and returns a cursor range query
func GenerateCursorQuery(shouldSecondarySortOnID bool, paginatedField string, comparisonOp string, cursorFieldValues []interface{}) (map[string]interface{}, error) {
	var query map[string]interface{}
	if (shouldSecondarySortOnID && len(cursorFieldValues) != 2) ||
		(!shouldSecondarySortOnID && len(cursorFieldValues) != 1) {
		return nil, errors.New("wrong number of cursor field values specified")
	}
	if shouldSecondarySortOnID {
		query = map[string]interface{}{"$or": []map[string]interface{}{
			{paginatedField: map[string]interface{}{comparisonOp: cursorFieldValues[0]}},
			{"$and": []map[string]interface{}{
				{paginatedField: map[string]interface{}{"$eq": cursorFieldValues[0]}},
				{"_id": map[string]interface{}{comparisonOp: cursorFieldValues[1]}},
			}},
		}}
	} else {
		query = map[string]interface{}{paginatedField: map[string]interface{}{comparisonOp: cursorFieldValues[0]}}
	}
	return query, nil
}

// FindStructFieldNameByBsonTag returns the struct field name of the provided bson tag or the empty string if it could not be found
func FindStructFieldNameByBsonTag(structType reflect.Type, tag string) string {
	if structType == nil || tag == "" {
		return ""
	}
	for i := 0; i < structType.NumField(); i++ {
		currentField := structType.Field(i)
		// Lookup for a bson key tag value
		if value, ok := currentField.Tag.Lookup("bson"); ok {
			// Check if the value has additional flags
			if idx := strings.IndexByte(value, ','); idx >= 0 {
				// Substring the key only
				value = value[:idx]
			}

			if value == tag {
				return currentField.Name
			}
		}
	}
	return ""
}
