package accounts

import (
	"context"

	"github.com/99designs/gqlgen/graphql"
	"github.com/vvakame/fedeway/internal/engine/subgraphs/accounts/graph"
	"github.com/vvakame/fedeway/internal/engine/subgraphs/accounts/graph/generated"
	"github.com/vvakame/fedeway/internal/engine/subgraphs/accounts/graph/model"
)

type Executable struct {
	executableSchema graphql.ExecutableSchema
}

func (exec *Executable) ExecutableSchema() graphql.ExecutableSchema {
	return exec.executableSchema
}

func (exec *Executable) Name() string {
	return "accounts"
}

func (exec *Executable) URL() string {
	return ""
}

func NewExecutableSchema() *Executable {
	es := generated.NewExecutableSchema(generated.Config{
		Resolvers: graph.NewResolver(),
		Directives: generated.DirectiveRoot{
			CacheControl: func(ctx context.Context, obj interface{}, next graphql.Resolver, maxAge *int, scope *model.CacheControlScope, inheritMaxAge *bool) (res interface{}, err error) {
				return next(ctx)
			},
			Stream: func(ctx context.Context, obj interface{}, next graphql.Resolver) (res interface{}, err error) {
				return next(ctx)
			},
			Tag: func(ctx context.Context, obj interface{}, next graphql.Resolver, name string) (res interface{}, err error) {
				return next(ctx)
			},
			Transform: func(ctx context.Context, obj interface{}, next graphql.Resolver, from string) (res interface{}, err error) {
				return next(ctx)
			},
		},
		Complexity: generated.ComplexityRoot{},
	})

	return &Executable{
		executableSchema: es,
	}
}
