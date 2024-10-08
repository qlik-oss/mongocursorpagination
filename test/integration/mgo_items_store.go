package integration

import (
	"time"

	"github.com/globalsign/mgo"
	"github.com/globalsign/mgo/bson"
	mongocursorpagination "github.com/qlik-oss/mongocursorpagination/mgo"
)

type (
	// Item is the mongo collection template representing an item.
	Item struct {
		ID        bson.ObjectId `bson:"_id"`
		Name      string        `bson:"name"`
		Data      string        `bson:"data"`
		CreatedAt time.Time     `bson:"createdAt"`
	}

	// Store allows operations on items.
	Store interface {
		Create(i *Item) (*Item, error)
		RemoveAll() error
		Find(query interface{}, next string, previous string, limit int, sortAscending bool, paginatedField string, collation mgo.Collation) ([]*Item, mongocursorpagination.Cursor, error)
		FindBSONRaw(query interface{}, next string, previous string, limit int, sortAscending bool, paginatedField string, collation mgo.Collation) ([]bson.Raw, mongocursorpagination.Cursor, error)
		EnsureIndices() error
		FindMultiplePaginatedFields(query interface{}, next string, previous string, limit int, sortOrders []int, paginatedFields []string, collation mgo.Collation) ([]*Item, mongocursorpagination.Cursor, error)
	}

	mgoStore struct {
		col *mgo.Collection
	}
)

// NewMgoStore returns a new Store that uses mgo.
func NewMgoStore(col *mgo.Collection) Store {
	return &mgoStore{
		col: col,
	}
}

// Create creates an item in the database and returns it
func (m *mgoStore) Create(c *Item) (*Item, error) {
	c.ID = bson.NewObjectId() // Generate ObjectID
	c.CreatedAt = bson.Now().UTC()
	return c, m.col.Insert(c)
}

// Find returns paginated items from the database matching the provided query
func (m *mgoStore) Find(query interface{}, next string, previous string, limit int, sortAscending bool, paginatedField string, collation mgo.Collation) ([]*Item, mongocursorpagination.Cursor, error) {
	var items []*Item
	cursor, err := m.find(query, next, previous, limit, sortAscending, paginatedField, collation, &items)
	return items, cursor, err
}

func (m *mgoStore) FindMultiplePaginatedFields(query interface{}, next string, previous string, limit int, sortOrders []int, paginatedFields []string, collation mgo.Collation) ([]*Item, mongocursorpagination.Cursor, error) {
	var items []*Item
	cursor, err := m.findMultiplePaginatedFields(query, next, previous, limit, sortOrders, paginatedFields, collation, &items)
	return items, cursor, err
}

func (m *mgoStore) FindBSONRaw(query interface{}, next string, previous string, limit int, sortAscending bool, paginatedField string, collation mgo.Collation) ([]bson.Raw, mongocursorpagination.Cursor, error) {
	var items []bson.Raw
	cursor, err := m.find(query, next, previous, limit, sortAscending, paginatedField, collation, &items)
	return items, cursor, err
}

func (m *mgoStore) find(query interface{}, next string, previous string, limit int, sortAscending bool, paginatedField string, collation mgo.Collation, results interface{}) (mongocursorpagination.Cursor, error) {
	bsonQuery := query.(bson.M)
	fp := mongocursorpagination.FindParams{
		DB:             m.col.Database,
		CollectionName: m.col.Name,
		Query:          bsonQuery,
		Limit:          limit,
		SortAscending:  sortAscending,
		PaginatedField: paginatedField,
		Collation:      &collation,
		Next:           next,
		Previous:       previous,
		CountTotal:     true,
	}
	c, err := mongocursorpagination.Find(fp, results)
	cursor := mongocursorpagination.Cursor{
		Previous:    c.Previous,
		Next:        c.Next,
		HasPrevious: c.HasPrevious,
		HasNext:     c.HasNext,
	}
	return cursor, err
}

func (m *mgoStore) findMultiplePaginatedFields(query interface{}, next string, previous string, limit int, sortOrders []int, paginatedFields []string, collation mgo.Collation, results interface{}) (mongocursorpagination.Cursor, error) {
	bsonQuery := query.(bson.M)
	fp := mongocursorpagination.FindParams{
		DB:              m.col.Database,
		CollectionName:  m.col.Name,
		Query:           bsonQuery,
		Limit:           limit,
		SortOrders:      sortOrders,
		PaginatedFields: paginatedFields,
		Collation:       &collation,
		Next:            next,
		Previous:        previous,
		CountTotal:      true,
	}
	c, err := mongocursorpagination.Find(fp, results)
	cursor := mongocursorpagination.Cursor{
		Previous:    c.Previous,
		Next:        c.Next,
		HasPrevious: c.HasPrevious,
		HasNext:     c.HasNext,
	}
	return cursor, err
}

// EnsureIndices creates indices and returns any error
func (m *mgoStore) EnsureIndices() error {
	err := m.col.EnsureIndex(mgo.Index{
		Name: "cover_find_by_name",
		// _id is required in the index' key as we secondary sort on _id when the paginated field is not _id
		Key:    []string{"name", "data", "_id"},
		Unique: false,
		Collation: &mgo.Collation{
			Locale:   "en",
			Strength: 3,
		},
		Background: true,
	})
	return err
}

func (m *mgoStore) RemoveAll() error {
	_, err := m.col.RemoveAll(bson.M{})
	return err
}
