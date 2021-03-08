package mongo

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	mcpbson "github.com/qlik-oss/mongocursorpagination/bson"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type (
	// FindParams holds the parameters to be used in a paginated find mongo query that will return a
	// Cursor.
	AggregateParams struct {
		Collection Collection

		// The find query to augment with pagination
		Pipeline []primitive.M
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
)

// Find executes a find mongo query by using the provided FindParams, fills the passed in result
// slice pointer and returns a Cursor.
func Aggregate(ctx context.Context, p AggregateParams, results interface{}) (Cursor, error) {
	var err error
	if results == nil {
		return Cursor{}, errors.New("results can't be nil")
	}

	if p.PaginatedField == "" {
		p.PaginatedField = "_id"
		p.Collation = nil
	}
	shouldSecondarySortOnID := p.PaginatedField != "_id"

	if p.Collection == nil {
		return Cursor{}, errors.New("Collection can't be nil")
	}

	if p.Limit <= 0 {
		return Cursor{}, errors.New("a limit of at least 1 is required")
	}

	nextCursorValues, err := parseCursor(p.Next, shouldSecondarySortOnID)
	if err != nil {
		return Cursor{}, &CursorError{fmt.Errorf("next cursor parse failed: %s", err)}
	}

	previousCursorValues, err := parseCursor(p.Previous, shouldSecondarySortOnID)
	if err != nil {
		return Cursor{}, &CursorError{fmt.Errorf("previous cursor parse failed: %s", err)}
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
	queries := p.Pipeline

	// Compute total count of documents matching filter - only computed if CountTotal is True
	var count int
	if p.CountTotal {
		count, err = executeAggregateCountQuery(ctx, p.Collection, queries)
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
		cursorQuery, err = mcpbson.GenerateCursorQuery(shouldSecondarySortOnID, p.PaginatedField, comparisonOp, cursorValues)
		if err != nil {
			return Cursor{}, err
		}

		queries = append(queries, primitive.M{"$match": cursorQuery})
	}

	// Setup the sort query
	var sort bson.M
	if shouldSecondarySortOnID {
		sort = primitive.M{
			"$sort": primitive.M{
				p.PaginatedField: sortDir,
				"_id":            sortDir,
			},
		}
	} else {
		sort = primitive.M{
			"$sort": primitive.M{
				"_id": sortDir,
			},
		}
	}
	queries = append(queries, sort)

	limit := primitive.M{"$limit": p.Limit + 1}

	queries = append(queries, limit)

	// Execute the augmented query, get an additional element to see if there's another page
	err = executeAggregateCursorQuery(ctx, p.Collection, queries, p.Collation, results)
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
			previousCursor, err = generateCursor(firstResult, p.PaginatedField, shouldSecondarySortOnID)
			if err != nil {
				return Cursor{}, fmt.Errorf("could not create a previous cursor: %s", err)
			}
		}

		// Generate the next cursor
		if hasNext {
			lastResult := resultsVal.Index(resultsVal.Len() - 1).Interface()
			nextCursor, err = generateCursor(lastResult, p.PaginatedField, shouldSecondarySortOnID)
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

func executeAggregateCursorQuery(ctx context.Context, c Collection, query []bson.M, collation *options.Collation, results interface{}) error {
	options := options.Aggregate()

	if collation != nil {
		options.SetCollation(collation)
	}
	cursor, err := c.Aggregate(ctx, query, options)

	if err != nil {
		return err
	}
	err = cursor.All(ctx, results)

	if err != nil {
		return err
	}
	return nil
}

var executeAggregateCountQuery = func(ctx context.Context, c Collection, pipeline []bson.M) (int, error) {
	count, err := c.CountDocuments(ctx, pipeline)
	if err != nil {
		return 0, err
	}
	return int(count), nil
}
