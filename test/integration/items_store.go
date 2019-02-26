package integration

import (
	"time"

	"github.com/globalsign/mgo"
	"github.com/globalsign/mgo/bson"
	"github.com/qlik-oss/mgo-cursor-pagination/mgocursor"
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
		Create(i Item) (Item, error)
		Find(query bson.M, next string, previous string, limit int, sortAscending bool, paginatedField string, collation mgo.Collation) ([]Item, mgocursor.Cursor, error)
		EnsureIndices() error
	}

	mongoStore struct {
		col mgo.Collection
	}
)

// NewMongoStore returns a new Store.
func NewMongoStore(col mgo.Collection) Store {
	return &mongoStore{
		col: col,
	}
}

// Create creates an item in the database and returns it
func (m *mongoStore) Create(c Item) (Item, error) {
	c.ID = bson.NewObjectId() // Generate ObjectID
	c.CreatedAt = bson.Now().UTC()
	return c, m.col.Insert(c)
}

// Find returns paginated items from the database matching the provided query
func (m *mongoStore) Find(query bson.M, next string, previous string, limit int, sortAscending bool, paginatedField string, collation mgo.Collation) ([]Item, mgocursor.Cursor, error) {
	fp := mgocursor.FindParams{
		DB:             m.col.Database,
		CollectionName: m.col.Name,
		Query:          query,
		Limit:          limit,
		SortAscending:  sortAscending,
		PaginatedField: paginatedField,
		Collation:      &collation,
		Next:           next,
		Previous:       previous,
		CountTotal:     true,
	}
	var items []Item
	c, err := mgocursor.Find(fp, &items)
	cursor := mgocursor.Cursor{
		Previous:    c.Previous,
		Next:        c.Next,
		HasPrevious: c.HasPrevious,
		HasNext:     c.HasNext,
	}
	return items, cursor, err
}

// EnsureIndices creates indices and returns any error
func (m *mongoStore) EnsureIndices() error {
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
