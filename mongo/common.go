package mongo

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"reflect"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type (
	MongoCursor interface {
		Close(context.Context) error
		Decode(interface{}) error
		ID() int64
		Next(context.Context) bool
		TryNext(context.Context) bool
		Err() error
		All(context.Context, interface{}) error
		RemainingBatchLength() int
	}

	Collection interface {
		CountDocuments(context.Context, interface{}, ...*options.CountOptions) (int64, error)
		Find(context.Context, interface{}, ...*options.FindOptions) (MongoCursor, error)
		Aggregate(context.Context, interface{}, ...*options.AggregateOptions) (MongoCursor, error)
	}

	// Cursor holds the pagination data about the find mongo query that was performed.
	Cursor struct {
		// The URL safe previous page cursor to pass in a Find call to get the previous page.
		// This is set to the empty string if there is no previous page.
		Previous string
		// The URL safe next page cursor to pass in a Find call to get the next page.
		// This is set to the empty string if there is no next page.
		Next string
		// true if there is a previous page, false otherwise
		HasPrevious bool
		// true if there is a next page, false otherwise
		HasNext bool
		// Total count of documents matching filter - only computed if CountTotal is True
		Count int
	}

	CursorError struct {
		err error
	}
)

func (e *CursorError) Error() string {
	return e.err.Error()
}

var parseCursor = func(cursor string, shouldSecondarySortOnID bool) ([]interface{}, error) {
	cursorValues := make([]interface{}, 0, 2)
	if cursor != "" {
		parsedCursor, err := decodeCursor(cursor)
		if err != nil {
			return nil, err
		}
		var id interface{}
		if shouldSecondarySortOnID {
			if len(parsedCursor) != 2 {
				return nil, errors.New("expecting a cursor with two elements")
			}
			paginatedFieldValue := parsedCursor[0].Value
			id = parsedCursor[1].Value
			cursorValues = append(cursorValues, paginatedFieldValue)
		} else {
			if len(parsedCursor) != 1 {
				return nil, errors.New("expecting a cursor with a single element")
			}
			id = parsedCursor[0].Value
		}
		cursorValues = append(cursorValues, id)
	}
	return cursorValues, nil
}

// decodeCursor decodes cursor data that was previously encoded with createCursor
func decodeCursor(cursor string) (bson.D, error) {
	var cursorData bson.D
	data, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return cursorData, err
	}

	err = bson.Unmarshal(data, &cursorData)
	return cursorData, err
}

func generateCursor(result interface{}, paginatedField string, shouldSecondarySortOnID bool) (string, error) {
	if result == nil {
		return "", fmt.Errorf("the specified result must be a non nil value")
	}
	// Handle pointer values and reduce number of times reflection is done on the same type.
	val := reflect.ValueOf(result)
	if val.Kind() == reflect.Ptr {
		val = reflect.Indirect(val)
	}

	var recordAsBytes []byte
	var err error

	switch v := result.(type) {
	case []byte:
		recordAsBytes = v
	default:
		recordAsBytes, err = bson.Marshal(result)
		if err != nil {
			return "", err
		}
	}

	var recordAsMap map[string]interface{}
	err = bson.Unmarshal(recordAsBytes, &recordAsMap)
	if err != nil {
		return "", err
	}
	paginatedFieldValue := recordAsMap[paginatedField]
	// Set the cursor data
	cursorData := make(bson.D, 0, 2)
	cursorData = append(cursorData, bson.E{Key: paginatedField, Value: paginatedFieldValue})
	if shouldSecondarySortOnID {
		// Get the value of the ID field
		id := recordAsMap["_id"]
		cursorData = append(cursorData, bson.E{Key: "_id", Value: id})
	}
	// Encode the cursor data into a url safe string
	cursor, err := encodeCursor(cursorData)
	if err != nil {
		return "", fmt.Errorf("failed to encode cursor using %v: %s", cursorData, err)
	}
	return cursor, nil
}

// encodeCursor encodes and returns cursor data that is url safe
func encodeCursor(cursorData bson.D) (string, error) {
	data, err := bson.Marshal(cursorData)
	return base64.RawURLEncoding.EncodeToString(data), err
}
