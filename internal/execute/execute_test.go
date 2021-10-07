package execute

import (
	"context"
	"encoding/json"
	testlogr "github.com/go-logr/logr/testing"
	"github.com/vvakame/fedeway/internal/log"
	"github.com/vvakame/fedeway/internal/testutils"
	"io/ioutil"
	"path"
	"strings"
	"testing"

	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/parser"
	"github.com/vektah/gqlparser/v2/validator"
)

func TestExecute(t *testing.T) {
	const testFileDir = "./_testdata/assets"
	const expectFileDir = "./_testdata/expected"

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

			b1, err := ioutil.ReadFile(path.Join(testFileDir, file.Name()))
			if err != nil {
				t.Fatal(err)
			}
			rawQuery := string(b1)

			operationName := testutils.FindOptionString(t, "operationName", string(b1))

			document, gErr := parser.ParseQuery(&ast.Source{
				Name:  file.Name(),
				Input: rawQuery,
			})
			if gErr != nil {
				t.Fatal(gErr)
			}

			schemaFile := testutils.FindSchemaFileName(t, string(b1))
			b2, err := ioutil.ReadFile(path.Join(testFileDir, schemaFile))
			if err != nil {
				t.Fatal(err)
			}

			schemaDoc, gErr := parser.ParseSchemas(
				validator.Prelude,
				&ast.Source{
					Name:  file.Name(),
					Input: string(b2),
				},
			)
			if gErr != nil {
				t.Fatal(gErr)
			}

			schema, gErr := validator.ValidateSchemaDocument(schemaDoc)
			if gErr != nil {
				t.Fatal(gErr)
			}

			gErrs := validator.Validate(schema, document)
			if len(gErrs) != 0 {
				t.Fatal(gErrs)
			}

			dataFile := testutils.FindOptionString(t, "data", string(b1))
			b3, err := ioutil.ReadFile(path.Join(testFileDir, dataFile))
			if err != nil {
				t.Fatal(err)
			}

			data := make(map[string]interface{})
			err = json.Unmarshal(b3, &data)
			if err != nil {
				t.Fatal(err)
			}

			variablesFile := testutils.FindOptionString(t, "variables", string(b1))
			variables := map[string]interface{}{}
			if variablesFile != "" {
				b4, err := ioutil.ReadFile(path.Join(testFileDir, variablesFile))
				if err != nil {
					t.Fatal(err)
				}
				err = json.Unmarshal(b4, &variables)
				if err != nil {
					t.Fatal(err)
				}
			}

			t.Logf("schema: %s, operation: %s, operationName: %s, dataFile: %s, variableFile: %s", schemaFile, file.Name(), operationName, dataFile, variablesFile)

			response := Execute(ctx, &ExecutionArgs{
				Schema:         schema,
				RawQuery:       rawQuery,
				Document:       document,
				RootValue:      data,
				VariableValues: variables,
				OperationName:  operationName,
				FieldResolver:  defaultFieldResolver,
				TypeResolver:   defaultTypeResolver,
			})

			responseBytes, err := json.MarshalIndent(response, "", "  ")
			if err != nil {
				t.Fatal(err)
			}

			fileName := file.Name()[:len(file.Name())-len(".graphqls")]

			testutils.CheckGoldenFile(t, responseBytes, path.Join(expectFileDir, fileName+".response.json"))
		})
	}
}
