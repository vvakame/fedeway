package execute

import (
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vvakame/fedeway/internal/utils"
)

// originalにはない構造だけど、Goは map が入れた順に出てくる仕様ではないので順番を保つためにこうするしかない
type Fields struct {
	Names    []string
	FieldMap map[string][]*ast.Field
}

func (fs *Fields) get(name string) []*ast.Field {
	if fs.FieldMap == nil {
		return nil
	}
	return fs.FieldMap[name]
}

func (fs *Fields) set(name string, fields []*ast.Field) {
	if fs.FieldMap == nil {
		fs.FieldMap = make(map[string][]*ast.Field)
	}
	_, ok := fs.FieldMap[name]
	if !ok {
		fs.Names = append(fs.Names, name)
	}
	fs.FieldMap[name] = fields
}

func collectFields(schema *ast.Schema, fragments ast.FragmentDefinitionList, variableValues map[string]interface{}, runtimeType *ast.Definition, selectionSet ast.SelectionSet, fields *Fields, visitedFragmentNames map[string]struct{}) *Fields {
	for _, selection := range selectionSet {
		switch selection := selection.(type) {
		case *ast.Field:
			if !shouldIncludeNode(variableValues, selection, selection.Directives) {
				continue
			}
			name := getFieldEntryKey(selection)
			fieldList := fields.get(name)
			fieldList = append(fieldList, selection)
			fields.set(name, fieldList)

		case *ast.InlineFragment:
			if !shouldIncludeNode(variableValues, selection, nil) ||
				!doesFragmentConditionMatch(schema, selection.TypeCondition, runtimeType) {
				continue
			}
			collectFields(
				schema,
				fragments,
				variableValues,
				runtimeType,
				selection.SelectionSet,
				fields,
				visitedFragmentNames,
			)

		case *ast.FragmentSpread:
			fragName := selection.Name
			if _, ok := visitedFragmentNames[fragName]; ok || !shouldIncludeNode(variableValues, selection, nil) {
				continue
			}
			visitedFragmentNames[fragName] = struct{}{}
			fragment := fragments.ForName(fragName)
			if fragment == nil ||
				!doesFragmentConditionMatch(schema, fragment.TypeCondition, runtimeType) {
				continue
			}
			collectFields(
				schema,
				fragments,
				variableValues,
				runtimeType,
				fragment.SelectionSet,
				fields,
				visitedFragmentNames,
			)
		}
	}

	return fields
}

// Determines if a field should be included based on the `@include` and `@skip`
// directives, where `@skip` has higher precedence than `@include`.
func shouldIncludeNode(variableValues map[string]interface{}, node ast.Selection, directives ast.DirectiveList) bool {
	if skip := directives.ForName("skip"); skip != nil {
		if v, ok := skip.ArgumentMap(variableValues)["if"].(bool); ok && v {
			return false
		}
	}

	if include := directives.ForName("include"); include != nil {
		if v, ok := include.ArgumentMap(variableValues)["if"].(bool); ok && !v {
			return false
		}
	}

	return true
}

// Determines if a fragment is applicable to the given type.
func doesFragmentConditionMatch(schema *ast.Schema, typeConditionNode string, typ *ast.Definition) bool {
	if typeConditionNode == "" {
		return true
	}
	conditionalType := schema.Types[typeConditionNode]
	if typ == conditionalType {
		return true
	}
	if utils.IsAbstractType(conditionalType) {
		// TODO 本来は schema.isSubType(conditionalType, type) だったんだけどこれでいいのかしら？
		return utils.IsTypeDefSubTypeOf(schema, typ, conditionalType)
	}
	return false
}

// Implements the logic to compute the key of a given field's entry
func getFieldEntryKey(node *ast.Field) string {
	if node.Alias != "" {
		return node.Alias
	}
	return node.Name
}
