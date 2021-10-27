package utils

import (
	"fmt"

	"github.com/vektah/gqlparser/v2/ast"
)

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

func IsObjectType(def *ast.Definition) bool {
	switch def.Kind {
	case ast.Object:
		return true
	default:
		return false
	}
}

func IsLeadType(def *ast.Definition) bool {
	switch def.Kind {
	case ast.Scalar, ast.Enum:
		return true
	default:
		return false
	}
}

func IsAbstractType(def *ast.Definition) bool {
	switch def.Kind {
	case ast.Interface, ast.Union:
		return true
	default:
		return false
	}
}

func IsObjectLike(value interface{}) bool {
	if value == nil {
		return false
	}
	if _, ok := value.(map[string]interface{}); ok {
		return true
	}
	return false
}

func DeepMerge(target, source interface{}) interface{} {
	if source == nil {
		return target
	}

	targetMap, ok := target.(map[string]interface{})
	if !ok {
		panic(fmt.Sprintf("target is not map[string]interface{} type. actual: %T", target))
	}
	sourceMap, ok := source.(map[string]interface{})
	if !ok {
		panic(fmt.Sprintf("source is not map[string]interface{} type. actual: %T", source))
	}

	for key := range sourceMap {
		if sourceMap[key] == nil {
			continue
		} else if v, ok := sourceMap[key].(string); ok && v == "__proto__" {
			// this block maybe removable on go code.
			continue
		}

		_, ok := targetMap[key]
		if !ok {
			targetMap[key] = sourceMap[key]
			continue
		}

		if _, ok := sourceMap[key].(map[string]interface{}); ok {
			targetMap[key] = DeepMerge(targetMap[key], sourceMap[key])
			continue
		}

		targetSlice, okTarget := targetMap[key].([]interface{})
		sourceSlice, okSource := sourceMap[key].([]interface{})
		if okTarget && okSource && len(targetSlice) == len(sourceSlice) {
			for i := 0; i < len(sourceSlice); i++ {
				_, okTarget = targetSlice[i].(map[string]interface{})
				_, okSource = sourceSlice[i].(map[string]interface{})
				if okTarget && okSource {
					targetSlice[i] = DeepMerge(targetSlice[i], sourceSlice[i])
				} else {
					targetSlice[i] = sourceSlice[i]
				}
			}
			targetMap[key] = targetSlice
			continue
		}

		targetMap[key] = sourceMap[key]
	}

	return target
}
