package bson

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenerateCursorQuery(t *testing.T) {
	var cases = []struct {
		name                    string
		shouldSecondarySortOnID bool
		paginatedField          string
		comparisonOp            string
		cursorFieldValues       []interface{}
		expectedQuery           map[string]interface{}
		expectedErr             error
	}{
		{
			"error when wrong number of cursor field values specified and shouldSecondarySortOnID is true",
			true,
			"name",
			"$gt",
			[]interface{}{"abc"},
			nil,
			errors.New("wrong number of cursor field values specified"),
		},
		{
			"error when wrong number of cursor field values specified and shouldSecondarySortOnID is false",
			false,
			"_id",
			"$lt",
			[]interface{}{},
			nil,
			errors.New("wrong number of cursor field values specified"),
		},
		{
			"error when an invalid comparison operator is specified",
			false,
			"name",
			"$blabla",
			[]interface{}{"abc"},
			nil,
			errors.New("invalid comparison operator specified: only $lt and $gt are allowed"),
		},
		{
			"return appropriate cursor query when shouldSecondarySortOnID is true",
			true,
			"name",
			"$gt",
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
			"return appropriate cursor query when shouldSecondarySortOnID is false",
			false,
			"_id",
			"$lt",
			[]interface{}{"123"},
			map[string]interface{}{"_id": map[string]interface{}{"$lt": "123"}},
			nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			query, err := GenerateCursorQuery(tc.shouldSecondarySortOnID, tc.paginatedField, tc.comparisonOp, tc.cursorFieldValues)
			require.Equal(t, tc.expectedQuery, query)
			require.Equal(t, tc.expectedErr, err)
		})
	}
}
