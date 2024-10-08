package bson

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenerateCursorQuery(t *testing.T) {
	var cases = []struct {
		name              string
		paginatedFields   []string
		comparisonOps     []string
		cursorFieldValues []interface{}
		expectedQuery     map[string]interface{}
		expectedErr       error
	}{
		{
			"error when wrong number of cursor field values specified",
			[]string{"name", "_id"},
			[]string{"$gt", "$gt"},
			[]interface{}{"abc"},
			nil,
			errors.New("wrong number of cursor field values specified"),
		},
		{
			"error when wrong number of comparison operators specified",
			[]string{"name", "_id"},
			[]string{"$gt"},
			[]interface{}{"abc", "abc"},
			nil,
			errors.New("wrong number of comparison operators specified"),
		},
		{
			"error when an invalid comparison operator is specified",
			[]string{"_id"},
			[]string{"$blabla"},
			[]interface{}{"abc"},
			nil,
			errors.New("invalid comparison operator specified: only $lt and $gt are allowed"),
		},
		{
			"return appropriate cursor query when sorting on single field",
			[]string{"name", "_id"},
			[]string{"$gt", "$gt"},
			[]interface{}{"test item", "123"},
			map[string]interface{}{"$or": []map[string]interface{}{
				{"name": map[string]interface{}{"$gt": "test item"}},
				{"$and": []map[string]interface{}{
					{"name": map[string]interface{}{"$gte": "test item"}},
					{"_id": map[string]interface{}{"$gt": "123"}}},
				},
			}},
			nil,
		},
		{
			"return appropriate cursor query when there is no paginated field",
			[]string{"_id"},
			[]string{"$lt"},
			[]interface{}{"123"},
			map[string]interface{}{"_id": map[string]interface{}{"$lt": "123"}},
			nil,
		},
		{
			"return appropriate cursor when sorting on multiple fields",
			[]string{"name", "createdAt", "_id"},
			[]string{"$lt", "$gt", "$lt"},
			[]interface{}{"test item", "2024", "123"},
			map[string]interface{}{"$and": []map[string]interface{}{
				{"$or": []map[string]interface{}{
					{"name": map[string]interface{}{"$lt": "test item"}},
					{"$and": []map[string]interface{}{
						{"name": map[string]interface{}{"$lte": "test item"}},
						{"_id": map[string]interface{}{"$lt": "123"}}}}}},
				{"$or": []map[string]interface{}{
					{"createdAt": map[string]interface{}{"$gt": "2024"}},
					{"$and": []map[string]interface{}{
						{"createdAt": map[string]interface{}{"$gte": "2024"}},
						{"_id": map[string]interface{}{"$gt": "123"}}}}}}}},
			nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			query, err := GenerateCursorQuery(tc.paginatedFields, tc.comparisonOps, tc.cursorFieldValues)
			require.Equal(t, tc.expectedQuery, query)
			require.Equal(t, tc.expectedErr, err)
		})
	}
}
