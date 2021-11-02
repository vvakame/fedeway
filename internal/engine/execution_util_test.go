package engine

import (
	"context"
	"testing"

	"github.com/99designs/gqlgen/graphql"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/parser"
	"github.com/vektah/gqlparser/v2/validator"
	"github.com/vvakame/fedeway/internal/engine/subgraphs/accounts"
	"github.com/vvakame/fedeway/internal/engine/subgraphs/books"
	"github.com/vvakame/fedeway/internal/engine/subgraphs/documents"
	"github.com/vvakame/fedeway/internal/engine/subgraphs/inventory"
	"github.com/vvakame/fedeway/internal/engine/subgraphs/product"
	"github.com/vvakame/fedeway/internal/engine/subgraphs/reviews"
	"github.com/vvakame/fedeway/internal/federation"
	"github.com/vvakame/fedeway/internal/planner"
)

type ServiceDefinitionModule interface {
	Name() string
	URL() string // optional
	ExecutableSchema() graphql.ExecutableSchema
}

func getFederatedTestingSchema(ctx context.Context, t *testing.T) (*planner.ComposedSchema, ServiceMap) {
	fixtures := []ServiceDefinitionModule{
		accounts.NewExecutableSchema(),
		books.NewExecutableSchema(),
		documents.NewExecutableSchema(),
		inventory.NewExecutableSchema(),
		product.NewExecutableSchema(),
		reviews.NewExecutableSchema(),
	}

	serviceMap := make(ServiceMap)
	services := make([]*federation.ServiceDefinition, 0, len(fixtures))
	for _, fixture := range fixtures {
		lds := &LocalDataSource{ExecutableSchema: fixture.ExecutableSchema()}
		sdl, gErrs := lds.SDL(ctx)
		if len(gErrs) != 0 {
			t.Fatal(fixture.Name(), gErrs)
		}

		schemaDoc, gErr := parser.ParseSchema(&ast.Source{
			Input: sdl,
		})
		if gErr != nil {
			t.Fatal(gErr)
		}

		services = append(services, &federation.ServiceDefinition{
			TypeDefs: schemaDoc,
			Name:     fixture.Name(),
			URL:      fixture.URL(),
		})
		serviceMap[fixture.Name()] = lds
	}

	_, sdl, metadata, err := federation.ComposeAndValidate(ctx, services)
	if err != nil {
		t.Fatal(err)
	}

	schemaDoc, gErr := parser.ParseSchemas(
		validator.Prelude,
		&ast.Source{
			Input:   sdl,
			BuiltIn: false,
		},
	)
	if gErr != nil {
		t.Fatal(gErr)
	}

	cs, err := planner.BuildComposedSchema(ctx, schemaDoc, metadata)
	if err != nil {
		t.Fatal(err)
	}

	// TODO js版だとQueryPlannerを返している

	return cs, serviceMap
}
