package graphql

import (
	"context"

	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

type extendSchemaImpl struct {
	typeDefs          ast.DefinitionList
	typeExtensionsMap map[string]ast.DefinitionList
	directiveDefs     ast.DirectiveDefinitionList
	schemaDef         *ast.SchemaDefinition
	schemaExtensions  ast.SchemaDefinitionList
	typeMap           map[string]*ast.Definition
}

var stdTypeMap = func() map[string]*ast.Definition {
	m := make(map[string]*ast.Definition)
	for _, node := range SpecifiedScalarTypes {
		m[node.Name] = node
	}
	for _, node := range IntrospectionTypes {
		m[node.Name] = node
	}

	return m
}()

func ExtendSchemaAssumeValidDSL(ctx context.Context, schema *ast.Schema, documentAST *ast.SchemaDocument) (*ast.Schema, *gqlerror.Error) {
	// NOTE original と signature は結構変わっている… このパッケージのここにあるべき関数ではないかも
	// 重複した定義のdedupeとかを実はやってた
	// see internal/reference/testbed/src/federationDedupeDefs.ts

	// 実験的に確かめたjs版の仕様
	// type Foo が複数登場した場合、後勝ち directiveやinterfaceなど (mergeされない)
	// extend の処理は type などの処理がすべて終わった後
	// field 名が被った場合は後勝ち 型やdirectiveなど (mergeされない replace)

	// Collect the type definitions and extensions found in the document.
	impl := &extendSchemaImpl{
		typeExtensionsMap: make(map[string]ast.DefinitionList),
		typeMap:           make(map[string]*ast.Definition),
	}

	// New directives and types are separate because a directives and types can
	// have the same name. For example, a type named "skip".

	for _, def := range documentAST.Schema {
		impl.schemaDef = def
	}
	for _, def := range documentAST.SchemaExtension {
		impl.schemaExtensions = append(impl.schemaExtensions, def)
	}
	for _, def := range documentAST.Definitions {
		impl.typeDefs = append(impl.typeDefs, def)
	}
	for _, def := range documentAST.Extensions {
		extendedTypeName := def.Name
		impl.typeExtensionsMap[extendedTypeName] = append(impl.typeExtensionsMap[extendedTypeName], def)
	}
	for _, def := range documentAST.Directives {
		impl.directiveDefs = append(impl.directiveDefs, def)
	}

	// If this document contains no new types, extensions, or directives then
	// return the same unmodified GraphQLSchema instance.
	if len(impl.typeExtensionsMap) == 0 &&
		len(impl.typeDefs) == 0 &&
		len(impl.directiveDefs) == 0 &&
		len(impl.schemaExtensions) == 0 &&
		impl.schemaDef == nil {
		return schema, nil
	}

	for _, existingType := range schema.Types {
		typ, gErr := impl.extendNamedType(existingType)
		if gErr != nil {
			return nil, gErr
		}
		impl.typeMap[existingType.Name] = typ
	}

	for _, typeNode := range impl.typeDefs {
		name := typeNode.Name
		if node := stdTypeMap[name]; node != nil {
			impl.typeMap[name] = node
		} else {
			impl.typeMap[name] = impl.buildType(typeNode)
		}
	}

	// Get the extended root operation types.
	operationTypesQuery := schema.Query
	if operationTypesQuery != nil {
		operationTypesQuery = impl.replaceNamedType(schema.Query)
	}
	operationTypesMutation := schema.Mutation
	if operationTypesMutation != nil {
		operationTypesMutation = impl.replaceNamedType(schema.Mutation)
	}
	operationTypesSubscription := schema.Subscription
	if operationTypesSubscription != nil {
		operationTypesSubscription = impl.replaceNamedType(schema.Subscription)
	}
	if impl.schemaDef != nil {
		query, mutation, subscription, gErr := impl.getOperationTypes(ast.SchemaDefinitionList{impl.schemaDef})
		if gErr != nil {
			return nil, gErr
		}
		if query != nil {
			operationTypesQuery = query
		}
		if mutation != nil {
			operationTypesMutation = mutation
		}
		if subscription != nil {
			operationTypesSubscription = subscription
		}
	}
	{
		query, mutation, subscription, gErr := impl.getOperationTypes(impl.schemaExtensions)
		if gErr != nil {
			return nil, gErr
		}
		if query != nil {
			operationTypesQuery = query
		}
		if mutation != nil {
			operationTypesMutation = mutation
		}
		if subscription != nil {
			operationTypesSubscription = subscription
		}
	}

	// Then produce and return a Schema config with these types.
	newSchema := &ast.Schema{
		Query:         operationTypesQuery,
		Mutation:      operationTypesMutation,
		Subscription:  operationTypesSubscription,
		Types:         impl.typeMap,
		Directives:    make(map[string]*ast.DirectiveDefinition),
		PossibleTypes: make(map[string][]*ast.Definition),
		Implements:    make(map[string][]*ast.Definition),
	}
	for name, directive := range schema.Directives {
		newSchema.Directives[name] = impl.replaceDirective(directive)
	}
	for _, directive := range impl.directiveDefs {
		newSchema.Directives[directive.Name] = impl.buildDirective(directive)
	}
	for _, def := range newSchema.Types {
		switch def.Kind {
		case ast.Union:
			for _, t := range def.Types {
				newSchema.AddPossibleType(def.Name, newSchema.Types[t])
				newSchema.AddImplements(t, def)
			}
		case ast.InputObject, ast.Object:
			for _, intf := range def.Interfaces {
				newSchema.AddPossibleType(intf, def)
				newSchema.AddImplements(def.Name, newSchema.Types[intf])
			}
			newSchema.AddPossibleType(def.Name, def)
		}
	}

	// TODO validator.ValidateSchemaDocument 相当のことをやりたい気がしなくもない

	return newSchema, nil
}

func (impl *extendSchemaImpl) extendScalarType(typ *ast.Definition) (*ast.Definition, *gqlerror.Error) {
	// NOTE originalだとextensionASTNodesを色々やってる
	// TODO specifiedByURL
	// 基本的にはシンプルに後勝ちでよいはず
	return typ, nil
}

func (impl *extendSchemaImpl) extendObjectType(typ *ast.Definition) (*ast.Definition, *gqlerror.Error) {
	return impl.extendObjectAndInterfaceType(typ)
}

func (impl *extendSchemaImpl) extendInterfaceType(typ *ast.Definition) (*ast.Definition, *gqlerror.Error) {
	return impl.extendObjectAndInterfaceType(typ)
}

func (impl *extendSchemaImpl) extendObjectAndInterfaceType(typ *ast.Definition) (*ast.Definition, *gqlerror.Error) {
	extensions := impl.typeExtensionsMap[typ.Name]

	for _, ext := range extensions {
		typ.Interfaces = append(typ.Interfaces, ext.Interfaces...)
	}

	var newFields ast.FieldList
	upsertField := func(field *ast.FieldDefinition) {
		for idx, exist := range newFields {
			if exist.Name == field.Name {
				newFields[idx] = field
				return
			}
		}
		newFields = append(newFields, field)
	}
	for _, field := range typ.Fields {
		replaced, gErr := impl.extendField(field)
		if gErr != nil {
			return nil, gErr
		}

		upsertField(replaced)
	}
	extFields, gErr := impl.buildFields(extensions)
	if gErr != nil {
		return nil, gErr
	}
	for _, field := range extFields {
		upsertField(field)
	}

	typ.Fields = newFields

	return typ, nil
}

func (impl *extendSchemaImpl) extendUnionType(typ *ast.Definition) (*ast.Definition, *gqlerror.Error) {
	extensions := impl.typeExtensionsMap[typ.Name]

	for _, ext := range extensions {
		typ.Types = append(typ.Types, ext.Types...)
	}

	return typ, nil
}

func (impl *extendSchemaImpl) extendEnumType(typ *ast.Definition) (*ast.Definition, *gqlerror.Error) {
	extensions := impl.typeExtensionsMap[typ.Name]

	var newEnumValues ast.EnumValueList
	upsertField := func(node *ast.EnumValueDefinition) {
		for idx, exist := range newEnumValues {
			if exist.Name == node.Name {
				newEnumValues[idx] = node
				return
			}
		}
		newEnumValues = append(newEnumValues, node)
	}
	for _, enumValue := range typ.EnumValues {
		upsertField(enumValue)
	}
	for _, ext := range extensions {
		for _, enumValue := range ext.EnumValues {
			upsertField(enumValue)
		}
	}

	typ.EnumValues = newEnumValues

	return typ, nil
}

func (impl *extendSchemaImpl) extendInputObjectType(typ *ast.Definition) (*ast.Definition, *gqlerror.Error) {
	extensions := impl.typeExtensionsMap[typ.Name]

	var newFields ast.FieldList
	upsertField := func(field *ast.FieldDefinition) {
		for idx, exist := range newFields {
			if exist.Name == field.Name {
				newFields[idx] = field
				return
			}
		}
		newFields = append(newFields, field)
	}
	for _, field := range typ.Fields {
		replaced, gErr := impl.extendField(field)
		if gErr != nil {
			return nil, gErr
		}

		upsertField(replaced)
	}
	extFields, gErr := impl.buildFields(extensions)
	if gErr != nil {
		return nil, gErr
	}
	for _, field := range extFields {
		upsertField(field)
	}

	typ.Fields = newFields

	return typ, nil
}

func (impl *extendSchemaImpl) extendNamedType(typ *ast.Definition) (*ast.Definition, *gqlerror.Error) {
	if IsIntrospectionType(typ.Name) || isSpecifiedScalarType(typ.Name) {
		// Builtin types are not extended.
		return typ, nil
	}
	switch typ.Kind {
	case ast.Scalar:
		return impl.extendScalarType(typ)
	case ast.Object:
		return impl.extendObjectType(typ)
	case ast.Interface:
		return impl.extendInterfaceType(typ)
	case ast.Union:
		return impl.extendUnionType(typ)
	case ast.Enum:
		return impl.extendEnumType(typ)
	case ast.InputObject:
		return impl.extendInputObjectType(typ)
	default:
		return nil, gqlerror.Errorf("unexpected kind: %s", typ.Kind)
	}
}

func (impl *extendSchemaImpl) extendField(typ *ast.FieldDefinition) (*ast.FieldDefinition, *gqlerror.Error) {
	// TODO originalは色々やってる
	return typ, nil
}

func (impl *extendSchemaImpl) buildType(astNode *ast.Definition) *ast.Definition {
	// TODO ここの実装originalに比べると相当乱暴
	// 基本的には 後勝ち が実装されていればよいはず…
	return astNode
}

func (impl *extendSchemaImpl) replaceNamedType(typ *ast.Definition) *ast.Definition {
	// Note: While this could make early assertions to get the correctly
	// typed values, that would throw immediately while type system
	// validation with validateSchema() will produce more actionable results.
	return impl.typeMap[typ.Name]
}

func (impl *extendSchemaImpl) getNamedType(typeName string) (*ast.Definition, *gqlerror.Error) {
	if def := stdTypeMap[typeName]; def != nil {
		return def, nil
	}
	if def := impl.typeMap[typeName]; def != nil {
		return def, nil
	}
	return nil, gqlerror.Errorf("unknown type: %s", typeName)
}

func (impl *extendSchemaImpl) getOperationTypes(nodes ast.SchemaDefinitionList) (*ast.Definition, *ast.Definition, *ast.Definition, *gqlerror.Error) {
	var query, mutation, subscription *ast.Definition

	for _, node := range nodes {
		operationTypesNodes := node.OperationTypes

		for _, operationType := range operationTypesNodes {
			// Note: While this could make early assertions to get the correctly
			// typed values below, that would throw immediately while type system
			// validation with validateSchema() will produce more actionable results.
			def, gErr := impl.getNamedType(operationType.Type)
			if gErr != nil {
				return nil, nil, nil, gErr
			}
			switch operationType.Operation {
			case ast.Query:
				query = def
			case ast.Mutation:
				mutation = def
			case ast.Subscription:
				subscription = def
			}
		}
	}

	return query, mutation, subscription, nil
}

func (impl *extendSchemaImpl) replaceDirective(directive *ast.DirectiveDefinition) *ast.DirectiveDefinition {
	// TODO originalは色々やってる ...けどこれでいいのでは？
	return directive
}

func (impl *extendSchemaImpl) buildDirective(directive *ast.DirectiveDefinition) *ast.DirectiveDefinition {
	// original: convert DirectiveDefinitionNode to GraphQLDirective
	// TODO originalは色々やってる ...けどこれでいいのでは？
	return directive
}

func (impl *extendSchemaImpl) buildFields(nodes ast.DefinitionList) (ast.FieldList, *gqlerror.Error) {
	// original: buildFieldMap
	var fields ast.FieldList
	upsertField := func(field *ast.FieldDefinition) {
		for idx, exist := range fields {
			if exist.Name == field.Name {
				fields[idx] = field
				return
			}
		}
		fields = append(fields, field)
	}

	for _, node := range nodes {
		nodeFields := node.Fields

		for _, field := range nodeFields {
			args, gErr := impl.buildArguments(field.Arguments)
			if gErr != nil {
				return nil, gErr
			}
			newField := &ast.FieldDefinition{
				// Note: While this could make assertions to get the correctly typed
				// value, that would throw immediately while type system validation
				// with validateSchema() will produce more actionable results.
				Name:        field.Name,
				Type:        field.Type,
				Description: field.Description,
				Arguments:   args,
				Directives:  field.Directives, // TODO ここでやる必要がある？
				Position:    field.Position,
			}
			upsertField(newField)
		}
	}

	return fields, nil
}

func (impl *extendSchemaImpl) buildArguments(args ast.ArgumentDefinitionList) (ast.ArgumentDefinitionList, *gqlerror.Error) {
	// TODO originalは色々やってる ...けどこれでいいのでは？
	return args, nil
}
