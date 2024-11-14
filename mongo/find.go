// Package mongocursorpagination eases the computation of pagination information of a find mongo query
// by augmenting the base query with cursor information and returning a cursor.
package mongo

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	mcpbson "github.com/qlik-oss/mongocursorpagination/bson"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	defaultCursorTimeout = 45 * time.Second
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
	}
	// FindParams holds the parameters to be used in a paginated find mongo query that will return a
	// Cursor.
	FindParams struct {
		Collection Collection

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
		// This parameter will also apply timeout of counting total results
		Collation *options.Collation
		// The value to start querying the page
		Next string
		// The value to start querying previous page
		Previous string
		// Whether to include total count of documents matching filter in the cursor
		// Specifying true makes an additional query
		CountTotal bool
		// The index to use for the operation. This should either be the index name as a string or the index specification
		// as a document. The default value is nil, which means that no hint will be sent.
		Hint interface{}
		// A document describing which fields will be included in the documents returned by the operation. The default value
		// is nil, which means all fields will be included.
		// Example: bson.D{"_id":0, "name": 1}
		Projection interface{}
		// This parameter will set the maxTimeMS option on the mongo find cursor, making sure we add a limit to the amount of time
		// mongo can process this on the backend. Will default to 45 seconds, but should be set to an appropriate duration
		// This parameter will also apply timeout of counting total results
		Timeout time.Duration
		// The names of multiple fields being paginated and sorted on. Takes precedence over PaginatedField
		PaginatedFields []string
		// The sort orders corresponding to PaginatedFields. Each value must be either 1 or -1
		SortOrders []int
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

// BuildQueries builds the queries without executing them
func BuildQueries(ctx context.Context, p FindParams) (queries []bson.M, sort bson.D, err error) {
	p = ensureMandatoryParams(p)
	var numPaginatedFields int
	if len(p.PaginatedFields) > 0 {
		numPaginatedFields = len(p.PaginatedFields)
	} else {
		numPaginatedFields = 1
	}

	if p.Collection == nil {
		return []bson.M{}, nil, errors.New("Collection can't be nil")
	}

	if p.Limit <= 0 {
		return []bson.M{}, nil, errors.New("a limit of at least 1 is required")
	}

	nextCursorValues, err := parseCursor(p.Next, numPaginatedFields)
	if err != nil {
		return []bson.M{}, nil, &CursorError{fmt.Errorf("next cursor parse failed: %s", err)}
	}

	previousCursorValues, err := parseCursor(p.Previous, numPaginatedFields)
	if err != nil {
		return []bson.M{}, nil, &CursorError{fmt.Errorf("previous cursor parse failed: %s", err)}
	}

	comparisonOps := generateComparisonOps(p)

	// Augment the specified find query with cursor data
	queries = []bson.M{p.Query}

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
			return []bson.M{}, nil, err
		}
		queries = append(queries, cursorQuery)
	}

	// Setup the sort query
	for i := range p.PaginatedFields {
		sort = append(sort, bson.E{Key: p.PaginatedFields[i], Value: p.SortOrders[i]})
	}

	return queries, sort, nil
}

// Find executes a find mongo query by using the provided FindParams, fills the passed in result
// slice pointer and returns a Cursor.
func Find(ctx context.Context, p FindParams, results interface{}) (Cursor, error) {
	var err error
	p = ensureMandatoryParams(p)
	err = validate(results, p.PaginatedFields)
	if err != nil {
		return Cursor{}, err
	}

	// Compute total count of documents matching filter - only computed if CountTotal is True
	var count int
	if p.CountTotal {
		count, err = executeCountQuery(ctx, p.Collection, []bson.M{p.Query}, p.Collation, p.Timeout)
		if err != nil {
			return Cursor{}, err
		}
	}

	queries, sort, err := BuildQueries(ctx, p)
	if err != nil {
		return Cursor{}, err
	}

	// Execute the augmented query, get an additional element to see if there's another page
	err = executeCursorQuery(ctx, p.Collection, queries, sort, p.Limit, p.Collation, p.Hint, p.Projection, p.Timeout, results)
	if err != nil {
		return Cursor{}, err
	}

	// Get the results slice's pointer and value
	resultsPtr := reflect.ValueOf(results)
	resultsVal := resultsPtr.Elem()

	hasMore := resultsVal.Len() > int(p.Limit)

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
	if len(p.PaginatedFields) == 0 {
		if p.PaginatedField == "_id" {
			p.PaginatedFields = []string{"_id"}
		} else {
			p.PaginatedFields = []string{p.PaginatedField, "_id"}
		}
	} else if p.PaginatedFields[len(p.PaginatedFields)-1] != "_id" {
		p.PaginatedFields = append(p.PaginatedFields, "_id")
		p.SortOrders = append(p.SortOrders, 1)
	}
	if len(p.SortOrders) == 0 {
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

var executeCountQuery = func(ctx context.Context, c Collection, queries []bson.M, collation *options.Collation, timeout time.Duration) (int, error) {
	options := options.Count()
	if collation != nil {
		options.SetCollation(collation)
	}
	if timeout > time.Duration(0) {
		options.SetMaxTime(timeout)
	} else {
		options.SetMaxTime(defaultCursorTimeout)
	}
	count, err := c.CountDocuments(ctx, bson.M{"$and": queries}, options)
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

func executeCursorQuery(ctx context.Context, c Collection, query []bson.M, sort bson.D, limit int64, collation *options.Collation, hint interface{}, projection interface{}, timeout time.Duration, results interface{}) error {
	options := options.Find()
	options.SetSort(sort)
	options.SetLimit(limit + 1)

	if collation != nil {
		options.SetCollation(collation)
	}
	if hint != nil {
		options.SetHint(hint)
	}
	if projection != nil {
		options.SetProjection(projection)
	}
	if timeout > time.Duration(0) {
		options.SetMaxTime(timeout)
	} else {
		options.SetMaxTime(defaultCursorTimeout)
	}
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
	// Set the cursor data
	cursorData := make(bson.D, 0, len(paginatedFields))
	for i := range paginatedFields {
		paginatedFieldValue := recordAsMap[paginatedFields[i]]
		if paginatedFieldValue != nil {
			cursorData = append(cursorData, bson.E{Key: paginatedFields[i], Value: paginatedFieldValue})
		}
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

// validate verifies that the results array is of a supported type and that its underlying struct has a bson tag that
// matches each paginated field
func validate(results interface{}, paginatedFields []string) error {
	if results == nil {
		return NewErrInvalidResults("expected results to be non nil")
	}

	// Check if results is a pointer
	val := reflect.TypeOf(results)
	if val.Kind() != reflect.Ptr {
		return NewErrInvalidResults("expected results to be a slice pointer")
	}

	// Dereference the pointer to get the slice type
	elem := val.Elem()

	// Ensure we are dealing with a slice
	if elem.Kind() != reflect.Slice {
		return NewErrInvalidResults("expected results to be a slice pointer")
	}

	// Get the element type of the slice
	elem = elem.Elem()

	// We can't validate bson.Raw as we don't have the bson tags
	if elem == reflect.TypeOf(bson.Raw{}) || elem == reflect.TypeOf(&bson.Raw{}) {
		return nil
	}

	// If the slice contains pointers to structs, dereference to get the struct type
	if elem.Kind() == reflect.Ptr {
		elem = elem.Elem()
	}

	// Ensure that elem is now a struct
	if elem.Kind() != reflect.Struct {
		return NewErrInvalidResults("expected results' element to be a struct or struct pointer")
	}

	for _, paginatedField := range paginatedFields {
		paginatedFieldFound := false
		for i := 0; i < elem.NumField(); i++ {
			field := elem.Field(i)
			tag := field.Tag.Get("bson")

			tagParts := strings.Split(tag, ",")
			fieldName := strings.TrimSpace(tagParts[0])

			if fieldName == paginatedField {
				paginatedFieldFound = true
				break
			}

			if len(tagParts) > 1 && strings.ToLower(strings.TrimSpace(tagParts[1])) == "inline" && validateInlineFields(field, paginatedField) {
				paginatedFieldFound = true
				break
			}
		}
		if !paginatedFieldFound {
			return NewErrPaginatedFieldNotFound(paginatedField)
		}
	}
	return nil
}


func validateInlineFields(field reflect.StructField ,  paginatedField string) bool {
	if field.Type.Kind() == reflect.Struct {
		// Iterate over fields of the embedded struct
		for j := 0; j < field.Type.NumField(); j++ {
			inlineField := field.Type.Field(j)
			inlineTag := inlineField.Tag.Get("bson")
			inlineTagParts := strings.Split(inlineTag, ",")
			inlineFieldName := strings.TrimSpace(inlineTagParts[0])

			// Check if the embedded struct contains the paginated field
			if inlineFieldName == paginatedField {
				return true
			}
		}
	}
	return false
}
