package federation

import (
	"context"

	"github.com/vektah/gqlparser/v2/ast"
)

func validateServicesBeforeNormalization(ctx context.Context, services []*ServiceDefinition) []error {
	var errors []error

	for _, serviceDefinition := range services {
		for _, validator := range preNormalizationValidators() {
			errors = append(errors, validator(serviceDefinition)...)
		}
	}

	return errors
}

func validateComposedSchema(schema *ast.Schema, serviceList []*ServiceDefinition) []error {
	var warningsOrErrors []error

	for _, validator := range postCompositionValidators() {
		warningsOrErrors = append(warningsOrErrors, validator(schema, serviceList)...)
	}

	return warningsOrErrors
}
