package federation

import "github.com/vektah/gqlparser/v2/ast"

var keyDirective = &ast.DirectiveDefinition{
	Name: "key",
	Arguments: ast.ArgumentDefinitionList{
		&ast.ArgumentDefinition{
			Name: "fields",
			Type: &ast.Type{
				NamedType: "String",
				NonNull:   true,
			},
		},
	},
	Locations: []ast.DirectiveLocation{
		ast.LocationObject,
		ast.LocationInterface,
	},
	Position: blankPos,
}

var extendsDirective = &ast.DirectiveDefinition{
	Name: "extends",
	Locations: []ast.DirectiveLocation{
		ast.LocationObject,
		ast.LocationInterface,
	},
	Position: blankPos,
}

var externalDirective = &ast.DirectiveDefinition{
	Name: "external",
	Locations: []ast.DirectiveLocation{
		ast.LocationObject,
		ast.LocationField,
	},
	Position: blankPos,
}

var requiresDirective = &ast.DirectiveDefinition{
	Name: "requires",
	Arguments: ast.ArgumentDefinitionList{
		&ast.ArgumentDefinition{
			Name: "fields",
			Type: &ast.Type{
				NamedType: "String",
				NonNull:   true,
			},
		},
	},
	Locations: []ast.DirectiveLocation{
		ast.LocationFieldDefinition,
	},
	Position: blankPos,
}

var providesDirective = &ast.DirectiveDefinition{
	Name: "provides",
	Arguments: ast.ArgumentDefinitionList{
		&ast.ArgumentDefinition{
			Name: "fields",
			Type: &ast.Type{
				NamedType: "String",
				NonNull:   true,
			},
		},
	},
	Locations: []ast.DirectiveLocation{
		ast.LocationFieldDefinition,
	},
	Position: blankPos,
}

var tagDirective = &ast.DirectiveDefinition{
	Name: "tag",
	Arguments: ast.ArgumentDefinitionList{
		&ast.ArgumentDefinition{
			Name: "name",
			Type: &ast.Type{
				NamedType: "String",
				NonNull:   true,
			},
		},
	},
	Locations: []ast.DirectiveLocation{
		ast.LocationFieldDefinition,
		ast.LocationObject,
		ast.LocationInterface,
		ast.LocationUnion,
	},
	IsRepeatable: true,
	Position: blankPos,
}

var federationDirectives = ast.DirectiveDefinitionList{
	keyDirective,
	extendsDirective,
	externalDirective,
	requiresDirective,
	providesDirective,
}

var otherKnownDirectiveDefinitions = ast.DirectiveDefinitionList{
	tagDirective,
}
