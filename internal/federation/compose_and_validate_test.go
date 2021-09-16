package federation

import (
	"context"
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
	t.SkipNow()

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

			serviceDefs = append(serviceDefs, &ServiceDefinition{
				TypeDefs: schemaDoc,
				Name:     name,
			})
		}
		sort.SliceStable(serviceDefs, func(i, j int) bool {
			return serviceDefs[i].Name < serviceDefs[j].Name
		})

		if len(serviceDefs) == 0 {
			t.Logf("%s doesn't have testing assets", dirPath)
			continue
		}

		t.Run(dir.Name(), func(t *testing.T) {
			ctx := context.Background()
			ctx = log.WithLogger(ctx, testlogr.NewTestLogger(t))

			schema, supergraphSDL, err := ComposeAndValidate(ctx, serviceDefs)
			if err != nil {
				t.Fatal(err)
			}
			_ = schema

			testutils.CheckGoldenFile(t, []byte(supergraphSDL), path.Join(expectFileDir, dir.Name()+".graphqls"))
		})
	}
}
