package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.
// Code generated by github.com/99designs/gqlgen version v0.17.36

import (
	"context"

	"github.com/vvakame/fedeway/internal/engine/subgraphs/inventory/graph/generated"
	"github.com/vvakame/fedeway/internal/engine/subgraphs/inventory/graph/model"
)

// FindBookByIsbn is the resolver for the findBookByIsbn field.
func (r *entityResolver) FindBookByIsbn(ctx context.Context, isbn string) (*model.Book, error) {
	for _, product := range r.inventory {
		book, ok := product.(*model.Book)
		if !ok {
			continue
		}
		if book.Isbn == isbn {
			return book, nil
		}
	}
	return nil, nil
}

// FindFurnitureBySku is the resolver for the findFurnitureBySku field.
func (r *entityResolver) FindFurnitureBySku(ctx context.Context, sku string) (*model.Furniture, error) {
	for _, product := range r.inventory {
		furniture, ok := product.(*model.Furniture)
		if !ok {
			continue
		}
		if furniture.Sku == sku {
			return furniture, nil
		}
	}
	return nil, nil
}

// FindUserByID is the resolver for the findUserByID field.
func (r *entityResolver) FindUserByID(ctx context.Context, id string) (*model.User, error) {
	return &model.User{ID: id}, nil
}

// Entity returns generated.EntityResolver implementation.
func (r *Resolver) Entity() generated.EntityResolver { return &entityResolver{r} }

type entityResolver struct{ *Resolver }
