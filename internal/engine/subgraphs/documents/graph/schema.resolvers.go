package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.

import (
	"context"
	"fmt"

	generated1 "github.com/vvakame/fedeway/internal/engine/subgraphs/documents/graph/generated"
	model1 "github.com/vvakame/fedeway/internal/engine/subgraphs/documents/graph/model"
)

func (r *queryResolver) Body(ctx context.Context) (model1.Body, error) {
	panic(fmt.Errorf("not implemented"))
}

// Query returns generated1.QueryResolver implementation.
func (r *Resolver) Query() generated1.QueryResolver { return &queryResolver{r} }

type queryResolver struct{ *Resolver }
