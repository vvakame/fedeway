package federation

import "github.com/vektah/gqlparser/v2/ast"

type ExternalFieldDefinition struct {
	Field          *ast.FieldDefinition
	ParentTypeName string
	ServiceName    string
}

// original: [serviceName: string]: ReadonlyArray<SelectionNode>[] | undefined;
type ServiceNameToKeyDirectivesMap map[string][]ast.SelectionSet

type ServiceDefinition struct {
	TypeDefs *ast.SchemaDocument
	Name     string
	URL      string // optional
}
