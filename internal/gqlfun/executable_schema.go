package gqlfun

import (
	"context"

	"github.com/99designs/gqlgen/graphql"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"
	"github.com/vektah/gqlparser/v2/parser"
	"github.com/vektah/gqlparser/v2/validator"
)

func CreateOperationContext(ctx context.Context, schema *ast.Schema, query string, vairables map[string]interface{}) (*graphql.OperationContext, gqlerror.List) {
	queryDoc, gErr := parser.ParseQuery(&ast.Source{
		Input:   query,
		BuiltIn: false,
	})
	if gErr != nil {
		return nil, gqlerror.List{gErr}
	}
	gErrs := validator.Validate(schema, queryDoc)
	if len(gErrs) != 0 {
		return nil, gErrs
	}

	oc := &graphql.OperationContext{
		RawQuery:             query,
		Variables:            vairables,
		OperationName:        "",
		Doc:                  queryDoc,
		Operation:            queryDoc.Operations[0],
		DisableIntrospection: false,
		RecoverFunc:          nil,
		ResolverMiddleware: func(ctx context.Context, next graphql.Resolver) (res interface{}, err error) {
			return next(ctx)
		},
		Stats: graphql.Stats{},
	}

	return oc, nil
}

func Execute(ctx context.Context, es graphql.ExecutableSchema, query string, vairables map[string]interface{}) (*graphql.Response, gqlerror.List) {
	oc, gErrs := CreateOperationContext(ctx, es.Schema(), query, vairables)
	if len(gErrs) != 0 {
		return nil, gErrs
	}
	ctx = graphql.WithOperationContext(ctx, oc)

	rh := es.Exec(ctx)
	resp := rh(ctx)
	if len(resp.Errors) != 0 {
		return resp, gErrs
	}

	return resp, nil
}
