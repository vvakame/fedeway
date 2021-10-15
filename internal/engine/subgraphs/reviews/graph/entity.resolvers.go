package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.

import (
	"context"
	"errors"

	"github.com/vvakame/fedeway/internal/engine/subgraphs/reviews/graph/generated"
	"github.com/vvakame/fedeway/internal/engine/subgraphs/reviews/graph/model"
)

func (r *entityResolver) FindBookByIsbn(ctx context.Context, isbn string) (*model.Book, error) {
	return nil, errors.New("FindBookByIsbn is not implemented")
}

func (r *entityResolver) FindCarByID(ctx context.Context, id string) (*model.Car, error) {
	return nil, errors.New("FindCarByID is not implemented")
}

func (r *entityResolver) FindFurnitureByUpc(ctx context.Context, upc string) (*model.Furniture, error) {
	return nil, errors.New("FindFurnitureByUpc is not implemented")
}

func (r *entityResolver) FindReviewByID(ctx context.Context, id string) (*model.Review, error) {
	for _, review := range r.reviews {
		if review.ID == id {
			return review, nil
		}
	}

	return nil, nil
}

func (r *entityResolver) FindUserByID(ctx context.Context, id string) (*model.User, error) {
	return nil, errors.New("FindUserByID is not implemented")
}

func (r *entityResolver) FindVanByID(ctx context.Context, id string) (*model.Van, error) {
	return nil, errors.New("FindVanByID is not implemented")
}

// Entity returns generated.EntityResolver implementation.
func (r *Resolver) Entity() generated.EntityResolver { return &entityResolver{r} }

type entityResolver struct{ *Resolver }
