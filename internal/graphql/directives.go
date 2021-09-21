package graphql

import "github.com/vektah/gqlparser/v2/ast"

// for formatter
var blankBuiltInPos = &ast.Position{
	Src: &ast.Source{
		BuiltIn: true,
	},
}

// Used to conditionally include fields or fragments.
var GraphQLIncludeDirective = &ast.DirectiveDefinition{
	Description: "Directs the executor to include this field or fragment only when the `if` argument is true.",
	Name:        "include",
	Arguments: ast.ArgumentDefinitionList{
		&ast.ArgumentDefinition{
			Description: "Included when true.",
			Name:        "if",
			Type: &ast.Type{
				NamedType: "Boolean",
				NonNull:   true,
			},
		},
	},
	Locations: []ast.DirectiveLocation{
		ast.LocationField,
		ast.LocationFragmentSpread,
		ast.LocationInlineFragment,
	},
	Position: blankBuiltInPos,
}

// Used to conditionally skip (exclude) fields or fragments.
var GraphQLSkipDirective = &ast.DirectiveDefinition{
	Description: "Directs the executor to skip this field or fragment when the `if` argument is true.",
	Name:        "skip",
	Arguments: ast.ArgumentDefinitionList{
		&ast.ArgumentDefinition{
			Description: "Skipped when true.",
			Name:        "if",
			Type: &ast.Type{
				NamedType: "Boolean",
				NonNull:   true,
			},
		},
	},
	Locations: []ast.DirectiveLocation{
		ast.LocationField,
		ast.LocationFragmentSpread,
		ast.LocationInlineFragment,
	},
	Position: blankBuiltInPos,
}

// Used to declare element of a GraphQL schema as deprecated.
var GraphQLDeprecatedDirective = &ast.DirectiveDefinition{
	Description: "Marks an element of a GraphQL schema as no longer supported.",
	Name:        "deprecated",
	Arguments: ast.ArgumentDefinitionList{
		&ast.ArgumentDefinition{
			Description: "Explains why this element was deprecated, usually also including a suggestion for how to access supported similar data. Formatted using the Markdown syntax, as specified by [CommonMark](https://commonmark.org/).",
			Name:        "reason",
			DefaultValue: &ast.Value{
				Raw:  "No longer supported",
				Kind: ast.StringValue,
			},
			Type: &ast.Type{
				NamedType: "String",
			},
		},
	},
	Locations: []ast.DirectiveLocation{
		ast.LocationFieldDefinition,
		ast.LocationArgumentDefinition,
		ast.LocationInputFieldDefinition,
		ast.LocationEnumValue,
	},
	Position: blankBuiltInPos,
}

// Used to provide a URL for specifying the behaviour of custom scalar definitions.
var GraphQLSpecifiedByDirective = &ast.DirectiveDefinition{
	Description: "Exposes a URL that specifies the behaviour of this scalar.",
	Name:        "specifiedBy",
	Arguments: ast.ArgumentDefinitionList{
		&ast.ArgumentDefinition{
			Description: "The URL that specifies the behaviour of this scalar.",
			Name:        "url",
			Type: &ast.Type{
				NamedType: "String",
				NonNull:   true,
			},
		},
	},
	Locations: []ast.DirectiveLocation{
		ast.LocationScalar,
	},
	Position: blankBuiltInPos,
}

// The full list of specified directives.
var SpecifiedDirectives = ast.DirectiveDefinitionList{
	GraphQLIncludeDirective,
	GraphQLSkipDirective,
	GraphQLDeprecatedDirective,
	GraphQLSpecifiedByDirective,
}
