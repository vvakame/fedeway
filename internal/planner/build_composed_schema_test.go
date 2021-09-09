package planner

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"path"
	"strings"
	"testing"

	testlogr "github.com/go-logr/logr/testing"
	"github.com/goccy/go-yaml"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/formatter"
	"github.com/vektah/gqlparser/v2/parser"
	"github.com/vektah/gqlparser/v2/validator"
	"github.com/vvakame/fedeway/internal/log"
)

func TestBuildComposedSchema(t *testing.T) {
	const testFileDir = "./testdata/buildComposedSchema/assets"
	expectFileDir := "./testdata/buildComposedSchema/expected"

	files, err := ioutil.ReadDir(testFileDir)
	if err != nil {
		t.Fatal(err)
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}
		if !strings.HasSuffix(file.Name(), ".graphqls") {
			continue
		}

		t.Run(file.Name(), func(t *testing.T) {
			ctx := context.Background()
			ctx = log.WithLogger(ctx, testlogr.NewTestLogger(t))

			b, err := ioutil.ReadFile(path.Join(testFileDir, file.Name()))
			if err != nil {
				t.Fatal(err)
			}

			schemaDoc, gErr := parser.ParseSchemas(
				validator.Prelude,
				&ast.Source{
					Name:  file.Name(),
					Input: string(b),
				},
			)
			if gErr != nil {
				t.Fatal(gErr)
			}

			schema, mh, err := buildComposedSchema(ctx, schemaDoc)
			if err != nil {
				t.Fatal(err)
			}

			fileName := file.Name()[:len(file.Name())-len(".graphqls")]

			var buf bytes.Buffer
			formatter.NewFormatter(&buf).FormatSchema(schema)
			checkGoldenFile(t, buf.Bytes(), path.Join(expectFileDir, fileName+".composed.graphqls"))

			b, err = yaml.Marshal(mh)
			if err != nil {
				t.Fatal(err)
			}
			checkGoldenFile(t, b, path.Join(expectFileDir, fileName+".metadata.yaml"))

			b, err = json.MarshalIndent(mh, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			checkGoldenFile(t, b, path.Join(expectFileDir, fileName+".metadata.json"))
		})
	}
}
