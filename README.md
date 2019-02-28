[![CircleCI](https://circleci.com/gh/qlik-oss/mgo-cursor-pagination.svg?style=svg)](https://circleci.com/gh/qlik-oss/mgo-cursor-pagination/tree/master)
[![Test Coverage](https://api.codeclimate.com/v1/badges/4e4e0f41b11af79ca677/test_coverage)](https://codeclimate.com/github/qlik-oss/mgo-cursor-pagination/test_coverage)
[![GoDoc](https://godoc.org/github.com/qlik-oss/mgo-cursor-pagination?status.svg)](https://godoc.org/github.com/qlik-oss/mgo-cursor-pagination)

# mgo-cursor-pagination

A go package for the mgo mongo driver ([globalsign/mgo](https://github.com/globalsign/mgo)) which ports the find functionality offered by the node.js [mongo-cursor-pagination](https://github.com/mixmaxhq/mongo-cursor-pagination) module. Also inspired by [icza/minquery](https://github.com/icza/minquery).

`mgo-cursor-pagination` helps implementing cursor based pagination in your mongodb backed service:
```
...where an API passes back a "cursor" (an opaque string) to tell the caller where to query the next or previous pages. The cursor is usually passed using query parameters next and previous...
```

`mgo-cursor-pagination` helps by providing a function that make it easy to query within a Mongo collection and returning a url-safe string that you can return with your HTTP response.

## Example

For this example we will be using an items mongo collection where items look like this:
```go
Item struct {
    ID        bson.ObjectId `bson:"_id"`
    Name      string        `bson:"name"`
    CreatedAt time.Time     `bson:"createdAt"`
}
```

Where the items collection is indexed:
```go
mgo.Index{
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
}
```

The [items store](./test/integration/items_store.go) offers a method to find items (e.g. by name) and paginate the results using the [find function](./mgocursor/find.go) exposed by `mgo-cursor-pagination`:
```go
import "github.com/qlik-oss/mgo-cursor-pagination/mgocursor"
...

// Find returns paginated items from the database matching the provided query
func (m *mongoStore) Find(query bson.M, next string, previous string, limit int, sortAscending bool, paginatedField string, collation mgo.Collation) ([]Item, mgocursor.Cursor, error) {
	fp := mgocursor.FindParams{
        Collection:     m.col,
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
```

Assuming there are 4 items in the collection that have the name "test item n", we can then get the first page of results sorted by name by calling the [items store](./test/integration/items_store.go)'s find method:
```go
searchQuery := bson.M{"name": bson.RegEx{Pattern: "test item.*", Options: "i"}}
englishCollation := mgo.Collation{Locale: "en", Strength: 3}

// Arguments: query, next, previous, limit, sortAsc, paginatedField, collation
foundItems, cursor, err := store.Find(searchQuery, "", "", 2, true, "name", englishCollation)
```

To get the next page:
```go
// Arguments: query, next, previous, limit, sortAsc, paginatedField, collation
foundItems, cursor, err = store.Find(searchQuery, cursor.Next, "", 2, true, "name", englishCollation)
```

from the second page, we can get to first page:
```go
// Arguments: query, next, previous, limit, sortAsc, paginatedField, collation
foundItems, cursor, err = store.Find(searchQuery, "", cursor.Previous, 2, true, "name", englishCollation)
```

See [items_store_test.go](./test/integration/items_store_test.go) for the integration test that uses the [items store](./test/integration/items_store.go)'s find method.
