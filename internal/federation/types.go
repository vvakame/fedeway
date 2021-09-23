package federation

import "github.com/vektah/gqlparser/v2/ast"

type ServiceName = string

type ExternalFieldDefinition struct {
	Field          *ast.FieldDefinition
	ParentTypeName string
	ServiceName    string
}

// original: [serviceName: string]: ReadonlyArray<SelectionNode>[] | undefined;
type ServiceNameToKeyDirectivesMap map[string][]ast.SelectionSet

type FederationTypeMap map[*ast.Definition]*FederationType

func (ftm FederationTypeMap) Get(node *ast.Definition) *FederationType {
	federationType, ok := ftm[node]
	if !ok {
		federationType = &FederationType{
			Externals: make(map[string][]*ExternalFieldDefinition),
		}
		ftm[node] = federationType
	}

	return federationType
}

type FederationType struct {
	ServiceName     ServiceName
	Keys            ServiceNameToKeyDirectivesMap
	Externals       map[string][]*ExternalFieldDefinition
	IsValueType     bool
	DirectiveUsages DirectiveUsages
}

type FederationFieldMap map[*ast.FieldDefinition]*FederationField

func (ffm FederationFieldMap) Get(node *ast.FieldDefinition) *FederationField {
	federationField, ok := ffm[node]
	if !ok {
		federationField = &FederationField{}
		ffm[node] = federationField
	}

	return federationField
}

type FederationField struct {
	ServiceName        ServiceName
	ParentType         *ast.Definition
	Provides           ast.SelectionSet
	Requires           ast.SelectionSet
	BelongsToValueType bool
	DirectiveUsages    DirectiveUsages
}

type FederationDirectiveMap map[*ast.DirectiveDefinition]*FederationDirective

func (fdm FederationDirectiveMap) Get(node *ast.DirectiveDefinition) *FederationDirective {
	federationDirective, ok := fdm[node]
	if !ok {
		federationDirective = &FederationDirective{}
		fdm[node] = federationDirective
	}

	return federationDirective
}

type FederationDirective struct {
	DirectiveDefinitions map[string]*ast.DirectiveDefinition
}

type ServiceDefinition struct {
	TypeDefs *ast.SchemaDocument
	Name     string
	URL      string // optional
}
