package federation

import (
	"bytes"
	"context"
	"fmt"
	"sort"

	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/formatter"
	"github.com/vektah/gqlparser/v2/gqlerror"
	"github.com/vektah/gqlparser/v2/validator"
	"github.com/vvakame/fedeway/internal/graphql"
)

func emptyQueryDefinition() *TypeDefinitionEntity {
	return &TypeDefinitionEntity{
		ServiceName: "",
		Definition: &ast.Definition{
			Kind: ast.Object,
			Name: "Query",
		},
	}
}

func emptyMutationDefinition() *TypeDefinitionEntity {
	return &TypeDefinitionEntity{
		ServiceName: "",
		Definition: &ast.Definition{
			Kind: ast.Object,
			Name: "Mutation",
		},
	}
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

func (vts ValueTypes) Has(name string) bool {
	for _, vt := range vts {
		if vt == name {
			return true
		}
	}
	return false
}

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
					continue
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
					continue
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
	{
		_, ok := typeDefinitionsMap["Query"]
		if !ok {
			typeDefinitionsMap["Query"] = []*TypeDefinitionEntity{emptyQueryDefinition()}
		}
	}
	{
		_, extOK := typeExtensionsMap["Mutation"]
		_, defOK := typeDefinitionsMap["Mutation"]
		if extOK && !defOK {
			typeDefinitionsMap["Mutation"] = []*TypeDefinitionEntity{emptyMutationDefinition()}
		}
	}

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

func buildSchemaFromDefinitionsAndExtensions(ctx context.Context, typeDefinitionsMap TypeDefinitionsMap, typeExtensionsMap TypeDefinitionsMap, directiveDefinitionsMap DirectiveDefinitionsMap, directiveMetadata *DirectiveMetadata, serviceList []*ServiceDefinition) (*ast.Schema, []error) {
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

	var schema *ast.Schema
	{
		schemaDoc := &ast.SchemaDocument{}
		schemaDoc.Directives = append(schemaDoc.Directives, CoreDirective)
		schemaDoc.Directives = append(schemaDoc.Directives, joinFieldDirective)
		schemaDoc.Directives = append(schemaDoc.Directives, joinTypeDirective)
		schemaDoc.Directives = append(schemaDoc.Directives, joinOwnerDirective)
		schemaDoc.Directives = append(schemaDoc.Directives, joinGraphDirective)
		schemaDoc.Directives = append(schemaDoc.Directives, graphql.SpecifiedDirectives...)
		schemaDoc.Directives = append(schemaDoc.Directives, federationDirectives...)
		schemaDoc.Directives = append(schemaDoc.Directives, otherKnownDirectiveDefinitionsToInclude...)
		schemaDoc.Definitions = append(schemaDoc.Definitions, fieldSetScalar, joinGraphEnum)
		// original では SpecifiedScalarTypes, IntrospectionTypes, CorePurpose は追加していないがここでは必要
		schemaDoc.Definitions = append(schemaDoc.Definitions, graphql.SpecifiedScalarTypes...)
		schemaDoc.Definitions = append(schemaDoc.Definitions, graphql.IntrospectionTypes...)
		schemaDoc.Definitions = append(schemaDoc.Definitions, CorePurpose)

		var gErr *gqlerror.Error
		schema, gErr = validator.ValidateSchemaDocument(schemaDoc)
		if gErr != nil {
			errors = append(errors, gErr)
			return nil, errors
		}
	}

	// Extend the blank schema with the base type definitions (as an AST node)
	definitionsDocument := &ast.SchemaDocument{}
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
			definitionsDocument.Definitions = append(definitionsDocument.Definitions, definitions...)
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
			definitionsDocument.Definitions = append(definitionsDocument.Definitions, definitions...)
			continue
		}

		first := typeDefinitions[0]
		rest := typeDefinitions[1:]

		for _, typeDefinition := range rest {
			definitionsDocument.Definitions = append(definitionsDocument.Definitions, typeDefinition.Definition)
		}

		{
			copied := *first.Definition
			first.Definition = &copied
		}
		first.Definition.Interfaces = uniqueInterfaces
		definitionsDocument.Definitions = append(definitionsDocument.Definitions, first.Definition)
	}
	directiveDefinitionNames := make([]string, 0, len(directiveDefinitionsMap))
	for k := range directiveDefinitionsMap {
		directiveDefinitionNames = append(directiveDefinitionNames, k)
	}
	sort.Strings(directiveDefinitionNames)
	for _, directiveDefinitionName := range directiveDefinitionNames {
		definitions := directiveDefinitionsMap[directiveDefinitionName]
		for _, definition := range definitions {
			definitionsDocument.Directives = append(definitionsDocument.Directives, definition)
			break // 先頭を処理したら後続は全部内容同じ… のはず
		}
	}

	// TODO errors = validateSDL(definitionsDocument, schema, compositionRules);

	schema, gErr := graphql.ExtendSchemaAssumeValidDSL(ctx, schema, definitionsDocument)
	if gErr != nil {
		// TODO original だと catch してスルーしてるけど？
		errors = append(errors, gErr)
		return nil, errors
	}

	// Extend the schema with the extension definitions (as an AST node)
	extensionsDocument := &ast.SchemaDocument{}
	typeExtensionNames := make([]string, 0, len(typeExtensionsMap))
	for k := range typeExtensionsMap {
		typeExtensionNames = append(typeExtensionNames, k)
	}
	sort.Strings(typeExtensionNames)
	for _, typeExtensionName := range typeExtensionNames {
		typeExtensions := typeExtensionsMap[typeExtensionName]
		for _, typeExtension := range typeExtensions {
			extensionsDocument.Extensions = append(extensionsDocument.Extensions, typeExtension.Definition)
		}
	}

	// TODO errors.push(...validateSDL(extensionsDocument, schema, compositionRules));

	schema, gErr = graphql.ExtendSchemaAssumeValidDSL(ctx, schema, extensionsDocument)
	if gErr != nil {
		errors = append(errors, gErr)
		return nil, errors
	}

	// TODO   errors.push(...validateSDL(extensionsDocument, schema, compositionRules));

	// Remove apollo type system directives from the final schema
	newDirectiveMap := make(map[string]*ast.DirectiveDefinition)
	for name, directive := range schema.Directives {
		if isFederationDirective(directive.Name) {
			continue
		}
		newDirectiveMap[name] = directive
	}
	schema.Directives = newDirectiveMap

	return schema, errors
}

// Using the various information we've collected about the schema, augment the
// `schema` itself with `federation` metadata to the types and fields
func addFederationMetadataToSchemaNodes(schema *ast.Schema, typeToServiceMap TypeToServiceMap, externalFields []*ExternalFieldDefinition, keyDirectivesMap KeyDirectivesMap, valueTypes ValueTypes, directiveDefinitionsMap DirectiveDefinitionsMap, directiveMetadata *DirectiveMetadata, graphNameToEnumValueName map[string]string) (*FederationMetadata, error) {
	// original では addFederationMetadataToSchemaNodes という名前
	// もともとの動作原理をざっくり解説しておく
	//   @join__owner, @join__type, @join__field, @join__graph あたりをschemaに追加するのが最終目的
	//   addFederationMetadataToSchemaNodes では ASTNodeのextensionsにfederationという属性を追加したい
	//   そして、その後の printSupergraphSdl で metadata とここまでの schema を組み合わせて最終的なSDLを生成している
	// ただ、Go版では既存の型に外部からフィールドを追加したりできないこと、schema 自体はresolverを持たない(executableではない)などの差がある
	// なので、ここでは metadata の生成と print でのSDL出力という構成を改め、 schema に直接各種データを盛り付けていくことにする

	federationTypeMap := FederationTypeMap{}
	federationFieldMap := FederationFieldMap{}
	federationDirectiveMap := FederationDirectiveMap{}

	for typeName, entity := range typeToServiceMap {
		owningService := entity.OwningService
		extensionFieldsToOwningServiceMap := entity.ExtensionFieldsToOwningServiceMap

		namedType := schema.Types[typeName]
		if namedType == nil {
			continue
		}

		// Extend each type in the GraphQLSchema with the serviceName that owns it
		// and the key directives that belong to it
		isValue := valueTypes.Has(typeName)
		var serviceName string
		if !isValue {
			serviceName = owningService
		}

		federationType := federationTypeMap.Get(namedType)
		federationType.ServiceName = serviceName
		federationType.IsValueType = isValue
		federationType.Keys = keyDirectivesMap[typeName]

		// For object types, add metadata for all the @provides directives from its fields
		if namedType.Kind == ast.Object {
			for _, field := range namedType.Fields {
				providesDirective := field.Directives.ForName("provides")
				if providesDirective != nil && len(providesDirective.Arguments) != 0 && providesDirective.Arguments[0].Value.Kind == ast.StringValue {
					provides, err := parseSelections(providesDirective.Arguments[0].Value.Raw)
					if err != nil {
						return nil, err
					}
					federationField := federationFieldMap.Get(field)
					federationField.ParentType = namedType
					federationField.ServiceName = serviceName
					federationField.Provides = provides
					federationField.BelongsToValueType = isValue
				}
			}
		}

		// For extension fields, do 2 things:
		// 1. Add serviceName metadata to all fields that belong to a type extension
		// 2. add metadata from the @requires directive for each field extension
		for fieldName, extendingServiceName := range extensionFieldsToOwningServiceMap {
			// TODO: Why don't we need to check for non-object types here
			if namedType.Kind == ast.Object {
				field := namedType.Fields.ForName(fieldName)
				if field == nil {
					continue
				}

				fieldMeta := federationFieldMap.Get(field)
				fieldMeta.ParentType = namedType
				fieldMeta.ServiceName = extendingServiceName

				requiresDirective := field.Directives.ForName("requires")
				if requiresDirective != nil && len(requiresDirective.Arguments) != 0 && requiresDirective.Arguments[0].Value.Kind == ast.StringValue {
					requires, err := parseSelections(requiresDirective.Arguments[0].Value.Raw)
					if err != nil {
						return nil, err
					}
					fieldMeta.Requires = requires
				}
			}
		}
	}

	// add externals metadata
	for _, field := range externalFields {
		namedType := schema.Types[field.ParentTypeName]
		if namedType == nil {
			continue
		}

		federationType := federationTypeMap.Get(namedType)
		fields := federationType.Externals[field.ServiceName]
		fields = append(fields, field)
		federationType.Externals[field.ServiceName] = fields
	}

	// add all definitions of a specific directive for validation later
	for directiveName := range directiveDefinitionsMap {
		directive := schema.Directives[directiveName]
		if directive == nil {
			continue
		}

		federationDirective := federationDirectiveMap.Get(directive)
		federationDirective.DirectiveDefinitions = directiveDefinitionsMap[directiveName]
	}

	// currently this is only used to capture @tag metadata but could be used
	// for others directives in the future
	directiveMetadata.applyMetadataToSupergraphSchema(schema)

	// Apollo addition: print @join__owner and @join__type usages
	// printTypeJoinDirectives
	for namedType, federationType := range federationTypeMap {
		ownerService := federationType.ServiceName
		keys := federationType.Keys

		if ownerService == "" && len(keys) == 0 {
			continue
		}

		// Separate owner @keys from the rest of the @keys so we can print them
		// adjacent to the @owner directive.
		ownerKeys := keys[ownerService]
		restKeys := make(map[string][]ast.SelectionSet)
		for k, v := range keys {
			if k == ownerService {
				continue
			}
			restKeys[k] = v
		}

		// We don't want to print an owner for interface types
		shouldPrintOwner := namedType.Kind == ast.Object

		ownerGraphEnumValue := graphNameToEnumValueName[ownerService]
		if ownerGraphEnumValue == "" {
			return nil, fmt.Errorf("unexpected enum value missing for subgraph %s", ownerService)
		}

		if shouldPrintOwner {
			namedType.Directives = append(namedType.Directives, &ast.Directive{
				Name: "join__owner",
				Arguments: ast.ArgumentList{
					{
						Name: "graph",
						Value: &ast.Value{
							Raw:  ownerGraphEnumValue,
							Kind: ast.EnumValue,
						},
					},
				},
			})
		}

		addJoinTypeDirective := func(service string, keys []ast.SelectionSet) error {
			for _, selections := range keys {
				typeGraphEnumValue := graphNameToEnumValueName[service]
				if typeGraphEnumValue == "" {
					return fmt.Errorf("unexpected enum value missing for subgraph %s", service)
				}

				namedType.Directives = append(namedType.Directives, &ast.Directive{
					Name: "join__type",
					Arguments: ast.ArgumentList{
						{
							Name: "graph",
							Value: &ast.Value{
								Raw:  typeGraphEnumValue,
								Kind: ast.EnumValue,
							},
						},
						{
							Name: "key",
							Value: &ast.Value{
								Raw:  printSelectionSet(selections),
								Kind: ast.StringValue,
							},
						},
					},
				})
			}
			return nil
		}
		err := addJoinTypeDirective(ownerService, ownerKeys)
		if err != nil {
			return nil, err
		}
		restNames := make([]string, 0, len(restKeys))
		for service := range restKeys {
			restNames = append(restNames, service)
		}
		sort.Strings(restNames)
		for _, service := range restNames {
			keys := restKeys[service]
			err = addJoinTypeDirective(service, keys)
			if err != nil {
				return nil, err
			}
		}
	}

	// Apollo addition: print @join__field directives
	// printJoinFieldDirectives
	for field, federationField := range federationFieldMap {
		parentType := federationField.ParentType

		joinFieldDirective := &ast.Directive{
			Name: "join__field",
		}

		serviceName := federationField.ServiceName

		// For entities (which we detect through the existence of `keys`),
		// although the join spec doesn't currently require `@join__field(graph:)` when
		// a field can be resolved from the owning service, the code we used
		// previously did include it in those cases. And especially since we want to
		// remove type ownership, I think it makes to keep the same behavior.
		if federationType := federationTypeMap.Get(parentType); serviceName == "" && len(federationType.Keys) != 0 {
			serviceName = federationType.ServiceName
		}

		if serviceName != "" {
			enumValue := graphNameToEnumValueName[serviceName]
			if enumValue == "" {
				return nil, fmt.Errorf("unexpected enum value missing for subgraph %s", serviceName)
			}

			joinFieldDirective.Arguments = append(joinFieldDirective.Arguments, &ast.Argument{
				Name: "graph",
				Value: &ast.Value{
					Raw:  enumValue,
					Kind: ast.EnumValue,
				},
			})
		}

		if len(federationField.Requires) > 0 {
			joinFieldDirective.Arguments = append(joinFieldDirective.Arguments, &ast.Argument{
				Name: "requires",
				Value: &ast.Value{
					Raw:  printSelectionSet(federationField.Requires),
					Kind: ast.StringValue,
				},
			})
		}

		if len(federationField.Provides) > 0 {
			joinFieldDirective.Arguments = append(joinFieldDirective.Arguments, &ast.Argument{
				Name: "provides",
				Value: &ast.Value{
					Raw:  printSelectionSet(federationField.Provides),
					Kind: ast.StringValue,
				},
			})
		}

		// A directive without arguments isn't valid (nor useful).
		if len(joinFieldDirective.Arguments) < 1 {
			continue
		}

		field.Directives = append(field.Directives, joinFieldDirective)
	}

	return &FederationMetadata{
		FederationTypeMap:      federationTypeMap,
		FederationFieldMap:     federationFieldMap,
		FederationDirectiveMap: federationDirectiveMap,
	}, nil
}

func composeServices(ctx context.Context, services []*ServiceDefinition) (*ast.Schema, string, *FederationMetadata, []error) {
	buildMapsResult, err := buildMapsFromServiceList(ctx, services)
	if err != nil {
		return nil, "", nil, []error{err}
	}

	typeToServiceMap := buildMapsResult.typeToServiceMap
	typeDefinitionsMap := buildMapsResult.typeDefinitionsMap
	typeExtensionsMap := buildMapsResult.typeExtensionsMap
	directiveDefinitionsMap := buildMapsResult.directiveDefinitionsMap
	externalFields := buildMapsResult.externalFields
	keyDirectivesMap := buildMapsResult.keyDirectivesMap
	valueTypes := buildMapsResult.valueTypes
	directiveMetadata := buildMapsResult.directiveMetadata

	schema, errors := buildSchemaFromDefinitionsAndExtensions(ctx, typeDefinitionsMap, typeExtensionsMap, directiveDefinitionsMap, directiveMetadata, services)

	// TODO: We should fix this to take non-default operation root types in
	// implementing services into account.
	// TODO originalのここの処理、Goではいらないと思っているんだけど正しい…？
	// TODO extensions.serviceList = serviceList

	// If multiple type definitions and extensions for the same type implement the
	// same interface, it will get added to the constructed object multiple times,
	// resulting in a schema validation error. We therefore need to remove
	// duplicate interfaces from object types manually.
	for _, typ := range schema.Types {
		if typ.Kind != ast.Object {
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
	}

	// TODO schema = lexicographicSortSchema(schema);

	// NOTE: addFederationMetadataToSchemaNodes では各nodeにextensionを追加していく
	//       この実装ではsupergraphSDLがあればQueryPlanが作成できて用が足りるのでこの処理は一旦実装しない
	// TODO ここで graphNameToEnumValueName ひねり出すのやめたほうがよさそう
	graphNameToEnumValueName, _ := getJoinGraphEnum(services)
	// NOTE: original では schema に変更を加えていない (@join__type とかが付随していない)
	metadata, err := addFederationMetadataToSchemaNodes(schema, typeToServiceMap, externalFields, keyDirectivesMap, valueTypes, directiveDefinitionsMap, directiveMetadata, graphNameToEnumValueName)
	if err != nil {
		errors = append(errors, err)
		return nil, "", nil, errors
	}

	if len(errors) != 0 {
		return nil, "", nil, errors
	}

	// NOTE: original は printSupergraphSdl で各種directiveを出力している schema には盛り込まれていないものが結構ある
	//       buildComposedSchema への入力として考えると schema に色々盛っていいし SDL に出す時に処理する必要もない(Goの実装だとprintのカスタマイズ性がかなり低いし

	// TODO originalではこの時点で @key とかの FederationDirective をどこにも保持しなくなっている
	// ここで除去するのは正しくない気がするが一旦そうする
	excludeFederationDirective := func(directives ast.DirectiveList) ast.DirectiveList {
		newDirectives := make(ast.DirectiveList, 0, len(directives))
		for _, directive := range directives {
			if isFederationDirective(directive.Name) {
				continue
			}
			newDirectives = append(newDirectives, directive)
		}
		return newDirectives
	}
	for _, typ := range schema.Types {
		typ.Directives = excludeFederationDirective(typ.Directives)
		for _, def := range typ.Fields {
			def.Directives = excludeFederationDirective(def.Directives)
			for _, def := range def.Arguments {
				def.Directives = excludeFederationDirective(def.Directives)
			}
		}
		for _, def := range typ.EnumValues {
			def.Directives = excludeFederationDirective(def.Directives)
		}
	}

	var buf bytes.Buffer
	formatter.NewFormatter(&buf).FormatSchema(schema)

	return schema, buf.String(), metadata, nil
}
