package federation

import (
	"bytes"
	"fmt"
	"reflect"

	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/parser"
)

// for formatter
var blankPos = &ast.Position{
	Src: &ast.Source{
		BuiltIn: false,
	},
}

func stripExternalFieldsFromTypeDefs(typeDefs *ast.SchemaDocument, serviceName string) (*ast.SchemaDocument, []*ExternalFieldDefinition) {
	var strippedFields []*ExternalFieldDefinition

	var typeDefsWithoutExternalFields *ast.SchemaDocument
	{
		copied := *typeDefs
		typeDefsWithoutExternalFields = &copied
	}
	// removeExternalFieldsFromExtensionVisitor
	extensions := make(ast.DefinitionList, 0, len(typeDefsWithoutExternalFields.Extensions))
	extensions = append(extensions, typeDefsWithoutExternalFields.Extensions...)
	typeDefsWithoutExternalFields.Extensions = nil
	for _, node := range extensions {
		switch node.Kind {
		case ast.Object, ast.Interface:
			{
				copied := *node
				node = &copied
			}
			if len(node.Fields) != 0 {
				field := node.Fields
				node.Fields = nil
				for _, field := range field {
					externalDirectives := field.Directives.ForNames("external")

					if len(externalDirectives) > 0 {
						strippedFields = append(strippedFields, &ExternalFieldDefinition{
							Field:          field,
							ParentTypeName: node.Name,
							ServiceName:    serviceName,
						})
					} else {
						node.Fields = append(node.Fields, field)
					}
				}
			}

			typeDefsWithoutExternalFields.Extensions = append(typeDefsWithoutExternalFields.Extensions, node)

		default:
			typeDefsWithoutExternalFields.Extensions = append(typeDefsWithoutExternalFields.Extensions, node)
		}
	}

	return typeDefsWithoutExternalFields, strippedFields
}

func stripTypeSystemDirectivesFromTypeDefs(typeDefs *ast.SchemaDocument) *ast.SchemaDocument {
	isKeep := func(node *ast.Directive) bool {
		switch node.Name {
		case "deprecated", "specifiedBy":
			// The `deprecated` directive is an exceptional case that we want to leave in
			return true
		case "key", "extends", "external", "requires", "provides", "tag":
			// apolloTypeSystemDirectives
			return true
		default:
			return false
		}
	}
	filterDirectives := func(directives ast.DirectiveList) ast.DirectiveList {
		newDirectives := make(ast.DirectiveList, 0, len(directives))
		for _, directive := range directives {
			if isKeep(directive) {
				newDirectives = append(newDirectives, directive)
			}
		}
		return newDirectives
	}

	var typeDefsWithoutTypeSystemDirectives *ast.SchemaDocument
	{
		copied := *typeDefs
		typeDefsWithoutTypeSystemDirectives = &copied
	}
	for _, schemaDef := range typeDefsWithoutTypeSystemDirectives.Schema {
		schemaDef.Directives = filterDirectives(schemaDef.Directives)
	}
	for _, schemaDef := range typeDefsWithoutTypeSystemDirectives.SchemaExtension {
		schemaDef.Directives = filterDirectives(schemaDef.Directives)
	}
	for _, def := range typeDefsWithoutTypeSystemDirectives.Definitions {
		def.Directives = filterDirectives(def.Directives)
		for _, fieldDef := range def.Fields {
			fieldDef.Directives = filterDirectives(fieldDef.Directives)
			for _, argDef := range fieldDef.Arguments {
				argDef.Directives = filterDirectives(argDef.Directives)
			}
		}
		for _, enumValue := range def.EnumValues {
			enumValue.Directives = filterDirectives(enumValue.Directives)
		}
	}
	for _, def := range typeDefsWithoutTypeSystemDirectives.Extensions {
		def.Directives = filterDirectives(def.Directives)
		for _, fieldDef := range def.Fields {
			fieldDef.Directives = filterDirectives(fieldDef.Directives)
			for _, argDef := range fieldDef.Arguments {
				argDef.Directives = filterDirectives(argDef.Directives)
			}
		}
		for _, enumValue := range def.EnumValues {
			enumValue.Directives = filterDirectives(enumValue.Directives)
		}
	}

	return typeDefsWithoutTypeSystemDirectives
}

// For lack of a "home of federation utilities", this function is copy/pasted
// verbatim across the federation, gateway, and query-planner packages. Any changes
// made here should be reflected in the other two locations as well.

// @param source A string representing a FieldSet
// @returns A parsed FieldSet
func parseSelections(source string) (ast.SelectionSet, error) {
	queryDocument, gErr := parser.ParseQuery(&ast.Source{
		Input: "{" + source + "}",
	})
	if gErr != nil {
		return nil, gErr
	}

	// TODO position should be strip
	return queryDocument.Operations[0].SelectionSet, nil
}

func hasMatchingFieldInDirectives(directives []*ast.Directive, fieldNameToMatch string, namedType *ast.Definition) (bool, error) {
	if namedType == nil {
		return false, nil
	}

	// for each key directive, get the fields arg
	for _, keyDirective := range directives {
		if len(keyDirective.Arguments) == 0 {
			continue
		}

		if keyDirective.Arguments[0].Value.Kind != ast.StringValue && keyDirective.Arguments[0].Value.Kind != ast.BlockValue {
			// filter out any null/undefined args
			continue
		}

		selections, err := parseSelections(keyDirective.Arguments[0].Value.Raw)
		if err != nil {
			return false, err
		}

		for _, selection := range selections {
			field, ok := selection.(*ast.Field)
			if !ok {
				continue
			}

			if field.Name == fieldNameToMatch {
				return true, nil
			}
		}
	}

	return false, nil
}

func logServiceAndType(serviceName, typeName, fieldName string) string {
	if fieldName != "" {
		fieldName = fmt.Sprintf(".%s", fieldName)
	}
	return fmt.Sprintf("[%s] %s%s ->", serviceName, typeName, fieldName)
}

// Used for finding a field on the `schema` that returns `typeToFind`
//
// Used in validation of external directives to find uses of a field in a
// `@provides` on another type.
func findFieldsThatReturnType(schema *ast.Schema, typeToFind *ast.Definition) ast.FieldList {
	if typeToFind.Kind != ast.Object {
		return nil
	}

	var fieldsThatReturnType ast.FieldList

	for _, selectionSetType := range schema.Types {
		// for our purposes, only object types have fields that we care about.
		if selectionSetType.Kind != ast.Object {
			continue
		}

		// push fields that have return `typeToFind`
		for _, field := range selectionSetType.Fields {
			fieldReturnType := schema.Types[field.Type.Name()]
			if fieldReturnType == typeToFind {
				fieldsThatReturnType = append(fieldsThatReturnType, field)
			}
		}
	}

	return fieldsThatReturnType
}

// Searches recursively to see if a selection set includes references to
// `typeToFind.fieldToFind`.
//
// Used in validation of external fields to find where/if a field is referenced
// in a nested selection set for `@requires`
//
// For every selection, look at the root of the selection's type.
// 1. If it's the type we're looking for, check its fields.
//    Return true if field matches. Skip to step 3 if not
// 2. If it's not the type we're looking for, skip to step 3
// 3. Get the return type for each subselection and run this function on the subselection.
func selectionIncludesField(
	schema *ast.Schema,
	selections ast.SelectionSet,
	selectionSetType *ast.Definition, // type which applies to `selections`
	typeToFind *ast.Definition, // type where the `@external` lives
	fieldToFind string,
) bool {

	for _, selection := range selections {
		var selectionName string
		switch selection := selection.(type) {
		case *ast.Field:
			selectionName = selection.Name
		case *ast.FragmentSpread, *ast.InlineFragment:
			continue
		}

		// if the selected field matches the fieldname we're looking for,
		// and its type is correct, we're done. Return true;
		if selectionName == fieldToFind &&
			selectionSetType.Name == typeToFind.Name {
			return true
		}

		// if the field selection has a subselection, check each field recursively

		// check to make sure the parent type contains the field
		if selectionName == "" {
			continue
		}
		var typeIncludesField bool
		for _, field := range selectionSetType.Fields {
			if field.Name == selectionName {
				typeIncludesField = true
				break
			}
		}
		if !typeIncludesField {
			continue
		}

		// get the return type of the selection
		var returnType *ast.Definition
		for _, field := range selectionSetType.Fields {
			if field.Name != selectionName {
				continue
			}
			returnType = schema.Types[field.Type.Name()]
			break
		}

		if returnType == nil || returnType.Kind != ast.Object {
			continue
		}

		subselections := selection.(*ast.Field).SelectionSet

		// using the return type of a given selection and all the subselections,
		// recursively search for matching selections. typeToFind and fieldToFind
		// stay the same
		if len(subselections) != 0 {
			selectionDoesIncludeField := selectionIncludesField(
				schema,
				subselections,
				returnType,
				typeToFind,
				fieldToFind,
			)
			if selectionDoesIncludeField {
				return true
			}
		}
	}

	return false
}

// Create a map of { fieldName: serviceName } for each field.
func mapFieldNamesToServiceName(fields ast.FieldList, serviceName string) map[string]string {
	result := make(map[string]string)
	for _, field := range fields {
		result[field.Name] = serviceName
	}
	return result
}

func mapEnumNamesToServiceName(fields ast.EnumValueList, serviceName string) map[string]string {
	// fork mapFieldNamesToServiceName for enum

	result := make(map[string]string)
	for _, field := range fields {
		result[field.Name] = serviceName
	}
	return result
}

func findDirectivesOnNode(node *ast.Definition, directiveName string) ast.DirectiveList {
	var directiveList ast.DirectiveList
	for _, directive := range node.Directives {
		if directive.Name == directiveName {
			directiveList = append(directiveList, directive)
		}
	}

	return directiveList
}

func typeNodesAreEquivalent(firstNode *ast.Definition, secondNode *ast.Definition) bool {
	// NOTE オリジナルの実装をだいぶ簡素化しているがベタ移植は難しいので一旦目的に合致してそうな実装を書く

	isDirectiveEqual := func(first, second *ast.Directive) bool {
		if first.Name != secondNode.Name {
			return false
		}
		if len(first.Arguments) != len(second.Arguments) {
			return false
		}
		for i := 0; i < len(first.Arguments); i++ {
			firstArg := first.Arguments[i]
			secondArg := second.Arguments[i]
			if firstArg.Name != secondArg.Name {
				return false
			}
			// TODO Value?
		}
		return true
	}
	isDirectiveListEqual := func(first, second ast.DirectiveList) bool {
		if len(first) != len(second) {
			return false
		}
		for i := 0; i < len(first); i++ {
			if !isDirectiveEqual(first[i], second[i]) {
				return false
			}
		}
		return true
	}

	if firstNode.Name != secondNode.Name {
		return false
	}
	if firstNode.Kind != secondNode.Kind {
		return false
	}
	if !isDirectiveListEqual(firstNode.Directives, secondNode.Directives) {
		return false
	}
	if !reflect.DeepEqual(firstNode.Interfaces, secondNode.Interfaces) {
		return false
	}
	{
		if len(firstNode.Fields) != len(secondNode.Fields) {
			return false
		}
		for i := 0; i < len(firstNode.Fields); i++ {
			firstField := firstNode.Fields[i]
			secondField := secondNode.Fields[i]
			if firstField.Name != secondField.Name {
				return true
			}
			if firstField.Type.String() != secondField.Type.String() {
				return false
			}
		}
	}
	if !reflect.DeepEqual(firstNode.Types, secondNode.Types) {
		return false
	}
	{
		if len(firstNode.EnumValues) != len(secondNode.EnumValues) {
			return false
		}
		for i := 0; i < len(firstNode.EnumValues); i++ {
			firstValue := firstNode.EnumValues[i]
			secondValue := secondNode.EnumValues[i]
			if firstValue.Name != secondValue.Name {
				return false
			}
			if !isDirectiveListEqual(firstValue.Directives, secondNode.Directives) {
				return false
			}
		}
	}

	return true
}

func isFederationDirective(directiveName string) bool {
	for _, node := range federationDirectives {
		if node.Name == directiveName {
			return true
		}
	}

	return false
}

func printSelectionSet(selections ast.SelectionSet) string {
	// alias とかはサポートしない…一旦… めんどいから…

	var buf bytes.Buffer

	pad := func() {
		if buf.Len() != 0 {
			buf.WriteString(" ")
		}
	}
	var p func(selections ast.SelectionSet)
	p = func(selections ast.SelectionSet) {
		for _, selection := range selections {
			switch v := selection.(type) {
			case *ast.Field:
				pad()
				buf.WriteString(v.Name)
				if len(v.SelectionSet) != 0 {
					pad()
					buf.WriteString("{")
					p(v.SelectionSet)
					pad()
					buf.WriteString("}")
				}

			default:
				panic(fmt.Errorf("unsupported Selection type: %T", selection))
			}
		}
	}

	p(selections)

	return buf.String()
}
