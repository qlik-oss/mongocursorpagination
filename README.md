[![CI](https://github.com/qlik-oss/mongocursorpagination/actions/workflows/ci.yml/badge.svg?branch=master)](https://github.com/qlik-oss/mongocursorpagination/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/qlik-oss/mongocursorpagination.svg)](https://pkg.go.dev/github.com/qlik-oss/mongocursorpagination)

# mongocursorpagination

A Go package for the official mongo driver ([mongodb/mongo-go-driver](https://github.com/mongodb/mongo-go-driver), v2) and the mgo mongo driver ([globalsign/mgo](https://github.com/globalsign/mgo)) which ports the find functionality offered by the [mongo-cursor-pagination](https://github.com/mixmaxhq/mongo-cursor-pagination) node.js module. Also inspired by [icza/minquery](https://github.com/icza/minquery).

`mongocursorpagination` helps implementing cursor based pagination in your mongodb backed service:
```
...where an API passes back a "cursor" (an opaque string) to tell the caller where to query the next or previous pages. The cursor is usually passed using query parameters next and previous...
```

`mongocursorpagination` helps by providing a function that make it easy to query within a Mongo collection and returning a url-safe string that you can return with your HTTP response.

## Installation

```sh
go get github.com/qlik-oss/mongocursorpagination
```

Then import the package matching your driver:

| Driver | Package |
|--------|---------|
| [mongo-go-driver v2](https://github.com/mongodb/mongo-go-driver) (`go.mongodb.org/mongo-driver/v2`) | `github.com/qlik-oss/mongocursorpagination/mongo` |
| [globalsign/mgo](https://github.com/globalsign/mgo) | `github.com/qlik-oss/mongocursorpagination/mgo` |

Releases are published as git tags; see [Releases](https://github.com/qlik-oss/mongocursorpagination/releases) for the changelog.

## How it works

`Find` takes a `FindParams` describing the query, the page size (`Limit`), the field(s) to paginate on and, optionally, a `Next` or `Previous` cursor obtained from a previous call. It decodes the results into the provided slice and returns a `Cursor`:

```go
Cursor struct {
    Previous    string // cursor to pass as FindParams.Previous to get the previous page ("" if none)
    Next        string // cursor to pass as FindParams.Next to get the next page ("" if none)
    HasPrevious bool
    HasNext     bool
    Count       int    // total documents matching the query, only set when FindParams.CountTotal is true
}
```

The paginated field must be:
1. Orderable. If duplicate values exist, results are secondarily ordered by `_id`.
2. Indexed, together with `_id`, for query performance on large collections.
3. Immutable. If the value changes between paged queries, a document could appear twice.
4. Named as its bson field name in the result struct (e.g. `"name"` for a field tagged `bson:"name"`).

To paginate on several fields, set `PaginatedFields` and the matching `SortOrders` (`1` or `-1` for each field) instead of `PaginatedField` and `SortAscending`.

## Examples

Both examples use an `items` collection where items look like this:
```go
type Item struct {
    ID        bson.ObjectID `bson:"_id"` // bson.ObjectId with mgo
    Name      string        `bson:"name"`
    CreatedAt time.Time     `bson:"createdAt"`
}
```

### mongo-go-driver

`FindParams.Collection` expects the package's `Collection` interface, whose `Find` method returns a `MongoCursor` interface. As `*mongo.Collection` returns a concrete `*mongo.Cursor`, wrap it with a small adapter:

```go
import (
    "context"

    "go.mongodb.org/mongo-driver/v2/bson"
    "go.mongodb.org/mongo-driver/v2/mongo"
    "go.mongodb.org/mongo-driver/v2/mongo/options"

    mongocursorpagination "github.com/qlik-oss/mongocursorpagination/mongo"
)

type collectionWrapper struct {
    collection *mongo.Collection
}

func (c *collectionWrapper) Find(ctx context.Context, filter any, opts ...options.Lister[options.FindOptions]) (mongocursorpagination.MongoCursor, error) {
    return c.collection.Find(ctx, filter, opts...)
}

func (c *collectionWrapper) CountDocuments(ctx context.Context, filter any, opts ...options.Lister[options.CountOptions]) (int64, error) {
    return c.collection.CountDocuments(ctx, filter, opts...)
}
```

Index the collection on the paginated field and `_id`:
```go
_, err := collection.Indexes().CreateOne(ctx, mongo.IndexModel{
    // _id is required in the index' key as we secondary sort on _id when the paginated field is not _id
    Keys: bson.D{{Key: "name", Value: 1}, {Key: "_id", Value: 1}},
    Options: options.Index().
        SetName("cover_find_by_name").
        SetCollation(&options.Collation{Locale: "en", Strength: 3}),
})
```

Then find items and paginate the results:
```go
// Find returns paginated items from the database matching the provided query
func (s *store) Find(ctx context.Context, query bson.M, next, previous string, limit int64) ([]Item, mongocursorpagination.Cursor, error) {
    fp := mongocursorpagination.FindParams{
        Collection:     &collectionWrapper{collection: s.collection},
        Query:          query,
        Limit:          limit,
        SortAscending:  true,
        PaginatedField: "name",
        Collation:      &options.Collation{Locale: "en", Strength: 3},
        Next:           next,
        Previous:       previous,
        CountTotal:     true,
    }
    var items []Item
    cursor, err := mongocursorpagination.Find(ctx, fp, &items)
    return items, cursor, err
}
```

The mongo-go-driver `FindParams` also supports `Hint`, `Projection` and `Timeout` (a deadline applied to the find and count queries, defaults to 45 seconds). `BuildQueries` returns the paginated queries and sort without executing them.

### mgo

Index the collection on the paginated field and `_id`:
```go
err := collection.EnsureIndex(mgo.Index{
    Name: "cover_find_by_name",
    // _id is required in the index' key as we secondary sort on _id when the paginated field is not _id
    Key:    []string{"name", "_id"},
    Unique: false,
    Collation: &mgo.Collation{
        Locale:   "en",
        Strength: 3,
    },
    Background: true,
})
```

Then find items and paginate the results:
```go
import (
    "github.com/globalsign/mgo"
    "github.com/globalsign/mgo/bson"

    mongocursorpagination "github.com/qlik-oss/mongocursorpagination/mgo"
)

// Find returns paginated items from the database matching the provided query
func (s *store) Find(query bson.M, next, previous string, limit int) ([]Item, mongocursorpagination.Cursor, error) {
    fp := mongocursorpagination.FindParams{
        DB:             s.collection.Database,
        CollectionName: s.collection.Name,
        Query:          query,
        Limit:          limit,
        SortAscending:  true,
        PaginatedField: "name",
        Collation:      &mgo.Collation{Locale: "en", Strength: 3},
        Next:           next,
        Previous:       previous,
        CountTotal:     true,
    }
    var items []Item
    cursor, err := mongocursorpagination.Find(fp, &items)
    return items, cursor, err
}
```

### Paging through results

Assuming there are 4 items in the collection that have the name "test item n", get the first page of 2 results sorted by name:
```go
query := bson.M{"name": bson.M{"$regex": "test item.*", "$options": "i"}}

items, cursor, err := s.Find(ctx, query, "", "", 2)
```

To get the next page, pass the returned `Next` cursor:
```go
items, cursor, err = s.Find(ctx, query, cursor.Next, "", 2)
```

From the second page, pass the returned `Previous` cursor to get back to the first page:
```go
items, cursor, err = s.Find(ctx, query, "", cursor.Previous, 2)
```

An invalid `Next` or `Previous` cursor returns a `*CursorError`, which an HTTP handler can map to a 400 response.

See the integration tests for complete examples:
- mongo-go-driver: [mongo_items_store.go](./test/integration/mongo_items_store.go) and [mongo_items_store_test.go](./test/integration/mongo_items_store_test.go)
- mgo: [mgo_items_store.go](./test/integration/mgo_items_store.go) and [mgo_items_store_test.go](./test/integration/mgo_items_store_test.go)

## Development

```sh
make lint              # run golangci-lint
make test-unit         # run unit tests
make test-integration  # run integration tests against a MongoDB container (requires Docker)
make mod-update        # update dependencies
```

The integration tests start a MongoDB container with [dockertest](https://github.com/ory/dockertest).
