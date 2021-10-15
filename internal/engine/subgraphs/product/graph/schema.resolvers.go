package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.

import (
	"context"
	"fmt"

	"github.com/vvakame/fedeway/internal/engine/subgraphs/product/graph/generated"
	"github.com/vvakame/fedeway/internal/engine/subgraphs/product/graph/model"
)

func (r *bookResolver) Upc(ctx context.Context, obj *model.Book) (string, error) {
	return obj.Isbn, nil
}

func (r *bookResolver) Sku(ctx context.Context, obj *model.Book) (string, error) {
	return obj.Isbn, nil
}

func (r *bookResolver) Name(ctx context.Context, obj *model.Book, delimeter *string) (*string, error) {
	s := fmt.Sprintf("%s%s(%d)", *obj.Title, *delimeter, *obj.Year)
	return &s, nil
}

func (r *queryResolver) Product(ctx context.Context, upc string) (model.Product, error) {
	for _, product := range r.products {
		switch product := product.(type) {
		case *model.Book:
			if product.Upc == upc {
				return product, nil
			}
		case *model.Furniture:
			if product.Upc == upc {
				return product, nil
			}
		default:
			return nil, fmt.Errorf("unknown type: %T", product)
		}
	}

	return nil, nil
}

func (r *queryResolver) Vehicle(ctx context.Context, id string) (model.Vehicle, error) {
	for _, vehicle := range r.vehicles {
		switch vehicle := vehicle.(type) {
		case *model.Car:
			if vehicle.ID == id {
				return vehicle, nil
			}
		case *model.Van:
			if vehicle.ID == id {
				return vehicle, nil
			}
		default:
			return nil, fmt.Errorf("unknown type: %T", vehicle)
		}
	}

	return nil, nil
}

func (r *queryResolver) TopProducts(ctx context.Context, first *int) ([]model.Product, error) {
	if first == nil {
		return r.products, nil
	}

	result := make([]model.Product, 0, *first)
	for i := 0; i < *first && i < len(r.products); i++ {
		result = append(result, r.products[i])
	}

	return result, nil
}

func (r *queryResolver) TopCars(ctx context.Context, first *int) ([]*model.Car, error) {
	var cars []*model.Car

	for _, vehicle := range r.vehicles {
		switch vehicle := vehicle.(type) {
		case *model.Car:
			cars = append(cars, vehicle)
		}
	}

	if first == nil {
		return cars, nil
	}

	result := make([]*model.Car, 0, *first)
	for i := 0; i < *first && i < len(cars); i++ {
		result = append(result, cars[i])
	}

	return result, nil
}

func (r *userResolver) Vehicle(ctx context.Context, obj *model.User) (model.Vehicle, error) {
	for _, vehicle := range r.vehicles {
		switch vehicle := vehicle.(type) {
		case *model.Car:
			if vehicle.ID == obj.ID {
				return vehicle, nil
			}
		case *model.Van:
			if vehicle.ID == obj.ID {
				return vehicle, nil
			}
		default:
			return nil, fmt.Errorf("unknown type: %T", vehicle)
		}
	}

	return nil, nil
}

func (r *userResolver) Thing(ctx context.Context, obj *model.User) (model.Thing, error) {
	for _, vehicle := range r.vehicles {
		switch vehicle := vehicle.(type) {
		case *model.Car:
			if vehicle.ID == obj.ID {
				return vehicle, nil
			}
		case *model.Van:
			continue
		default:
			return nil, fmt.Errorf("unknown type: %T", vehicle)
		}
	}

	return nil, nil
}

// Book returns generated.BookResolver implementation.
func (r *Resolver) Book() generated.BookResolver { return &bookResolver{r} }

// Query returns generated.QueryResolver implementation.
func (r *Resolver) Query() generated.QueryResolver { return &queryResolver{r} }

// User returns generated.UserResolver implementation.
func (r *Resolver) User() generated.UserResolver { return &userResolver{r} }

type bookResolver struct{ *Resolver }
type queryResolver struct{ *Resolver }
type userResolver struct{ *Resolver }
