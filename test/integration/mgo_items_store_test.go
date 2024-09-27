package integration

import (
	"os"
	"testing"
	"time"

	"github.com/globalsign/mgo"
	"github.com/globalsign/mgo/bson"
	mgocursorpagination "github.com/qlik-oss/mongocursorpagination/mgo"
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

func createItem(t *testing.T, store Store, name string, data string) *Item {
	t.Helper()
	item := &Item{
		ID:        "",
		Name:      name,
		Data:      data,
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

	item4 := createItem(t, store, "test item 4", "")
	item1 := createItem(t, store, "test item 1", "")
	item3 := createItem(t, store, "test item 3", "")
	item2 := createItem(t, store, "test item 2", "")

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

	// Cleanup
	err = store.RemoveAll()
	require.NoError(t, err)
}

func TestFindManyPaginationWithDuplicatedPaginatedField(t *testing.T) {
	store := newStore(t)

	searchQuery := bson.M{"name": "duplicated name"}
	englishCollation := mgo.Collation{Locale: "en", Strength: 3}

	// Get empty array when no items created
	foundItems, cursor, err := store.Find(searchQuery, "", "", 4, false, "name", englishCollation)
	require.NoError(t, err)
	require.Empty(t, foundItems)
	require.False(t, cursor.HasNext)
	require.False(t, cursor.HasPrevious)

	item1 := createItem(t, store, "duplicated name", "")
	item2 := createItem(t, store, "duplicated name", "")
	item3 := createItem(t, store, "duplicated name", "")
	item4 := createItem(t, store, "duplicated name", "")

	// Get first page of search for items
	foundItems, cursor, err = store.Find(searchQuery, "", "", 2, false, "name", englishCollation)
	require.NoError(t, err)
	require.Equal(t, 2, len(foundItems))
	require.True(t, cursor.HasNext)
	require.False(t, cursor.HasPrevious)
	require.Equal(t, item4.ID, foundItems[0].ID)
	require.Equal(t, item3.ID, foundItems[1].ID)

	// Get 2nd page of search for items
	foundItems, cursor, err = store.Find(searchQuery, cursor.Next, "", 2, false, "name", englishCollation)
	require.NoError(t, err)
	require.Equal(t, 2, len(foundItems))
	require.False(t, cursor.HasNext)
	require.True(t, cursor.HasPrevious)
	require.Equal(t, item2.ID, foundItems[0].ID)
	require.Equal(t, item1.ID, foundItems[1].ID)

	// Get previous page of search for items
	foundItems, cursor, err = store.Find(searchQuery, "", cursor.Previous, 2, false, "name", englishCollation)
	require.NoError(t, err)
	require.Equal(t, 2, len(foundItems))
	require.True(t, cursor.HasNext)
	require.False(t, cursor.HasPrevious)
	require.Equal(t, item4.ID, foundItems[0].ID)
	require.Equal(t, item3.ID, foundItems[1].ID)

	// Cleanup
	err = store.RemoveAll()
	require.NoError(t, err)
}

func TestCollectionsFindManyCursorError(t *testing.T) {
	store := newStore(t)

	searchQuery := bson.M{"name": bson.RegEx{Pattern: "test item.*", Options: "i"}}
	englishCollation := mgo.Collation{Locale: "en", Strength: 3}

	// Get empty array when no items created
	foundItems, cursor, err := store.Find(searchQuery, "bad_cursor_string", "", 4, true, "name", englishCollation)
	require.Error(t, err)
	require.IsType(t, &mgocursorpagination.CursorError{}, err)
	require.Empty(t, foundItems)
	require.False(t, cursor.HasNext)
	require.False(t, cursor.HasPrevious)
}

func TestMgoPaginationBSONRaw(t *testing.T) {
	store := newStore(t)
	searchQuery := bson.M{"name": bson.RegEx{Pattern: "test item.*", Options: "i"}}
	englishCollation := mgo.Collation{Locale: "en", Strength: 3}

	item1 := createItem(t, store, "test item 1", "")
	item2 := createItem(t, store, "test item 2", "")
	createItem(t, store, "test item 3", "")
	createItem(t, store, "test item 4", "")

	foundItems, cursor, err := store.FindBSONRaw(searchQuery, "", "", 2, true, "name", englishCollation)
	require.NoError(t, err)
	require.Equal(t, 2, len(foundItems))
	require.True(t, cursor.HasNext)
	require.False(t, cursor.HasPrevious)

	var result0 Item
	var result1 Item
	err = foundItems[0].Unmarshal(&result0)
	require.NoError(t, err)
	err = foundItems[1].Unmarshal(&result1)
	require.NoError(t, err)
	require.Equal(t, item1.ID, result0.ID)
	require.Equal(t, item2.ID, result1.ID)

	// Cleanup
	err = store.RemoveAll()
	require.NoError(t, err)
}

func TestCollectionFindMultiplePaginatedFields(t *testing.T) {
	store := newStore(t)

	searchQuery := bson.M{"name": bson.RegEx{Pattern: "test item.*", Options: "i"}}
	englishCollation := mgo.Collation{Locale: "en", Strength: 3}

	// Get empty array when no items created
	foundItems, cursor, err := store.FindMultiplePaginatedFields(searchQuery, "", "", 4, []int{1, -1}, []string{"data", "name"}, englishCollation)
	require.NoError(t, err)
	require.Empty(t, foundItems)
	require.False(t, cursor.HasNext)
	require.False(t, cursor.HasPrevious)

	item1 := createItem(t, store, "test item 1", "5")
	item2 := createItem(t, store, "test item 2", "5")
	item3 := createItem(t, store, "test item 3", "5")
	item4 := createItem(t, store, "test item 4", "5")
	item5 := createItem(t, store, "test item 5", "4")
	item6 := createItem(t, store, "test item 6", "4")
	item7 := createItem(t, store, "test item 7", "3")
	item8 := createItem(t, store, "test item 8", "2")

	// Get first page of search for items
	foundItems, cursor, err = store.FindMultiplePaginatedFields(searchQuery, "", "", 4, []int{1, -1}, []string{"data", "name"}, englishCollation)
	require.NoError(t, err)
	require.Equal(t, 4, len(foundItems))
	require.True(t, cursor.HasNext)
	require.False(t, cursor.HasPrevious)
	require.Equal(t, item8.ID, foundItems[0].ID)
	require.Equal(t, item7.ID, foundItems[1].ID)
	require.Equal(t, item6.ID, foundItems[2].ID)
	require.Equal(t, item5.ID, foundItems[3].ID)

	// Get 2nd page of search for items
	foundItems, cursor, err = store.FindMultiplePaginatedFields(searchQuery, cursor.Next, "", 4, []int{1, -1}, []string{"data", "name"}, englishCollation)
	require.NoError(t, err)
	require.Equal(t, 4, len(foundItems))
	require.False(t, cursor.HasNext)
	require.True(t, cursor.HasPrevious)
	require.Equal(t, item4.ID, foundItems[0].ID)
	require.Equal(t, item3.ID, foundItems[1].ID)
	require.Equal(t, item2.ID, foundItems[2].ID)
	require.Equal(t, item1.ID, foundItems[3].ID)

	// Get previous page of search for items
	foundItems, cursor, err = store.FindMultiplePaginatedFields(searchQuery, "", cursor.Previous, 4, []int{1, -1}, []string{"data", "name"}, englishCollation)
	require.NoError(t, err)
	require.Equal(t, 4, len(foundItems))
	require.True(t, cursor.HasNext)
	require.False(t, cursor.HasPrevious)
	require.Equal(t, item8.ID, foundItems[0].ID)
	require.Equal(t, item7.ID, foundItems[1].ID)
	require.Equal(t, item6.ID, foundItems[2].ID)
	require.Equal(t, item5.ID, foundItems[3].ID)
	// Cleanup
	err = store.RemoveAll()
	require.NoError(t, err)
}
