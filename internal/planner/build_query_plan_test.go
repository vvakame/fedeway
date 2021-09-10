package planner

import (
	"bytes"
	"context"
	"io/ioutil"
	"path"
	"regexp"
	"strings"
	"testing"

	testlogr "github.com/go-logr/logr/testing"
	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/parser"
	"github.com/vvakame/fedeway/internal/log"
	"github.com/vvakame/fedeway/internal/plan"
)

func TestBuildQueryPlan(t *testing.T) {
	const testFileDir = "./testdata/buildQueryPlan/assets"
	const expectFileDir = "./testdata/buildQueryPlan/expected"

	files, err := ioutil.ReadDir(testFileDir)
	if err != nil {
		t.Fatal(err)
	}

	re, err := regexp.Compile("^# schema:\\s*([^\\s]+)")
	if err != nil {
		t.Fatal(err)
	}

	var preludeSource string
	{
		b, err := ioutil.ReadFile(path.Join(testFileDir, "prelude.graphqls"))
		if err != nil {
			t.Fatal(err)
		}
		preludeSource = string(b)
	}

	prelude := &ast.Source{
		Name:    "prelude.graphql",
		Input:   preludeSource,
		BuiltIn: true,
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

			b1, err := ioutil.ReadFile(path.Join(testFileDir, file.Name()))
			if err != nil {
				t.Fatal(err)
			}

			var schemaFile string
			{
				ss := re.FindStringSubmatch(string(b1))
				if len(ss) != 2 {
					t.Fatal("schema file directive mismatch")
				}
				schemaFile = ss[1]
			}

			b2, err := ioutil.ReadFile(path.Join(testFileDir, schemaFile))
			if err != nil {
				t.Fatal(err)
			}

			schemaDoc, gErr := parser.ParseSchemas(
				prelude,
				&ast.Source{
					Name:  file.Name(),
					Input: string(b2),
				},
			)
			if gErr != nil {
				t.Fatal(gErr)
			}

			schema, mh, err := buildComposedSchema(ctx, schemaDoc)
			if err != nil {
				t.Fatal(err)
			}

			query, gErrs := gqlparser.LoadQuery(schema, string(b1))
			if gErrs != nil {
				t.Fatal(gErrs)
			}

			if len(query.Operations) == 0 {
				t.Fatal("operation length is 0")
			}

			t.Logf("schema: %s, operation: %s", schemaFile, file.Name())

			opctx, err := buildOperationContext(ctx, schema, mh, query, "")
			if err != nil {
				t.Fatal(err)
			}

			qp, err := BuildQueryPlan(ctx, opctx)
			if err != nil {
				t.Fatal(err)
			}

			var buf bytes.Buffer
			plan.NewFormatter(&buf).FormatQueryPlan(qp)

			checkGoldenFile(t, buf.Bytes(), path.Join(expectFileDir, file.Name()+".txt"))
		})
	}
}
