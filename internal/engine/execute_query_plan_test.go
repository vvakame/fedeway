package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/99designs/gqlgen/graphql"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/formatter"
	"github.com/vektah/gqlparser/v2/parser"
	"github.com/vektah/gqlparser/v2/validator"
	"github.com/vvakame/fedeway/internal/planner"
)

func TestExecuteQueryPlan(t *testing.T) {
	ctx := context.Background()
	composedSchema, serviceMap := getFederatedTestingSchema(ctx, t)
	var buf bytes.Buffer
	formatter.NewFormatter(&buf).FormatSchema(composedSchema.Schema)
	t.Log(buf.String())

	query := `query { me { id vehicle { id } } }`
	queryDoc, gErr := parser.ParseQuery(&ast.Source{
		Input: query,
	})
	if gErr != nil {
		t.Fatal(gErr)
	}
	gErrs := validator.Validate(composedSchema.Schema, queryDoc)
	if len(gErrs) != 0 {
		t.Fatal(gErrs)
	}

	opctx, err := planner.BuildOperationContext(ctx, composedSchema, queryDoc, "")
	if err != nil {
		t.Fatal(err)
	}

	plan, err := planner.BuildQueryPlan(ctx, opctx)
	if err != nil {
		t.Fatal(err)
	}

	oc := &graphql.OperationContext{
		RawQuery:             query,
		Variables:            make(map[string]interface{}),
		Doc:                  queryDoc,
		Operation:            queryDoc.Operations.ForName(""),
		DisableIntrospection: true,
		RecoverFunc:          graphql.DefaultRecover, // TODO configurable
		ResolverMiddleware: func(ctx context.Context, next graphql.Resolver) (res interface{}, err error) {
			return next(ctx)
		},
		Stats: graphql.Stats{}, // TODO
	}

	resp := ExecuteQueryPlan(ctx, plan, serviceMap, composedSchema.Schema, oc)

	if len(resp.Errors) != 0 {
		t.Fatal(resp.Errors)
	}

	t.Log(string(resp.Data))

	extensionsBytes, err := json.MarshalIndent(resp.Extensions, "", "  ")
	if err != nil {
		t.Fatal(err)
	}

	t.Log(string(extensionsBytes))
}
