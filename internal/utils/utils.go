package utils

import "github.com/vektah/gqlparser/v2/ast"

func IsTypeSubTypeOf(schema *ast.Schema, maybeSubType *ast.Definition, superType *ast.Definition) bool {
	if maybeSubType == superType {
		return true
	}

	// TODO
	return false
}
