package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/99designs/gqlgen/graphql"
)

var _ DataSource = (*RemoteDataSource)(nil)

type RemoteDataSource struct {
	URL string

	Client *http.Client
}

func (ds *RemoteDataSource) Process(ctx context.Context, oc *graphql.OperationContext) *graphql.Response {
	hc := ds.Client
	if hc == nil {
		hc = http.DefaultClient
	}

	ctx = graphql.WithResponseContext(
		ctx,
		// TODO make configurable
		graphql.DefaultErrorPresenter,
		graphql.DefaultRecover,
	)

	type RawParams struct {
		Query         string                 `json:"query"`
		OperationName string                 `json:"operationName,omitempty"`
		Variables     map[string]interface{} `json:"variables,omitempty"`
	}

	params := &RawParams{
		Query:         oc.RawQuery,
		OperationName: oc.OperationName,
		Variables:     oc.Variables,
	}
	b, err := json.Marshal(params)
	if err != nil {
		graphql.AddError(ctx, err)
		return &graphql.Response{
			Errors: graphql.GetErrors(ctx),
		}
	}

	req, err := http.NewRequestWithContext(ctx, "POST", ds.URL, bytes.NewBuffer(b))
	if err != nil {
		graphql.AddError(ctx, err)
		return &graphql.Response{
			Errors: graphql.GetErrors(ctx),
		}
	}
	req.Header.Add("Content-Type", "application/json")

	resp, err := hc.Do(req)
	if err != nil {
		graphql.AddError(ctx, err)
		return &graphql.Response{
			Errors: graphql.GetErrors(ctx),
		}
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	b, err = io.ReadAll(resp.Body)
	if err != nil {
		graphql.AddError(ctx, err)
		return &graphql.Response{
			Errors: graphql.GetErrors(ctx),
		}
	}

	if resp.StatusCode != http.StatusOK {
		graphql.AddErrorf(ctx, "unexpected response code: %d", resp.StatusCode)
		return &graphql.Response{
			Errors: graphql.GetErrors(ctx),
		}
	}

	gqlResp := &graphql.Response{}
	err = json.Unmarshal(b, gqlResp)
	if err != nil {
		graphql.AddError(ctx, err)
		return &graphql.Response{
			Errors: graphql.GetErrors(ctx),
		}
	}

	return gqlResp
}
