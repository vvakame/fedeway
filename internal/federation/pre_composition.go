package federation

import (
	"github.com/vektah/gqlparser/v2/gqlerror"
)

func preCompositionValidators() []func(*ServiceDefinition) []error {
	return []func(definition *ServiceDefinition) []error{
		// TODO let's implements below rules!
		externalUsedOnBase,
		// requiresUsedOnBase,
		// keyFieldsMissingExternal,
		// reservedFieldUsed,
		// duplicateEnumOrScalar,
		// duplicateEnumValue,
	}
}

// There are no fields with @external on base type definitions
func externalUsedOnBase(service *ServiceDefinition) []error {
	serviceName := service.Name
	typeDefs := service.TypeDefs

	var errors []error

	for _, typeDefinition := range typeDefs.Definitions {
		for _, field := range typeDefinition.Fields {
			for _, directive := range field.Directives {
				name := directive.Name
				if name == "external" {
					gErr := gqlerror.ErrorPosf(
						directive.Position,
						"%s Found extraneous @external directive. @external cannot be used on base types.",
						logServiceAndType(serviceName, typeDefinition.Name, field.Name),
					)
					gErr.Extensions["code"] = "EXTERNAL_USED_ON_BASE"
					errors = append(errors, gErr)
				}
			}
		}
	}

	return errors
}
