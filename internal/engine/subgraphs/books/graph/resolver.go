//go:generate go run github.com/99designs/gqlgen

package graph

import "github.com/vvakame/fedeway/internal/engine/subgraphs/books/graph/model"

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require here.

type Resolver struct {
	libraries []*model.Library
	books     []*model.Book
}

func NewResolver() *Resolver {
	strptr := func(s string) *string {
		return &s
	}
	intptr := func(i int) *int {
		return &i
	}

	libraries := []*model.Library{
		{
			ID:   "1",
			Name: strptr("NYC Public Library"),
		},
	}

	books := []*model.Book{
		{
			Isbn:  "0262510871",
			Title: strptr("Structure and Interpretation of Computer Programs"),
			Year:  intptr(1996),
			Metadata: []model.MetadataOrError{
				&model.KeyValue{
					Key:   "Condition",
					Value: "excellent",
				},
			},
		},
		{
			Isbn:  "0136291554",
			Title: strptr("Object Oriented Software Construction"),
			Year:  intptr(1997),
			Metadata: []model.MetadataOrError{
				&model.KeyValue{
					Key:   "Condition",
					Value: "used",
				},
				&model.Error{
					Code:    intptr(401),
					Message: strptr("Unauthorized"),
				},
			},
		},
		{
			Isbn:  "0201633612",
			Title: strptr("Design Patterns"),
			Year:  intptr(1995),
			Metadata: []model.MetadataOrError{
				&model.KeyValue{
					Key:   "Condition",
					Value: "excellent",
				},
			},
		},
		{
			Isbn:  "1234567890",
			Title: strptr("The Year Was Null"),
		},
		{
			Isbn:  "404404404",
			Title: strptr(""),
			Year:  intptr(404),
		},
		{
			Isbn:  "0987654321",
			Title: strptr("No Books Like This Book!"),
			Year:  intptr(2019),
		},
	}
	findBook := func(isbn string) *model.Book {
		for _, book := range books {
			if book.Isbn == isbn {
				return book
			}
		}
		return nil
	}
	findBook("0201633612").SimilarBooks = []*model.Book{
		findBook("0201633612"),
		findBook("0136291554"),
	}

	return &Resolver{
		libraries: libraries,
		books:     books,
	}
}
