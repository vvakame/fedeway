package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/99designs/gqlgen/graphql"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"
	"github.com/vektah/gqlparser/v2/parser"
	"github.com/vektah/gqlparser/v2/validator"
	"github.com/vvakame/fedeway/internal/engine"
	"github.com/vvakame/fedeway/internal/federation"
	"github.com/vvakame/fedeway/internal/planner"
)

var _ graphql.ExecutableSchema = (*gatewayImpl)(nil)
var _ engine.DataSource = (DataSource)(nil)

type GatewayConfig struct {
	ServiceDefinitions []*ServiceDefinition
}

type DataSource interface {
	Process(ctx context.Context, oc *graphql.OperationContext) *graphql.Response
}

type ServiceDefinition struct {
	Name       string
	URL        string // optional
	DataSource DataSource
}

type gatewayImpl struct {
	sync.RWMutex

	serviceDefinitions []*ServiceDefinition
	composedSchema     *planner.ComposedSchema
	serviceMap         engine.ServiceMap
}

func NewGateway(ctx context.Context, cfg *GatewayConfig) (graphql.ExecutableSchema, error) {
	g := &gatewayImpl{
		serviceDefinitions: cfg.ServiceDefinitions,
	}
	err := g.validate()
	if err != nil {
		return nil, err
	}

	// TODO make async

	err = g.fetchSDLs(ctx)
	if err != nil {
		return nil, err
	}

	return g, nil
}

func (g *gatewayImpl) validate() error {
	if len(g.serviceDefinitions) == 0 {
		return fmt.Errorf("service definitions are must required")
	}

	for _, serviceDef := range g.serviceDefinitions {
		if serviceDef.DataSource == nil {
			serviceDef.DataSource = &engine.RemoteDataSource{
				URL: serviceDef.URL,
			}
		}
	}

	return nil
}

func (g *gatewayImpl) fetchSDLs(ctx context.Context) error {
	g.RLock()
	serviceDefinitions := g.serviceDefinitions
	g.RUnlock()

	// TODO make parallel

	serviceMap := make(engine.ServiceMap)
	services := make([]*federation.ServiceDefinition, 0, len(serviceDefinitions))
	for _, serviceDef := range serviceDefinitions {
		sdl, err := g.fetchSDL(ctx, serviceDef.DataSource)
		if err != nil {
			return err
		}

		schemaDoc, gErr := parser.ParseSchema(&ast.Source{
			Input: sdl,
		})
		if gErr != nil {
			return gErr
		}

		services = append(services, &federation.ServiceDefinition{
			TypeDefs: schemaDoc,
			Name:     serviceDef.Name,
			URL:      serviceDef.URL,
		})
		serviceMap[serviceDef.Name] = serviceDef.DataSource
	}

	_, sdl, _, err := federation.ComposeAndValidate(ctx, services)
	if err != nil {
		return err
	}

	schemaDoc, gErr := parser.ParseSchemas(
		validator.Prelude,
		&ast.Source{
			Input:   sdl,
			BuiltIn: false,
		},
	)
	if gErr != nil {
		return err
	}

	cs, err := planner.BuildComposedSchema(ctx, schemaDoc)
	if err != nil {
		return err
	}

	g.Lock()
	g.composedSchema = cs
	g.serviceMap = serviceMap
	g.Unlock()

	return nil
}

func (g *gatewayImpl) fetchSDL(ctx context.Context, datasource engine.DataSource) (string, error) {
	source := &ast.Source{
		Input: `{ _service { sdl }}`,
	}
	queryDoc, gErr := parser.ParseQuery(source)
	if gErr != nil {
		return "", gErr
	}
	resp := datasource.Process(ctx, &graphql.OperationContext{
		RawQuery:  source.Input,
		Variables: make(map[string]interface{}),
		Doc:       queryDoc,
		Operation: queryDoc.Operations[0],
	})
	if len(resp.Errors) != 0 {
		return "", resp.Errors
	}

	type Resp struct {
		Service struct {
			SDL string `json:"sdl"`
		} `json:"_service"`
	}

	v := &Resp{}
	err := json.Unmarshal(resp.Data, v)
	if err != nil {
		return "", gqlerror.List{gqlerror.Errorf(err.Error())}
	}

	if v.Service.SDL == "" {
		return "", gqlerror.List{gqlerror.Errorf("sdl fetch failed")}
	}

	return v.Service.SDL, nil
}

func (g *gatewayImpl) Schema() *ast.Schema {
	g.RLock()
	if g.composedSchema == nil {
		panic("gateway doesn't have composed schema")
	}
	schema := g.composedSchema.Schema
	g.RUnlock()

	return schema
}

func (g *gatewayImpl) Complexity(typeName, fieldName string, childComplexity int, args map[string]interface{}) (int, bool) {
	// TODO implement
	return 0, false
}

func (g *gatewayImpl) Exec(ctx context.Context) graphql.ResponseHandler {
	g.RLock()
	schema := g.composedSchema.Schema
	serviceMap := g.serviceMap
	g.RUnlock()

	oc := graphql.GetOperationContext(ctx)

	opctx, err := planner.BuildOperationContext(ctx, g.composedSchema, oc.Doc, oc.OperationName)
	if err != nil {
		graphql.AddError(ctx, err)
		return func(ctx context.Context) *graphql.Response {
			return &graphql.Response{Errors: graphql.GetErrors(ctx)}
		}
	}

	plan, err := planner.BuildQueryPlan(ctx, opctx)
	if err != nil {
		graphql.AddError(ctx, err)
		return func(ctx context.Context) *graphql.Response {
			return &graphql.Response{Errors: graphql.GetErrors(ctx)}
		}
	}

	resp := engine.ExecuteQueryPlan(ctx, plan, serviceMap, schema, oc)
	return func(ctx context.Context) *graphql.Response {
		return resp
	}
}
