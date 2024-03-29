package planner

import (
	"errors"
	"fmt"

	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"
	"github.com/vektah/gqlparser/v2/parser"
	"github.com/vvakame/fedeway/internal/graphql"
)

func getFieldDef(schema *ast.Schema, parentType *ast.Definition, fieldName string) *ast.FieldDefinition {
	if fieldName == graphql.SchemaMetaFieldDef.Name && schema.Query == parentType {
		return graphql.SchemaMetaFieldDef
	}
	if fieldName == graphql.TypeMetaFieldDef.Name && schema.Query == parentType {
		return graphql.TypeMetaFieldDef
	}
	if fieldName == graphql.TypeNameMetaFieldDef.Name && (parentType.Kind == ast.Object ||
		parentType.Kind == ast.Interface ||
		parentType.Kind == ast.Union) {
		return graphql.TypeNameMetaFieldDef
	}
	if parentType.Kind == ast.Object ||
		parentType.Kind == ast.Interface {

		return parentType.Fields.ForName(fieldName)
	}

	return nil
}

func containsType(defs []*ast.Definition, elem *ast.Definition) bool {
	for _, def := range defs {
		if def == elem {
			return true
		}
	}

	return false
}

func parseFieldSet(source string) (ast.SelectionSet, error) {
	// gqlparser が FieldSet をparseする関数を公開してくれていないのでごまかす
	s := &ast.Source{
		Input: fmt.Sprintf("query { %s }", source),
	}
	doc, err := parser.ParseQuery(s)
	if err != nil {
		return nil, err
	}

	selectionSet := doc.Operations[0].SelectionSet

	var stripPos func(selectionSet ast.SelectionSet)
	stripPos = func(selectionSet ast.SelectionSet) {
		for _, selection := range selectionSet {
			switch selection := selection.(type) {
			case *ast.Field:
				selection.Position = nil
				stripPos(selection.SelectionSet)
			case *ast.FragmentSpread:
				selection.Position = nil
			case *ast.InlineFragment:
				selection.Position = nil
				stripPos(selection.SelectionSet)
			}
		}
	}

	// no necessary to remove position. but this position is not exact.
	stripPos(selectionSet)

	return selectionSet, nil
}

func getArgumentValuesForDirective(directiveDef *ast.DirectiveDefinition, directives ast.DirectiveList) (map[string]interface{}, error) {
	if directiveDef.IsRepeatable {
		return nil, errors.New("use getArgumentValuesForRepeatableDirective for repeatable directives")
	}

	if len(directives) == 0 {
		return make(map[string]interface{}), nil
	}

	var directiveNode *ast.Directive
	for _, directiveNode2 := range directives {
		if directiveNode2.Name == directiveDef.Name {
			directiveNode = directiveNode2
			break
		}
	}

	if directiveNode == nil {
		return make(map[string]interface{}), nil
	}

	return getArgumentValues(directiveDef, directiveNode, nil)
}

func getArgumentValuesForRepeatableDirective(directiveDef *ast.DirectiveDefinition, directives ast.DirectiveList) ([]map[string]interface{}, error) {
	if len(directives) == 0 {
		return make([]map[string]interface{}, 0), nil
	}

	var directiveNodes []*ast.Directive
	for _, directiveNode := range directives {
		if directiveNode.Name == directiveDef.Name {
			directiveNodes = append(directiveNodes, directiveNode)
		}
	}

	result := make([]map[string]interface{}, 0, len(directiveNodes))
	for _, directiveNode := range directiveNodes {
		argValues, err := getArgumentValues(directiveDef, directiveNode, nil)
		if err != nil {
			return nil, err
		}
		result = append(result, argValues)
	}

	return result, nil
}

func getArgumentValues(def *ast.DirectiveDefinition, node *ast.Directive, variableValues map[string]interface{}) (map[string]interface{}, error) {
	if variableValues == nil {
		variableValues = make(map[string]interface{})
	}

	coercedValues := make(map[string]interface{})

	argumentNodes := node.Arguments

	for _, argDef := range def.Arguments {
		name := argDef.Name
		argType := argDef.Type
		argumentNode := argumentNodes.ForName(name)

		if argumentNode == nil {
			if argDef.DefaultValue != nil {
				coercedValues[name] = argDef.DefaultValue
			} else if argType.NonNull {
				return nil, gqlerror.ErrorPosf(node.Position, `argument "%s" of required type "%s" was not provided`, name, argType.String())
			}
			continue
		}

		valueNode := argumentNode.Value
		isNull := valueNode.Kind == ast.NullValue

		if valueNode.Kind == ast.Variable {
			variableName := valueNode.VariableDefinition.Variable
			if variableValues[variableName] == nil {
				if argDef.DefaultValue != nil {
					coercedValues[name] = argDef.DefaultValue
				} else if argType.NonNull {
					return nil, gqlerror.ErrorPosf(
						valueNode.Position,
						`argument "%s" of required type "%s" was provided the variable "$%s" which was not provided a runtime value`,
						name, argType.String(), variableName,
					)
				}
				continue
			}
			isNull = variableValues[variableName] == nil
		}

		if isNull && argType.NonNull {
			return nil, gqlerror.ErrorPosf(
				valueNode.Position,
				`argument "%s" of non-null type "%s" must not be null`,
				name, argType.String(),
			)
		}

		coercedValue, err := valueNode.Value(variableValues)
		if err != nil {
			return nil, err
		}
		// NOTE Goは null と undefined の区別がつかないので undefined の時エラーにすることができない

		coercedValues[name] = coercedValue
	}

	return coercedValues, nil
}

func isCompositeType(def *ast.Definition) bool {
	switch def.Kind {
	case ast.Object, ast.Interface, ast.Union:
		return true
	default:
		return false
	}
}
