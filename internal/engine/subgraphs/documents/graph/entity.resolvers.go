package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.

import (
	"context"
	"errors"

	"github.com/vvakame/fedeway/internal/engine/subgraphs/documents/graph/generated"
	"github.com/vvakame/fedeway/internal/engine/subgraphs/documents/graph/model"
)

// FindNoopByNoop is the resolver for the findNoopByNoop field.
func (r *entityResolver) FindNoopByNoop(ctx context.Context, noop *string) (*model.Noop, error) {
	return nil, errors.New("not implemented")
}

// Entity returns generated.EntityResolver implementation.
func (r *Resolver) Entity() generated.EntityResolver { return &entityResolver{r} }

type entityResolver struct{ *Resolver }
