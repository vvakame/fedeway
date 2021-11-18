package federation

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"path"
	"sort"
	"strings"
	"testing"

	testlogr "github.com/go-logr/logr/testing"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/parser"
	"github.com/vvakame/fedeway/internal/log"
	"github.com/vvakame/fedeway/internal/testutils"
)

func TestComposeAndValidate(t *testing.T) {
	// test case are ported from federation-js/src/composition/__tests__/composeAndValidate.test.ts
	// TODO porting all of the tests
	// TODO remove option:skip: true from test cases

	const testFileDir = "./_testdata/composeAndValidate/assets"
	expectFileDir := "./_testdata/composeAndValidate/expected"

	dirs, err := ioutil.ReadDir(testFileDir)
	if err != nil {
		t.Fatal(err)
	}

	for _, dir := range dirs {
		if !dir.IsDir() {
			continue
		}

		t.Run(dir.Name(), func(t *testing.T) {
			dirPath := path.Join(testFileDir, dir.Name())
			files, err := ioutil.ReadDir(dirPath)
			if err != nil {
				t.Fatal(err)
			}

			var serviceDefs []*ServiceDefinition
			for _, file := range files {
				if file.IsDir() {
					continue
				} else if !strings.HasSuffix(file.Name(), ".graphqls") {
					continue
				}

				filePath := path.Join(testFileDir, dir.Name(), file.Name())
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
				urlValue := testutils.FindOptionString(t, "url", string(b))

				schemaDoc, gErr := parser.ParseSchema(&ast.Source{
					Name:  file.Name(),
					Input: string(b),
				})
				if gErr != nil {
					t.Fatal(gErr)
				}

				serviceDefs = append(serviceDefs, &ServiceDefinition{
					TypeDefs: schemaDoc,
					Name:     name,
					URL:      urlValue,
				})
			}
			sort.SliceStable(serviceDefs, func(i, j int) bool {
				return serviceDefs[i].Name < serviceDefs[j].Name
			})

			if len(serviceDefs) == 0 {
				t.Logf("%s doesn't have testing assets", dirPath)
				t.SkipNow()
			}

			ctx := context.Background()
			ctx = log.WithLogger(ctx, testlogr.NewTestLogger(t))

			schema, supergraphSDL, _, err := ComposeAndValidate(ctx, serviceDefs)
			if err != nil {
				b, err := json.MarshalIndent(err, "", "  ")
				if err != nil {
					t.Fatal(err)
				}

				testutils.CheckGoldenFile(t, b, path.Join(expectFileDir, dir.Name()+".error.json"))
				return
			}
			_ = schema

			testutils.CheckGoldenFile(t, []byte(supergraphSDL), path.Join(expectFileDir, dir.Name()+".graphqls"))
		})
	}
}
