package integration

import (
	"os"
	"testing"
	"time"

	"github.com/globalsign/mgo"
	"github.com/globalsign/mgo/bson"
	"github.com/stretchr/testify/require"
)

func newStore(t *testing.T) Store {
	t.Helper()
	mongoAddr := os.Getenv("MONGO_URI")
	require.NotEmpty(t, mongoAddr, "MONGO_URI is required")
	session, err := mgo.Dial(mongoAddr)
	require.NoError(t, err, "error connecting to mongo")
	col := session.DB("test_db").C("items")
	store := NewMgoStore(col)
	err = store.EnsureIndices()
	require.NoError(t, err)
	return store
}

func createItem(t *testing.T, store Store, name string) *Item {
	t.Helper()
	item := &Item{
		ID:        "",
		Name:      name,
		CreatedAt: time.Now(),
	}
	item, err := store.Create(item)
	require.NoError(t, err)
	return item
}

func TestCollectionsFindManyPagination(t *testing.T) {
	store := newStore(t)

	searchQuery := bson.M{"name": bson.RegEx{Pattern: "test item.*", Options: "i"}}
	englishCollation := mgo.Collation{Locale: "en", Strength: 3}

	// Get empty array when no items created
	foundItems, cursor, err := store.Find(searchQuery, "", "", 4, true, "name", englishCollation)
	require.NoError(t, err)
	require.Empty(t, foundItems)
	require.False(t, cursor.HasNext)
	require.False(t, cursor.HasPrevious)

	item4 := createItem(t, store, "test item 4")
	item1 := createItem(t, store, "test item 1")
	item3 := createItem(t, store, "test item 3")
	item2 := createItem(t, store, "test item 2")

	// Get first page of search for items
	foundItems, cursor, err = store.Find(searchQuery, "", "", 2, true, "name", englishCollation)
	require.NoError(t, err)
	require.Equal(t, 2, len(foundItems))
	require.True(t, cursor.HasNext)
	require.False(t, cursor.HasPrevious)
	require.Equal(t, item1.ID, foundItems[0].ID)
	require.Equal(t, item2.ID, foundItems[1].ID)

	// Get 2nd page of search for items
	foundItems, cursor, err = store.Find(searchQuery, cursor.Next, "", 2, true, "name", englishCollation)
	require.NoError(t, err)
	require.Equal(t, 2, len(foundItems))
	require.False(t, cursor.HasNext)
	require.True(t, cursor.HasPrevious)
	require.Equal(t, item3.ID, foundItems[0].ID)
	require.Equal(t, item4.ID, foundItems[1].ID)

	// Get previous page of search for items
	foundItems, cursor, err = store.Find(searchQuery, "", cursor.Previous, 2, true, "name", englishCollation)
	require.NoError(t, err)
	require.Equal(t, 2, len(foundItems))
	require.True(t, cursor.HasNext)
	require.False(t, cursor.HasPrevious)
	require.Equal(t, item1.ID, foundItems[0].ID)
	require.Equal(t, item2.ID, foundItems[1].ID)
}
