package engine

import (
	"context"

	"github.com/99designs/gqlgen/graphql"
)

type DataSource interface {
	Process(ctx context.Context, oc *graphql.OperationContext) *graphql.Response
}
