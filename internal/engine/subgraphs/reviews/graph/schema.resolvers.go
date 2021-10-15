package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.

import (
	"context"
	"errors"
	"strconv"

	"github.com/vvakame/fedeway/internal/engine/subgraphs/reviews/graph/generated"
	"github.com/vvakame/fedeway/internal/engine/subgraphs/reviews/graph/model"
)

func (r *bookResolver) Reviews(ctx context.Context, obj *model.Book) ([]*model.Review, error) {
	var result []*model.Review
	for _, review := range r.reviews {
		switch product := review.Product.(type) {
		case *model.Book:
			if product.Isbn == obj.Isbn {
				result = append(result, review)
			}
		case *model.Furniture:
			continue
		}
	}

	return result, nil
}

func (r *bookResolver) RelatedReviews(ctx context.Context, obj *model.Book) ([]*model.Review, error) {
	var result []*model.Review
	for _, similarBool := range obj.SimilarBooks {
		for _, review := range r.reviews {
			switch product := review.Product.(type) {
			case *model.Book:
				if product.Isbn == similarBool.Isbn {
					result = append(result, review)
				}
			case *model.Furniture:
				continue
			}
		}
	}

	return result, nil
}

func (r *carResolver) RetailPrice(ctx context.Context, obj *model.Car) (*string, error) {
	return obj.Price, nil
}

func (r *furnitureResolver) Reviews(ctx context.Context, obj *model.Furniture) ([]*model.Review, error) {
	var result []*model.Review
	for _, review := range r.reviews {
		switch product := review.Product.(type) {
		case *model.Book:
			continue
		case *model.Furniture:
			if product.Upc == obj.Upc {
				result = append(result, review)
			}
		}
	}

	return result, nil
}

func (r *mutationResolver) ReviewProduct(ctx context.Context, input model.ReviewProduct) (model.Product, error) {
	latestID := r.reviews[len(r.reviews)-1].ID
	i, err := strconv.ParseInt(latestID, 10, 32)
	if err != nil {
		return nil, err
	}
	newID := strconv.FormatInt(i+1, 10)

	product := &model.Furniture{
		Upc: input.Upc,
	}

	r.reviews = append(r.reviews, &model.Review{
		ID:       newID,
		Body:     &input.Body,
		AuthorID: "1",
		Product:  product,
	})

	return product, nil
}

func (r *mutationResolver) UpdateReview(ctx context.Context, review model.UpdateReviewInput) (*model.Review, error) {
	var target *model.Review
	for _, exist := range r.reviews {
		if exist.ID == review.ID {
			target = exist
		}
	}

	if target == nil {
		return nil, nil
	}

	target.Body = review.Body

	return target, nil
}

func (r *mutationResolver) DeleteReview(ctx context.Context, id string) (*bool, error) {
	newList := make([]*model.Review, 0, len(r.reviews))
	var deleted bool
	for _, review := range r.reviews {
		if review.ID == id {
			deleted = true
			continue
		}
		newList = append(newList, review)
	}

	r.reviews = newList

	return &deleted, nil
}

func (r *queryResolver) TopReviews(ctx context.Context, first *int) ([]*model.Review, error) {
	if first == nil {
		return r.reviews, nil
	}

	result := make([]*model.Review, 0, *first)
	for i := 0; i < *first && i < len(r.reviews); i++ {
		result = append(result, r.reviews[i])
	}

	return result, nil
}

func (r *reviewResolver) Author(ctx context.Context, obj *model.Review) (*model.User, error) {
	return &model.User{
		ID: obj.AuthorID,
	}, nil
}

func (r *userResolver) Username(ctx context.Context, obj *model.User) (*string, error) {
	for _, username := range r.usernames {
		if username.ID == obj.ID {
			return username.Username, nil
		}
	}

	return nil, nil
}

func (r *userResolver) Reviews(ctx context.Context, obj *model.User) ([]*model.Review, error) {
	var result []*model.Review
	for _, review := range r.reviews {
		if review.AuthorID == obj.ID {
			result = append(result, review)
		}
	}

	return result, nil
}

func (r *userResolver) NumberOfReviews(ctx context.Context, obj *model.User) (int, error) {
	var count int
	for _, review := range r.reviews {
		if review.AuthorID == obj.ID {
			count++
		}
	}

	return count, nil
}

func (r *userResolver) GoodAddress(ctx context.Context, obj *model.User) (*bool, error) {
	if len(obj.Metadata) == 0 || obj.Metadata[0].Address == nil {
		return nil, errors.New("unexpected metadata. gqlgen not supported nested @requires")
	}

	b := *obj.Metadata[0].Address == "1"
	return &b, nil
}

func (r *vanResolver) RetailPrice(ctx context.Context, obj *model.Van) (*string, error) {
	return obj.Price, nil
}

// Book returns generated.BookResolver implementation.
func (r *Resolver) Book() generated.BookResolver { return &bookResolver{r} }

// Car returns generated.CarResolver implementation.
func (r *Resolver) Car() generated.CarResolver { return &carResolver{r} }

// Furniture returns generated.FurnitureResolver implementation.
func (r *Resolver) Furniture() generated.FurnitureResolver { return &furnitureResolver{r} }

// Mutation returns generated.MutationResolver implementation.
func (r *Resolver) Mutation() generated.MutationResolver { return &mutationResolver{r} }

// Query returns generated.QueryResolver implementation.
func (r *Resolver) Query() generated.QueryResolver { return &queryResolver{r} }

// Review returns generated.ReviewResolver implementation.
func (r *Resolver) Review() generated.ReviewResolver { return &reviewResolver{r} }

// User returns generated.UserResolver implementation.
func (r *Resolver) User() generated.UserResolver { return &userResolver{r} }

// Van returns generated.VanResolver implementation.
func (r *Resolver) Van() generated.VanResolver { return &vanResolver{r} }

type bookResolver struct{ *Resolver }
type carResolver struct{ *Resolver }
type furnitureResolver struct{ *Resolver }
type mutationResolver struct{ *Resolver }
type queryResolver struct{ *Resolver }
type reviewResolver struct{ *Resolver }
type userResolver struct{ *Resolver }
type vanResolver struct{ *Resolver }
