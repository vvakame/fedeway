package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.

import (
	"context"

	"github.com/vvakame/fedeway/internal/engine/subgraphs/books/graph/generated"
	"github.com/vvakame/fedeway/internal/engine/subgraphs/books/graph/model"
)

func (r *queryResolver) Book(ctx context.Context, isbn string) (*model.Book, error) {
	for _, book := range r.books {
		if book.Isbn == isbn {
			return book, nil
		}
	}

	return nil, nil
}

func (r *queryResolver) Books(ctx context.Context) ([]*model.Book, error) {
	return r.books, nil
}

func (r *queryResolver) Library(ctx context.Context, id string) (*model.Library, error) {
	for _, library := range r.libraries {
		if library.ID == id {
			return library, nil
		}
	}

	return nil, nil
}

// Query returns generated.QueryResolver implementation.
func (r *Resolver) Query() generated.QueryResolver { return &queryResolver{r} }

type queryResolver struct{ *Resolver }