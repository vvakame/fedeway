package federation

import (
	"context"
	"encoding/json"
	"fmt"
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

func TestPreNormalizationValidators(t *testing.T) {
	t.Parallel()

	type Spec struct {
		Name string
		Rule func(*ServiceDefinition) []error
	}

	specs := []*Spec{
		{
			Name: "rootFieldUsed",
			Rule: rootFieldUsed,
		},
		{
			Name: "tagDirective",
			Rule: tagDirectiveRule,
		},
	}

	for _, spec := range specs {
		spec := spec
		t.Run(spec.Name, func(t *testing.T) {
			t.Parallel()

			testFileDir := fmt.Sprintf("./_testdata/validate/%s/assets", spec.Name)
			expectFileDir := fmt.Sprintf("./_testdata/validate/%s/expected", spec.Name)

			rule := spec.Rule

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

					errs := rule(serviceDef)
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
		})
	}
}
