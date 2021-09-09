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

type contextFieldMetadataKey struct{}

var _ json.Marshaler = (*metadataHolder)(nil)
var _ yaml.InterfaceMarshaler = (*metadataHolder)(nil)

type metadataHolder struct {
	schema         *ast.Schema
	SchemaMetadata *FederationSchemaMetadata
	TypeMetadata   map[*ast.Definition]FederationTypeMetadata
	FieldMetadata  map[*ast.FieldDefinition]*FederationFieldMetadata
}

func (mh *metadataHolder) marshalObject() (interface{}, error) {
	type metadataHolderYAML struct {
		Schema *FederationSchemaMetadata
		Type   map[string]FederationTypeMetadata
		Field  map[string]*FederationFieldMetadata
	}

	result := &metadataHolderYAML{
		Schema: mh.SchemaMetadata,
	}

	result.Type = make(map[string]FederationTypeMetadata)
	for def, meta := range mh.TypeMetadata {
		result.Type[def.Name] = meta
	}

	{
		lookupBaseType := func(fieldDef *ast.FieldDefinition) (*ast.Definition, error) {
			var baseType *ast.Definition
		OUTER:
			for _, typ := range mh.schema.Types {
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
		for def, meta := range mh.FieldMetadata {
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

func (mh *metadataHolder) MarshalYAML() (interface{}, error) {
	return mh.marshalObject()
}

func (mh *metadataHolder) MarshalJSON() ([]byte, error) {
	obj, err := mh.marshalObject()
	if err != nil {
		return nil, err
	}

	return json.Marshal(obj)
}

func newMetadataHolder(schema *ast.Schema) *metadataHolder {
	metadataHolder := &metadataHolder{
		schema:         schema,
		SchemaMetadata: nil,
		TypeMetadata:   make(map[*ast.Definition]FederationTypeMetadata),
		FieldMetadata:  make(map[*ast.FieldDefinition]*FederationFieldMetadata),
	}
	return metadataHolder
}

func (mh *metadataHolder) setSchemaMetadata(value *FederationSchemaMetadata) {
	mh.SchemaMetadata = value
}

func (mh *metadataHolder) setTypeMetadata(typ *ast.Definition, value FederationTypeMetadata) {
	if mh.TypeMetadata == nil {
		mh.TypeMetadata = make(map[*ast.Definition]FederationTypeMetadata)
	}
	mh.TypeMetadata[typ] = value
}

func (mh *metadataHolder) setFieldMetadata(fieldDef *ast.FieldDefinition, value *FederationFieldMetadata) {
	if mh.FieldMetadata == nil {
		mh.FieldMetadata = make(map[*ast.FieldDefinition]*FederationFieldMetadata)
	}
	mh.FieldMetadata[fieldDef] = value
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
