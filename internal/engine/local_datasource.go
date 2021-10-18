package engine

import (
	"context"
	"encoding/json"
	"github.com/vektah/gqlparser/v2/gqlerror"
	"github.com/vvakame/fedeway/internal/gqlfun"

	"github.com/99designs/gqlgen/graphql"
)

var _ DataSource = (*LocalDataSource)(nil)

type LocalDataSource struct {
	ExecutableSchema graphql.ExecutableSchema
}

func (lds *LocalDataSource) SDL(ctx context.Context) (string, gqlerror.List) {
	resp, gErrs := gqlfun.Execute(ctx, lds.ExecutableSchema, `{ _service { sdl }}`, nil)
	if len(gErrs) != 0 {
		return "", gErrs
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
	rh := ds.ExecutableSchema.Exec(ctx)
	return rh(ctx)
}
