package engine

import "github.com/vektah/gqlparser/v2/ast"

// type FragmentMap = { [fragmentName: string]: FragmentDefinitionNode };
type FragmentMap map[string]*ast.FragmentDefinition

type OperationContext struct {
	Schema    *ast.Schema
	Operation ast.OperationDefinition
	Fragments FragmentMap
}
