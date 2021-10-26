package graphql

import "github.com/vektah/gqlparser/v2/ast"

var GraphQLInt = &ast.Definition{
	Kind:        ast.Scalar,
	Description: "The `Int` scalar type represents non-fractional signed whole numeric values. Int can represent values between -(2^31) and 2^31 - 1.",
	Name:        "Int",
	Position:    blankBuiltInPos,
	BuiltIn:     true,
}

var GraphQLFloat = &ast.Definition{
	Kind:        ast.Scalar,
	Description: "The `Float` scalar type represents signed double-precision fractional values as specified by [IEEE 754](https://en.wikipedia.org/wiki/IEEE_floating_point).",
	Name:        "Float",
	Position:    blankBuiltInPos,
	BuiltIn:     true,
}

var GraphQLString = &ast.Definition{
	Kind:        ast.Scalar,
	Description: "The `String` scalar type represents textual data, represented as UTF-8 character sequences. The String type is most often used by GraphQL to represent free-form human-readable text.",
	Name:        "String",
	Position:    blankBuiltInPos,
	BuiltIn:     true,
}

var GraphQLBoolean = &ast.Definition{
	Kind:        ast.Scalar,
	Description: "The `Boolean` scalar type represents `true` or `false`.",
	Name:        "Boolean",
	Position:    blankBuiltInPos,
	BuiltIn:     true,
}

var GraphQLID = &ast.Definition{
	Kind:        ast.Scalar,
	Description: "The `ID` scalar type represents a unique identifier, often used to refetch an object or as key for a cache. The ID type appears in a JSON response as a String; however, it is not intended to be human-readable. When expected as an input type, any string (such as `\"4\"`) or integer (such as `4`) input value will be accepted as an ID.",
	Name:        "ID",
	Position:    blankBuiltInPos,
	BuiltIn:     true,
}

var SpecifiedScalarTypes = ast.DefinitionList{
	GraphQLString,
	GraphQLInt,
	GraphQLFloat,
	GraphQLBoolean,
	GraphQLID,
}

func isSpecifiedScalarType(typeName string) bool {
	for _, def := range SpecifiedScalarTypes {
		if def.Name == typeName {
			return true
		}
	}
	return false
}
