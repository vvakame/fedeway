//go:generate go run github.com/99designs/gqlgen

package graph

import "github.com/vvakame/fedeway/internal/engine/subgraphs/reviews/graph/model"

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require here.

type Resolver struct {
	usernames []*model.User
	reviews   []*model.Review
}

func NewResolver() *Resolver {
	strptr := func(s string) *string {
		return &s
	}
	intptr := func(i int) *int {
		return &i
	}
	_ = strptr
	_ = intptr

	usernames := []*model.User{
		{
			ID:       "1",
			Username: strptr("@ada"),
		},
		{
			ID:       "2",
			Username: strptr("@complete"),
		},
	}
	reviews := []*model.Review{
		{
			ID:       "1",
			AuthorID: "1",
			Product: &model.Furniture{
				Upc: "1",
			},
			Body: strptr("Love it!"),
			Metadata: []model.MetadataOrError{
				&model.Error{
					Code:    intptr(418),
					Message: strptr("I'm a teapot"),
				},
			},
		},
		{
			ID:       "2",
			AuthorID: "1",
			Product: &model.Furniture{
				Upc: "2",
			},
			Body: strptr("Too expensive."),
		},
		{
			ID:       "3",
			AuthorID: "2",
			Product: &model.Furniture{
				Upc: "3",
			},
			Body: strptr("Could be better."),
		},
		{
			ID:       "4",
			AuthorID: "2",
			Product: &model.Furniture{
				Upc: "1",
			},
			Body: strptr("Prefer something else."),
		},
		{
			ID:       "4",
			AuthorID: "2",
			Product: &model.Book{
				Isbn: "0262510871",
			},
			Body: strptr("Wish I had read this before."),
			Metadata: []model.MetadataOrError{
				&model.KeyValue{
					Key:   "likes",
					Value: "5",
				},
			},
		},
		{
			ID:       "6",
			AuthorID: "1",
			Product: &model.Book{
				Isbn: "0201633612",
			},
			Body: strptr("A classic."),
		},
	}

	return &Resolver{
		usernames: usernames,
		reviews:   reviews,
	}
}
