package federation

import "github.com/vektah/gqlparser/v2/ast"

func postCompositionValidators() []func(*ast.Schema, []*ServiceDefinition) []error {
	return []func(schema *ast.Schema, serviceList []*ServiceDefinition) []error{
		// TODO 実装すること
		// externalUnused,
		// externalMissingOnBase,
		// externalTypeMismatch,
		// requiresFieldsMissingExternal,
		// requiresFieldsMissingOnBase,
		// keyFieldsMissingOnBase,
		// keyFieldsSelectInvalidType,
		// providesFieldsMissingExternal,
		// providesFieldsSelectInvalidType,
		// providesNotOnEntity,
		// executableDirectivesInAllServices,
		// executableDirectivesIdentical,
		// keysMatchBaseService,
	}
}
