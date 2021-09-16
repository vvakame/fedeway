package federation

import (
	"fmt"
	"reflect"

	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/parser"
)

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
	var typeDefsWithoutTypeSystemDirectives *ast.SchemaDocument
	{
		copied := *typeDefs
		typeDefsWithoutTypeSystemDirectives = &copied
	}
	directives := typeDefsWithoutTypeSystemDirectives.Directives
	typeDefsWithoutTypeSystemDirectives.Directives = nil
	for _, node := range directives {
		// The `deprecated` directive is an exceptional case that we want to leave in
		if node.Name == "deprecated" || node.Name == "specifiedBy" {
			typeDefsWithoutTypeSystemDirectives.Directives = append(typeDefsWithoutTypeSystemDirectives.Directives, node)
			continue
		}

		// TODO originalだと定義が外だしされてる
		switch node.Name {
		case "key", "extends", "external", "requires", "provides",
			"tag":
			continue
		default:
			typeDefsWithoutTypeSystemDirectives.Directives = append(typeDefsWithoutTypeSystemDirectives.Directives, node)
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

	if firstNode.Name != secondNode.Name {
		return false
	}
	if firstNode.Kind != secondNode.Kind {
		return false
	}
	if !reflect.DeepEqual(firstNode.Directives, secondNode.Directives) {
		return false
	}
	if !reflect.DeepEqual(firstNode.Interfaces, secondNode.Interfaces) {
		return false
	}
	if !reflect.DeepEqual(firstNode.Fields, secondNode.Fields) {
		return false
	}
	if !reflect.DeepEqual(firstNode.Types, secondNode.Types) {
		return false
	}
	if !reflect.DeepEqual(firstNode.EnumValues, secondNode.EnumValues) {
		return false
	}

	return true
}
