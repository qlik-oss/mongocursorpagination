package mgo

import (
	"encoding/base64"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/globalsign/mgo"
	"github.com/globalsign/mgo/bson"
	"github.com/stretchr/testify/require"
)

type item struct {
	ID        bson.ObjectId `json:"id" bson:"_id"`
	Name      string        `json:"name" bson:"name"`
	UserID    string        `json:"userId" bson:"userId,omitempty"`
	CreatedAt time.Time     `json:"createdAt" bson:"createdAt"`
}

func TestFind(t *testing.T) {
	var cases = []struct {
		name               string
		findParams         FindParams
		results            interface{}
		executeCountQuery  func(db MgoDb, collectionName string, queries []bson.M) (int, error)
		executeCursorQuery func(db MgoDb, collectionName string, query []bson.M, sort []string, limit int, collation *mgo.Collation, results interface{}) error
		expectedCursor     Cursor
		expectedErr        error
	}{
		{
			name:               "errors when results is nil",
			findParams:         FindParams{},
			results:            nil,
			executeCountQuery:  nil,
			executeCursorQuery: nil,
			expectedCursor:     Cursor{},
			expectedErr:        errors.New("results can't be nil"),
		},
		{
			name:               "errors when DB is nil",
			findParams:         FindParams{},
			results:            &[]item{},
			executeCountQuery:  nil,
			executeCursorQuery: nil,
			expectedCursor:     Cursor{},
			expectedErr:        errors.New("DB can't be nil"),
		},
		{
			name: "errors when limit is less than 1",
			findParams: FindParams{
				DB:             &mgo.Database{},
				CollectionName: "items",
				Limit:          0,
			},
			results:            &[]item{},
			executeCountQuery:  nil,
			executeCursorQuery: nil,
			expectedCursor:     Cursor{},
			expectedErr:        errors.New("a limit of at least 1 is required"),
		},
		{
			name: "errors when next cursor is bad",
			findParams: FindParams{
				DB:             &mgo.Database{},
				CollectionName: "items",
				Limit:          2,
				Next:           "XXXXXaGVsbG8=",
			},
			results:            &[]item{},
			executeCountQuery:  nil,
			executeCursorQuery: nil,
			expectedCursor:     Cursor{},
			expectedErr:        &CursorError{err: errors.New("next cursor parse failed: illegal base64 data at input byte 12")},
		},
		{
			name: "errors when previous cursor is bad",
			findParams: FindParams{
				DB:             &mgo.Database{},
				CollectionName: "items",
				Limit:          2,
				Previous:       "XXXXXaGVsbG8=",
			},
			results:            &[]item{},
			executeCountQuery:  nil,
			executeCursorQuery: nil,
			expectedCursor:     Cursor{},
			expectedErr:        &CursorError{err: errors.New("previous cursor parse failed: illegal base64 data at input byte 12")},
		},
		{
			name: "errors when executeCountQuery errors",
			findParams: FindParams{
				DB:             &mgo.Database{},
				CollectionName: "items",
				Query:          bson.M{"name": bson.RegEx{Pattern: "test item.*", Options: "i"}},
				SortAscending:  false,
				PaginatedField: "name",
				Limit:          2,
				CountTotal:     true,
			},
			results: &[]item{},
			executeCountQuery: func(db MgoDb, collectionName string, queries []bson.M) (int, error) {
				return 0, errors.New("error")
			},
			executeCursorQuery: nil,
			expectedCursor:     Cursor{},
			expectedErr:        errors.New("error"),
		},
		{
			name: "errors when executeCursorQuery errors",
			findParams: FindParams{
				DB:             &mgo.Database{},
				CollectionName: "items",
				Query:          bson.M{"name": bson.RegEx{Pattern: "test item.*", Options: "i"}},
				SortAscending:  false,
				PaginatedField: "name",
				Limit:          2,
				CountTotal:     true,
			},
			results: &[]item{},
			executeCountQuery: func(db MgoDb, collectionName string, queries []bson.M) (int, error) {
				return 2, nil
			},
			executeCursorQuery: func(db MgoDb, collectionName string, query []bson.M, sort []string, limit int, collation *mgo.Collation, results interface{}) error {
				return errors.New("error")
			},
			expectedCursor: Cursor{},
			expectedErr:    errors.New("error"),
		},
		{
			name: "return cursor with next and count also populates results when next and prev not specified (using item pointer)",
			findParams: FindParams{
				DB:             &mgo.Database{},
				CollectionName: "items",
				Query:          bson.M{"name": bson.RegEx{Pattern: "test item.*", Options: "i"}},
				SortAscending:  false,
				PaginatedField: "name",
				Limit:          2,
				CountTotal:     true,
			},
			results: &[]*item{},
			executeCountQuery: func(db MgoDb, collectionName string, queries []bson.M) (int, error) {
				return 3, nil
			},
			executeCursorQuery: func(db MgoDb, collectionName string, query []bson.M, sort []string, limit int, collation *mgo.Collation, results interface{}) error {
				resultv := reflect.ValueOf(results)
				resultv.Elem().Set(reflect.ValueOf([]*item{
					&item{ID: bson.ObjectIdHex("1addf533e81549de7696cb04"), Name: "test item 1", CreatedAt: time.Now()},
					&item{ID: bson.ObjectIdHex("2addf533e81549de7696cb04"), Name: "test item 2", CreatedAt: time.Now()},
					&item{ID: bson.ObjectIdHex("3addf533e81549de7696cb04"), Name: "test item 3", CreatedAt: time.Now()},
				}))
				return nil
			},
			expectedCursor: Cursor{
				Previous:    "",
				Next:        "LAAAAAJuYW1lAAwAAAB0ZXN0IGl0ZW0gMgAHX2lkACrd9TPoFUnedpbLBAA",
				HasPrevious: false,
				HasNext:     true,
				Count:       3,
			},
			expectedErr: nil,
		},
		{
			name: "return cursor with previous and count also populates results when next is specified",
			findParams: FindParams{
				DB:             &mgo.Database{},
				CollectionName: "items",
				Query:          bson.M{"name": bson.RegEx{Pattern: "test item.*", Options: "i"}},
				SortAscending:  true,
				PaginatedField: "name",
				Limit:          2,
				Next:           "LAAAAAJuYW1lAAwAAAB0ZXN0IGl0ZW0gMgAHX2lkACrd9TPoFUnedpbLBAA",
				CountTotal:     true,
			},
			results: &[]item{},
			executeCountQuery: func(db MgoDb, collectionName string, queries []bson.M) (int, error) {
				return 2, nil
			},
			executeCursorQuery: func(db MgoDb, collectionName string, query []bson.M, sort []string, limit int, collation *mgo.Collation, results interface{}) error {
				resultv := reflect.ValueOf(results)
				resultv.Elem().Set(reflect.ValueOf([]item{
					{ID: bson.ObjectIdHex("1addf533e81549de7696cb04"), Name: "test item 1", CreatedAt: time.Now()},
					{ID: bson.ObjectIdHex("2addf533e81549de7696cb04"), Name: "test item 2", CreatedAt: time.Now()},
				}))
				return nil
			},
			expectedCursor: Cursor{
				Previous:    "LAAAAAJuYW1lAAwAAAB0ZXN0IGl0ZW0gMQAHX2lkABrd9TPoFUnedpbLBAA",
				Next:        "",
				HasPrevious: true,
				HasNext:     false,
				Count:       2,
			},
			expectedErr: nil,
		},
		{
			name: "return cursor with next and count also populates results when previous is specified",
			findParams: FindParams{
				DB:             &mgo.Database{},
				CollectionName: "items",
				Query:          bson.M{"name": bson.RegEx{Pattern: "test item.*", Options: "i"}},
				SortAscending:  true,
				PaginatedField: "name",
				Limit:          2,
				Previous:       "LAAAAAJuYW1lAAwAAAB0ZXN0IGl0ZW0gMQAHX2lkABrd9TPoFUnedpbLBAA",
				CountTotal:     true,
			},
			results: &[]item{},
			executeCountQuery: func(db MgoDb, collectionName string, queries []bson.M) (int, error) {
				return 2, nil
			},
			executeCursorQuery: func(db MgoDb, collectionName string, query []bson.M, sort []string, limit int, collation *mgo.Collation, results interface{}) error {
				resultv := reflect.ValueOf(results)
				resultv.Elem().Set(reflect.ValueOf([]item{
					{ID: bson.ObjectIdHex("1addf533e81549de7696cb04"), Name: "test item 1", CreatedAt: time.Now()},
					{ID: bson.ObjectIdHex("2addf533e81549de7696cb04"), Name: "test item 2", CreatedAt: time.Now()},
				}))
				return nil
			},
			expectedCursor: Cursor{
				Previous:    "",
				Next:        "LAAAAAJuYW1lAAwAAAB0ZXN0IGl0ZW0gMQAHX2lkABrd9TPoFUnedpbLBAA",
				HasPrevious: false,
				HasNext:     true,
				Count:       2,
			},
			expectedErr: nil,
		},
		{
			name: "return cursor with next also populates results when no pagination field specified",
			findParams: FindParams{
				DB:             &mgo.Database{},
				CollectionName: "items",
				Query:          bson.M{},
				SortAscending:  true,
				Limit:          2,
				CountTotal:     false,
			},
			results:           &[]item{},
			executeCountQuery: nil,
			executeCursorQuery: func(db MgoDb, collectionName string, query []bson.M, sort []string, limit int, collation *mgo.Collation, results interface{}) error {
				resultv := reflect.ValueOf(results)
				resultv.Elem().Set(reflect.ValueOf([]item{
					{ID: bson.ObjectIdHex("1addf533e81549de7696cb04"), Name: "test item 1", CreatedAt: time.Now()},
					{ID: bson.ObjectIdHex("2addf533e81549de7696cb04"), Name: "test item 2", CreatedAt: time.Now()},
					{ID: bson.ObjectIdHex("3addf533e81549de7696cb04"), Name: "test item 3", CreatedAt: time.Now()},
				}))
				return nil
			},
			expectedCursor: Cursor{
				Previous: "",
				// Next:        "FgAAAAdfaWQAKt31M-gVSd52lssEAA",
				Next:        "JwAAAAdfaWQAKt31M-gVSd52lssEB19pZAAq3fUz6BVJ3naWywQA",
				HasPrevious: false,
				HasNext:     true,
				Count:       0,
			},
			expectedErr: nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			executeCountQueryOri := executeCountQuery
			executeCountQuery = tc.executeCountQuery
			defer func() {
				executeCountQuery = executeCountQueryOri
			}()

			executeCursorQueryOri := executeCursorQuery
			executeCursorQuery = tc.executeCursorQuery
			defer func() {
				executeCursorQuery = executeCursorQueryOri
			}()

			cursor, err := Find(tc.findParams, tc.results)

			require.Equal(t, tc.expectedErr, err)
			if tc.expectedErr == nil {
				require.NotNil(t, tc.results)
				// Handle different slice types ([]item and []*item)
				v := reflect.ValueOf(tc.results)
				if v.Kind() == reflect.Ptr {
					v = reflect.Indirect(v)
				}
				length := v.Len()
				require.Equal(t, tc.findParams.Limit, length)
				require.Equal(t, tc.expectedCursor.Count, cursor.Count)
			}
			require.Equal(t, tc.expectedCursor, cursor)
		})
	}
}

func TestParseCursor(t *testing.T) {
	var cases = []struct {
		name                      string
		cursor                    string
		numPaginatedFields        int
		expectedCursorFieldValues []interface{}
		expectedErr               error
	}{
		{
			"return appropriate cursor field values when shouldSecondarySortOnID is true",
			"LAAAAAJuYW1lAAwAAAB0ZXN0IGl0ZW0gMQAHX2lkABrd9TPoFUnedpbLBAA",
			2,
			[]interface{}{"test item 1", bson.ObjectIdHex("1addf533e81549de7696cb04")},
			nil,
		},
		{
			"return appropriate cursor field values when shouldSecondarySortOnID is false",
			"FgAAAAdfaWQAWt31M-gVSd52lssEAA",
			1,
			[]interface{}{bson.ObjectIdHex("5addf533e81549de7696cb04")},
			nil,
		},
		{
			"errors when decode fails",
			"XXXXXaGVsbG8=",
			2,
			nil,
			base64.CorruptInputError(12),
		},
		{
			"errors when expecting cursor with 2 elements and only 1 present",
			"FgAAAAdfaWQAWt31M-gVSd52lssEAA",
			2,
			nil,
			errors.New("expecting a cursor with 2 elements"),
		},
		{
			"errors when expecting cursor with 1 elements and only 2 present",
			"LwAAAAJuYW1lAAoAAAB0ZXN0IGl0ZW0AAl9pZAANAAAAWt31M-gVSd52lssEAAA",
			1,
			nil,
			errors.New("expecting a cursor with a single element"),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cursorFieldValues, err := parseCursor(tc.cursor, tc.numPaginatedFields)
			require.Equal(t, tc.expectedCursorFieldValues, cursorFieldValues)
			require.Equal(t, tc.expectedErr, err)
		})
	}
}

func TestDecodeCursor(t *testing.T) {
	var cases = []struct {
		name               string
		cursor             string
		expectedCursorData bson.D
		expectedErr        error
	}{
		{
			"errors when base 64 decode fails",
			"XXXXXaGVsbG8=",
			nil,
			base64.CorruptInputError(12),
		},
		{
			"decodes cursor data when valid input cursor provided",
			"FgAAAAdfaWQAWt31M-gVSd52lssEAA",
			bson.D{bson.DocElem{Name: "_id", Value: bson.ObjectIdHex("5addf533e81549de7696cb04")}},
			nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cursorData, err := decodeCursor(tc.cursor)
			require.Equal(t, tc.expectedCursorData, cursorData)
			require.Equal(t, tc.expectedErr, err)
		})
	}
}

func TestGenerateCursor(t *testing.T) {
	var cases = []struct {
		name                    string
		result                  interface{}
		paginatedField          string
		paginatedFields         []string
		shouldSecondarySortOnID bool
		encodeCursor            func(cursorData bson.D) (string, error)
		expectedCursor          string
		expectedErr             error
	}{
		{
			"return the generated cursor for an item search paginated by _id",
			item{ID: bson.ObjectIdHex("5addf533e81549de7696cb04"), Name: "test item", CreatedAt: time.Now()},
			"_id",
			[]string{"_id"},
			false,
			nil,
			"FgAAAAdfaWQAWt31M-gVSd52lssEAA",
			nil,
		},
		{
			"return the generated cursor for an item search paginated by name",
			item{ID: bson.ObjectIdHex("1addf533e81549de7696cb04"), Name: "test item 1", CreatedAt: time.Now()},
			"name",
			[]string{"name", "_id"},
			true,
			nil,
			"LAAAAAJuYW1lAAwAAAB0ZXN0IGl0ZW0gMQAHX2lkABrd9TPoFUnedpbLBAA",
			nil,
		},
		{
			"errors when invalid result _id is set",
			item{ID: "123", Name: "test item", CreatedAt: time.Now()},
			"_id",
			[]string{"_id"},
			false,
			nil,
			"",
			errors.New("ObjectIDs must be exactly 12 bytes long (got 3)"),
		},
		{
			"errors when result is nil",
			nil,
			"_id",
			[]string{"_id"},
			false,
			nil,
			"",
			errors.New("the specified result must be a non nil value"),
		},
		{
			"errors when paginated field not found and result is bson.Raw",
			&[]bson.Raw{},
			"creatorId",
			[]string{"creatorId", "_id"},
			false,
			nil,
			"",
			errors.New("paginated field creatorId not found"),
		},
		{
			"errors when encoding fails",
			item{ID: bson.ObjectIdHex("1addf533e81549de7696cb04"), Name: "test item", CreatedAt: time.Now()},
			"name",
			[]string{"name"},
			false,
			func(cursorData bson.D) (string, error) {
				return "", errors.New("error")
			},
			"",
			errors.New("failed to encode cursor using [{name test item}]: error"),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.encodeCursor != nil {
				encodeCursorOri := encodeCursor
				encodeCursor = tc.encodeCursor
				defer func() {
					encodeCursor = encodeCursorOri
				}()
			}

			cursor, err := generateCursor(tc.result, tc.paginatedFields)
			require.Equal(t, tc.expectedCursor, cursor)
			require.Equal(t, tc.expectedErr, err)
		})
	}
}

func TestEncodeCursorCursor(t *testing.T) {
	var cases = []struct {
		name           string
		cursorData     bson.D
		expectedCursor string
		expectedErr    error
	}{
		{
			"encode cursor when valid input cursor data provided",
			bson.D{bson.DocElem{Name: "_id", Value: bson.ObjectIdHex("5addf533e81549de7696cb04")}},
			"FgAAAAdfaWQAWt31M-gVSd52lssEAA",
			nil,
		},
		// 	Can't find an easy way to make bson marshaling error so skipping that test case
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cursor, err := encodeCursor(tc.cursorData)
			require.Equal(t, tc.expectedCursor, cursor)
			require.Equal(t, tc.expectedErr, err)
		})
	}
}
