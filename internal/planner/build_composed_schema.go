package planner

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"
	"github.com/vektah/gqlparser/v2/validator"
	"github.com/vvakame/fedeway/internal/federation"
	"github.com/vvakame/fedeway/internal/graphql"
)

func BuildComposedSchema(ctx context.Context, document *ast.SchemaDocument, metadata *federation.FederationMetadata) (*ComposedSchema, error) {
	// TODO metadata に依存しないようにする(@join__* からの情報でやりくりする)

	schema, gErr := validator.ValidateSchemaDocument(document)
	if gErr != nil {
		return nil, gErr
	}

	coreName := "core"
	coreDirective := schema.Directives[coreName]

	if coreDirective == nil {
		return nil, errors.New("expected core Schema, but can't find @core directive")
	}

	joinName := "join"
	getJoinDirective := func(name string) (*ast.DirectiveDefinition, error) {
		fullyQualifiedName := fmt.Sprintf("%s__%s", joinName, name)
		directive := schema.Directives[fullyQualifiedName]
		if directive == nil {
			return nil, fmt.Errorf("composed Schema should define @%s directive", fullyQualifiedName)
		}
		return directive, nil
	}

	ownerDirective, err := getJoinDirective("owner")
	if err != nil {
		return nil, err
	}
	typeDirective, err := getJoinDirective("type")
	if err != nil {
		return nil, err
	}
	fieldDirective, err := getJoinDirective("field")
	if err != nil {
		return nil, err
	}
	graphDirective, err := getJoinDirective("graph")
	if err != nil {
		return nil, err
	}

	graphEnumType := schema.Types[fmt.Sprintf("%s__Graph", joinName)]
	if graphEnumType == nil {
		return nil, fmt.Errorf("%s__Graph should be an enum", joinName)
	}

	cs := newComposedSchema(schema, metadata)

	graphMap := make(map[string]*Graph)
	cs.getSchemaMetadata().Graphs = graphMap

	for _, graphValue := range graphEnumType.EnumValues {
		name := graphValue.Name

		graphDirectiveArgs, err := getArgumentValuesForDirective(graphDirective, graphValue.Directives)
		if err != nil {
			return nil, err
		}
		if len(graphDirectiveArgs) == 0 {
			return nil, gqlerror.Errorf(
				"%s value %s in composed Schema should have a @%s directive",
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

		ownerDirectiveArgs, err := getArgumentValuesForDirective(ownerDirective, typ.Directives)
		if err != nil {
			return nil, err
		}

		typeMetadata := cs.getTypeMetadata(typ)
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
				return nil, gqlerror.Errorf(
					`programming error: found unexpected 'graph' argument value "%s" in @%s directive`,
					graphName, ownerDirective.Name,
				)
			}
			typeMetadata.IsValueType = false
			typeMetadata.GraphName = graph.Name
			typeMetadata.Keys = make(map[string]ast.SelectionSet)
		} else {
			typeMetadata.IsValueType = true
		}

		typeDirectivesArgs, err := getArgumentValuesForRepeatableDirective(typeDirective, typ.Directives)
		if err != nil {
			return nil, err
		}

		if typeMetadata.IsValueType && len(typeDirectivesArgs) != 0 {
			return nil, fmt.Errorf(
				`GraphQL type "%s" cannot have a @%s directive without an @%s directive`,
				typ.Name, typeDirective.Name, ownerDirective.Name,
			)
		}

		for _, typeDirectiveArgs := range typeDirectivesArgs {
			if len(typeDirectiveArgs) == 0 {
				return nil, fmt.Errorf(
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
				return nil, gqlerror.Errorf(
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
				return nil, err
			}

			typeMetadata.Keys[graph.Name] = keyFields
		}

		for _, fieldDef := range typ.Fields {
			fieldDirectiveArgs, err := getArgumentValuesForDirective(fieldDirective, fieldDef.Directives)
			if err != nil {
				return nil, err
			}

			if len(fieldDirectiveArgs) == 0 {
				continue
			}

			fieldMetadata := cs.getFieldMetadata(fieldDef)
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
					return nil, gqlerror.Errorf(
						`programming error: found unexpected 'graph' argument value "%s" in @%s directive`,
						graphName, fieldDirective.Name,
					)
				}
				fieldMetadata.GraphName = graph.Name
			}

			var requires ast.SelectionSet
			{
				v, ok := fieldDirectiveArgs["requires"]
				if ok {
					s, ok := v.(string)
					if ok {
						requires, err = parseFieldSet(s)
						if err != nil {
							return nil, err
						}
					} else {
						return nil, fmt.Errorf("unexpected 'requires' type: %T", v)
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
							return nil, err
						}
					} else {
						return nil, fmt.Errorf("unexpected 'provides' type: %T", v)
					}
				}
			}
			fieldMetadata.Provides = provides
		}
	}

	return cs, nil
}
