//go:generate go run github.com/99designs/gqlgen

package graph

import "github.com/vvakame/fedeway/internal/engine/subgraphs/product/graph/model"

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require here.

type Resolver struct {
	products []model.Product
	vehicles []model.Vehicle
}

func NewResolver() *Resolver {
	strptr := func(s string) *string {
		return &s
	}
	intptr := func(i int) *int {
		return &i
	}

	products := []model.Product{
		&model.Furniture{
			Upc:   "1",
			Sku:   "TABLE1",
			Name:  strptr("Table"),
			Price: strptr("899"),
			Brand: &model.Ikea{
				Asile: intptr(10),
			},
			Metadata: []model.MetadataOrError{
				&model.KeyValue{
					Key:   "Condition",
					Value: "excellent",
				},
			},
		},
		&model.Furniture{
			Upc:   "2",
			Sku:   "COUCH1",
			Name:  strptr("Couch"),
			Price: strptr("1299"),
			Brand: &model.Amazon{
				Referrer: strptr("https://canopy.co"),
			},
			Metadata: []model.MetadataOrError{
				&model.KeyValue{
					Key:   "Condition",
					Value: "used",
				},
			},
		},
		&model.Furniture{
			Upc:   "3",
			Sku:   "CHAIR1",
			Name:  strptr("Chair"),
			Price: strptr("54"),
			Brand: &model.Ikea{
				Asile: intptr(10),
			},
			Metadata: []model.MetadataOrError{
				&model.KeyValue{
					Key:   "Condition",
					Value: "like new",
				},
			},
		},
		&model.Book{
			Isbn:  "0262510871",
			Price: strptr("39"),
		},
		&model.Book{
			Isbn:  "0136291554",
			Price: strptr("29"),
		},
		&model.Book{
			Isbn:  "0201633612",
			Price: strptr("49"),
		},
		&model.Book{
			Isbn:  "1234567890",
			Price: strptr("59"),
		},
		&model.Book{
			Isbn:  "404404404",
			Price: strptr("0"),
		},
		&model.Book{
			Isbn:  "0987654321",
			Price: strptr("29"),
		},
	}
	vehicles := []model.Vehicle{
		&model.Car{
			ID:          "1",
			Description: strptr("Humble Toyota"),
			Price:       strptr("9990"),
		},
		&model.Car{
			ID:          "2",
			Description: strptr("Awesome Tesla"),
			Price:       strptr("12990"),
		},
		&model.Van{
			ID:          "3",
			Description: strptr("Just a van..."),
			Price:       strptr("15990"),
		},
	}

	return &Resolver{
		products: products,
		vehicles: vehicles,
	}
}
