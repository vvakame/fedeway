package federation

import (
	"fmt"
	"strings"

	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

func preNormalizationValidators() []func(*ServiceDefinition) []error {
	return []func(definition *ServiceDefinition) []error{
		rootFieldUsed,
		tagDirectiveRule,
	}
}

// When a schema definition or extension is provided, warn user against using
// default root operation type names for types or extensions
// (Query, Mutation, Subscription)
func rootFieldUsed(service *ServiceDefinition) []error {
	serviceName := service.Name
	typeDefs := service.TypeDefs

	var errors []error

	defaultRootOperationNames := []string{"Query", "Mutation", "Subscription"}
	defaultRootOperationNameLookup := map[ast.Operation]string{
		ast.Query:        "Query",
		ast.Mutation:     "Mutation",
		ast.Subscription: "Subscription",
	}

	disallowedTypeNames := make(map[string]bool)

	var hasSchemaDefinitionOrExtension bool
	schemaDefList := make(ast.SchemaDefinitionList, 0, len(typeDefs.Schema)+len(typeDefs.SchemaExtension))
	schemaDefList = append(schemaDefList, typeDefs.Schema...)
	schemaDefList = append(schemaDefList, typeDefs.SchemaExtension...)
	for _, def := range schemaDefList {
		for _, node := range def.OperationTypes {
			// If we find at least one root operation type definition, we know the user has
			// specified either a schema definition or extension.
			hasSchemaDefinitionOrExtension = true

			var foundDefaultName bool
			for _, defaultRootOperationName := range defaultRootOperationNames {
				if node.Type == defaultRootOperationName {
					foundDefaultName = true
					break
				}
			}

			if !foundDefaultName {
				disallowedTypeNames[defaultRootOperationNameLookup[node.Operation]] = true
			}
		}
	}

	// If a schema or schema extension is defined, we need to warn for each improper
	// usage of default root operation names. The conditions for an improper usage are:
	//  1. root operation type is defined as a non-default name (i.e. query: RootQuery)
	//  2. the respective default operation type name is used as a regular type
	if hasSchemaDefinitionOrExtension {
		defList := make(ast.DefinitionList, 0, len(typeDefs.Definitions)+len(typeDefs.Extensions))
		defList = append(defList, typeDefs.Definitions...)
		defList = append(defList, typeDefs.Extensions...)

		for _, def := range defList {
			if disallowedTypeNames[def.Name] {
				rootOperationName := def.Name

				gErr := gqlerror.ErrorPosf(
					def.Position,
					"%s Found invalid use of default root operation name `%s`. `%s` is disallowed when `Schema.%s` is set to a type other than `%s`",
					logServiceAndType(serviceName, rootOperationName, ""),
					rootOperationName, rootOperationName, strings.ToLower(rootOperationName), rootOperationName,
				)
				if gErr.Extensions == nil {
					gErr.Extensions = make(map[string]interface{})
				}
				gErr.Extensions["code"] = fmt.Sprintf("ROOT_%s_USED", strings.ToUpper(rootOperationName))
				errors = append(errors, gErr)
			}
		}
	}

	return errors
}

// If there are tag usages in the service definition, check that the tag directive
// definition is included and correct.
func tagDirectiveRule(service *ServiceDefinition) []error {
	// NOTE original name: tagDirective

	serviceName := service.Name
	typeDefs := service.TypeDefs

	var errors []error

	// TODO 本家では3つの標準GraphQLのチェックをしている めんどいのでここでは割愛する
	// KnownArgumentNamesOnDirectivesRule,
	// KnownDirectivesRule,
	// ProvidedRequiredArgumentsOnDirectivesRule,

	tagDirectiveDefinition := typeDefs.Directives.ForName("tag")

	// Ensure the tag directive definition is correct
	if tagDirectiveDefinition != nil {
		printedTagDefinition := "directive @tag(name: String!) repeatable on FIELD_DEFINITION | INTERFACE | OBJECT | UNION"

		// NOTE originalはprintして想定と比較してるけどそのほうがめんどいので各部をチェックする
		var foundError bool
		if len(tagDirectiveDefinition.Arguments) != 1 {
			foundError = true
		} else if arg := tagDirectiveDefinition.Arguments[0]; arg.Name != "name" {
			foundError = true
		} else if arg.Type.String() != "String!" {
			foundError = true
		} else if !tagDirectiveDefinition.IsRepeatable {
			foundError = true
		} else if len(tagDirectiveDefinition.Locations) != 4 {
			foundError = true
		} else {
			for _, loc := range tagDirectiveDefinition.Locations {
				switch loc {
				case ast.LocationFieldDefinition,
					ast.LocationInterface,
					ast.LocationObject,
					ast.LocationUnion:
				// ok
				default:
					foundError = true
					break
				}
			}
		}

		if foundError {
			gErr := gqlerror.ErrorPosf(
				tagDirectiveDefinition.Position,
				"%s Found @tag definition in service %s, but the @tag directive definition was invalid. Please ensure the directive definition in your schema's type definitions matches the following:\n\t%s",
				logDirective("tag"),
				serviceName,
				printedTagDefinition,
			)
			if gErr.Extensions == nil {
				gErr.Extensions = make(map[string]interface{})
			}
			gErr.Extensions["code"] = "TAG_DIRECTIVE_DEFINITION_INVALID"
			errors = append(errors, gErr)
		}
	}

	return errors
}
