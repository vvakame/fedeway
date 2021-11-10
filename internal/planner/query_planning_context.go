package planner

import (
	"context"
	"fmt"

	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"
	"github.com/vektah/gqlparser/v2/validator"
	"github.com/vvakame/fedeway/internal/graphql"
)

var typenameField = &ast.Field{
	Alias: "__typename",
	Name:  "__typename",
}

func buildQueryPlanningContext(operationContext *OperationContext, autoFragmentation bool) (*queryPlanningContext, error) {
	qpc := &queryPlanningContext{
		queryDocument:       operationContext.QueryDocument,
		operation:           operationContext.QueryDocument.Operations.ForName(operationContext.OperationName),
		operationName:       operationContext.OperationName,
		schema:              operationContext.Schema,
		metadata:            operationContext.metadata,
		fragments:           operationContext.Fragments,
		autoFragmentation:   autoFragmentation,
		internalFragments:   make(map[string]*internalFragment),
		variableDefinitions: make(map[string]*ast.VariableDefinition),
	}

	for _, varDef := range qpc.operation.VariableDefinitions {
		qpc.variableDefinitions[varDef.Variable] = varDef
	}

	return qpc, nil
}

type queryPlanningContext struct {
	queryDocument     *ast.QueryDocument
	operation         *ast.OperationDefinition
	operationName     string
	schema            *ast.Schema
	metadata          *ComposedSchema
	fragments         ast.FragmentDefinitionList
	autoFragmentation bool

	internalFragments   map[string]*internalFragment
	variableDefinitions map[string]*ast.VariableDefinition
}

type internalFragment struct {
	Name         string
	Definition   *ast.FragmentDefinition
	SelectionSet ast.SelectionSet
}

func (qpctx *queryPlanningContext) collectFields(ctx context.Context, scope *Scope, selectionSet ast.SelectionSet) (FieldSet, error) {
	var fields FieldSet
	for _, selection := range selectionSet {
		switch selection := selection.(type) {
		case *ast.Field:
			fieldDef, err := qpctx.getFieldDef(ctx, scope.parentType, selection)
			if err != nil {
				return nil, err
			}
			fields = append(fields, &Field{
				Scope:     scope,
				FieldNode: selection,
				FieldDef:  fieldDef,
			})

		case *ast.InlineFragment:
			newScope, err := qpctx.scopeForInlineFragment(ctx, scope, selection, selection.Directives)
			if err != nil {
				return nil, err
			}
			if newScope != nil {
				newFields, err := qpctx.collectFields(ctx, newScope, selection.SelectionSet)
				if err != nil {
					return nil, err
				}
				fields = append(fields, newFields...)
			}

		case *ast.FragmentSpread:
			fragment := qpctx.fragments.ForName(selection.Name)
			if fragment == nil {
				// TODO should be error?
				continue
			}

			newScope, err := qpctx.scopeForFragment(ctx, scope, fragment, selection.Directives)
			if err != nil {
				return nil, err
			}
			if newScope != nil {
				newFields, err := qpctx.collectFields(ctx, newScope, fragment.SelectionSet)
				if err != nil {
					return nil, err
				}
				fields = append(fields, newFields...)
			}

		default:
			return nil, fmt.Errorf("unexpected selection type: %T", selection)
		}
	}

	return fields, nil
}

func (qpctx *queryPlanningContext) getFieldDef(ctx context.Context, parentType *ast.Definition, fieldNode *ast.Field) (*ast.FieldDefinition, error) {
	fieldName := fieldNode.Name

	fieldDef := getFieldDef(qpctx.schema, parentType, fieldName)
	if fieldDef == nil {
		return nil, gqlerror.ErrorPosf(
			fieldNode.Position,
			"cannot query field '%s' on type '%s'",
			fieldNode.Name,
			parentType.Name,
		)
	}

	return fieldDef, nil
}

func (qpctx *queryPlanningContext) getFragmentCondition(ctx context.Context, parentType *ast.Definition, fragment *ast.FragmentDefinition) *ast.Definition {
	typeConditionNode := qpctx.schema.Types[fragment.TypeCondition]
	if typeConditionNode == nil {
		return parentType
	}

	return typeConditionNode
}

func (qpctx *queryPlanningContext) getInlineFragmentCondition(ctx context.Context, parentType *ast.Definition, fragment *ast.InlineFragment) *ast.Definition {
	typeConditionNode := qpctx.schema.Types[fragment.TypeCondition]
	if typeConditionNode == nil {
		return parentType
	}

	return typeConditionNode
}

func (qpctx *queryPlanningContext) scopeForFragment(ctx context.Context, currentScope *Scope, fragment *ast.FragmentDefinition, appliedDirectives ast.DirectiveList) (*Scope, error) {
	condition := qpctx.getFragmentCondition(ctx, currentScope.parentType, fragment)
	newScope, err := currentScope.refine(ctx, condition, appliedDirectives)
	if err != nil {
		return nil, err
	}

	if len(newScope.possibleRuntimeTypes()) == 0 {
		return nil, nil
	}

	return newScope, nil
}

func (qpctx *queryPlanningContext) scopeForInlineFragment(ctx context.Context, currentScope *Scope, fragment *ast.InlineFragment, appliedDirectives ast.DirectiveList) (*Scope, error) {
	condition := qpctx.getInlineFragmentCondition(ctx, currentScope.parentType, fragment)
	newScope, err := currentScope.refine(ctx, condition, appliedDirectives)
	if err != nil {
		return nil, err
	}

	if len(newScope.possibleRuntimeTypes()) == 0 {
		return nil, nil
	}

	return newScope, nil
}

func (qpctx *queryPlanningContext) getBaseService(ctx context.Context, parentType *ast.Definition) string {
	metadata := qpctx.getFederationMetadataForType(parentType)
	if metadata == nil || metadata.IsValueType {
		return ""
	}
	return metadata.GraphName
}

func (qpctx *queryPlanningContext) getOwningService(ctx context.Context, parentType *ast.Definition, fieldDef *ast.FieldDefinition) string {
	meta := qpctx.getFederationMetadataForField(fieldDef)
	if meta != nil {
		return meta.GraphName
	}

	return qpctx.getBaseType(ctx, parentType)
}

func (qpctx *queryPlanningContext) getKeyFields(ctx context.Context, scope *Scope, serviceName string, fetchAll bool) (FieldSet, error) {
	var keyFields FieldSet

	keyFields = append(keyFields, &Field{
		Scope:     scope,
		FieldNode: typenameField,
		FieldDef:  graphql.TypeNameMetaFieldDef,
	})

	for _, possibleType := range scope.possibleRuntimeTypes() {
		typ := qpctx.getFederationMetadataForType(possibleType)
		var keys ast.SelectionSet
		if typ != nil && !typ.IsValueType {
			keys = typ.Keys[serviceName]
		}

		if len(keys) == 0 {
			continue
		}

		if fetchAll {
			for _, key := range keys {
				newScope, err := scope.refine(ctx, possibleType, nil)
				if err != nil {
					return nil, err
				}
				fields, err := qpctx.collectFields(ctx, newScope, ast.SelectionSet{key})
				if err != nil {
					return nil, err
				}
				keyFields = append(keyFields, fields...)
			}
		} else {
			newScope, err := scope.refine(ctx, possibleType, nil)
			if err != nil {
				return nil, err
			}
			fields, err := qpctx.collectFields(ctx, newScope, ast.SelectionSet{keys[0]})
			if err != nil {
				return nil, err
			}
			keyFields = append(keyFields, fields...)
		}
	}

	return keyFields, nil
}

func (qpctx *queryPlanningContext) getRequiredFields(ctx context.Context, scope *Scope, fieldDef *ast.FieldDefinition, serviceName string) (FieldSet, error) {
	requiredFields, err := qpctx.getKeyFields(ctx, scope, serviceName, false)
	if err != nil {
		return nil, err
	}

	fieldFederationMetadata := qpctx.getFederationMetadataForField(fieldDef)
	if fieldFederationMetadata != nil && len(fieldFederationMetadata.Requires) != 0 {
		newFields, err := qpctx.collectFields(ctx, scope, fieldFederationMetadata.Requires)
		if err != nil {
			return nil, err
		}
		requiredFields = append(requiredFields, newFields...)
	}

	return requiredFields, nil
}

func (qpctx *queryPlanningContext) getProvidedFields(ctx context.Context, fieldDef *ast.FieldDefinition, serviceName string) (FieldSet, error) {
	returnType := qpctx.schema.Types[fieldDef.Type.Name()]
	if !isCompositeType(returnType) {
		return nil, nil
	}

	scope, err := newScope(qpctx, returnType, nil, nil)
	if err != nil {
		return nil, err
	}
	providedFields, err := qpctx.getKeyFields(ctx, scope, serviceName, true)
	if err != nil {
		return nil, err
	}

	fieldFederationMetadata := qpctx.getFederationMetadataForField(fieldDef)
	if fieldFederationMetadata != nil && len(fieldFederationMetadata.Provides) != 0 {
		newScope, err := newScope(qpctx, returnType, nil, nil)
		if err != nil {
			return nil, err
		}
		fields, err := qpctx.collectFields(ctx, newScope, fieldFederationMetadata.Provides)
		if err != nil {
			return nil, err
		}
		providedFields = append(providedFields, fields...)
		return providedFields, nil
	}

	return providedFields, nil
}

func (qpctx *queryPlanningContext) getPossibleTypes(typ *ast.Definition) []*ast.Definition {
	if typ.IsAbstractType() {
		return qpctx.schema.GetPossibleTypes(typ)
	}

	return []*ast.Definition{typ}
}

func (qpctx *queryPlanningContext) getVariableUsages(selectionSet ast.SelectionSet, fragments ast.FragmentDefinitionList) ast.VariableDefinitionList {
	var usages ast.VariableDefinitionList

	// Construct a document of the selection set and fragment definitions so we
	// can visit them, adding all variable usages to the `usages` object.
	document := &ast.QueryDocument{
		Operations: ast.OperationList{
			&ast.OperationDefinition{
				Operation:    ast.Query,
				SelectionSet: selectionSet,
			},
		},
		Fragments: fragments,
	}

	observers := &validator.Events{}
	observers.OnValue(func(walker *validator.Walker, value *ast.Value) {
		if value.Kind == ast.Variable {
			if usages.ForName(value.Raw) == nil {
				varDef := qpctx.variableDefinitions[value.Raw]
				if varDef == nil {
					panic(fmt.Sprintf("variable %s definition not found", value.Raw))
				}
				usages = append(usages, varDef)
			}
		}
	})
	validator.Walk(qpctx.schema, document, observers)

	return usages
}

func (qpctx *queryPlanningContext) getBaseType(ctx context.Context, parentType *ast.Definition) string {
	return qpctx.getFederationMetadataForType(parentType).GraphName
}

func (qpctx *queryPlanningContext) getFederationMetadataForType(typeDef *ast.Definition) *FederationTypeMetadata {
	return qpctx.metadata.TypeMetadata[typeDef]
}

func (qpctx *queryPlanningContext) getFederationMetadataForField(field *ast.FieldDefinition) *FederationFieldMetadata {
	return qpctx.metadata.FieldMetadata[field]
}
