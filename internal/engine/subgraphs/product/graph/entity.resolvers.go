package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.

import (
	"context"
	"errors"

	"github.com/vvakame/fedeway/internal/engine/subgraphs/product/graph/generated"
	"github.com/vvakame/fedeway/internal/engine/subgraphs/product/graph/model"
)

func (r *entityResolver) FindBookByIsbn(ctx context.Context, isbn string) (*model.Book, error) {
	for _, product := range r.products {
		switch product := product.(type) {
		case *model.Book:
			if product.Isbn == isbn {
				return product, nil
			}
		}
	}

	return nil, nil
}

func (r *entityResolver) FindCarByID(ctx context.Context, id string) (*model.Car, error) {
	for _, vehicle := range r.vehicles {
		switch vehicle := vehicle.(type) {
		case *model.Car:
			if vehicle.ID == id {
				return vehicle, nil
			}
		}
	}

	return nil, nil
}

func (r *entityResolver) FindFurnitureByUpc(ctx context.Context, upc string) (*model.Furniture, error) {
	for _, product := range r.products {
		switch product := product.(type) {
		case *model.Furniture:
			if product.Upc == upc {
				return product, nil
			}
		}
	}

	return nil, nil
}

func (r *entityResolver) FindUserByID(ctx context.Context, id string) (*model.User, error) {
	return nil, errors.New("FindUserByID is not implemented")
}

func (r *entityResolver) FindVanByID(ctx context.Context, id string) (*model.Van, error) {
	for _, vehicle := range r.vehicles {
		switch vehicle := vehicle.(type) {
		case *model.Van:
			if vehicle.ID == id {
				return vehicle, nil
			}
		}
	}

	return nil, nil
}

// Entity returns generated.EntityResolver implementation.
func (r *Resolver) Entity() generated.EntityResolver { return &entityResolver{r} }

type entityResolver struct{ *Resolver }
