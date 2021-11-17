package graphql

import (
	"sort"

	"github.com/vektah/gqlparser/v2/ast"
)

func LexicographicSortSchema(schema *ast.Schema) *ast.Schema {
	sortArgumentList := func(args ast.ArgumentList) {
		sort.Slice(args, func(i, j int) bool {
			argA := args[i]
			argB := args[j]
			return argA.Name < argB.Name
		})
	}
	sortDirectiveList := func(directives ast.DirectiveList) {
		sort.Slice(directives, func(i, j int) bool {
			directiveA := directives[i]
			directiveB := directives[j]
			return directiveA.Name < directiveB.Name
		})

		for _, directive := range directives {
			sortArgumentList(directive.Arguments)
		}
	}
	sortArgumentDefinitionList := func(argDefs ast.ArgumentDefinitionList) {
		sort.Slice(argDefs, func(i, j int) bool {
			argDefA := argDefs[i]
			argDefB := argDefs[j]
			return argDefA.Name < argDefB.Name
		})

		for _, argDef := range argDefs {
			sortDirectiveList(argDef.Directives)
		}
	}
	sortFieldList := func(fields ast.FieldList) {
		sort.Slice(fields, func(i, j int) bool {
			fieldA := fields[i]
			fieldB := fields[j]
			return fieldA.Name < fieldB.Name
		})

		for _, field := range fields {
			sortArgumentDefinitionList(field.Arguments)
			sortDirectiveList(field.Directives)
		}
	}
	sortEnumValueList := func(enumValues ast.EnumValueList) {
		sort.Slice(enumValues, func(i, j int) bool {
			enumValueA := enumValues[i]
			enumValueB := enumValues[j]
			return enumValueA.Name < enumValueB.Name
		})

		for _, enumValue := range enumValues {
			sortDirectiveList(enumValue.Directives)
		}
	}
	sortDefinition := func(def *ast.Definition) {
		if def == nil {
			return
		}

		sortDirectiveList(def.Directives)
		sort.Strings(def.Interfaces)
		sortFieldList(def.Fields)
		sort.Strings(def.Types)
		sortEnumValueList(def.EnumValues)
	}
	sortDefinitionList := func(defs []*ast.Definition) {
		sort.Slice(defs, func(i, j int) bool {
			defA := defs[i]
			defB := defs[j]
			return defA.Name < defB.Name
		})
	}

	sortDefinition(schema.Query)
	sortDefinition(schema.Mutation)
	sortDefinition(schema.Subscription)
	for _, def := range schema.Types {
		sortDefinition(def)
	}
	for _, defs := range schema.PossibleTypes {
		sortDefinitionList(defs)
	}
	for _, defs := range schema.Implements {
		sortDefinitionList(defs)
	}

	return schema
}
