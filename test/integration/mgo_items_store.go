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
		CreatedAt time.Time     `bson:"createdAt"`
	}

	// Store allows operations on items.
	Store interface {
		Create(i *Item) (*Item, error)
		RemoveAll() error
		Find(query interface{}, next string, previous string, limit int, sortAscending bool, paginatedField string, collation mgo.Collation) ([]*Item, mongocursorpagination.Cursor, error)
		EnsureIndices() error
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
	var items []*Item
	c, err := mongocursorpagination.Find(fp, &items)
	cursor := mongocursorpagination.Cursor{
		Previous:    c.Previous,
		Next:        c.Next,
		HasPrevious: c.HasPrevious,
		HasNext:     c.HasNext,
	}
	return items, cursor, err
}

// EnsureIndices creates indices and returns any error
func (m *mgoStore) EnsureIndices() error {
	err := m.col.EnsureIndex(mgo.Index{
		Name: "cover_find_by_name",
		Key: []string{
			"name",
		},
		Unique: false,
		Collation: &mgo.Collation{
			Locale:   "en",
			Strength: 3,
		},
		Background: true,
	})
	return err
}

// EnsureIndices creates indices and returns any error
func (m *mgoStore) RemoveAll() error {
	_, err := m.col.RemoveAll(bson.M{})
	return err
}
