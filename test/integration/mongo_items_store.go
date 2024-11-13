package integration

import (
	"context"
	"time"

	mongocursorpagination "github.com/qlik-oss/mongocursorpagination/mongo"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type (
	MongoItem struct {
		ID        primitive.ObjectID `bson:"_id"`
		Name      string             `bson:"name"`
		Data      string             `bson:"data"`
		CreatedAt time.Time          `bson:"createdAt"`
		Inline    InlineItem         `bson:",inline"`
	}

	InlineItem struct {
		Sample string `bson:"sample"`
	}

	MongoStore interface {
		Create(context.Context, *MongoItem) (*MongoItem, error)
		RemoveAll(context.Context) error
		Find(ctx context.Context, query interface{}, next string, previous string, limit int64, sortAscending bool, paginatedField string, collation *options.Collation, hint interface{}, projection interface{}) ([]*MongoItem, mongocursorpagination.Cursor, error)
		FindBSONRaw(ctx context.Context, query interface{}, next string, previous string, limit int64, sortAscending bool, paginatedField string, collation *options.Collation, hint interface{}, projection interface{}) ([]bson.Raw, mongocursorpagination.Cursor, error)
		FindMultiplePaginatedFields(ctx context.Context, query interface{}, next string, previous string, limit int64, sortOrders []int, paginatedFields []string, collation *options.Collation, hint interface{}, projection interface{}) ([]*MongoItem, mongocursorpagination.Cursor, error)
	}

	mongoStore struct {
		col *mongoCollectionWrapper
	}

	mongoCollectionWrapper struct {
		collection *mongo.Collection
	}
)

func (c *mongoCollectionWrapper) Find(ctx context.Context, filter interface{}, opts ...*options.FindOptions) (mongocursorpagination.MongoCursor, error) {
	return c.collection.Find(ctx, filter, opts...)
}

func (c *mongoCollectionWrapper) InsertOne(ctx context.Context, document interface{}, opts ...*options.InsertOneOptions) (*mongo.InsertOneResult, error) {
	return c.collection.InsertOne(ctx, document, opts...)
}

func (c *mongoCollectionWrapper) CountDocuments(ctx context.Context, filter interface{}, opts ...*options.CountOptions) (int64, error) {
	return c.collection.CountDocuments(ctx, filter, opts...)
}

func (c *mongoCollectionWrapper) DeleteMany(ctx context.Context, filter interface{}, opts ...*options.DeleteOptions) (*mongo.DeleteResult, error) {
	return c.collection.DeleteMany(ctx, filter, opts...)
}

func NewMongoStore(col *mongoCollectionWrapper) MongoStore {
	return &mongoStore{
		col: col,
	}
}

// Create creates an item in the database and returns it
func (m *mongoStore) Create(ctx context.Context, c *MongoItem) (*MongoItem, error) {
	c.CreatedAt = time.Now()

	result, err := m.col.InsertOne(ctx, c)
	if err != nil {
		return nil, err
	}

	c.ID = result.InsertedID.(primitive.ObjectID)
	return c, nil
}

// Find returns paginated items from the database matching the provided query
func (m *mongoStore) Find(ctx context.Context, query interface{}, next string, previous string, limit int64, sortAscending bool, paginatedField string, collation *options.Collation, hint interface{}, projection interface{}) ([]*MongoItem, mongocursorpagination.Cursor, error) {
	var items []*MongoItem
	cursor, err := m.mongoFind(ctx, query, next, previous, limit, sortAscending, paginatedField, collation, hint, projection, &items)
	return items, cursor, err
}

func (m *mongoStore) FindMultiplePaginatedFields(ctx context.Context, query interface{}, next string, previous string, limit int64, sortOrders []int, paginatedFields []string, collation *options.Collation, hint interface{}, projection interface{}) ([]*MongoItem, mongocursorpagination.Cursor, error) {
	var items []*MongoItem
	cursor, err := m.mongoFindMultiplePaginatedFields(ctx, query, next, previous, limit, sortOrders, paginatedFields, collation, hint, projection, &items)
	return items, cursor, err
}

func (m *mongoStore) FindBSONRaw(ctx context.Context, query interface{}, next string, previous string, limit int64, sortAscending bool, paginatedField string, collation *options.Collation, hint interface{}, projection interface{}) ([]bson.Raw, mongocursorpagination.Cursor, error) {
	var items []bson.Raw
	cursor, err := m.mongoFind(ctx, query, next, previous, limit, sortAscending, paginatedField, collation, hint, projection, &items)
	return items, cursor, err
}

func (m *mongoStore) mongoFind(ctx context.Context, query interface{}, next string, previous string, limit int64, sortAscending bool, paginatedField string, collation *options.Collation, hint interface{}, projection interface{}, results interface{}) (mongocursorpagination.Cursor, error) {
	bsonQuery := query.(bson.M)
	fp := mongocursorpagination.FindParams{
		Collection:     m.col,
		Query:          bsonQuery,
		Limit:          limit,
		SortAscending:  sortAscending,
		PaginatedField: paginatedField,
		Collation:      collation,
		Next:           next,
		Previous:       previous,
		CountTotal:     true,
		Hint:           hint,
		Projection:     projection,
	}
	c, err := mongocursorpagination.Find(ctx, fp, results)
	cursor := mongocursorpagination.Cursor{
		Previous:    c.Previous,
		Next:        c.Next,
		HasPrevious: c.HasPrevious,
		HasNext:     c.HasNext,
	}
	return cursor, err
}

func (m *mongoStore) mongoFindMultiplePaginatedFields(ctx context.Context, query interface{}, next string, previous string, limit int64, sortOrders []int, paginatedFields []string, collation *options.Collation, hint interface{}, projection interface{}, results interface{}) (mongocursorpagination.Cursor, error) {
	bsonQuery := query.(bson.M)
	fp := mongocursorpagination.FindParams{
		Collection:      m.col,
		Query:           bsonQuery,
		Limit:           limit,
		SortOrders:      sortOrders,
		PaginatedFields: paginatedFields,
		Collation:       collation,
		Next:            next,
		Previous:        previous,
		CountTotal:      true,
		Hint:            hint,
		Projection:      projection,
	}
	c, err := mongocursorpagination.Find(ctx, fp, results)
	cursor := mongocursorpagination.Cursor{
		Previous:    c.Previous,
		Next:        c.Next,
		HasPrevious: c.HasPrevious,
		HasNext:     c.HasNext,
	}
	return cursor, err
}

func (m *mongoStore) RemoveAll(ctx context.Context) error {
	_, err := m.col.DeleteMany(ctx, bson.M{})
	return err
}
