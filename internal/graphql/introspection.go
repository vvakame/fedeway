package graphql

import "github.com/vektah/gqlparser/v2/ast"

var __Schema = &ast.Definition{
	Kind:        ast.Object,
	Description: "A GraphQL Schema defines the capabilities of a GraphQL server. It exposes all available types and directives on the server, as well as the entry points for query, mutation, and subscription operations.",
	Name:        "__Schema",
	Fields: ast.FieldList{
		{
			Description: "A GraphQL Schema defines the capabilities of a GraphQL server. It exposes all available types and directives on the server, as well as the entry points for query, mutation, and subscription operations.",
			Name:        "description",
			Type: &ast.Type{
				NamedType: GraphQLString.Name,
				Position:  blankBuiltInPos,
			},
			Position: blankBuiltInPos,
		},
		{
			Description: "A list of all types supported by this server.",
			Name:        "types",
			Type: &ast.Type{
				Elem: &ast.Type{
					NamedType: __Type.Name,
					NonNull:   true,
					Position:  blankBuiltInPos,
				},
				NonNull:  true,
				Position: blankBuiltInPos,
			},
			Position: blankBuiltInPos,
		},
		{
			Description: "The type that query operations will be rooted at.",
			Name:        "queryType",
			Type: &ast.Type{
				NamedType: __Type.Name,
				NonNull:   true,
				Position:  blankBuiltInPos,
			},
			Position: blankBuiltInPos,
		},
		{
			Description: "If this server supports mutation, the type that mutation operations will be rooted at.",
			Name:        "mutationType",
			Type: &ast.Type{
				NamedType: __Type.Name,
				Position:  blankBuiltInPos,
			},
			Position: blankBuiltInPos,
		},
		{
			Description: "If this server support subscription, the type that subscription operations will be rooted at.",
			Name:        "subscriptionType",
			Type: &ast.Type{
				NamedType: __Type.Name,
				Position:  blankBuiltInPos,
			},
			Position: blankBuiltInPos,
		},
		{
			Description: "A list of all directives supported by this server.",
			Name:        "directives",
			Type: &ast.Type{
				Elem: &ast.Type{
					NamedType: __Directive.Name,
					NonNull:   true,
					Position:  blankBuiltInPos,
				},
				NonNull:  true,
				Position: blankBuiltInPos,
			},
			Position: blankBuiltInPos,
		},
	},
	Position: blankBuiltInPos,
	BuiltIn:  true,
}

var __Directive = &ast.Definition{
	Kind:        ast.Object,
	Description: "A Directive provides a way to describe alternate runtime execution and type validation behavior in a GraphQL document.\\n\\nIn some cases, you need to provide options to alter GraphQL's execution behavior in ways field arguments will not suffice, such as conditionally including or skipping a field. Directives provide this by describing additional information to the executor.",
	Name:        "__Directive",
	Fields: ast.FieldList{
		{
			Name: "name",
			Type: &ast.Type{
				NamedType: GraphQLString.Name,
				NonNull:   true,
				Position:  blankBuiltInPos,
			},
			Position: blankBuiltInPos,
		},
		{
			Description: "A Directive provides a way to describe alternate runtime execution and type validation behavior in a GraphQL document.\\n\\nIn some cases, you need to provide options to alter GraphQL's execution behavior in ways field arguments will not suffice, such as conditionally including or skipping a field. Directives provide this by describing additional information to the executor.",
			Name:        "description",
			Type: &ast.Type{
				NamedType: GraphQLString.Name,
				Position:  blankBuiltInPos,
			},
			Position: blankBuiltInPos,
		},
		{
			Name: "isRepeatable",
			Type: &ast.Type{
				NamedType: GraphQLBoolean.Name,
				NonNull:   true,
				Position:  blankBuiltInPos,
			},
			Position: blankBuiltInPos,
		},
		{
			Name: "locations",
			Type: &ast.Type{
				Elem: &ast.Type{
					NamedType: __DirectiveLocation.Name,
					NonNull:   true,
					Position:  blankBuiltInPos,
				},
				NonNull:  true,
				Position: blankBuiltInPos,
			},
			Position: blankBuiltInPos,
		},
		{
			Name: "args",
			Type: &ast.Type{
				Elem: &ast.Type{
					NamedType: __InputValue.Name,
					NonNull:   true,
					Position:  blankBuiltInPos,
				},
				NonNull:  true,
				Position: blankBuiltInPos,
			},
			Position: blankBuiltInPos,
		},
	},
	Position: blankBuiltInPos,
	BuiltIn:  true,
}

var __DirectiveLocation = &ast.Definition{
	Kind:        ast.Enum,
	Description: "A Directive can be adjacent to many parts of the GraphQL language, a __DirectiveLocation describes one such possible adjacencies.",
	Name:        "__DirectiveLocation",
	EnumValues: ast.EnumValueList{
		{
			Description: "Location adjacent to a query operation.",
			Name:        "QUERY",
			Position:    blankBuiltInPos,
		},
		{
			Description: "Location adjacent to a mutation operation.",
			Name:        "MUTATION",
			Position:    blankBuiltInPos,
		},
		{
			Description: "Location adjacent to a subscription operation.",
			Name:        "SUBSCRIPTION",
			Position:    blankBuiltInPos,
		},
		{
			Description: "Location adjacent to a field.",
			Name:        "FIELD",
			Position:    blankBuiltInPos,
		},
		{
			Description: "Location adjacent to a fragment definition.",
			Name:        "FRAGMENT_DEFINITION",
			Position:    blankBuiltInPos,
		},
		{
			Description: "Location adjacent to a fragment spread.",
			Name:        "FRAGMENT_SPREAD",
			Position:    blankBuiltInPos,
		},
		{
			Description: "Location adjacent to an inline fragment.",
			Name:        "INLINE_FRAGMENT",
			Position:    blankBuiltInPos,
		},
		{
			Description: "Location adjacent to a variable definition.",
			Name:        "VARIABLE_DEFINITION",
			Position:    blankBuiltInPos,
		},
		{
			Description: "Location adjacent to a schema definition.",
			Name:        "SCHEMA",
			Position:    blankBuiltInPos,
		},
		{
			Description: "",
			Name:        "",
			Position:    blankBuiltInPos,
		},
		{
			Description: "Location adjacent to a scalar definition.",
			Name:        "SCALAR",
			Position:    blankBuiltInPos,
		},
		{
			Description: "Location adjacent to an object type definition.",
			Name:        "OBJECT",
			Position:    blankBuiltInPos,
		},
		{
			Description: "Location adjacent to a field definition.",
			Name:        "FIELD_DEFINITION",
			Position:    blankBuiltInPos,
		},
		{
			Description: "Location adjacent to an argument definition.",
			Name:        "ARGUMENT_DEFINITION",
			Position:    blankBuiltInPos,
		},
		{
			Description: "Location adjacent to an interface definition.",
			Name:        "INTERFACE",
			Position:    blankBuiltInPos,
		},
		{
			Description: "Location adjacent to a union definition.",
			Name:        "UNION",
			Position:    blankBuiltInPos,
		},
		{
			Description: "Location adjacent to an enum definition.",
			Name:        "ENUM",
			Position:    blankBuiltInPos,
		},
		{
			Description: "Location adjacent to an enum value definition.",
			Name:        "ENUM_VALUE",
			Position:    blankBuiltInPos,
		},
		{
			Description: "Location adjacent to an input object type definition.",
			Name:        "INPUT_OBJECT",
			Position:    blankBuiltInPos,
		},
		{
			Description: "Location adjacent to an input object field definition.",
			Name:        "INPUT_FIELD_DEFINITION",
			Position:    blankBuiltInPos,
		},
	},
	Position: blankBuiltInPos,
	BuiltIn:  true,
}

var __Type = &ast.Definition{
	Kind:        ast.Object,
	Description: "The fundamental unit of any GraphQL Schema is the type. There are many kinds of types in GraphQL as represented by the `__TypeKind` enum.\\n\\nDepending on the kind of a type, certain fields describe information about that type. Scalar types provide no information beyond a name, description and optional `specifiedByURL`, while Enum types provide their values. Object and Interface types provide the fields they describe. Abstract types, Union and Interface, provide the Object types possible at runtime. List and NonNull types compose other types.",
	Name:        "__Type",
	Fields: ast.FieldList{
		{
			Name: "kind",
			Type: &ast.Type{
				NamedType: __TypeKind.Name,
				NonNull:   true,
				Position:  blankBuiltInPos,
			},
			Position: blankBuiltInPos,
		},
		{
			Name: "name",
			Type: &ast.Type{
				NamedType: GraphQLString.Name,
				Position:  blankBuiltInPos,
			},
			Position: blankBuiltInPos,
		},
		{
			Name: "description",
			Type: &ast.Type{
				NamedType: GraphQLString.Name,
				Position:  blankBuiltInPos,
			},
			Position: blankBuiltInPos,
		},
		{
			Name: "specifiedByURL",
			Type: &ast.Type{
				NamedType: GraphQLString.Name,
				Position:  blankBuiltInPos,
			},
			Position: blankBuiltInPos,
		},
		{
			Name: "fields",
			Type: &ast.Type{
				NamedType: __Field.Name,
				NonNull:   true,
				Position:  blankBuiltInPos,
			},
			Arguments: ast.ArgumentDefinitionList{
				{
					Name: "includeDeprecated",
					Type: &ast.Type{
						NamedType: GraphQLBoolean.Name,
						Position:  blankBuiltInPos,
					},
					DefaultValue: &ast.Value{
						Raw:      "false",
						Kind:     ast.BooleanValue,
						Position: blankBuiltInPos,
					},
					Position: blankBuiltInPos,
				},
			},
			Position: blankBuiltInPos,
		},
		{
			Name: "interfaces",
			Type: &ast.Type{
				Elem: &ast.Type{
					NamedType: "__Type", // __Type.Name,
					NonNull:   true,
					Position:  blankBuiltInPos,
				},
				Position: blankBuiltInPos,
			},
			Position: blankBuiltInPos,
		},
		{
			Name: "possibleTypes",
			Type: &ast.Type{
				Elem: &ast.Type{
					NamedType: "__Type", // __Type.Name,
					NonNull:   true,
					Position:  blankBuiltInPos,
				},
				Position: blankBuiltInPos,
			},
			Position: blankBuiltInPos,
		},
		{
			Name: "enumValues",
			Type: &ast.Type{
				Elem: &ast.Type{
					NamedType: __EnumValue.Name,
					NonNull:   true,
					Position:  blankBuiltInPos,
				},
				Position: blankBuiltInPos,
			},
			Arguments: ast.ArgumentDefinitionList{
				{
					Name: "includeDeprecated",
					Type: &ast.Type{
						NamedType: GraphQLBoolean.Name,
						Position:  blankBuiltInPos,
					},
					DefaultValue: &ast.Value{
						Raw:      "false",
						Kind:     ast.BooleanValue,
						Position: blankBuiltInPos,
					},
					Position: blankBuiltInPos,
				},
			},
			Position: blankBuiltInPos,
		},
		{
			Name: "inputFields",
			Type: &ast.Type{
				Elem: &ast.Type{
					NamedType: __InputValue.Name,
					NonNull:   true,
					Position:  blankBuiltInPos,
				},
				Position: blankBuiltInPos,
			},
			Arguments: ast.ArgumentDefinitionList{
				{
					Name: "includeDeprecated",
					Type: &ast.Type{
						NamedType: GraphQLBoolean.Name,
						Position:  blankBuiltInPos,
					},
					DefaultValue: &ast.Value{
						Raw:      "false",
						Kind:     ast.BooleanValue,
						Position: blankBuiltInPos,
					},
					Position: blankBuiltInPos,
				},
			},
			Position: blankBuiltInPos,
		},
		{
			Name: "ofType",
			Type: &ast.Type{
				NamedType: "__Type", // __Type.Name,
				Position:  blankBuiltInPos,
			},
			Position: blankBuiltInPos,
		},
	},
	Position: blankBuiltInPos,
	BuiltIn:  true,
}

var __Field = &ast.Definition{
	Kind:        ast.Object,
	Description: "Object and Interface types are described by a list of Fields, each of which has a name, potentially a list of arguments, and a return type.",
	Name:        "__Field",
	Fields: ast.FieldList{
		{
			Name: "name",
			Type: &ast.Type{
				NamedType: GraphQLString.Name,
				NonNull:   true,
				Position:  blankBuiltInPos,
			},
			Position: blankBuiltInPos,
		},
		{
			Name: "description",
			Type: &ast.Type{
				NamedType: GraphQLString.Name,
				Position:  blankBuiltInPos,
			},
			Position: blankBuiltInPos,
		},
		{
			Name: "args",
			Type: &ast.Type{
				Elem: &ast.Type{
					NamedType: __InputValue.Name,
					NonNull:   true,
					Position:  blankBuiltInPos,
				},
				NonNull:  true,
				Position: blankBuiltInPos,
			},
			Arguments: ast.ArgumentDefinitionList{
				{
					Name: "includeDeprecated",
					Type: &ast.Type{
						NamedType: GraphQLBoolean.Name,
						Position:  blankBuiltInPos,
					},
					DefaultValue: &ast.Value{
						Raw:      "false",
						Kind:     ast.BooleanValue,
						Position: blankBuiltInPos,
					},
					Position: blankBuiltInPos,
				},
			},
			Position: blankBuiltInPos,
		},
		{
			Name: "type",
			Type: &ast.Type{
				NamedType: "__Type", // __Type.Name,
				NonNull:   true,
				Position:  blankBuiltInPos,
			},
			Position: blankBuiltInPos,
		},
		{
			Name: "isDeprecated",
			Type: &ast.Type{
				NamedType: GraphQLBoolean.Name,
				NonNull:   true,
				Position:  blankBuiltInPos,
			},
			Position: blankBuiltInPos,
		},
		{
			Name: "deprecationReason",
			Type: &ast.Type{
				NamedType: GraphQLString.Name,
				Position:  blankBuiltInPos,
			},
			Position: blankBuiltInPos,
		},
	},
	Position: blankBuiltInPos,
	BuiltIn:  true,
}

var __InputValue = &ast.Definition{
	Kind:        ast.Object,
	Description: "Arguments provided to Fields or Directives and the input fields of an InputObject are represented as Input Values which describe their type and optionally a default value.",
	Name:        "__InputValue",
	Fields: ast.FieldList{
		{
			Name: "name",
			Type: &ast.Type{
				NamedType: GraphQLString.Name,
				NonNull:   true,
				Position:  blankBuiltInPos,
			},
			Position: blankBuiltInPos,
		},
		{
			Name: "description",
			Type: &ast.Type{
				NamedType: GraphQLString.Name,
				Position:  blankBuiltInPos,
			},
			Position: blankBuiltInPos,
		},
		{
			Name: "type",
			Type: &ast.Type{
				NamedType: "__Type", // __Type.Name,
				NonNull:   true,
				Position:  blankBuiltInPos,
			},
			Position: blankBuiltInPos,
		},
		{
			Description: "A GraphQL-formatted string representing the default value for this input value.",
			Name:        "defaultValue",
			Type: &ast.Type{
				NamedType: GraphQLString.Name,
				Position:  blankBuiltInPos,
			},
			Position: blankBuiltInPos,
		},
		{
			Name: "isDeprecated",
			Type: &ast.Type{
				NamedType: GraphQLBoolean.Name,
				NonNull:   true,
				Position:  blankBuiltInPos,
			},
			Position: blankBuiltInPos,
		},
		{
			Name: "deprecationReason",
			Type: &ast.Type{
				NamedType: GraphQLString.Name,
				Position:  blankBuiltInPos,
			},
			Position: blankBuiltInPos,
		},
	},
	Position: blankBuiltInPos,
	BuiltIn:  true,
}

var __EnumValue = &ast.Definition{
	Kind:        ast.Object,
	Description: "One possible value for a given Enum. Enum values are unique values, not a placeholder for a string or numeric value. However an Enum value is returned in a JSON response as a string.",
	Name:        "__EnumValue",
	Fields: ast.FieldList{
		{
			Name: "name",
			Type: &ast.Type{
				NamedType: GraphQLString.Name,
				NonNull:   true,
				Position:  blankBuiltInPos,
			},
			Position: blankBuiltInPos,
		},
		{
			Name: "description",
			Type: &ast.Type{
				NamedType: GraphQLString.Name,
				Position:  blankBuiltInPos,
			},
			Position: blankBuiltInPos,
		},
		{
			Name: "isDeprecated",
			Type: &ast.Type{
				NamedType: GraphQLBoolean.Name,
				NonNull:   true,
				Position:  blankBuiltInPos,
			},
			Position: blankBuiltInPos,
		},
		{
			Name: "deprecationReason",
			Type: &ast.Type{
				NamedType: GraphQLString.Name,
				Position:  blankBuiltInPos,
			},
			Position: blankBuiltInPos,
		},
	},
	Position: blankBuiltInPos,
	BuiltIn:  true,
}

var __TypeKind = &ast.Definition{
	Kind:        ast.Enum,
	Description: "An enum describing what kind of type a given `__Type` is.",
	Name:        "__TypeKind",
	EnumValues: ast.EnumValueList{
		{
			Description: "Indicates this type is a scalar.",
			Name:        "SCALAR",
			Position:    blankBuiltInPos,
		},
		{
			Description: "Indicates this type is an object. `fields` and `interfaces` are valid fields.",
			Name:        "OBJECT",
			Position:    blankBuiltInPos,
		},
		{
			Description: "Indicates this type is an interface. `fields`, `interfaces`, and `possibleTypes` are valid fields.",
			Name:        "INTERFACE",
			Position:    blankBuiltInPos,
		},
		{
			Description: "Indicates this type is a union. `possibleTypes` is a valid field.",
			Name:        "UNION",
			Position:    blankBuiltInPos,
		},
		{
			Description: "Indicates this type is an enum. `enumValues` is a valid field.",
			Name:        "ENUM",
			Position:    blankBuiltInPos,
		},
		{
			Description: "Indicates this type is an input object. `inputFields` is a valid field.",
			Name:        "INPUT_OBJECT",
			Position:    blankBuiltInPos,
		},
		{
			Description: "Indicates this type is a list. `ofType` is a valid field.",
			Name:        "LIST",
			Position:    blankBuiltInPos,
		},
		{
			Description: "Indicates this type is a non-null. `ofType` is a valid field.",
			Name:        "NON_NULL",
			Position:    blankBuiltInPos,
		},
	},
	Position: blankBuiltInPos,
	BuiltIn:  true,
}

var IntrospectionTypes = ast.DefinitionList{
	__Schema,
	__Directive,
	__DirectiveLocation,
	__Type,
	__Field,
	__InputValue,
	__EnumValue,
	__TypeKind,
}

var SchemaMetaFieldDef = &ast.FieldDefinition{
	Name:        "__schema",
	Description: "Access the current type schema of this server.",
	Type:        ast.NamedType("__Schema", nil),
}

var TypeMetaFieldDef = &ast.FieldDefinition{
	Name:        "__type",
	Description: "Request the type information of a single type.",
	Type:        ast.NamedType("__Type", nil),
	Arguments: []*ast.ArgumentDefinition{
		{
			Name:     "name",
			Type:     ast.NonNullNamedType("String", nil),
			Position: blankBuiltInPos,
		},
	},
	Position: blankBuiltInPos,
}

var TypeNameMetaFieldDef = &ast.FieldDefinition{
	Name:     "__typename",
	Type:     ast.NamedType("String", nil),
	Position: blankBuiltInPos,
}

func IsIntrospectionType(typeName string) bool {
	for _, def := range IntrospectionTypes {
		if def.Name == typeName {
			return true
		}
	}
	return false
}
