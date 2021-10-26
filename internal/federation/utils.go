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

	// TODO positionをstripしたほうがよいといえばよい
	return queryDocument.Operations[0].SelectionSet, nil
}

func logServiceAndType(serviceName, typeName, fieldName string) string {
	if fieldName != "" {
		fieldName = fmt.Sprintf(".%s", fieldName)
	}
	return fmt.Sprintf("[%s] %s%s -> ", serviceName, typeName, fieldName)
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
