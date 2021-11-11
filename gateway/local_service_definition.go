package gateway

import (
	"github.com/99designs/gqlgen/graphql"
	"github.com/vvakame/fedeway/internal/engine"
)

func NewLocalServiceDefinition(name string, es graphql.ExecutableSchema) *ServiceDefinition {
	lds := &engine.LocalDataSource{ExecutableSchema: es}
	return &ServiceDefinition{
		Name:       name,
		DataSource: lds,
	}
}
