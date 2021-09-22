package federation

import (
	"bytes"
	"context"
	"sort"

	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/formatter"
	"github.com/vektah/gqlparser/v2/parser"
	"github.com/vektah/gqlparser/v2/validator"
	"github.com/vvakame/fedeway/internal/graphql"
)

var emptyQueryDefinition = &TypeDefinitionEntity{
	ServiceName: "",
	Definition: &ast.Definition{
		Kind: ast.Object,
		Name: "Query",
	},
}
var emptyMutationDefinition = &TypeDefinitionEntity{
	ServiceName: "",
	Definition: &ast.Definition{
		Kind: ast.Object,
		Name: "Mutation",
	},
}

// Map of all type definitions to eventually be passed to extendSchema
type TypeDefinitionsMap map[string][]*TypeDefinitionEntity
type TypeDefinitionEntity struct {
	ServiceName string
	Definition  *ast.Definition
}

// Map of all directive definitions to eventually be passed to extendSchema
// original: [name: string]: { [serviceName: string]: DirectiveDefinitionNode };
type DirectiveDefinitionsMap map[string]map[string]*ast.DirectiveDefinition

// A map of base types to their owning service. Used by query planner to direct traffic.
// This contains the base type's "owner". Any fields that extend this type in another service
// are listed under "extensionFieldsToOwningServiceMap". extensionFieldsToOwningServiceMap are in the format { myField: my-service-name }
//
// Example resulting typeToServiceMap shape:
//
// const typeToServiceMap = {
//   Product: {
//     serviceName: "ProductService",
//     extensionFieldsToOwningServiceMap: {
//       reviews: "ReviewService", // Product.reviews comes from the ReviewService
//       dimensions: "ShippingService",
//       weight: "ShippingService"
//     }
//   }
// }
// original: [typeName: string]: { owningService?: string; extensionFieldsToOwningServiceMap: { [fieldName: string]: string }; };
type TypeToServiceMap map[string]*TypeToServiceEntity
type TypeToServiceEntity struct {
	OwningService                     string // optional
	ExtensionFieldsToOwningServiceMap map[string]string
}

// Map of types to their key directives (maintains association to their services)
//
// Example resulting KeyDirectivesMap shape:
//
// const keyDirectives = {
//   Product: {
//     serviceA: ["sku", "upc"]
//     serviceB: ["color {id value}"] // Selection node simplified for readability
//   }
// }
// original: [typeName: string]: ServiceNameToKeyDirectivesMap;
type KeyDirectivesMap map[string]ServiceNameToKeyDirectivesMap

// A set of type names that have been determined to be a value type, a type
// shared across at least 2 services.
// original: type ValueTypes = Set<string>;
type ValueTypes []string

type buildMaps struct {
	typeToServiceMap        TypeToServiceMap
	typeDefinitionsMap      TypeDefinitionsMap
	typeExtensionsMap       TypeDefinitionsMap
	directiveDefinitionsMap DirectiveDefinitionsMap
	externalFields          []*ExternalFieldDefinition
	keyDirectivesMap        KeyDirectivesMap
	valueTypes              ValueTypes
	directiveMetadata       *DirectiveMetadata
}

func buildMapsFromServiceList(ctx context.Context, serviceList []*ServiceDefinition) (*buildMaps, error) {
	typeDefinitionsMap := TypeDefinitionsMap{}
	typeExtensionsMap := TypeDefinitionsMap{}
	directiveDefinitionsMap := DirectiveDefinitionsMap{}
	typeToServiceMap := TypeToServiceMap{}
	var externalFields []*ExternalFieldDefinition
	keyDirectivesMap := KeyDirectivesMap{}
	valueTypes := ValueTypes{}
	directiveMetadata := newDirectiveMetadata(serviceList)

	for _, service := range serviceList {
		typeDefs := service.TypeDefs
		serviceName := service.Name

		// Build a new SDL with @external fields removed, as well as information about
		// the fields that were removed.
		typeDefsWithoutExternalFields, strippedFields := stripExternalFieldsFromTypeDefs(typeDefs, serviceName)

		externalFields = append(externalFields, strippedFields...)

		// Type system directives from downstream services are not a concern of the
		// gateway, but rather the services on which the fields live which serve
		// those types.  In other words, its up to an implementing service to
		// act on such directives, not the gateway.
		typeDefsWithoutTypeSystemDirectives := stripTypeSystemDirectivesFromTypeDefs(typeDefsWithoutExternalFields)

		{
			definition := make([]*ast.Definition, 0, len(typeDefsWithoutTypeSystemDirectives.Definitions)+len(typeDefsWithoutTypeSystemDirectives.Extensions))
			definition = append(definition, typeDefsWithoutTypeSystemDirectives.Definitions...)
			definition = append(definition, typeDefsWithoutTypeSystemDirectives.Extensions...)
			for _, definition := range definition {
				if definition.Kind != ast.Object {
					continue
				}
				typeName := definition.Name

				for _, keyDirective := range definition.Directives.ForNames("key") {
					if len(keyDirective.Arguments) != 0 && keyDirective.Arguments[0].Value.Kind == ast.StringValue {
						if _, ok := keyDirectivesMap[typeName]; !ok {
							keyDirectivesMap[typeName] = ServiceNameToKeyDirectivesMap{}
						}
						// Add @key metadata to the array
						selectionSet, err := parseSelections(keyDirective.Arguments[0].Value.Raw)
						if err != nil {
							return nil, err
						}
						keyDirectivesMap[typeName][serviceName] = append(keyDirectivesMap[typeName][serviceName], selectionSet)
					}
				}
			}
		}
		for _, definition := range typeDefsWithoutTypeSystemDirectives.Definitions {
			typeName := definition.Name
			// This type is a base definition (not an extension). If this type is already in the typeToServiceMap, then
			// 1. It was declared by a previous service, but this newer one takes precedence, or...
			// 2. It was extended by a service before declared
			if _, ok := typeToServiceMap[typeName]; !ok {
				typeToServiceMap[typeName] = &TypeToServiceEntity{
					ExtensionFieldsToOwningServiceMap: make(map[string]string),
				}
			}

			typeToServiceMap[typeName].OwningService = serviceName

			// If this type already exists in the definitions map, push this definition to the array (newer defs
			// take precedence). If the types are determined to be identical, add the type name
			// to the valueTypes Set.
			//
			// If not, create the definitions array and add it to the typeDefinitionsMap.
			if _, ok := typeDefinitionsMap[typeName]; ok {
				isValueType := typeNodesAreEquivalent(
					typeDefinitionsMap[typeName][len(typeDefinitionsMap[typeName])-1].Definition,
					definition,
				)
				if isValueType {
					valueTypes = append(valueTypes, typeName)
				}
			}
			typeDefinitionsMap[typeName] = append(typeDefinitionsMap[typeName], &TypeDefinitionEntity{
				ServiceName: serviceName,
				Definition:  definition,
			})
		}
		for _, definition := range typeDefsWithoutTypeSystemDirectives.Extensions {
			typeName := definition.Name

			// This definition is an extension of an OBJECT type defined in another service.
			// TODO: handle extensions of non-object types?
			if definition.Kind == ast.Object || definition.Kind == ast.InputObject {
				if len(definition.Fields) == 0 {
					// TODO this break is not exactly same as original.
					break
				}

				fields := mapFieldNamesToServiceName(definition.Fields, serviceName)

				if _, v := typeToServiceMap[typeName]; !v {
					typeToServiceMap[typeName] = &TypeToServiceEntity{
						ExtensionFieldsToOwningServiceMap: make(map[string]string),
					}
				}

				// If the type already exists in the typeToServiceMap, add the extended fields. If not, create the object
				// and add the extensionFieldsToOwningServiceMap, but don't add a serviceName. That will be added once that service
				// definition is processed.
				for k, v := range fields {
					typeToServiceMap[typeName].ExtensionFieldsToOwningServiceMap[k] = v
				}
			}

			if definition.Kind == ast.Enum {
				if len(definition.EnumValues) == 0 {
					// TODO this break is not exactly same as original.
					break
				}

				values := mapEnumNamesToServiceName(
					definition.EnumValues,
					serviceName,
				)

				if _, v := typeToServiceMap[typeName]; !v {
					typeToServiceMap[typeName] = &TypeToServiceEntity{
						ExtensionFieldsToOwningServiceMap: make(map[string]string),
					}
				}

				for k, v := range values {
					typeToServiceMap[typeName].ExtensionFieldsToOwningServiceMap[k] = v
				}
			}

			// If an extension for this type already exists in the extensions map, push this extension to the
			// array (since a type can be extended by multiple services). If not, create the extensions array
			// and add it to the typeExtensionsMap.
			typeExtensionsMap[typeName] = append(typeExtensionsMap[typeName], &TypeDefinitionEntity{
				ServiceName: serviceName,
				Definition:  definition,
			})
		}
		for _, definition := range typeDefsWithoutTypeSystemDirectives.Directives {
			directiveName := definition.Name

			// The composed schema should only contain directives and their
			// ExecutableDirectiveLocations. This filters out any TypeSystemDirectiveLocations.
			// A new DirectiveDefinitionNode with this filtered list will be what is
			// added to the schema.
			var executableLocations []ast.DirectiveLocation
			for _, location := range definition.Locations {
				switch location {
				case ast.LocationQuery, ast.LocationMutation, ast.LocationSubscription,
					ast.LocationField, ast.LocationFragmentDefinition, ast.LocationFragmentSpread,
					ast.LocationInlineFragment, ast.LocationVariableDefinition:
					executableLocations = append(executableLocations, location)
				default:
					// ignore
				}
			}

			// If none of the directive's locations are executable, we don't need to
			// include it in the composed schema at all.
			if len(executableLocations) == 0 {
				// TODO this break is not exactly same as original.
				// いやーここ間違ってない？
				continue
			}

			var definitionWithExecutableLocations *ast.DirectiveDefinition
			{
				copied := *definition
				definitionWithExecutableLocations = &copied
			}
			definitionWithExecutableLocations.Locations = executableLocations

			if _, ok := directiveDefinitionsMap[directiveName]; !ok {
				directiveDefinitionsMap[directiveName] = make(map[string]*ast.DirectiveDefinition)
			}
			directiveDefinitionsMap[directiveName][serviceName] = definitionWithExecutableLocations
		}
	}

	// Since all Query/Mutation definitions in service schemas are treated as
	// extensions, we don't have a Query or Mutation DEFINITION in the definitions
	// list. Without a Query/Mutation definition, we can't _extend_ the type.
	// extendSchema will complain about this. We can't add an empty
	// GraphQLObjectType to the schema constructor, so we add an empty definition
	// here. We only add mutation if there is a mutation extension though.
	// …とオリジナルではなっているが、 extendSchema を使わないこと & SchemaDocument をvalidateするときエラーになるためここではこれを行わない
	//if _, ok := typeDefinitionsMap["Query"]; !ok {
	//	typeDefinitionsMap["Query"] = []*TypeDefinitionEntity{emptyQueryDefinition}
	//}
	//if _, ok := typeDefinitionsMap["Mutation"]; !ok {
	//	typeDefinitionsMap["Mutation"] = []*TypeDefinitionEntity{emptyMutationDefinition}
	//}

	return &buildMaps{
		typeToServiceMap:        typeToServiceMap,
		typeDefinitionsMap:      typeDefinitionsMap,
		typeExtensionsMap:       typeExtensionsMap,
		directiveDefinitionsMap: directiveDefinitionsMap,
		externalFields:          externalFields,
		keyDirectivesMap:        keyDirectivesMap,
		valueTypes:              valueTypes,
		directiveMetadata:       directiveMetadata,
	}, nil
}

func buildSchemaFromDefinitionsAndExtensions(typeDefinitionsMap TypeDefinitionsMap, typeExtensionsMap TypeDefinitionsMap, directiveDefinitionsMap DirectiveDefinitionsMap, directiveMetadata *DirectiveMetadata, serviceList []*ServiceDefinition) (*ast.SchemaDocument, []error) {
	// TODO errors の型が gqlerror のやつかもしれない
	var errors []error

	// We only want to include the definitions of other known Apollo directives
	// (currently just @tag) if there are usages.
	var otherKnownDirectiveDefinitionsToInclude ast.DirectiveDefinitionList
	for _, directive := range otherKnownDirectiveDefinitions {
		if directiveMetadata.HasUsage(directive.Name) {
			otherKnownDirectiveDefinitionsToInclude = append(otherKnownDirectiveDefinitionsToInclude, directive)
		}
	}

	_, fieldSetScalar, joinTypeDirective, joinFieldDirective, joinOwnerDirective, joinGraphEnum, joinGraphDirective := getJoinDefinitions(serviceList)

	// original では ast.Schema を組み立てているが、validatorの詳細が公開されていなかったりするので ast.SchemaDocument を組み立てる
	schemaDoc := &ast.SchemaDocument{}
	// prelude をベースにしないと String とか各種scalarがなくてめんどくさいことになる
	schemaDoc, gErr := parser.ParseSchema(validator.Prelude)
	if gErr != nil {
		errors = append(errors, gErr)
		return nil, errors
	}

	schemaDoc.Directives = append(schemaDoc.Directives, CoreDirective)
	schemaDoc.Directives = append(schemaDoc.Directives, joinFieldDirective)
	schemaDoc.Directives = append(schemaDoc.Directives, joinTypeDirective)
	schemaDoc.Directives = append(schemaDoc.Directives, joinOwnerDirective)
	schemaDoc.Directives = append(schemaDoc.Directives, joinGraphDirective)
	// @include, @skip, @deprecated は Prelude に含まれるので扱わない
	// schemaDoc.Directives = append(schemaDoc.Directives, graphql.SpecifiedDirectives...)
	schemaDoc.Directives = append(schemaDoc.Directives, graphql.GraphQLSpecifiedByDirective)
	schemaDoc.Directives = append(schemaDoc.Directives, federationDirectives...)
	schemaDoc.Directives = append(schemaDoc.Directives, otherKnownDirectiveDefinitionsToInclude...)
	// original では CorePurpose は追加していないがここでは必要
	schemaDoc.Definitions = append(schemaDoc.Definitions, CorePurpose, fieldSetScalar, joinGraphEnum)

	// Extend the blank schema with the base type definitions (as an AST node)
	// originalではschemaとdefinitionsDocumentを別個に扱っているがこの実装では最初からschema documentを増築していく
	typeDefinitionNames := make([]string, 0, len(typeDefinitionsMap))
	for k := range typeDefinitionsMap {
		typeDefinitionNames = append(typeDefinitionNames, k)
	}
	sort.Strings(typeDefinitionNames)
	for _, typeDefinitionName := range typeDefinitionNames {
		typeDefinitions := typeDefinitionsMap[typeDefinitionName]
		// See if any of our Objects or Interfaces implement any interfaces at all.
		// If not, we can return early.
		var foundInterfaceLike bool
		definitions := make(ast.DefinitionList, 0, len(typeDefinitions))
		for _, typeDefinition := range typeDefinitions {
			definitions = append(definitions, typeDefinition.Definition)
			if len(typeDefinition.Definition.Interfaces) != 0 {
				foundInterfaceLike = true
				break
			}
		}
		if !foundInterfaceLike {
			// TODO ここで ServiceName の情報が失われている気がするが…？
			schemaDoc.Definitions = append(schemaDoc.Definitions, definitions...)
			continue
		}

		var uniqueInterfaces []string
		for _, objectTypeDef := range typeDefinitions {
		OUTER:
			for _, interfaceName := range objectTypeDef.Definition.Interfaces {
				for _, knownInterface := range uniqueInterfaces {
					if interfaceName == knownInterface {
						continue OUTER
					}
				}
				uniqueInterfaces = append(uniqueInterfaces, interfaceName)
			}
		}

		// No interfaces, no aggregation - just return what we got.
		if len(uniqueInterfaces) == 0 {
			// TODO ここで ServiceName の情報が失われている気がするが…？
			schemaDoc.Definitions = append(schemaDoc.Definitions, definitions...)
			continue
		}

		first := typeDefinitions[0]
		rest := typeDefinitions[1:]

		for _, typeDefinition := range rest {
			// TODO ここで ServiceName の情報が失われている気がするが…？
			schemaDoc.Definitions = append(schemaDoc.Definitions, typeDefinition.Definition)
		}

		// TODO ここで ServiceName の情報が失われている気がするが…？
		{
			copied := *first.Definition
			first.Definition = &copied
		}
		first.Definition.Interfaces = uniqueInterfaces
		schemaDoc.Definitions = append(schemaDoc.Definitions, first.Definition)
	}
	directiveDefinitionNames := make([]string, 0, len(directiveDefinitionsMap))
	for k := range directiveDefinitionsMap {
		directiveDefinitionNames = append(directiveDefinitionNames, k)
	}
	sort.Strings(directiveDefinitionNames)
	for _, directiveDefinitionName := range directiveDefinitionNames {
		definitions := directiveDefinitionsMap[directiveDefinitionName]
		for _, definition := range definitions {
			schemaDoc.Directives = append(schemaDoc.Directives, definition)
			break // 先頭を処理したら後続は全部内容同じ… のはず
		}
	}

	// TODO errors = validateSDL(definitionsDocument, schema, compositionRules);

	// Extend the schema with the extension definitions (as an AST node)
	typeExtensionNames := make([]string, 0, len(typeExtensionsMap))
	for k := range typeExtensionsMap {
		typeExtensionNames = append(typeExtensionNames, k)
	}
	sort.Strings(typeExtensionNames)
	for _, typeExtensionName := range typeExtensionNames {
		typeExtensions := typeExtensionsMap[typeExtensionName]
		for _, typeExtension := range typeExtensions {
			// TODO ここで ServiceName の情報が失われている気がするが…？
			schemaDoc.Extensions = append(schemaDoc.Extensions, typeExtension.Definition)
		}
	}

	// TODO   errors.push(...validateSDL(extensionsDocument, schema, compositionRules));

	// Remove apollo type system directives from the final schema
	// NOTE: …とoriginalではなっているが、これをやっちゃうとvalidatorが通らなくなる @key とかが普通に利用されているので
	//       もしやる場合、 *ast.Schema に対してこの操作をしたほうがよい
	//newDirectives := make(ast.DirectiveDefinitionList, 0, len(schemaDoc.Directives))
	//for _, directive := range schemaDoc.Directives {
	//	if isFederationDirective(directive) {
	//		continue
	//	}
	//	newDirectives = append(newDirectives, directive)
	//}
	//schemaDoc.Directives = newDirectives

	// TODO ここvalidateしてschemaにしてから返すか悩ましい
	return schemaDoc, errors
}

func composeServices(ctx context.Context, services []*ServiceDefinition) (*ast.Schema, string, []error) {
	buildMapsResult, err := buildMapsFromServiceList(ctx, services)
	if err != nil {
		return nil, "", []error{err}
	}

	typeToServiceMap := buildMapsResult.typeToServiceMap
	typeDefinitionsMap := buildMapsResult.typeDefinitionsMap
	typeExtensionsMap := buildMapsResult.typeExtensionsMap
	directiveDefinitionsMap := buildMapsResult.directiveDefinitionsMap
	externalFields := buildMapsResult.externalFields
	keyDirectivesMap := buildMapsResult.keyDirectivesMap
	valueTypes := buildMapsResult.valueTypes
	directiveMetadata := buildMapsResult.directiveMetadata

	schemaDoc, errors := buildSchemaFromDefinitionsAndExtensions(typeDefinitionsMap, typeExtensionsMap, directiveDefinitionsMap, directiveMetadata, services)

	// TODO: We should fix this to take non-default operation root types in
	// implementing services into account.
	// TODO originalのここの処理、Goではいらないと思っているんだけど正しい…？
	// TODO extensions.serviceList = serviceList

	// If multiple type definitions and extensions for the same type implement the
	// same interface, it will get added to the constructed object multiple times,
	// resulting in a schema validation error. We therefore need to remove
	// duplicate interfaces from object types manually.
	{
		transformObject := func(definitions ast.DefinitionList) ast.DefinitionList {
			newDefinitions := make(ast.DefinitionList, 0, len(definitions))

			for _, typ := range definitions {
				if typ.Kind != ast.Object {
					newDefinitions = append(newDefinitions, typ)
					continue
				}

				newInterfaces := make([]string, 0, len(typ.Interfaces))
			OUTER:
				for _, interfaceName := range typ.Interfaces {
					for _, knownName := range newInterfaces {
						if interfaceName == knownName {
							continue OUTER
						}
					}
					newInterfaces = append(newInterfaces, interfaceName)
				}
				if len(typ.Interfaces) != len(newInterfaces) {
					typ.Interfaces = newInterfaces
				}

				newDefinitions = append(newDefinitions, typ)
			}

			return newDefinitions
		}

		schemaDoc.Definitions = transformObject(schemaDoc.Definitions)
		schemaDoc.Extensions = transformObject(schemaDoc.Extensions)
	}

	// addFederationMetadataToSchemaNodes で schema になってないと処理が厳しい箇所があるのでここでvalidateすることにしてみる
	schema, gErr := validator.ValidateSchemaDocument(schemaDoc)
	if gErr != nil {
		errors = append(errors, gErr)
		return nil, "", errors
	}

	// TODO schema = lexicographicSortSchema(schema);

	// NOTE: addFederationMetadataToSchemaNodes では各nodeにextensionを追加していく
	//       この実装ではsupergraphSDLがあればQueryPlanが作成できて用が足りるのでこの処理は一旦実装しない
	// addFederationMetadataToSchemaNodes(schema, typeToServiceMap, externalFields, keyDirectivesMap, valueTypes, directiveDefinitionsMap, directiveMetadata)
	_ = typeToServiceMap
	_ = externalFields
	_ = keyDirectivesMap
	_ = valueTypes

	if len(errors) != 0 {
		return nil, "", errors
	}

	// NOTE: original は printSupergraphSdl で各種directiveを出力している schema には盛り込まれていないものが結構ある
	//       buildComposedSchema への入力として考えると schema に色々盛っていいし SDL に出す時に処理する必要もない(Goの実装だとprintのカスタマイズ性がかなり低いし

	var buf bytes.Buffer
	formatter.NewFormatter(&buf).FormatSchema(schema)

	return schema, buf.String(), nil
}
