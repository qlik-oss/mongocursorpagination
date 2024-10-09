package integration

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	mongocursorpagination "github.com/qlik-oss/mongocursorpagination/mongo"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func newMongoStore(t *testing.T) MongoStore {
	store := NewMongoStore(newMongoCollection(t))
	return store
}

func newMongoCollection(t *testing.T) *mongoCollectionWrapper {
	t.Helper()
	mongoAddr := os.Getenv("MONGO_URI")
	require.NotEmpty(t, mongoAddr, "MONGO_URI is required")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoAddr))
	require.NoError(t, err, "error connecting to mongo")
	col := mongoCollectionWrapper{
		collection: client.Database("test_db").Collection("items"),
	}
	return &col
}

func createMongoItem(t *testing.T, mongoStore MongoStore, name string) *MongoItem {
	t.Helper()
	item := &MongoItem{
		ID:        primitive.NewObjectID(),
		Name:      name,
		CreatedAt: time.Now(),
	}
	item, err := mongoStore.Create(context.Background(), item)
	require.NoError(t, err)
	return item
}

func TestMongoFindManyPagination(t *testing.T) {
	store := newMongoStore(t)
	searchQuery := bson.M{"name": primitive.Regex{Pattern: "test item.*", Options: "i"}}
	englishCollation := options.Collation{Locale: "en", Strength: 3}

	// Get empty array when no items created
	foundItems, cursor, err := store.Find(context.Background(), searchQuery, "", "", 4, true, "name", &englishCollation, nil, nil)
	require.NoError(t, err)
	require.Empty(t, foundItems)
	require.False(t, cursor.HasNext)
	require.False(t, cursor.HasPrevious)

	item4 := createMongoItem(t, store, "test item 4")
	item1 := createMongoItem(t, store, "test item 1")
	item3 := createMongoItem(t, store, "test item 3")
	item2 := createMongoItem(t, store, "test item 2")

	// Get first page of search for items
	foundItems, cursor, err = store.Find(context.Background(), searchQuery, "", "", 2, true, "name", &englishCollation, nil, nil)
	require.NoError(t, err)
	require.Equal(t, 2, len(foundItems))
	require.True(t, cursor.HasNext)
	require.False(t, cursor.HasPrevious)
	require.Equal(t, item1.ID, foundItems[0].ID)
	require.Equal(t, item2.ID, foundItems[1].ID)

	// Get 2nd page of search for items
	foundItems, cursor, err = store.Find(context.Background(), searchQuery, cursor.Next, "", 2, true, "name", &englishCollation, nil, nil)
	require.NoError(t, err)
	require.Equal(t, 2, len(foundItems))
	require.False(t, cursor.HasNext)
	require.True(t, cursor.HasPrevious)
	require.Equal(t, item3.ID, foundItems[0].ID)
	require.Equal(t, item4.ID, foundItems[1].ID)

	// Get previous page of search for items
	foundItems, cursor, err = store.Find(context.Background(), searchQuery, "", cursor.Previous, 2, true, "name", &englishCollation, nil, nil)
	require.NoError(t, err)
	require.Equal(t, 2, len(foundItems))
	require.True(t, cursor.HasNext)
	require.False(t, cursor.HasPrevious)
	require.Equal(t, item1.ID, foundItems[0].ID)
	require.Equal(t, item2.ID, foundItems[1].ID)

	// Cleanup
	err = store.RemoveAll(context.Background())
	require.NoError(t, err)
}

func TestPaginationWithoutPaginatedField(t *testing.T) {
	const itemNamePrefix = "TestPaginationWithoutPaginatedField"
	store := newMongoStore(t)
	searchQuery := bson.M{"name": primitive.Regex{Pattern: fmt.Sprintf("%s.*", itemNamePrefix)}}

	item1 := createMongoItem(t, store, fmt.Sprintf("%s-1", itemNamePrefix))
	item2 := createMongoItem(t, store, fmt.Sprintf("%s-2", itemNamePrefix))

	// Call Find without paginatedField argument.
	foundItems, cursor, err := store.Find(context.Background(), searchQuery, "", "", 1, true, "", &options.Collation{}, nil, nil)
	require.NoError(t, err)
	require.Len(t, foundItems, 1)
	require.Equal(t, item1.Name, foundItems[0].Name)
	require.True(t, cursor.HasNext)

	// Validate that cursor.Next works as expected.
	foundItems, cursor, err = store.Find(context.Background(), searchQuery, cursor.Next, "", 1, true, "", &options.Collation{}, nil, nil)
	require.NoError(t, err)
	require.Len(t, foundItems, 1)
	require.Equal(t, item2.Name, foundItems[0].Name)

	// Cleanup.
	err = store.RemoveAll(context.Background())
	require.NoError(t, err)
}

func TestMongoFindCursorError(t *testing.T) {
	store := newMongoStore(t)
	searchQuery := bson.M{"name": primitive.Regex{Pattern: "test item.*", Options: "i"}}
	englishCollation := options.Collation{Locale: "en", Strength: 3}
	foundItems, cursor, err := store.Find(context.Background(), searchQuery, "bad_cursor_string", "", 4, true, "name", &englishCollation, nil, nil)
	require.Error(t, err)
	require.IsType(t, &mongocursorpagination.CursorError{}, err)
	require.Empty(t, foundItems)
	require.False(t, cursor.HasNext)
	require.False(t, cursor.HasPrevious)
}

func TestMongoPaginationBSONRaw(t *testing.T) {
	store := newMongoStore(t)
	searchQuery := bson.M{"name": primitive.Regex{Pattern: "test item.*", Options: "i"}}
	englishCollation := options.Collation{Locale: "en", Strength: 3}

	item1 := createMongoItem(t, store, "test item 1")
	item2 := createMongoItem(t, store, "test item 2")
	createMongoItem(t, store, "test item 3")
	createMongoItem(t, store, "test item 4")

	foundItems, cursor, err := store.FindBSONRaw(context.Background(), searchQuery, "", "", 2, true, "name", &englishCollation, nil, nil)
	require.NoError(t, err)
	require.Equal(t, 2, len(foundItems))
	require.True(t, cursor.HasNext)
	require.False(t, cursor.HasPrevious)
	require.Equal(t, item1.ID, foundItems[0].Lookup("_id").ObjectID())
	require.Equal(t, item2.ID, foundItems[1].Lookup("_id").ObjectID())

	// Cleanup
	err = store.RemoveAll(context.Background())
	require.NoError(t, err)
}

func TestMongoBuildPaginatedQueries(t *testing.T) {
	col := newMongoCollection(t)
	englishCollation := options.Collation{Locale: "en", Strength: 3}
	oid1, _ := primitive.ObjectIDFromHex("5addf533e81549de7696cb04")
	query1 := bson.M{
		"$or": []bson.M{
			{"name": "foo"},
			{"name": ""},
		},
	}
	query2 := bson.M{
		"$or": []map[string]interface{}{
			{"name": map[string]interface{}{
				"$gt": "foo",
			}},
			{
				"$and": []map[string]interface{}{
					{
						"name": map[string]interface{}{
							"$gte": "foo",
						},
					},
					{
						"_id": map[string]interface{}{
							"$gt": oid1,
						},
					},
				},
			},
		},
	}

	testCases := []struct {
		name          string
		params        mongocursorpagination.FindParams
		expectQueries []bson.M
		expectSort    bson.D
		expectError   error
	}{
		{
			name: "Sort on name, first page",
			params: mongocursorpagination.FindParams{
				Collection:     col,
				Query:          query1,
				Limit:          int64(42),
				SortAscending:  true,
				PaginatedField: "name",
				Collation:      &englishCollation,
				Next:           "",
				Previous:       "",
				CountTotal:     false,
			},
			expectQueries: []bson.M{query1},
			expectSort:    bson.D{primitive.E{Key: "name", Value: 1}, primitive.E{Key: "_id", Value: 1}},
			expectError:   nil,
		},
		{
			name: "Sort on name, second page",
			params: mongocursorpagination.FindParams{
				Collection:     col,
				Query:          query1,
				Limit:          int64(42),
				SortAscending:  true,
				PaginatedField: "name",
				Collation:      &englishCollation,
				Next:           encodeCursor(t, bson.D{bson.E{Key: "name", Value: "foo"}, bson.E{Key: "_id", Value: oid1}}),
				Previous:       "",
				CountTotal:     false,
			},
			expectQueries: []bson.M{query1, query2},
			expectSort:    bson.D{bson.E{Key: "name", Value: 1}, bson.E{Key: "_id", Value: 1}},
			expectError:   nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			queries, sort, err := mongocursorpagination.BuildQueries(context.TODO(), tc.params)
			require.Equal(t, tc.expectError, err, "BuildQueries returned unexpected error")
			require.Equal(t, tc.expectQueries, queries, "Expected queries did not match")
			require.Equal(t, tc.expectSort, sort, "Expected sort did not match")
		})
	}
}

func TestMongoProjection(t *testing.T) {
	store := newMongoStore(t)

	_ = createMongoItem(t, store, "test item 0")
	_ = createMongoItem(t, store, "test item 1")

	searchQuery := bson.M{}
	projection := bson.D{
		{Key: "_id", Value: 0}, // Do not return ID
		{Key: "name", Value: 1},
	}

	foundItems, _, err := store.FindBSONRaw(context.Background(), searchQuery, "", "", 2, true, "name", nil, nil, projection)
	require.NoError(t, err)
	require.Equal(t, 2, len(foundItems))
	require.Equal(t, `{"name": "test item 0"}`, foundItems[0].String())
	require.Equal(t, `{"name": "test item 1"}`, foundItems[1].String())

	// Cleanup
	err = store.RemoveAll(context.Background())
	require.NoError(t, err)
}

func TestMongoHint(t *testing.T) {
	store := newMongoStore(t)
	searchQuery := bson.M{}

	for _, c := range "abcdefg" {
		_ = createMongoItem(t, store, string(c))
	}

	_, _, err := store.Find(context.Background(), searchQuery, "", "", 10, true, "_id", nil, "indexName_id", nil)
	require.True(t, errors.As(err, &mongo.CommandError{}), "non existing index by name should result in a command error")

	_, _, err = store.Find(context.Background(), searchQuery, "", "", 10, true, "_id", nil, bson.D{{Key: "created", Value: 1}}, nil)
	require.True(t, errors.As(err, &mongo.CommandError{}), "non existing index by specification document should result in a command error")

	_, _, err = store.Find(context.Background(), searchQuery, "", "", 10, true, "_id", nil, "_id_", nil)
	require.NoError(t, err, "hinting the default _id index by name should succeed")

	_, _, err = store.Find(context.Background(), searchQuery, "", "", 10, true, "_id", nil, bson.D{{Key: "_id", Value: 1}}, nil)
	require.NoError(t, err, "hinting the default _id index by specification document should succeed")

	// Cleanup
	err = store.RemoveAll(context.Background())
	require.NoError(t, err)
}

func encodeCursor(t *testing.T, cursorData bson.D) string {
	data, err := bson.Marshal(cursorData)
	require.NoError(t, err, "invalid cursorData given to encodeCursor")
	return base64.RawURLEncoding.EncodeToString(data)
}
