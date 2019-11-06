// Package mongocursorpagination eases the computation of pagination information of a find mongo query
// by augmenting the base query with cursor information and returning a cursor.
package mongocursorpagination

import (
	"context"
	"encoding/base64"
	"fmt"
	"reflect"
	"strings"

	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type (
	// FindParams holds the parameters to be used in a paginated find mongo query that will return a
	// Cursor.
	FindParams struct {
		Collection *mongo.Collection

		// The find query to augment with pagination
		Query primitive.M
		// The number of results to fetch, should be > 0
		Limit int64
		// true, if the results should be sort ascending, false otherwise
		SortAscending bool
		// The name of the mongo collection field being paginated and sorted on. This field must:
		// 1. Be orderable. We must sort by this value. If duplicate values for paginatedField field
		//    exist, the results will be secondarily ordered by the _id
		// 2. Be indexed. For large collections, this should be indexed for query performance
		// 3. Be immutable. If the value changes between paged queries, it could appear twice
		// 4. Match the bson field name the result struct. e.g.:
		//
		//    PaginatedField would be "name" when paginating employees by name
		//
		//    type Employee struct {
		//        ID          bson.ObjectId `bson:"_id"`
		//        Name        string        `bson:"name"`
		//    }
		//
		PaginatedField string
		Collation      *options.Collation
		// The value to start querying the page
		Next string
		// The value to start querying previous page
		Previous string
		// Whether or not to include total count of documents matching filter in the cursor
		// Specifying true makes an additional query
		CountTotal bool
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
)

// Find finds stuff
func Find(p FindParams, results interface{}) (Cursor, error) {
	if results == nil {
		return Cursor{}, errors.New("results can't be nil")
	}
	if p.PaginatedField == "" {
		p.PaginatedField = "_id"
		p.Collation = nil
	}
	shouldSecondarySortOnID := p.PaginatedField != "_id"
	if p.Limit <= 0 {
		return Cursor{}, errors.New("a limit of at least 1 is required")
	}
	nextCursorValues, err := parseCursor(p.Next, shouldSecondarySortOnID)
	if err != nil {
		return Cursor{}, fmt.Errorf("next cursor parse failed: %s", err)
	}
	previousCursorValues, err := parseCursor(p.Previous, shouldSecondarySortOnID)
	if err != nil {
		return Cursor{}, fmt.Errorf("previous cursor parse failed: %s", err)
	}
	// Figure out the sort direction and comparison operator that will be used in the augmented query
	sortAsc := (!p.SortAscending && p.Previous != "") || (p.SortAscending && p.Previous == "")
	comparisonOp := "$gt"
	sortDir := 1
	if !sortAsc {
		comparisonOp = "$lt"
		sortDir = -1
	}
	// Augment the specified find query with cursor data
	queries := []bson.M{p.Query}
	// Setup the pagination query
	if p.Next != "" || p.Previous != "" {
		var cursorValues []interface{}
		if p.Next != "" {
			cursorValues = nextCursorValues
		} else if p.Previous != "" {
			cursorValues = previousCursorValues
		}
		var cursorQuery bson.M
		cursorQuery, err = generateCursorQuery(shouldSecondarySortOnID, p.PaginatedField, comparisonOp, cursorValues)
		if err != nil {
			return Cursor{}, err
		}
		queries = append(queries, cursorQuery)
	}
	// Setup the sort query
	var sort bson.D
	if shouldSecondarySortOnID {
		sort = bson.D{{Key: p.PaginatedField, Value: sortDir}, {Key: "_id", Value: sortDir}}
	} else {
		sort = bson.D{{Key: "_id", Value: sortDir}}
	}
	// Execute the augmented query, get an additional element to see if there's another page
	err = executeCursorQuery(p.Collection, queries, sort, p.Limit, p.Collation, results)
	if err != nil {
		return Cursor{}, err
	}
	var c Cursor
	err = processResults(p, shouldSecondarySortOnID, &c, results)
	return c, err
}

func processResults(p FindParams, shouldSecondarySortOnID bool, cursor *Cursor, results interface{}) error {
	var err error
	// Get the results slice's pointer and value
	resultsPtr := reflect.ValueOf(results)
	resultsVal := resultsPtr.Elem()

	hasMore := resultsVal.Len() > int(p.Limit)

	// Remove the extra element that we added to see if there was another page
	if hasMore {
		resultsVal = resultsVal.Slice(0, resultsVal.Len()-1)
	}

	cursor.HasPrevious = p.Next != "" || (p.Previous != "" && hasMore)
	cursor.HasNext = p.Previous != "" || hasMore

	if resultsVal.Len() > 0 {
		// If we sorted reverse to get the previous page, correct the sort order
		if p.Previous != "" {
			for left, right := 0, resultsVal.Len()-1; left < right; left, right = left+1, right-1 {
				leftValue := resultsVal.Index(left).Interface()
				resultsVal.Index(left).Set(resultsVal.Index(right))
				resultsVal.Index(right).Set(reflect.ValueOf(leftValue))
			}
		}

		// Generate the previous cursor
		if cursor.HasPrevious {
			firstResult := resultsVal.Index(0).Interface()
			cursor.Previous, err = generateCursor(firstResult, p.PaginatedField, shouldSecondarySortOnID)
			if err != nil {
				return fmt.Errorf("could not create a previous cursor: %s", err)
			}
		}

		// Generate the next cursor
		if cursor.HasNext {
			lastResult := resultsVal.Index(resultsVal.Len() - 1).Interface()
			cursor.Next, err = generateCursor(lastResult, p.PaginatedField, shouldSecondarySortOnID)
			if err != nil {
				return fmt.Errorf("could not create a next cursor: %s", err)
			}
		}
	}
	resultsPtr.Elem().Set(resultsVal)
	return nil
}

func generateCursor(result interface{}, paginatedField string, shouldSecondarySortOnID bool) (string, error) {
	if result == nil {
		return "", fmt.Errorf("the specified result must be a non nil value")
	}
	// Find the result struct field name that has a tag matching the paginated filed name
	resultStructFieldName := findStructFieldNameByBsonTag(reflect.TypeOf(result), paginatedField)
	// Check if a tag matching the paginated field name was found
	if resultStructFieldName == "" {
		return "", fmt.Errorf("paginated field %s not found", paginatedField)
	}

	// Get the value of the resultStructFieldName
	paginatedFieldValue := reflect.ValueOf(result).FieldByName(resultStructFieldName).Interface()
	// Set the cursor data
	cursorData := make(bson.D, 0, 2)
	cursorData = append(cursorData, bson.E{Key: paginatedField, Value: paginatedFieldValue})
	if shouldSecondarySortOnID {
		// Find the result struct id field name that has a tag matching the _id field name
		resultStructIDFieldName := findStructFieldNameByBsonTag(reflect.TypeOf(result), "_id")
		// Get the value of the ID field
		id := reflect.ValueOf(result).FieldByName(resultStructIDFieldName).Interface().(primitive.ObjectID)
		cursorData = append(cursorData, bson.E{Key: "_id", Value: id})
	}
	// Encode the cursor data into a url safe string
	cursor, err := encodeCursor(cursorData)
	if err != nil {
		return "", fmt.Errorf("failed to encode cursor using %v: %s", cursorData, err)
	}
	return cursor, nil
}

func parseCursor(cursor string, shouldSecondarySortOnID bool) ([]interface{}, error) {
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

func encodeCursor(cursorData bson.D) (string, error) {
	data, err := bson.Marshal(cursorData)
	return base64.RawURLEncoding.EncodeToString(data), err
}

func decodeCursor(cursor string) (bson.D, error) {
	var cursorData bson.D
	data, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return cursorData, err
	}

	err = bson.Unmarshal(data, &cursorData)
	return cursorData, err
}

func generateCursorQuery(shouldSecondarySortOnID bool, paginatedField string, comparisonOp string, cursorFieldValues []interface{}) (bson.M, error) {
	var query bson.M
	if (shouldSecondarySortOnID && len(cursorFieldValues) != 2) ||
		(!shouldSecondarySortOnID && len(cursorFieldValues) != 1) {
		return nil, errors.New("wrong number of cursor field values specified")
	}
	if shouldSecondarySortOnID {
		query = bson.M{"$or": []bson.M{
			{paginatedField: bson.M{comparisonOp: cursorFieldValues[0]}},
			{"$and": []bson.M{
				{paginatedField: bson.M{"$eq": cursorFieldValues[0]}},
				{"_id": bson.M{comparisonOp: cursorFieldValues[1]}},
			}},
		}}
	} else {
		query = bson.M{paginatedField: bson.M{comparisonOp: cursorFieldValues[0]}}
	}
	return query, nil
}

func executeCursorQuery(c *mongo.Collection, query []bson.M, sort bson.D, limit int64, collation *options.Collation, results interface{}) error {
	options := options.Find()
	options.SetSort(sort)
	options.SetLimit(limit + 1)

	if collation != nil {
		options.SetCollation(collation)
	}
	ctx := context.Background()
	cursor, err := c.Find(ctx, bson.M{"$and": query}, options)
	if err != nil {
		return err
	}
	err = cursor.All(ctx, results)
	if err != nil {
		return err
	}
	return nil
}

func findStructFieldNameByBsonTag(structType reflect.Type, tag string) string {
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
