package planner

import "github.com/vektah/gqlparser/v2/ast"

type Field struct {
	Scope     *Scope
	FieldNode *ast.Field
	FieldDef  *ast.FieldDefinition
}

func (field *Field) MarshalLog() interface{} {
	result := make(map[string]interface{})
	result["Scope"] = field.Scope.MarshalLog()
	// TODO わかるデータ量もうちょい増やす
	result["FieldNode"] = field.FieldNode.Name
	result["FieldDef"] = field.FieldDef.Name
	return result
}

type FieldSet []*Field

func (fieldSet FieldSet) MarshalLog() interface{} {
	result := make([]interface{}, 0, len(fieldSet))
	for _, field := range fieldSet {
		result = append(result, field.MarshalLog())
	}
	return result
}

func matchesField(fieldA *Field, fieldB *Field) bool {
	// TODO: Compare parent type and arguments
	return fieldA.FieldDef.Name == fieldB.FieldDef.Name
}

func groupByResponseName(fields FieldSet) []FieldSet {
	// original return types are map[string][]FieldSet

	var fieldSets []FieldSet
	nameIdx := make(map[string]int)

	for _, field := range fields {
		name := getResponseName(field.FieldNode)
		idx, ok := nameIdx[name]
		if !ok {
			idx = len(nameIdx)
			nameIdx[name] = idx
			fieldSets = append(fieldSets, FieldSet{})
		}

		fieldSets[idx] = append(fieldSets[idx], field)
	}

	return fieldSets
}

func groupByScope(fields FieldSet) []FieldSet {
	// original return types are map[string][]FieldSet

	var fieldSets []FieldSet
	nameIdx := make(map[string]int)

	for _, field := range fields {
		name := field.Scope.identityKey()
		idx, ok := nameIdx[name]
		if !ok {
			idx = len(nameIdx)
			nameIdx[name] = idx
			fieldSets = append(fieldSets, FieldSet{})
		}

		fieldSets[idx] = append(fieldSets[idx], field)
	}

	return fieldSets
}

func selectionSetFromFieldSet(schema *ast.Schema, fields FieldSet, parentType *ast.Definition) ast.SelectionSet {
	var result ast.SelectionSet
	for _, fieldsByScope := range groupByScope(fields) {
		scope := fieldsByScope[0].Scope
		var selectionSet ast.SelectionSet
		for _, fieldsByResponseName := range groupByResponseName(fieldsByScope) {
			fieldNode := combineFields(schema, fieldsByResponseName).FieldNode
			selectionSet = append(selectionSet, fieldNode)
		}
		selection := wrapInInlineFragmentIfNeeded(selectionSet, scope, parentType)
		result = append(result, selection...)
	}

	return result
}

func wrapInInlineFragmentIfNeeded(selections ast.SelectionSet, scope *Scope, parentType *ast.Definition) ast.SelectionSet {
	shouldWrap := scope.enclosing != nil || parentType == nil || scope.isStrictlyRefining(parentType)
	var newSelections ast.SelectionSet
	if shouldWrap {
		newSelections = wrapInInlineFragment(selections, scope.parentType, scope.directives)
	} else {
		newSelections = selections
	}
	if scope.enclosing == nil {
		return newSelections
	}
	return wrapInInlineFragmentIfNeeded(newSelections, scope.enclosing, parentType)
}

func wrapInInlineFragment(selections ast.SelectionSet, typeCondition *ast.Definition, directives ast.DirectiveList) ast.SelectionSet {
	return ast.SelectionSet{
		&ast.InlineFragment{
			TypeCondition: typeCondition.Name,
			Directives:    directives,
			SelectionSet:  selections,
		},
	}
}

func combineFields(schema *ast.Schema, fields FieldSet) *Field {
	scope := fields[0].Scope
	fieldNode := fields[0].FieldNode
	fieldDef := fields[0].FieldDef
	returnType := getNamedType(schema, fieldDef.Type)

	if isCompositeType(returnType) {
		copied := *fieldNode
		var selectionSet []*ast.Field
		for _, field := range fields {
			selectionSet = append(selectionSet, field.FieldNode)
		}
		copied.SelectionSet = mergeSelectionSets(selectionSet)
		return &Field{
			Scope:     scope,
			FieldNode: &copied,
			FieldDef:  fieldDef,
		}
	}

	return &Field{
		Scope:     scope,
		FieldNode: fieldNode,
		FieldDef:  fieldDef,
	}
}

func mergeSelectionSets(fieldNodes []*ast.Field) ast.SelectionSet {
	var selections ast.SelectionSet

	for _, fieldNode := range fieldNodes {
		selections = append(selections, fieldNode.SelectionSet...)
	}

	return mergeFieldNodeSelectionSets(selections)
}

func mergeFieldNodeSelectionSets(selectionNodes ast.SelectionSet) ast.SelectionSet {
	var fieldNodes []*ast.Field
	var fragmentNodes []ast.Selection
	for _, selectionNode := range selectionNodes {
		switch selectionNode := selectionNode.(type) {
		case *ast.Field:
			fieldNodes = append(fieldNodes, selectionNode)
		default:
			fragmentNodes = append(fragmentNodes, selectionNode)
		}
	}

	// XXX: This code has more problems and should be replaced by proper recursive
	// selection set merging, but removing the unnecessary distinction between
	// aliased fields and non-aliased fields at least fixes the test.
	var mergedFieldNodes []*ast.Field
	{
		var names []string
		grouped := make(map[string][]*ast.Field)
		for _, fieldNode := range fieldNodes {
			name := fieldNode.Alias
			if name == "" {
				name = fieldNode.Name
			}
			fields, ok := grouped[name]
			if !ok {
				names = append(names, name)
			}
			fields = append(fields, fieldNode)
			grouped[name] = fields
		}

		for _, name := range names {
			nodesWithSameResponseName := grouped[name]
			copied := *nodesWithSameResponseName[0]
			node := &copied
			if len(node.SelectionSet) != 0 {
				node.SelectionSet = nil
				for _, node2 := range nodesWithSameResponseName {
					node.SelectionSet = append(node.SelectionSet, node2.SelectionSet...)
				}
			}

			mergedFieldNodes = append(mergedFieldNodes, node)
		}
	}

	var result ast.SelectionSet
	for _, node := range mergedFieldNodes {
		result = append(result, node)
	}
	for _, node := range fragmentNodes {
		result = append(result, node)
	}

	return result
}

func getNamedType(schema *ast.Schema, typ *ast.Type) *ast.Definition {
	return schema.Types[typ.Name()]
}

func getResponseName(field *ast.Field) string {
	if field.Alias != "" {
		return field.Alias
	}

	return field.Name
}
