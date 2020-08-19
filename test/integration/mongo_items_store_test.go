package integration

import (
	"context"
	"os"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func newMongoStore(t *testing.T) MongoStore {
	t.Helper()
	mongoAddr := os.Getenv("MONGO_URI")
	require.NotEmpty(t, mongoAddr, "MONGO_URI is required")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoAddr))
	require.NoError(t, err, "error connecting to mongo")
	col := client.Database("test_db").Collection("items")

	store := NewMongoStore(col)
	require.NoError(t, err)
	return store
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
	foundItems, cursor, err := store.Find(context.Background(), searchQuery, "", "", 4, true, "name", &englishCollation)
	require.NoError(t, err)
	require.Empty(t, foundItems)
	require.False(t, cursor.HasNext)
	require.False(t, cursor.HasPrevious)

	item4 := createMongoItem(t, store, "test item 4")
	item1 := createMongoItem(t, store, "test item 1")
	item3 := createMongoItem(t, store, "test item 3")
	item2 := createMongoItem(t, store, "test item 2")

	// Get first page of search for items
	foundItems, cursor, err = store.Find(context.Background(), searchQuery, "", "", 2, true, "name", &englishCollation)
	require.NoError(t, err)
	require.Equal(t, 2, len(foundItems))
	require.True(t, cursor.HasNext)
	require.False(t, cursor.HasPrevious)
	require.Equal(t, item1.ID, foundItems[0].ID)
	require.Equal(t, item2.ID, foundItems[1].ID)

	// Get 2nd page of search for items
	foundItems, cursor, err = store.Find(context.Background(), searchQuery, cursor.Next, "", 2, true, "name", &englishCollation)
	require.NoError(t, err)
	require.Equal(t, 2, len(foundItems))
	require.False(t, cursor.HasNext)
	require.True(t, cursor.HasPrevious)
	require.Equal(t, item3.ID, foundItems[0].ID)
	require.Equal(t, item4.ID, foundItems[1].ID)

	// Get previous page of search for items
	foundItems, cursor, err = store.Find(context.Background(), searchQuery, "", cursor.Previous, 2, true, "name", &englishCollation)
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

func TestMongoPaginationBSONRaw(t *testing.T) {
	store := newMongoStore(t)
	searchQuery := bson.M{"name": primitive.Regex{Pattern: "test item.*", Options: "i"}}
	englishCollation := options.Collation{Locale: "en", Strength: 3}

	item1 := createMongoItem(t, store, "test item 1")
	item2 := createMongoItem(t, store, "test item 2")
	createMongoItem(t, store, "test item 3")
	createMongoItem(t, store, "test item 4")

	foundItems, cursor, err := store.FindBSONRaw(context.Background(), searchQuery, "", "", 2, true, "name", &englishCollation)
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
