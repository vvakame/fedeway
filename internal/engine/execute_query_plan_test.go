package engine

import (
	"context"
	"testing"
)

func TestExecuteQueryPlan(t *testing.T) {
	ctx := context.Background()
	schema := getFederatedTestingSchema(ctx, t)
	t.Log(schema)
}
