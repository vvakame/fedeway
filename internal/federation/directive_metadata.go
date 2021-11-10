package federation

import "github.com/vektah/gqlparser/v2/ast"

type DirectiveMetadata struct {
	DirectiveUsagesPerSubgraph DirectiveUsagesPerSubgraph
}

// directive name => usages
type DirectiveUsages map[string]ast.DirectiveList

// field name => directive name => usages
type DirectiveUsagesPerField map[string]DirectiveUsages

// type name => {
//   directives: DirectiveUsages,
//   fields: DirectiveUsagesPerField
// }
type DirectiveUsagesPerType map[string]*DirectiveUsagesPerTypeEntity
type DirectiveUsagesPerTypeEntity struct {
	Directives DirectiveUsages
	Fields     DirectiveUsagesPerField
}

// subgraph name => DirectiveUsagesPerType
type DirectiveUsagesPerSubgraph map[string]DirectiveUsagesPerType

func newDirectiveMetadata(subgraphs []*ServiceDefinition) *DirectiveMetadata {
	dm := &DirectiveMetadata{}
	dm.DirectiveUsagesPerSubgraph = make(map[string]DirectiveUsagesPerType)

	collectDirectiveUsages := func(directives ast.DirectiveList, usagesOnNode DirectiveUsages) {
		for _, directive := range directives {
			if _, ok := usagesOnNode[directive.Name]; !ok {
				usagesOnNode[directive.Name] = ast.DirectiveList{}
			}
			usagesOnNode[directive.Name] = append(usagesOnNode[directive.Name], directive)
		}
	}

	for _, subgraph := range subgraphs {
		subgraphName := subgraph.Name

		definitions := make(ast.DefinitionList, 0, len(subgraph.TypeDefs.Definitions)+len(subgraph.TypeDefs.Extensions))
		definitions = append(definitions, subgraph.TypeDefs.Definitions...)
		definitions = append(definitions, subgraph.TypeDefs.Extensions...)
		for _, node := range definitions {
			switch node.Kind {
			case ast.Object, ast.Interface, ast.Union:
				// visit each object-like type to build the map of directive usages
			default:
				continue
			}

			var directiveUsagesPerType DirectiveUsagesPerType
			if _, ok := dm.DirectiveUsagesPerSubgraph[subgraphName]; !ok {
				dm.DirectiveUsagesPerSubgraph[subgraphName] = DirectiveUsagesPerType{}
			}
			directiveUsagesPerType = dm.DirectiveUsagesPerSubgraph[subgraphName]

			if _, ok := directiveUsagesPerType[node.Name]; !ok {
				directiveUsagesPerType[node.Name] = &DirectiveUsagesPerTypeEntity{
					Directives: DirectiveUsages{},
					Fields:     DirectiveUsagesPerField{},
				}
			}
			usagesOnType := directiveUsagesPerType[node.Name].Directives
			usagesByFieldName := directiveUsagesPerType[node.Name].Fields

			// Collect directive usages on the type node
			collectDirectiveUsages(node.Directives, usagesOnType)

			// Collect directive usages on each field node
			for _, field := range node.Fields {
				if _, ok := usagesByFieldName[field.Name]; !ok {
					usagesByFieldName[field.Name] = DirectiveUsages{}
				}
				usagesOnField := usagesByFieldName[field.Name]
				collectDirectiveUsages(field.Directives, usagesOnField)
			}
		}
	}

	return dm
}

// visit the entire map for any usages of a directive
func (dm *DirectiveMetadata) HasUsage(directiveName string) bool {
	for _, directiveUsagesPerType := range dm.DirectiveUsagesPerSubgraph {
		for _, du := range directiveUsagesPerType {
			directives := du.Directives
			fields := du.Fields
			usagesOnType, ok := directives[directiveName]
			if ok && len(usagesOnType) > 0 {
				return true
			}

			for _, directiveUsages := range fields {
				usagesOnField, ok := directiveUsages[directiveName]
				if ok && len(usagesOnField) > 0 {
					return true
				}
			}
		}
	}
	return false
}

// traverse the map of directive usages and apply metadata to the corresponding
// `extensions` fields on the provided schema.
func (dm *DirectiveMetadata) applyMetadataToSupergraphSchema(schema *ast.Schema) {
	federationTypeMap := FederationTypeMap{}
	federationFieldMap := FederationFieldMap{}

	for _, directiveUsagesPerType := range dm.DirectiveUsagesPerSubgraph {
		for typeName, entity := range directiveUsagesPerType {
			directives := entity.Directives
			fields := entity.Fields

			namedType := schema.Types[typeName]
			if namedType == nil {
				continue
			}

			existingMetadata := federationTypeMap.Get(namedType)
			directiveUsages := existingMetadata.DirectiveUsages
			if len(directiveUsages) > 0 {
				for directiveName, usages := range directiveUsages {
					usages = append(usages, directives[directiveName]...)
				}
			} else {
				directiveUsages = directives
			}
			existingMetadata.DirectiveUsages = directiveUsages

			for fieldName, usagesPerDirective := range fields {
				field := namedType.Fields.ForName(fieldName)
				if field == nil {
					continue
				}

				originalMetadata := federationFieldMap.Get(field)
				directiveUsages := originalMetadata.DirectiveUsages

				if len(directiveUsages) > 0 {
					for directiveName, usages := range directiveUsages {
						usages = append(usages, usagesPerDirective[directiveName]...)
					}
				} else {
					directiveUsages = directives
				}
				originalMetadata.DirectiveUsages = directiveUsages
			}
		}
	}
}
