package gateway

import (
	"github.com/vvakame/fedeway/internal/engine"
)

func NewRemoteServiceDefinition(name string, endpointURL string) *ServiceDefinition {
	rds := &engine.RemoteDataSource{
		URL: endpointURL,
	}
	return &ServiceDefinition{
		Name:       name,
		URL:        endpointURL,
		DataSource: rds,
	}
}
