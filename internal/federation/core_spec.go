package federation

import "github.com/vektah/gqlparser/v2/ast"

var CorePurpose = &ast.Definition{
	Kind: ast.Enum,
	Name: "core__Purpose",
	EnumValues: ast.EnumValueList{
		&ast.EnumValueDefinition{
			Name:        "EXECUTION",
			Description: "`EXECUTION` features provide metadata necessary to for operation execution.",
		},
		&ast.EnumValueDefinition{
			Name:        "SECURITY",
			Description: "`SECURITY` features provide metadata necessary to securely resolve fields.",
		},
	},
}

var CoreDirective = &ast.DirectiveDefinition{
	Name: "core",
	Arguments: ast.ArgumentDefinitionList{
		&ast.ArgumentDefinition{
			Name: "feature",
			Type: &ast.Type{
				NamedType: "String",
				NonNull:   true,
			},
		},
		&ast.ArgumentDefinition{
			Name: "as",
			Type: &ast.Type{
				NamedType: "String",
			},
		},
		&ast.ArgumentDefinition{
			Name: "for",
			Type: &ast.Type{
				NamedType: CorePurpose.Name,
			},
		},
	},
	Locations: []ast.DirectiveLocation{
		ast.LocationSchema,
	},
	IsRepeatable: true,
	Position:     blankPos,
}
