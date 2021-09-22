package federation

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/vektah/gqlparser/v2/ast"
)

var FieldSetScalar = &ast.Definition{
	Kind:     ast.Scalar,
	Name:     "join__FieldSet",
	Position: blankPos,
}

var JoinGraphDirective = &ast.DirectiveDefinition{
	Name: "join__graph",
	Arguments: ast.ArgumentDefinitionList{
		&ast.ArgumentDefinition{
			Name: "name",
			Type: &ast.Type{
				NamedType: "String",
				NonNull:   true,
			},
		},
		&ast.ArgumentDefinition{
			Name: "url",
			Type: &ast.Type{
				NamedType: "String",
				NonNull:   true,
			},
		},
	},
	Locations: []ast.DirectiveLocation{
		ast.LocationEnumValue,
	},
	Position: blankPos,
}

// Expectations
// 1. The input is first sorted using `String.localeCompare`, so the output is deterministic
// 2. Non-Alphanumeric characters are replaced with _ (alphaNumericUnderscoreOnly)
// 3. Numeric first characters are prefixed with _ (noNumericFirstChar)
// 4. Names ending in an underscore followed by numbers `_\d+` are suffixed with _ (noUnderscoreNumericEnding)
// 5. Names are uppercased (toUpper)
// 6. After transformations 1-5, duplicates are suffixed with _{n} where {n} is number of times we've seen the dupe
//
// Note: Collisions with name's we've generated are also accounted for
func getJoinGraphEnum(serviceList []*ServiceDefinition) (map[string]string, *ast.Definition) {
	sortedServiceList := append([]*ServiceDefinition{}, serviceList...)
	sort.SliceStable(sortedServiceList, func(i, j int) bool {
		return sortedServiceList[i].Name < sortedServiceList[j].Name
	})

	sanitizeGraphQLName := func(name string) string {
		// replace all non-word characters (\W). Word chars are _a-zA-Z0-9
		alphaNumericUnderscoreOnly := regexp.MustCompile("[\\W]").ReplaceAllString(name, "_")
		// prefix a digit in the first position with an _
		noNumericFirstChar := alphaNumericUnderscoreOnly
		if regexp.MustCompile("^\\d").MatchString(noNumericFirstChar) {
			noNumericFirstChar = "_" + noNumericFirstChar
		}
		// suffix an underscore + digit in the last position with an _
		noUnderscoreNumericEnding := noNumericFirstChar
		if regexp.MustCompile("_\\d$").MatchString(noUnderscoreNumericEnding) {
			noUnderscoreNumericEnding = noUnderscoreNumericEnding + "_"
		}

		// toUpper not really necessary but follows convention of enum values
		toUpper := strings.ToUpper(noUnderscoreNumericEnding)
		return toUpper
	}

	// duplicate enum values can occur due to sanitization and must be accounted for
	// collect the duplicates in an array so we can uniquify them in a second pass.
	sanitizedNameToServiceDefinitions := make(map[string][]*ServiceDefinition)
	for _, service := range sortedServiceList {
		name := service.Name
		sanitized := sanitizeGraphQLName(name)
		sanitizedNameToServiceDefinitions[sanitized] = append(sanitizedNameToServiceDefinitions[sanitized], service)
	}

	// if no duplicates for a given name, add it as is
	// if duplicates exist, append _{n} (index-1) to each duplicate in the array
	enumValueNameToServiceDefinition := make(map[string]*ServiceDefinition)
	for sanitizedName, services := range sanitizedNameToServiceDefinitions {
		if len(services) == 1 {
			enumValueNameToServiceDefinition[sanitizedName] = services[0]
		} else {
			for idx, service := range services {
				enumValueNameToServiceDefinition[fmt.Sprintf("%s_%d", sanitizedName, idx+1)] = service
			}
		}
	}

	graphNameToEnumValueName := make(map[string]string)
	joinGraphEnum := &ast.Definition{
		Kind:     ast.Enum,
		Name:     "join__Graph",
		Position: blankPos,
	}
	for enumValueName, service := range enumValueNameToServiceDefinition {
		graphNameToEnumValueName[service.Name] = enumValueName
		// TODO ここで service を使ってないのはoriginalと違って明らかにおかしいんだけどどういう意図のコードなのかわからず、AST上再現もできないので一旦ほうっておく…
		// TODO enum join__Graph に対して @join__graph をいつ付与しているか現時点では謎で、ここなのでは…？という気がしなくもない…
		joinGraphEnum.EnumValues = append(joinGraphEnum.EnumValues, &ast.EnumValueDefinition{
			Name: enumValueName,
		})
	}

	return graphNameToEnumValueName, joinGraphEnum
}

func getJoinFieldDirective(joinGraphEnum *ast.Definition) *ast.DirectiveDefinition {
	return &ast.DirectiveDefinition{
		Name: "join__field",
		Arguments: ast.ArgumentDefinitionList{
			&ast.ArgumentDefinition{
				Name: "graph",
				Type: &ast.Type{
					NamedType: joinGraphEnum.Name,
				},
			},
			&ast.ArgumentDefinition{
				Name: "requires",
				Type: &ast.Type{
					NamedType: FieldSetScalar.Name,
				},
			},
			&ast.ArgumentDefinition{
				Name: "provides",
				Type: &ast.Type{
					NamedType: FieldSetScalar.Name,
				},
			},
		},
		Locations: []ast.DirectiveLocation{
			ast.LocationFieldDefinition,
		},
		Position: blankPos,
	}
}

func getJoinOwnerDirective(joinGraphEnum *ast.Definition) *ast.DirectiveDefinition {
	return &ast.DirectiveDefinition{
		Name: "join__owner",
		Arguments: ast.ArgumentDefinitionList{
			&ast.ArgumentDefinition{
				Name: "graph",
				Type: &ast.Type{
					NamedType: joinGraphEnum.Name,
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
}

func getJoinDefinitions(serviceList []*ServiceDefinition) (map[string]string, *ast.Definition, *ast.DirectiveDefinition, *ast.DirectiveDefinition, *ast.DirectiveDefinition, *ast.Definition, *ast.DirectiveDefinition) {
	graphNameToEnumValueName, joinGraphEnum := getJoinGraphEnum(serviceList)
	joinFieldDirective := getJoinFieldDirective(joinGraphEnum)
	joinOwnerDirective := getJoinOwnerDirective(joinGraphEnum)

	joinTypeDirective := &ast.DirectiveDefinition{
		Name: "join__type",
		Arguments: ast.ArgumentDefinitionList{
			&ast.ArgumentDefinition{
				Name: "graph",
				Type: &ast.Type{
					NamedType: joinGraphEnum.Name,
					NonNull:   true,
				},
			},
			&ast.ArgumentDefinition{
				Name: "key",
				Type: &ast.Type{
					NamedType: FieldSetScalar.Name,
				},
			},
		},
		Locations: []ast.DirectiveLocation{
			ast.LocationObject,
			ast.LocationInterface,
		},
		IsRepeatable: true,
		Position:     blankPos,
	}

	return graphNameToEnumValueName, FieldSetScalar, joinTypeDirective, joinFieldDirective, joinOwnerDirective, joinGraphEnum, JoinGraphDirective
}
