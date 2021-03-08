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
		CreatedAt time.Time          `bson:"createdAt"`
	}

	MongoStore interface {
		Create(context.Context, *MongoItem) (*MongoItem, error)
		RemoveAll(context.Context) error
		Find(ctx context.Context, query interface{}, next, previous string, limit int64, sortAscending bool, paginatedField string, collation *options.Collation) ([]*MongoItem, mongocursorpagination.Cursor, error)
		FindBSONRaw(ctx context.Context, query interface{}, next string, previous string, limit int64, sortAscending bool, paginatedField string, collation *options.Collation) ([]bson.Raw, mongocursorpagination.Cursor, error)
		Aggregate(ctx context.Context, pipeline interface{}, next, previous string, limit int64, sortAscending bool, paginatedField string, collation *options.Collation) ([]*MongoItem, mongocursorpagination.Cursor, error)
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

func (c *mongoCollectionWrapper) Aggregate(ctx context.Context, pipeline interface{}, opts ...*options.AggregateOptions) (mongocursorpagination.MongoCursor, error) {
	return c.collection.Aggregate(ctx, pipeline, opts...)
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

// Aggregate returns paginated items from the database matching the provided pipeline
func (m *mongoStore) Aggregate(ctx context.Context, pipeline interface{}, next string, previous string, limit int64, sortAscending bool, paginatedField string, collation *options.Collation) ([]*MongoItem, mongocursorpagination.Cursor, error) {
	var items []*MongoItem
	cursor, err := m.mongoAggregate(ctx, pipeline, next, previous, limit, sortAscending, paginatedField, collation, &items)
	return items, cursor, err
}

func (m *mongoStore) mongoAggregate(ctx context.Context, pipeline interface{}, next string, previous string, limit int64, sortAscending bool, paginatedField string, collation *options.Collation, results interface{}) (mongocursorpagination.Cursor, error) {
	bsonQuery := pipeline.([]bson.M)

	ap := mongocursorpagination.AggregateParams{
		Collection:     m.col,
		Pipeline:       bsonQuery,
		Limit:          limit,
		SortAscending:  sortAscending,
		PaginatedField: paginatedField,
		Collation:      collation,
		Next:           next,
		Previous:       previous,
	}

	c, err := mongocursorpagination.Aggregate(ctx, ap, results)

	cursor := mongocursorpagination.Cursor{
		Previous:    c.Previous,
		Next:        c.Next,
		HasPrevious: c.HasPrevious,
		HasNext:     c.HasNext,
	}

	return cursor, err
}

// Find returns paginated items from the database matching the provided query
func (m *mongoStore) Find(ctx context.Context, query interface{}, next string, previous string, limit int64, sortAscending bool, paginatedField string, collation *options.Collation) ([]*MongoItem, mongocursorpagination.Cursor, error) {
	var items []*MongoItem
	cursor, err := m.mongoFind(ctx, query, next, previous, limit, sortAscending, paginatedField, collation, &items)
	return items, cursor, err
}

func (m *mongoStore) FindBSONRaw(ctx context.Context, query interface{}, next string, previous string, limit int64, sortAscending bool, paginatedField string, collation *options.Collation) ([]bson.Raw, mongocursorpagination.Cursor, error) {
	var items []bson.Raw
	cursor, err := m.mongoFind(ctx, query, next, previous, limit, sortAscending, paginatedField, collation, &items)
	return items, cursor, err
}

func (m *mongoStore) mongoFind(ctx context.Context, query interface{}, next string, previous string, limit int64, sortAscending bool, paginatedField string, collation *options.Collation, results interface{}) (mongocursorpagination.Cursor, error) {
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
