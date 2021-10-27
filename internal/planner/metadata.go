package planner

import (
	"encoding/json"
	"fmt"

	"github.com/goccy/go-yaml"
	"github.com/vektah/gqlparser/v2/ast"
)

type FederationSchemaMetadata struct {
	Graphs map[string]*Graph
}

type FederationTypeMetadata interface {
	isFederationTypeMetadata()
}

var _ FederationTypeMetadata = (*FederationEntityTypeMetadata)(nil)
var _ FederationTypeMetadata = (*FederationValueTypeMetadata)(nil)

type FederationEntityTypeMetadata struct {
	GraphName string
	Keys      map[string]ast.SelectionSet // readonly (FieldNode | InlineFragmentNode)[];
}

func (metadata *FederationEntityTypeMetadata) isFederationTypeMetadata() {}

type FederationValueTypeMetadata struct{}

func (metadata *FederationValueTypeMetadata) isFederationTypeMetadata() {}

type FederationFieldMetadata struct {
	GraphName string
	Requires  ast.SelectionSet // readonly (FieldNode | InlineFragmentNode)[];
	Provides  ast.SelectionSet // readonly (FieldNode | InlineFragmentNode)[];
}

var _ json.Marshaler = (*ComposedSchema)(nil)
var _ yaml.InterfaceMarshaler = (*ComposedSchema)(nil)

type ComposedSchema struct {
	Schema         *ast.Schema `yaml:"-"`
	SchemaMetadata *FederationSchemaMetadata
	TypeMetadata   map[*ast.Definition]FederationTypeMetadata
	FieldMetadata  map[*ast.FieldDefinition]*FederationFieldMetadata
}

func (cs *ComposedSchema) marshalObject() (interface{}, error) {
	type metadataHolderYAML struct {
		Schema *FederationSchemaMetadata
		Type   map[string]FederationTypeMetadata
		Field  map[string]*FederationFieldMetadata
	}

	result := &metadataHolderYAML{
		Schema: cs.SchemaMetadata,
	}

	result.Type = make(map[string]FederationTypeMetadata)
	for def, meta := range cs.TypeMetadata {
		result.Type[def.Name] = meta
	}

	{
		lookupBaseType := func(fieldDef *ast.FieldDefinition) (*ast.Definition, error) {
			var baseType *ast.Definition
		OUTER:
			for _, typ := range cs.Schema.Types {
				for _, field := range typ.Fields {
					if field == fieldDef {
						baseType = typ
						break OUTER
					}
				}
			}
			if baseType == nil {
				return nil, fmt.Errorf("failed to lookup '%s' base type", fieldDef.Name)
			}

			return baseType, nil
		}

		result.Field = make(map[string]*FederationFieldMetadata)
		for def, meta := range cs.FieldMetadata {
			baseType, err := lookupBaseType(def)
			if err != nil {
				return nil, err
			}
			name := fmt.Sprintf("%s.%s", baseType.Name, def.Name)
			result.Field[name] = meta
		}
	}

	return result, nil
}

func (cs *ComposedSchema) MarshalYAML() (interface{}, error) {
	return cs.marshalObject()
}

func (cs *ComposedSchema) MarshalJSON() ([]byte, error) {
	obj, err := cs.marshalObject()
	if err != nil {
		return nil, err
	}

	return json.Marshal(obj)
}

func newComposedSchema(schema *ast.Schema) *ComposedSchema {
	metadataHolder := &ComposedSchema{
		Schema:         schema,
		SchemaMetadata: nil,
		TypeMetadata:   make(map[*ast.Definition]FederationTypeMetadata),
		FieldMetadata:  make(map[*ast.FieldDefinition]*FederationFieldMetadata),
	}
	return metadataHolder
}

func (cs *ComposedSchema) setSchemaMetadata(value *FederationSchemaMetadata) {
	cs.SchemaMetadata = value
}

func (cs *ComposedSchema) setTypeMetadata(typ *ast.Definition, value FederationTypeMetadata) {
	if cs.TypeMetadata == nil {
		cs.TypeMetadata = make(map[*ast.Definition]FederationTypeMetadata)
	}
	cs.TypeMetadata[typ] = value
}

func (cs *ComposedSchema) setFieldMetadata(fieldDef *ast.FieldDefinition, value *FederationFieldMetadata) {
	if cs.FieldMetadata == nil {
		cs.FieldMetadata = make(map[*ast.FieldDefinition]*FederationFieldMetadata)
	}
	cs.FieldMetadata[fieldDef] = value
}

func toEntityTypeMetadata(meta FederationTypeMetadata) *FederationEntityTypeMetadata {
	switch meta := meta.(type) {
	case *FederationEntityTypeMetadata:
		return meta
	default:
		return nil
	}
}

type Graph struct {
	Name string
	URL  string
}
