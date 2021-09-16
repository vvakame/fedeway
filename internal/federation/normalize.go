package federation

import (
	"context"
	"fmt"

	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vvakame/fedeway/internal/log"
)

func normalizeTypeDefs(ctx context.Context, typeDefs *ast.SchemaDocument) *ast.SchemaDocument {
	// The order of this is important - `stripCommonPrimitives` must come after
	// `defaultRootOperationTypes` because it depends on the `Query` type being named
	// its default: `Query`.
	return stripCommonPrimitives(
		ctx,
		defaultRootOperationTypes(
			replaceExtendedDefinitionsWithExtensions(typeDefs),
		),
	)
}

func defaultRootOperationTypes(typeDefs *ast.SchemaDocument) *ast.SchemaDocument {
	// Array of default root operation names
	defaultRootOperationNames := []string{"Query", "Mutation", "Subscription"}
	defaultRootOperationNameLookup := map[ast.Operation]string{
		ast.Query:        "Query",
		ast.Mutation:     "Mutation",
		ast.Subscription: "Subscription",
	}
	includesDefaultRootOperationNames := func(name string) bool {
		for _, defaultRootOperationName := range defaultRootOperationNames {
			if name == defaultRootOperationName {
				return true
			}
		}
		return false
	}

	// Map of the given root operation type names to their respective default operation
	// type names, i.e. {RootQuery: 'Query'}
	rootOperationTypeMap := make(map[string]string)

	var hasSchemaDefinitionOrExtension bool
	{
		schemaDefList := make(ast.SchemaDefinitionList, 0, len(typeDefs.Schema)+len(typeDefs.SchemaExtension))
		schemaDefList = append(schemaDefList, typeDefs.Schema...)
		schemaDefList = append(schemaDefList, typeDefs.SchemaExtension...)
		for _, def := range schemaDefList {
			for _, node := range def.OperationTypes {
				// If we find at least one root operation type definition, we know the user has
				// specified either a schema definition or extension.
				hasSchemaDefinitionOrExtension = true
				// Build the map of root operation type name to its respective default
				rootOperationTypeMap[node.Type] = defaultRootOperationNameLookup[node.Operation]
			}
		}
	}

	// In this case, there's no defined schema or schema extension, so we use defaults
	if !hasSchemaDefinitionOrExtension {
		rootOperationTypeMap = map[string]string{
			"Query":        "Query",
			"Mutation":     "Mutation",
			"Subscription": "Subscription",
		}
	}

	// A conflicting default definition exists when the user provides a schema
	// definition, but also defines types that use the default root operation
	// names (Query, Mutation, Subscription). Those types need to be removed.
	var schemaWithoutConflictingDefaultDefinitions *ast.SchemaDocument
	if !hasSchemaDefinitionOrExtension {
		// If no schema definition or extension exists, then there aren't any
		// conflicting defaults to worry about.
		schemaWithoutConflictingDefaultDefinitions = typeDefs
	} else {
		// If the user provides a schema definition or extension, then using default
		// root operation names is considered an error for composition. This visit
		// drops the invalid type definitions/extensions altogether, as well as
		// fields that reference them.
		//
		// Example:
		//
		// schema {
		//   query: RootQuery
		// }
		//
		// type Query { <--- this type definition is invalid (as well as Mutation or Subscription)
		//   ...
		// }
		{
			copied := *typeDefs
			typeDefs = &copied
			typeDefs.Definitions = append(ast.DefinitionList{}, typeDefs.Definitions...)
			typeDefs.Extensions = append(ast.DefinitionList{}, typeDefs.Extensions...)
		}

		definitions := typeDefs.Definitions
		typeDefs.Definitions = nil
		extensions := typeDefs.Extensions
		typeDefs.Extensions = nil

		var processDef func(defs ast.DefinitionList, isExtension bool)
		var processFieldDef func(defs ast.FieldList) ast.FieldList
		processDef = func(defs ast.DefinitionList, isExtension bool) {
			for _, node := range defs {
				switch node.Kind {
				case ast.Object:
					{
						copied := *node
						node = &copied
					}
					if _, ok := rootOperationTypeMap[node.Name]; includesDefaultRootOperationNames(node.Name) && !ok {
						if isExtension {
							typeDefs.Extensions = append(typeDefs.Extensions, node)
						} else {
							typeDefs.Definitions = append(typeDefs.Definitions, node)
						}
					}

					node.Fields = processFieldDef(node.Fields)

				default:
					if isExtension {
						typeDefs.Extensions = append(typeDefs.Extensions, node)
					} else {
						typeDefs.Definitions = append(typeDefs.Definitions, node)
					}
				}
			}
		}
		processFieldDef = func(defs ast.FieldList) ast.FieldList {
			// This visitor handles the case where:
			// 1) A schema definition or extension is provided by the user
			// 2) A field exists that is of a _default_ root operation type. (Query, Mutation, Subscription)
			//
			// Example:
			//
			// schema {
			//   mutation: RootMutation
			// }
			//
			// type RootMutation {
			//   updateProduct: Query <--- remove this field altogether
			// }
			newDefs := make(ast.FieldList, 0, len(defs))
			for _, node := range defs {
				if includesDefaultRootOperationNames(node.Type.Name()) {
					continue
				}

				newDefs = append(newDefs, node)
			}

			return newDefs
		}

		processDef(definitions, false)
		processDef(extensions, true)

		schemaWithoutConflictingDefaultDefinitions = typeDefs
	}

	var schemaWithDefaultRootTypes *ast.SchemaDocument
	{
		copied := *schemaWithoutConflictingDefaultDefinitions
		schemaWithDefaultRootTypes = &copied
	}
	// Schema definitions and extensions are extraneous since we're transforming
	// the root operation types to their defaults.
	{
		schemaWithDefaultRootTypes.Schema = append(ast.SchemaDefinitionList{}, schemaWithDefaultRootTypes.Schema...)
		schemaWithDefaultRootTypes.SchemaExtension = append(ast.SchemaDefinitionList{}, schemaWithDefaultRootTypes.SchemaExtension...)

		processSchemaDef := func(schemaDefList ast.SchemaDefinitionList) {
			for idx, doc := range schemaDefList {
				{
					copied := *doc
					doc = &copied
					schemaDefList[idx] = doc
					// TODO ここコピーが不完全… operationType が元の値を編集している
				}
				for _, operationType := range doc.OperationTypes {
					switch operationType.Operation {
					case ast.Query:
						operationType.Type = "Query"
					case ast.Mutation:
						operationType.Type = "Mutation"
					case ast.Subscription:
						operationType.Type = "Subscription"
					default:
						panic(fmt.Sprintf("unknown operation type: %s", operationType.Operation))
					}
				}
			}
		}
		processSchemaDef(schemaWithDefaultRootTypes.Schema)
		processSchemaDef(schemaWithDefaultRootTypes.SchemaExtension)

		// schema {
		//   query: RootQuery
		// }
		//
		// extend type RootQuery { <--- update this to `extend type Query`
		//   ...
		// }
		definitions := schemaWithDefaultRootTypes.Definitions
		schemaWithDefaultRootTypes.Definitions = nil
		extensions := schemaWithDefaultRootTypes.Extensions
		schemaWithDefaultRootTypes.Extensions = nil

		processDef := func(defs ast.DefinitionList, isExtension bool) {
			for _, node := range defs {
				if rootOperationTypeMap[node.Name] != "" || includesDefaultRootOperationNames(node.Name) {
					{
						copied := *node
						node = &copied
					}
					if operationName := rootOperationTypeMap[node.Name]; operationName != "" {
						node.Name = rootOperationTypeMap[node.Name]
					}

					schemaWithDefaultRootTypes.Extensions = append(schemaWithDefaultRootTypes.Extensions, node)
				} else {
					if isExtension {
						schemaWithDefaultRootTypes.Extensions = append(schemaWithDefaultRootTypes.Extensions, node)
					} else {
						schemaWithDefaultRootTypes.Definitions = append(schemaWithDefaultRootTypes.Definitions, node)
					}
				}
			}
		}
		processDef(definitions, false)
		processDef(extensions, true)
	}

	// Corresponding NamedTypes must also make the name switch, in the case that
	// they reference a root operation type that we've transformed
	//
	// schema {
	//   query: RootQuery
	//   mutation: RootMutation
	// }
	//
	// type RootQuery {
	//   ...
	// }
	//
	// type RootMutation {
	//   updateProduct: RootQuery <--- rename `RootQuery` to `Query`
	// }
	{ // TODO node.Fields がコピーから保護されていない
		for _, node := range schemaWithDefaultRootTypes.Definitions {
			for _, field := range node.Fields {
				if _, ok := rootOperationTypeMap[field.Type.Name()]; ok {
					copied := *field
					field = &copied
					field.Type.NamedType = rootOperationTypeMap[field.Type.Name()]
				}
			}
		}
	}

	return schemaWithDefaultRootTypes
}

func replaceExtendedDefinitionsWithExtensions(typeDefs *ast.SchemaDocument) *ast.SchemaDocument {
	{
		copied := *typeDefs
		typeDefs = &copied
		typeDefs.Definitions = append(ast.DefinitionList{}, typeDefs.Definitions...)
		typeDefs.Extensions = append(ast.DefinitionList{}, typeDefs.Extensions...)
	}

	definitions := typeDefs.Definitions
	typeDefs.Definitions = nil
	extensions := typeDefs.Extensions
	typeDefs.Extensions = nil

	processDef := func(defs ast.DefinitionList, isExtension bool) {
		for _, node := range defs {
			if node.Kind != ast.Object && node.Kind != ast.Interface {
				if isExtension {
					typeDefs.Extensions = append(typeDefs.Extensions, node)
				} else {
					typeDefs.Definitions = append(typeDefs.Definitions, node)
				}
				continue
			}

			isExtensionDefinition := len(findDirectivesOnNode(node, "extends")) > 0

			if !isExtensionDefinition {
				if isExtension {
					typeDefs.Extensions = append(typeDefs.Extensions, node)
				} else {
					typeDefs.Definitions = append(typeDefs.Definitions, node)
				}
				continue
			}

			var filteredDirectives ast.DirectiveList
			for _, directive := range node.Directives {
				if directive.Name != "extends" {
					filteredDirectives = append(filteredDirectives, directive)
				}
			}

			if len(filteredDirectives) != 0 {
				copied := *node
				copied.Directives = filteredDirectives
				typeDefs.Extensions = append(typeDefs.Extensions, &copied)
			} else if isExtension {
				typeDefs.Extensions = append(typeDefs.Extensions, node)
			} else {
				typeDefs.Definitions = append(typeDefs.Definitions, node)
			}
		}
	}
	processDef(definitions, false)
	processDef(extensions, true)

	return typeDefs
}

// For non-ApolloServer libraries that support federation, this allows a
// library to report the entire schema's SDL rather than an awkward, stripped out
// subset of the schema. Generally there's no need to include the federation
// primitives, but in many cases it's more difficult to exclude them.
//
// This removes the following from a GraphQL Document:
// directives: @external, @key, @requires, @provides, @extends, @skip, @include, @deprecated, @specifiedBy
// scalars: _Any, _FieldSet
// union: _Entity
// object type: _Service
// Query fields: _service, _entities
func stripCommonPrimitives(ctx context.Context, document *ast.SchemaDocument) *ast.SchemaDocument {
	// TODO コピーするのマジでだるいので諦めてもいいんじゃなかろうか

	logger := log.FromContext(ctx)
	logger.Info("caution! this method is not implemented!")

	// TODO あとで実装する

	return document
}
