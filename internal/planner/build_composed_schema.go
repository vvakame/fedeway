package planner

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"
	"github.com/vektah/gqlparser/v2/validator"
	"github.com/vvakame/fedeway/internal/graphql"
)

func buildComposedSchema(ctx context.Context, document *ast.SchemaDocument) (*ast.Schema, *metadataHolder, error) {
	schema, gErr := validator.ValidateSchemaDocument(document)
	if gErr != nil {
		return nil, nil, gErr
	}

	coreName := "core"
	coreDirective := schema.Directives[coreName]

	if coreDirective == nil {
		return nil, nil, errors.New("expected core schema, but can't find @core directive")
	}

	joinName := "join"
	getJoinDirective := func(name string) (*ast.DirectiveDefinition, error) {
		fullyQualifiedName := fmt.Sprintf("%s__%s", joinName, name)
		directive := schema.Directives[fullyQualifiedName]
		if directive == nil {
			return nil, fmt.Errorf("composed schema should define @%s directive", fullyQualifiedName)
		}
		return directive, nil
	}

	ownerDirective, err := getJoinDirective("owner")
	if err != nil {
		return nil, nil, err
	}
	typeDirective, err := getJoinDirective("type")
	if err != nil {
		return nil, nil, err
	}
	fieldDirective, err := getJoinDirective("field")
	if err != nil {
		return nil, nil, err
	}
	graphDirective, err := getJoinDirective("graph")
	if err != nil {
		return nil, nil, err
	}

	graphEnumType := schema.Types[fmt.Sprintf("%s__Graph", joinName)]
	if graphEnumType == nil {
		return nil, nil, fmt.Errorf("%s__Graph should be an enum", joinName)
	}

	mh := newMetadataHolder(schema)

	graphMap := make(map[string]*Graph)
	mh.setSchemaMetadata(&FederationSchemaMetadata{Graphs: graphMap})

	for _, graphValue := range graphEnumType.EnumValues {
		name := graphValue.Name

		graphDirectiveArgs, err := getArgumentValuesForDirective(graphDirective, graphValue.Directives)
		if err != nil {
			return nil, nil, err
		}
		if len(graphDirectiveArgs) == 0 {
			return nil, nil, gqlerror.Errorf(
				"%s value %s in composed schema should have a @%s directive",
				graphEnumType.Name, name, graphDirective.Name,
			)
		}

		var graphName string
		{
			v, ok := graphDirectiveArgs["name"]
			if ok {
				s, ok := v.(string)
				if ok {
					graphName = s
				}
			}
		}
		var url string
		{
			v, ok := graphDirectiveArgs["url"]
			if ok {
				s, ok := v.(string)
				if ok {
					url = s
				}
			}
		}

		graphMap[name] = &Graph{
			Name: graphName,
			URL:  url,
		}
	}

	typeNames := make([]string, 0, len(schema.Types))
	for typeName := range schema.Types {
		typeNames = append(typeNames, typeName)
	}
	sort.Strings(typeNames)

	for _, typeName := range typeNames {
		typ := schema.Types[typeName]

		if graphql.IsIntrospectionType(typ.Name) {
			continue
		}
		if typ.Kind != ast.Object {
			// We currently only allow join spec directives on object types.
			continue
		}

		// TODO type.astNode assert?

		ownerDirectiveArgs, err := getArgumentValuesForDirective(ownerDirective, typ.Directives)
		if err != nil {
			return nil, nil, err
		}

		var typeMetadata FederationTypeMetadata
		if len(ownerDirectiveArgs) != 0 {
			var graphName string
			{
				v, ok := ownerDirectiveArgs["graph"]
				if ok {
					s, ok := v.(string)
					if ok {
						graphName = s
					}
				}
			}
			graph := graphMap[graphName]
			if graph == nil {
				return nil, nil, gqlerror.Errorf(
					`programming error: found unexpected 'graph' argument value "%s" in @%s directive`,
					graphName, ownerDirective.Name,
				)
			}
			typeMetadata = &FederationEntityTypeMetadata{
				GraphName: graph.Name,
				Keys:      make(map[string]ast.SelectionSet),
			}
		} else {
			typeMetadata = &FederationValueTypeMetadata{}
		}

		mh.setTypeMetadata(typ, typeMetadata)

		typeDirectivesArgs, err := getArgumentValuesForRepeatableDirective(typeDirective, typ.Directives)
		if err != nil {
			return nil, nil, err
		}

		if _, ok := typeMetadata.(*FederationEntityTypeMetadata); !ok && len(typeDirectivesArgs) != 0 {
			// TODO 条件式これであってるか怪しい…
			return nil, nil, fmt.Errorf(
				`GraphQL type "%s" cannot have a @%s directive without an @%s directive`,
				typ.Name, typeDirective.Name, ownerDirective.Name,
			)
		}

		for _, typeDirectiveArgs := range typeDirectivesArgs {
			if len(typeDirectiveArgs) == 0 {
				return nil, nil, fmt.Errorf(
					`GraphQL type "%s" must provide a 'graph' argument to the @%s directive`,
					typ.Name, typeDirective.Name,
				)
			}

			var graphName string
			{
				v, ok := typeDirectiveArgs["graph"]
				if ok {
					s, ok := v.(string)
					if ok {
						graphName = s
					}
				}
			}
			graph := graphMap[graphName]
			if graph == nil {
				return nil, nil, gqlerror.Errorf(
					`programming error: found unexpected 'graph' argument value "%s" in @%s directive`,
					graphName, typeDirective.Name,
				)
			}

			var source string
			{
				v, ok := typeDirectiveArgs["key"]
				if ok {
					s, ok := v.(string)
					if ok {
						source = s
					}
				}
			}
			keyFields, err := parseFieldSet(source)
			if err != nil {
				return nil, nil, err
			}

			entityTypeMetadata, ok := typeMetadata.(*FederationEntityTypeMetadata)
			if !ok {
				return nil, nil, fmt.Errorf("unexpected type %T", typeMetadata)
			}

			entityTypeMetadata.Keys[graph.Name] = keyFields
		}

		for _, fieldDef := range typ.Fields {
			//if len(fieldDef.Directives) == 0 {
			//	// TODO この条件マジ？
			//	return nil, nil, gqlerror.ErrorPosf(
			//		fieldDef.Position,
			//		`field "%s.%s" should contain AST nodes`,
			//		typ.Name, fieldDef.Name,
			//	)
			//}

			fieldDirectiveArgs, err := getArgumentValuesForDirective(fieldDirective, fieldDef.Directives)
			if err != nil {
				return nil, nil, err
			}

			if len(fieldDirectiveArgs) == 0 {
				continue
			}

			var fieldMetadata *FederationFieldMetadata
			var graphName string
			{
				v, ok := fieldDirectiveArgs["graph"]
				if ok {
					s, ok := v.(string)
					if ok {
						graphName = s
					}
				}
			}
			if graphName != "" {
				graph := graphMap[graphName]
				if graph == nil {
					return nil, nil, gqlerror.Errorf(
						`programming error: found unexpected 'graph' argument value "%s" in @%s directive`,
						graphName, fieldDirective.Name,
					)
				}
				fieldMetadata = &FederationFieldMetadata{
					GraphName: graph.Name,
				}
			} else {
				fieldMetadata = &FederationFieldMetadata{}
			}

			mh.setFieldMetadata(fieldDef, fieldMetadata)

			var requires ast.SelectionSet
			{
				v, ok := fieldDirectiveArgs["requires"]
				if ok {
					s, ok := v.(string)
					if ok {
						requires, err = parseFieldSet(s)
						if err != nil {
							return nil, nil, err
						}
					} else {
						return nil, nil, fmt.Errorf("unexpected 'requires' type: %T", v)
					}
				}
			}
			fieldMetadata.Requires = requires

			var provides ast.SelectionSet
			{
				v, ok := fieldDirectiveArgs["provides"]
				if ok {
					s, ok := v.(string)
					if ok {
						provides, err = parseFieldSet(s)
						if err != nil {
							return nil, nil, err
						}
					} else {
						return nil, nil, fmt.Errorf("unexpected 'provides' type: %T", v)
					}
				}
			}
			fieldMetadata.Provides = provides
		}
	}

	return schema, mh, nil
}
