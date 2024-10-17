package mongo

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type (
	Item struct {
		ID        primitive.ObjectID `bson:"_id"`
		Name      string             `bson:"name"`
		Data      string             `bson:"data"`
		CreatedAt time.Time          `bson:"createdAt"`
	}
)

func TestValidate(t *testing.T) {
	var cases = []struct {
		name            string
		results         interface{}
		paginatedFields []string
		expectedErr     error
	}{
		{
			name:            "errors when results is nil",
			results:         nil,
			paginatedFields: nil,
			expectedErr:     NewErrInvalidResults("expected results to be non nil"),
		},
		{
			name:            "errors when results is not a pointer",
			results:         struct{}{},
			paginatedFields: nil,
			expectedErr:     NewErrInvalidResults("expected results to be a slice pointer"),
		},
		{
			name:            "errors when results is not a slice pointer",
			results:         &struct{}{},
			paginatedFields: nil,
			expectedErr:     NewErrInvalidResults("expected results to be a slice pointer"),
		},
		{
			name:            "passes validation when results' element type is a bson.Raw",
			results:         &[]bson.Raw{},
			paginatedFields: nil,
			expectedErr:     nil,
		},
		{
			name:            "passes validation when results' element type is a bson.Raw pointer",
			results:         &[]*bson.Raw{},
			paginatedFields: nil,
			expectedErr:     nil,
		},
		{
			name:            "errors when results' element type is not a struct",
			results:         &[]*bool{},
			paginatedFields: nil,
			expectedErr:     NewErrInvalidResults("expected results' element to be a struct or struct pointer"),
		},
		{
			name:            "passes validation when results is of a supported type and all paginatedFields are found",
			results:         &[]Item{},
			paginatedFields: []string{"_id", "data"},
			expectedErr:     nil,
		},
		{
			name:            "passes validation when results is of a supported type and paginatedFields is nil",
			results:         &[]*Item{},
			paginatedFields: nil,
			expectedErr:     nil,
		},
		{
			name:            "errors when results is of a supported type but a paginatedFields is not found",
			results:         &[]Item{},
			paginatedFields: []string{"_id", "data", "invalid"},
			expectedErr:     NewErrPaginatedFieldNotFound("invalid"),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validate(tc.results, tc.paginatedFields)
			require.Equal(t, tc.expectedErr, err)
		})
	}
}
