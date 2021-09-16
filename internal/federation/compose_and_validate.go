package federation

import (
	"context"

	"github.com/hashicorp/go-multierror"
	"github.com/vektah/gqlparser/v2/ast"
)

func ComposeAndValidate(ctx context.Context, serviceList []*ServiceDefinition) (schema *ast.Schema, supergraphSDL string, err error) {
	var errors []error

	errors = validateServicesBeforeNormalization(ctx, serviceList)

	normalizedServiceList := make([]*ServiceDefinition, 0, len(serviceList))
	for _, service := range serviceList {
		typeDefs := service.TypeDefs
		typeDefs = normalizeTypeDefs(ctx, typeDefs)
		normalizedServiceList = append(normalizedServiceList, &ServiceDefinition{
			TypeDefs: typeDefs,
			Name:     service.Name,
			URL:      service.URL,
		})
	}

	errors = append(errors, validateServicesBeforeComposition(normalizedServiceList)...)

	var errs []error
	schema, supergraphSDL, errs = composeServices(ctx, normalizedServiceList)

	if len(errs) != 0 {
		errors = append(errors, errs...)
	}

	errors = append(errors, validateComposedSchema(schema, serviceList)...)

	if len(errors) > 0 {
		err := multierror.Append(nil, errors...)
		return schema, "", err
	}

	return schema, supergraphSDL, nil
}
