package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"path"
	"strings"
	"testing"

	"github.com/99designs/gqlgen/graphql"
	testlogr "github.com/go-logr/logr/testing"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/parser"
	"github.com/vektah/gqlparser/v2/validator"
	"github.com/vvakame/fedeway/internal/log"
	planpkg "github.com/vvakame/fedeway/internal/plan"
	"github.com/vvakame/fedeway/internal/planner"
	"github.com/vvakame/fedeway/internal/testutils"
)

func TestExecuteQueryPlan(t *testing.T) {
	const testFileDir = "./_testdata/executeQueryPlan/assets"
	expectFileDir := "./_testdata/executeQueryPlan/expected"

	files, err := ioutil.ReadDir(testFileDir)
	if err != nil {
		t.Fatal(err)
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}
		if !strings.HasSuffix(file.Name(), ".graphql") {
			continue
		}

		t.Run(file.Name(), func(t *testing.T) {
			ctx := context.Background()
			ctx = log.WithLogger(ctx, testlogr.NewTestLogger(t))

			composedSchema, serviceMap := getFederatedTestingSchema(ctx, t)

			b, err := ioutil.ReadFile(path.Join(testFileDir, file.Name()))
			if err != nil {
				t.Fatal(err)
			}

			queryDoc, gErr := parser.ParseQuery(&ast.Source{
				Input: string(b),
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

			var buf bytes.Buffer
			planpkg.NewFormatter(&buf).FormatQueryPlan(plan)
			t.Log(buf.String())

			oc := &graphql.OperationContext{
				RawQuery:             string(b),
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

			responseBytes, err := json.MarshalIndent(resp, "", "  ")
			if err != nil {
				t.Fatal(err)
			}

			fileName := file.Name()[:len(file.Name())-len(".graphql")]

			testutils.CheckGoldenFile(t, responseBytes, path.Join(expectFileDir, fileName+".response.json"))

			buf.Reset()
			planpkg.NewFormatter(&buf).FormatQueryPlan(plan)

			testutils.CheckGoldenFile(t, buf.Bytes(), path.Join(expectFileDir, fileName+".plan.txt"))
		})
	}
}
