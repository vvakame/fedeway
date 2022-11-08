package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.

import (
	"context"

	"github.com/vvakame/fedeway/internal/engine/subgraphs/books/graph/generated"
	"github.com/vvakame/fedeway/internal/engine/subgraphs/books/graph/model"
)

// FindBookByIsbn is the resolver for the findBookByIsbn field.
func (r *entityResolver) FindBookByIsbn(ctx context.Context, isbn string) (*model.Book, error) {
	for _, book := range r.books {
		if book.Isbn == isbn {
			return book, nil
		}
	}
	return nil, nil
}

// FindLibraryByID is the resolver for the findLibraryByID field.
func (r *entityResolver) FindLibraryByID(ctx context.Context, id string) (*model.Library, error) {
	for _, library := range r.libraries {
		if library.ID == id {
			return library, nil
		}
	}
	return nil, nil
}

// Entity returns generated.EntityResolver implementation.
func (r *Resolver) Entity() generated.EntityResolver { return &entityResolver{r} }

type entityResolver struct{ *Resolver }
