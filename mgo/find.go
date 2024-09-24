// Package mongocursorpagination eases the computation of pagination information of a find mongo query
// by augmenting the base query with cursor information and returning a cursor.
package mgo

import (
	"encoding/base64"
	"errors"
	"fmt"
	"reflect"

	"github.com/globalsign/mgo"
	"github.com/globalsign/mgo/bson"
	mcpbson "github.com/qlik-oss/mongocursorpagination/bson"
)

type (
	MgoDb interface {
		C(string) *mgo.Collection
	}
	MgoQuery interface {
		All(result interface{}) error
		Sort(fields ...string) MgoQuery
		Limit(n int) MgoQuery
		Count() (n int, err error)
		Collation(*mgo.Collation) MgoQuery
	}
	// FindParams holds the parameters to be used in a paginated find mongo query that will return a
	// Cursor.
	FindParams struct {
		// The mongo database to use
		DB MgoDb
		// The name of the mongo collection to query
		CollectionName string
		// The find query to augment with pagination
		Query bson.M
		// The number of results to fetch, should be > 0
		Limit int
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
		// The collation to use for the sort ordering.
		// See https://docs.mongodb.com/manual/reference/collation-locales-defaults/#supported-languages-and-locales
		// This is ignored if PaginatedField is empty
		Collation *mgo.Collation
		// The value to start querying the page
		Next string
		// The value to start querying previous page
		Previous string
		// Whether or not to include total count of documents matching filter in the cursor
		// Specifying true makes an additionnal query
		CountTotal bool
		// The names of multiple fields being paginated and sorted on
		PaginatedFields []string
		SortOrders      []int
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

// Find executes a find mongo query by using the provided FindParams, fills the passed in result
// slice pointer and returns a Cursor.
func Find(p FindParams, results interface{}) (Cursor, error) {
	var err error
	if results == nil {
		return Cursor{}, errors.New("results can't be nil")
	}
	p = ensureMandatoryParams(p)

	if p.PaginatedField == "" {
		p.PaginatedField = "_id"
		p.Collation = nil
	}
	var numPaginatedFields int
	if p.PaginatedFields != nil && len(p.PaginatedFields) > 0 {
		numPaginatedFields = len(p.PaginatedFields)
	} else {
		numPaginatedFields = 1
	}

	if p.DB == nil {
		return Cursor{}, errors.New("DB can't be nil")
	}

	if p.Limit <= 0 {
		return Cursor{}, errors.New("a limit of at least 1 is required")
	}

	nextCursorValues, err := parseCursor(p.Next, numPaginatedFields)
	if err != nil {
		return Cursor{}, &CursorError{fmt.Errorf("next cursor parse failed: %s", err)}
	}

	previousCursorValues, err := parseCursor(p.Previous, numPaginatedFields)
	if err != nil {
		return Cursor{}, &CursorError{fmt.Errorf("previous cursor parse failed: %s", err)}
	}

	comparisonOps := generateComparisonOps(p)

	// Augment the specified find query with cursor data
	queries := []bson.M{p.Query}

	// Compute total count of documents matching filter - only computed if CountTotal is True
	var count int
	if p.CountTotal {
		count, err = executeCountQuery(p.DB, p.CollectionName, queries)
		if err != nil {
			return Cursor{}, err
		}
	}

	// Setup the pagination query
	if p.Next != "" || p.Previous != "" {
		var cursorValues []interface{}
		if p.Next != "" {
			cursorValues = nextCursorValues
		} else if p.Previous != "" {
			cursorValues = previousCursorValues
		}
		var cursorQuery bson.M
		cursorQuery, err = mcpbson.GenerateCursorQuery(p.PaginatedFields, comparisonOps, cursorValues)
		if err != nil {
			return Cursor{}, err
		}
		queries = append(queries, cursorQuery)
	}

	// Setup the sort query
	var sort []string
	for i := range p.PaginatedFields {
		sortDir := ""
		if p.SortOrders[i] == -1 {
			sortDir = "-"
		}
		sort = append(sort, fmt.Sprintf("%s%s", sortDir, p.PaginatedFields[i]))
	}

	// Execute the augmented query, get an additional element to see if there's another page
	err = executeCursorQuery(p.DB, p.CollectionName, queries, sort, p.Limit, p.Collation, results)
	if err != nil {
		return Cursor{}, err
	}

	// Get the results slice's pointer and value
	resultsPtr := reflect.ValueOf(results)
	resultsVal := resultsPtr.Elem()

	hasMore := resultsVal.Len() > p.Limit

	// Remove the extra element that we added to see if there was another page
	if hasMore {
		resultsVal = resultsVal.Slice(0, resultsVal.Len()-1)
	}

	hasPrevious := p.Next != "" || (p.Previous != "" && hasMore)
	hasNext := p.Previous != "" || hasMore

	var previousCursor string
	var nextCursor string

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
		if hasPrevious {
			firstResult := resultsVal.Index(0).Interface()
			previousCursor, err = generateCursor(firstResult, p.PaginatedFields)
			if err != nil {
				return Cursor{}, fmt.Errorf("could not create a previous cursor: %s", err)
			}
		}

		// Generate the next cursor
		if hasNext {
			lastResult := resultsVal.Index(resultsVal.Len() - 1).Interface()
			nextCursor, err = generateCursor(lastResult, p.PaginatedFields)
			if err != nil {
				return Cursor{}, fmt.Errorf("could not create a next cursor: %s", err)
			}
		}
	}

	// Create the response cursor
	cursor := Cursor{
		Previous:    previousCursor,
		HasPrevious: hasPrevious,
		Next:        nextCursor,
		HasNext:     hasNext,
		Count:       count,
	}

	// Save the modified result slice in the result pointer
	resultsPtr.Elem().Set(resultsVal)

	return cursor, nil
}

func generateComparisonOps(p FindParams) []string {
	comparisonOps := make([]string, 0, len(p.SortOrders))
	for i := range p.SortOrders {
		// Figure out the sort direction and comparison operator that will be used in the augmented query
		sortAsc := (p.SortOrders[i] == -1 && p.Previous != "") || (p.SortOrders[i] == 1 && p.Previous == "")
		if sortAsc {
			comparisonOps = append(comparisonOps, "$gt")
			p.SortOrders[i] = 1
		} else {
			comparisonOps = append(comparisonOps, "$lt")
			p.SortOrders[i] = -1
		}
	}
	return comparisonOps
}
func ensureMandatoryParams(p FindParams) FindParams {
	if p.PaginatedField == "" {
		p.PaginatedField = "_id"
		p.Collation = nil
	}
	if p.PaginatedFields == nil || len(p.PaginatedFields) == 0 {
		if p.PaginatedField == "" {
			p.PaginatedFields = []string{"_id"}
			p.Collation = nil
		} else {
			p.PaginatedFields = []string{p.PaginatedField, "_id"}
		}
	}
	if p.SortOrders == nil || len(p.SortOrders) == 0 {
		p.SortOrders = []int{}
		if p.SortAscending {
			for i := 0; i < len(p.PaginatedFields); i++ {
				p.SortOrders = append(p.SortOrders, 1)
			}
		} else {
			for i := 0; i < len(p.PaginatedFields); i++ {
				p.SortOrders = append(p.SortOrders, -1)
			}
		}
	}
	return p
}

var parseCursor = func(cursor string, numPaginatedFields int) ([]interface{}, error) {
	cursorValues := make([]interface{}, 0, numPaginatedFields)
	if cursor != "" {
		parsedCursor, err := decodeCursor(cursor)
		if err != nil {
			return nil, err
		}
		if len(parsedCursor) != numPaginatedFields {
			if numPaginatedFields == 1 {
				return nil, errors.New("expecting a cursor with a single element")
			}
			return nil, fmt.Errorf("expecting a cursor with %d elements", numPaginatedFields)
		}
		for _, obj := range parsedCursor {
			cursorValues = append(cursorValues, obj.Value)
		}
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

var executeCountQuery = func(db MgoDb, collectionName string, queries []bson.M) (int, error) {
	return db.C(collectionName).Find(bson.M{"$and": queries}).Count()
}

var executeCursorQuery = func(db MgoDb, collectionName string, query []bson.M, sort []string, limit int, collation *mgo.Collation, results interface{}) error {
	if collation == nil {
		return db.C(collectionName).Find(bson.M{"$and": query}).Sort(sort...).Limit(limit + 1).All(results)
	}
	return db.C(collectionName).Find(bson.M{"$and": query}).Sort(sort...).Collation(collation).Limit(limit + 1).All(results)
}

func generateCursor(result interface{}, paginatedFields []string) (string, error) {
	if result == nil {
		return "", fmt.Errorf("the specified result must be a non nil value")
	}
	// Handle pointer values and reduce number of times reflection is done on the same type.
	val := reflect.ValueOf(result)
	if val.Kind() == reflect.Ptr {
		_ = reflect.Indirect(val)
	}

	var recordAsBytes []byte
	var err error

	switch v := result.(type) {
	case bson.Raw:
		recordAsBytes = v.Data
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

	// Set the cursor data
	cursorData := make(bson.D, 0, len(paginatedFields))
	for i := range paginatedFields {
		paginatedFieldValue := recordAsMap[paginatedFields[i]]
		if paginatedFieldValue == nil {
			return "", fmt.Errorf("paginated field %s not found", paginatedFields[i])
		}
		cursorData = append(cursorData, bson.DocElem{Name: paginatedFields[i], Value: paginatedFieldValue})
	}
	// Encode the cursor data into a url safe string
	cursor, err := encodeCursor(cursorData)
	if err != nil {
		return "", fmt.Errorf("failed to encode cursor using %v: %s", cursorData, err)
	}
	paginatedFieldValue := recordAsMap[paginatedFields[0]]
	if paginatedFieldValue == nil {
		return "", fmt.Errorf("paginated field %s not found", paginatedFields[0])
	}

	return cursor, nil
}

// encodeCursor encodes and returns cursor data that is url safe
var encodeCursor = func(cursorData bson.D) (string, error) {
	data, err := bson.Marshal(cursorData)
	return base64.RawURLEncoding.EncodeToString(data), err
}
