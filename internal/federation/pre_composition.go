package federation

import (
	"fmt"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"
	"github.com/vektah/gqlparser/v2/parser"
	"github.com/vektah/gqlparser/v2/validator"
	"github.com/vvakame/fedeway/internal/graphql"
)

func preCompositionValidators() []func(*ServiceDefinition) []error {
	return []func(definition *ServiceDefinition) []error{
		// TODO let's implements below rules!
		externalUsedOnBase,
		requiresUsedOnBase,
		keyFieldsMissingExternal,
		reservedFieldUsed,
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
					if gErr.Extensions == nil {
						gErr.Extensions = make(map[string]interface{})
					}
					gErr.Extensions["code"] = "EXTERNAL_USED_ON_BASE"
					errors = append(errors, gErr)
				}
			}
		}
	}

	return errors
}

// There are no fields with @requires on base type definitions
func requiresUsedOnBase(service *ServiceDefinition) []error {
	serviceName := service.Name
	typeDefs := service.TypeDefs

	var errors []error

	for _, typeDefinition := range typeDefs.Definitions {
		if typeDefinition.Kind != ast.Object {
			continue
		}

		for _, field := range typeDefinition.Fields {
			for _, directive := range field.Directives {
				name := directive.Name
				if name == "requires" {
					gErr := gqlerror.ErrorPosf(
						directive.Position,
						"%s Found extraneous @requires directive. @requires cannot be used on base types.",
						logServiceAndType(serviceName, typeDefinition.Name, field.Name),
					)
					if gErr.Extensions == nil {
						gErr.Extensions = make(map[string]interface{})
					}
					gErr.Extensions["code"] = "REQUIRES_USED_ON_BASE"
					errors = append(errors, gErr)
				}
			}
		}
	}

	return errors
}

// For every @key directive, it must reference a field marked as @external
func keyFieldsMissingExternal(service *ServiceDefinition) []error {
	serviceName := service.Name
	typeDefs := service.TypeDefs

	var errors []error

	// Build an array that accounts for all key directives on type extensions.
	type S struct {
		TypeName    string
		KeyArgument string
	}
	var keyDirectiveInfoOnTypeExtensions []*S
	for _, node := range typeDefs.Extensions {
		keyDirectivesOnTypeExtension := node.Directives.ForNames("key")

		for _, keyDirective := range keyDirectivesOnTypeExtension {
			if len(keyDirective.Arguments) == 0 {
				continue
			}
			if keyDirective.Arguments[0].Value.Kind != ast.StringValue {
				continue
			}
			keyDirectiveInfoOnTypeExtensions = append(keyDirectiveInfoOnTypeExtensions, &S{
				TypeName:    node.Name,
				KeyArgument: keyDirective.Arguments[0].Value.Raw,
			})
		}
	}

	// this allows us to build a partial schema
	schemaDoc, gErr := parser.ParseSchema(validator.Prelude)
	if gErr != nil {
		errors = append(errors, gErr)
		return errors
	}
	schemaDoc.Directives = append(schemaDoc.Directives, graphql.GraphQLSpecifiedByDirective) // TODO remove this line later when validator.Prelude contains @specifiedBy directive
	schemaDoc.Directives = append(schemaDoc.Directives, apolloTypeSystemDirectives...)
	schemaDoc.Merge(typeDefs)

	schema, gErr := validator.ValidateSchemaDocument(schemaDoc)
	if gErr != nil {
		errors = append(errors, gErr)
		return errors
	}

	for _, ext := range keyDirectiveInfoOnTypeExtensions {
		typeName := ext.TypeName
		keyArgument := ext.KeyArgument

		keyDirectiveSelectionSet, gErr := parser.ParseQuery(&ast.Source{
			Input: fmt.Sprintf(`fragment __generated on %s { %s }`, typeName, keyArgument),
		})
		if gErr != nil {
			errors = append(errors, gErr)
			return errors
		}
		gErrs := validator.Validate(schema, keyDirectiveSelectionSet)
		if len(gErrs) != 0 && len(gErrs) != 1 {
			// 1 means Fragment "__generated" is never used.
			for _, gErr := range gErrs {
				errors = append(errors, gErr)
			}
			return errors
		}

		var validateSelectionSet func(selectionSet ast.SelectionSet)
		validateSelectionSet = func(selectionSet ast.SelectionSet) {
			for _, selection := range selectionSet {
				switch node := selection.(type) {
				case *ast.Field:
					fieldDef := node.Definition
					parentType := schema.Types[node.ObjectDefinition.Name]
					if parentType == nil {
						continue
					}
					if fieldDef == nil {
						// TODO: find all fields that have @external and suggest them / heursitic match
						gErr := gqlerror.ErrorPosf(
							node.Position,
							"%s A @key directive specifies a field which is not found in this service. Add a field to this type with @external.",
							logServiceAndType(serviceName, parentType.Name, ""),
						)
						if gErr.Extensions == nil {
							gErr.Extensions = make(map[string]interface{})
						}
						gErr.Extensions["code"] = "KEY_FIELDS_MISSING_EXTERNAL"
						errors = append(errors, gErr)
						continue
					}

					externalDirectivesOnField := fieldDef.Directives.ForNames("external")

					if len(externalDirectivesOnField) == 0 {
						gErr := gqlerror.ErrorPosf(
							node.Position,
							"%s A @key directive specifies the `%s` field which has no matching @external field.",
							logServiceAndType(serviceName, parentType.Name, ""),
							fieldDef.Name,
						)
						if gErr.Extensions == nil {
							gErr.Extensions = make(map[string]interface{})
						}
						gErr.Extensions["code"] = "KEY_FIELDS_MISSING_EXTERNAL"
						errors = append(errors, gErr)
					}

					validateSelectionSet(node.SelectionSet)

				default:
					errors = append(errors, fmt.Errorf("unsupported selection type: %T", selection))
				}
			}
		}

		validateSelectionSet(keyDirectiveSelectionSet.Fragments[0].SelectionSet)
	}

	return errors
}

// Schemas should not define the _service or _entitites fields on the query root
func reservedFieldUsed(service *ServiceDefinition) []error {
	serviceName := service.Name
	typeDefs := service.TypeDefs

	var errors []error

	rootQueryName := "Query"
	for _, schemaDef := range typeDefs.Schema {
		for _, node := range schemaDef.OperationTypes {
			// find the Query type if redefined
			if node.Operation == ast.Query {
				rootQueryName = node.Type
			}
		}
	}
	for _, schemaDef := range typeDefs.SchemaExtension {
		for _, node := range schemaDef.OperationTypes {
			// find the Query type if redefined
			if node.Operation == ast.Query {
				rootQueryName = node.Type
			}
		}
	}

	for _, node := range typeDefs.Definitions {
		if node.Name != rootQueryName {
			continue
		}
		for _, field := range node.Fields {
			fieldName := field.Name
			switch fieldName {
			case "_service", "_entities":
				gErr := gqlerror.ErrorPosf(
					field.Position,
					"%s %s is a field reserved for federation and can't be used at the Query root.",
					logServiceAndType(serviceName, rootQueryName, field.Name),
					fieldName,
				)
				if gErr.Extensions == nil {
					gErr.Extensions = make(map[string]interface{})
				}
				gErr.Extensions["code"] = "RESERVED_FIELD_USED"
				errors = append(errors, gErr)
			}
		}
	}

	return errors
}
