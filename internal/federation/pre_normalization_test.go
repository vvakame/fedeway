package federation

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"path"
	"strings"
	"testing"

	testlogr "github.com/go-logr/logr/testing"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/parser"
	"github.com/vvakame/fedeway/internal/log"
	"github.com/vvakame/fedeway/internal/testutils"
)

func TestRootFieldUsed(t *testing.T) {
	// test case are ported from federation-js/src/composition/validate/preNormalization/__tests__/rootFieldUsed.test.ts
	// TODO porting all of the tests
	// TODO remove option:skip: true from test cases

	const testFileDir = "./_testdata/validate/rootFieldUsed/assets"
	expectFileDir := "./_testdata/validate/rootFieldUsed/expected"

	files, err := ioutil.ReadDir(testFileDir)
	if err != nil {
		t.Fatal(err)
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		} else if !strings.HasSuffix(file.Name(), ".graphqls") {
			continue
		}

		t.Run(file.Name(), func(t *testing.T) {
			filePath := path.Join(testFileDir, file.Name())
			b, err := ioutil.ReadFile(filePath)
			if err != nil {
				t.Fatal(err)
			}

			if testutils.FindOptionBool(t, "skip", string(b)) {
				t.Logf("test case skip by %s", filePath)
				t.SkipNow()
			}

			name := testutils.FindOptionString(t, "name", string(b))
			if name == "" {
				t.Fatalf("option:name is not exists on %s", filePath)
			}

			schemaDoc, gErr := parser.ParseSchema(&ast.Source{
				Name:  file.Name(),
				Input: string(b),
			})
			if gErr != nil {
				t.Fatal(gErr)
			}

			serviceDef := &ServiceDefinition{
				TypeDefs: schemaDoc,
				Name:     name,
			}

			ctx := context.Background()
			ctx = log.WithLogger(ctx, testlogr.NewTestLogger(t))

			errs := rootFieldUsed(serviceDef)
			if errs == nil {
				// for pretty print
				errs = []error{}
			}

			b, err = json.MarshalIndent(errs, "", "  ")
			if err != nil {
				t.Fatal(err)
			}

			fileName := file.Name()[:len(file.Name())-len(".graphqls")]

			testutils.CheckGoldenFile(t, b, path.Join(expectFileDir, fileName+".error.json"))
		})
	}
}
