package engine

import (
	"context"
	"encoding/json"
	"github.com/vektah/gqlparser/v2/gqlerror"
	"github.com/vektah/gqlparser/v2/validator"
	"github.com/vvakame/fedeway/internal/gqlfun"

	"github.com/99designs/gqlgen/graphql"
)

var _ DataSource = (*LocalDataSource)(nil)

type LocalDataSource struct {
	ExecutableSchema graphql.ExecutableSchema
}

func (lds *LocalDataSource) SDL(ctx context.Context) (string, gqlerror.List) {
	resp := gqlfun.Execute(ctx, lds.ExecutableSchema, `{ _service { sdl }}`, nil)
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

func (ds *LocalDataSource) Process(ctx context.Context, oc *graphql.OperationContext) *graphql.Response {
	ctx = graphql.WithOperationContext(ctx, oc)
	// TODO make configurable
	ctx = graphql.WithResponseContext(ctx, graphql.DefaultErrorPresenter, graphql.DefaultRecover)

	gErrs := validator.Validate(ds.ExecutableSchema.Schema(), oc.Doc)
	if len(gErrs) != 0 {
		return &graphql.Response{Errors: gErrs}
	}

	rh := ds.ExecutableSchema.Exec(ctx)
	resp := rh(ctx)
	gErrs = graphql.GetErrors(ctx)
	if len(gErrs) != 0 {
		return &graphql.Response{Errors: gErrs}
	}

	return resp
}
