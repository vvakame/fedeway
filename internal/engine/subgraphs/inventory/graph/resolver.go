//go:generate go run github.com/99designs/gqlgen

package graph

import "github.com/vvakame/fedeway/internal/engine/subgraphs/inventory/graph/model"

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require here.

type Resolver struct {
	inventory []model.Product
}

func NewResolver() *Resolver {
	boolptr := func(b bool) *bool {
		return &b
	}

	inventory := []model.Product{
		&model.Furniture{
			Sku:     "TABLE1",
			InStock: boolptr(true),
			IsHeavy: boolptr(false),
		},
		&model.Furniture{
			Sku:     "COUCH1",
			InStock: boolptr(false),
			IsHeavy: boolptr(true),
		},
		&model.Furniture{
			Sku:     "CHAIR1",
			InStock: boolptr(true),
			IsHeavy: boolptr(false),
		},
		&model.Book{
			Isbn:         "0262510871",
			InStock:      boolptr(true),
			IsCheckedOut: boolptr(true),
		},
		&model.Book{
			Isbn:         "0136291554",
			InStock:      boolptr(false),
			IsCheckedOut: boolptr(false),
		},
		&model.Book{
			Isbn:         "0201633612",
			InStock:      boolptr(true),
			IsCheckedOut: boolptr(false),
		},
	}

	return &Resolver{
		inventory: inventory,
	}
}
