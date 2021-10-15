package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.

import (
	"context"
	"errors"

	"github.com/vvakame/fedeway/internal/engine/subgraphs/inventory/graph/generated"
	"github.com/vvakame/fedeway/internal/engine/subgraphs/inventory/graph/model"
)

func (r *userResolver) GoodDescription(ctx context.Context, obj *model.User) (*bool, error) {
	if len(obj.Metadata) == 0 || obj.Metadata[0].Description == nil {
		return nil, errors.New("unexpected object value")
	}

	b := *obj.Metadata[0].Description == "2"
	return &b, nil
}

// User returns generated.UserResolver implementation.
func (r *Resolver) User() generated.UserResolver { return &userResolver{r} }

type userResolver struct{ *Resolver }
