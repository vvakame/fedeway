package utils

import "github.com/vektah/gqlparser/v2/ast"

func IsTypeDefSubTypeOf(schema *ast.Schema, maybeSubType, superType *ast.Definition) bool {
	// NOTE this implementation is alternative of IsTypeSubTypeOf.
	// but this is not exactly same as it.
	// *ast.Definition doesn't have nullable and list information. just type.

	// Equivalent type is a valid subtype
	if maybeSubType == superType {
		return true
	}

	// If superType type is an abstract type, check if it is super type of maybeSubType.
	// Otherwise, the child type is not a valid subtype of the parent type.
	if !IsAbstractType(superType) {
		return false
	}
	if maybeSubType.Kind != ast.Interface && maybeSubType.Kind != ast.Object {
		return false
	}
	for _, def := range schema.GetPossibleTypes(superType) {
		if def == maybeSubType {
			return true
		}
	}
	return false
}

func IsAbstractType(def *ast.Definition) bool {
	switch def.Kind {
	case ast.Interface, ast.Union:
		return true
	default:
		return false
	}
}
