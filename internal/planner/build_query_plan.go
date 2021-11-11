package planner

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/formatter"
	"github.com/vektah/gqlparser/v2/gqlerror"
	"github.com/vvakame/fedeway/internal/graphql"
	"github.com/vvakame/fedeway/internal/log"
	"github.com/vvakame/fedeway/internal/plan"
	"github.com/vvakame/fedeway/internal/utils"
)

type OperationContext struct {
	QueryDocument *ast.QueryDocument
	OperationName string
	Schema        *ast.Schema
	Fragments     ast.FragmentDefinitionList
	metadata      *ComposedSchema
}

type queryPlanConfig struct {
	autoFragmentation bool
}

type QueryPlanOption func(cfg *queryPlanConfig)

func WithAutoFragmentation(autoFragmentation bool) QueryPlanOption {
	return func(cfg *queryPlanConfig) {
		cfg.autoFragmentation = autoFragmentation
	}
}

func BuildQueryPlan(ctx context.Context, operationContext *OperationContext, opts ...QueryPlanOption) (*plan.QueryPlan, error) {
	logger := log.FromContext(ctx)

	cfg := &queryPlanConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	qpctx, err := buildQueryPlanningContext(operationContext, cfg.autoFragmentation)
	if err != nil {
		return nil, err
	}

	if qpctx.operation.Operation == ast.Subscription {
		return nil, gqlerror.Errorf("subscription is not supported")
	}

	rootType, err := getOperationRootType(ctx, qpctx.schema, qpctx.operation)
	if err != nil {
		return nil, err
	}

	isMutation := qpctx.operation.Operation == ast.Mutation
	_ = isMutation

	logger.Info(
		"building plan",
		//"operation", qpctx.operation.Operation,
		"rootType", rootType.Name,
		//"fragments", qpctx.fragments,
		"autoFragmentation", qpctx.autoFragmentation,
	)

	scope, err := createScope(qpctx, rootType)
	if err != nil {
		return nil, err
	}

	fields, err := qpctx.collectFields(ctx, scope, qpctx.operation.SelectionSet)
	if err != nil {
		return nil, err
	}

	logger.Info(
		"collected root field",
		"fields", fields.MarshalLog(),
	)

	var groups []*FetchGroup
	if qpctx.operation.Operation == ast.Mutation {
		groups, err = splitRootFieldsSerially(ctx, qpctx, fields)
	} else {
		groups, err = splitRootFields(ctx, qpctx, fields)
	}
	if err != nil {
		return nil, err
	}

	var nodes []plan.PlanNode
	for _, group := range groups {
		node, err := executionNodeForGroup(ctx, qpctx, group, rootType)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, node)
	}

	var node plan.PlanNode
	if len(nodes) == 0 {
		// OK. e.g. IntrospectionQuery.
	} else if qpctx.operation.Operation == ast.Mutation {
		node, err = flatWrapSequence(ctx, nodes)
	} else {
		node, err = flatWrapParallel(ctx, nodes)
	}
	if err != nil {
		return nil, err
	}

	qp := &plan.QueryPlan{
		Node: node,
	}

	return qp, nil
}

func getOperationRootType(ctx context.Context, schema *ast.Schema, operation *ast.OperationDefinition) (*ast.Definition, error) {
	switch operation.Operation {
	case ast.Query:
		return schema.Query, nil
	case ast.Mutation:
		return schema.Mutation, nil
	case ast.Subscription:
		return schema.Subscription, nil
	default:
		return nil, fmt.Errorf("unexpected operation: %s", operation.Operation)
	}
}

func splitRootFieldsSerially(ctx context.Context, qpctx *queryPlanningContext, fields FieldSet) ([]*FetchGroup, error) {
	fetchGroups := make([]*FetchGroup, 0)

	groupForField := func(serviceName string) *FetchGroup {
		if len(fetchGroups) != 0 {
			previousGroup := fetchGroups[len(fetchGroups)-1]
			if previousGroup.ServiceName == serviceName {
				return previousGroup
			}
		}
		group := &FetchGroup{
			ServiceName: serviceName,
		}
		fetchGroups = append(fetchGroups, group)

		return group
	}

	err := splitFields(ctx, qpctx, ast.Path{}, fields, func(field *Field) (*FetchGroup, error) {
		scope := field.Scope
		fieldDef := field.FieldDef
		fieldNode := field.FieldNode

		// The root type is necessarily an object type.
		owningService := qpctx.getOwningService(ctx, scope.parentType, fieldDef)

		if owningService == "" {
			return nil, gqlerror.ErrorPosf(fieldNode.Position, "couldn't find owning service for field %s.%s", scope.parentType.Name, fieldDef.Name)
		}

		return groupForField(owningService), nil
	})
	if err != nil {
		return nil, err
	}

	return fetchGroups, nil
}

func splitRootFields(ctx context.Context, qpctx *queryPlanningContext, fields FieldSet) ([]*FetchGroup, error) {
	groupsByService := make(map[string]*FetchGroup)

	groupForService := func(serviceName string) *FetchGroup {
		group := groupsByService[serviceName]
		if group == nil {
			group = &FetchGroup{
				ServiceName: serviceName,
			}
			groupsByService[serviceName] = group
		}
		return group
	}

	err := splitFields(ctx, qpctx, ast.Path{}, fields, func(field *Field) (*FetchGroup, error) {
		scope := field.Scope
		fieldDef := field.FieldDef
		fieldNode := field.FieldNode

		// The root type is necessarily an object type.
		owningService := qpctx.getOwningService(ctx, scope.parentType, fieldDef)

		if owningService == "" {
			return nil, gqlerror.ErrorPosf(fieldNode.Position, "couldn't find owning service for field %s.%s", scope.parentType.Name, fieldDef.Name)
		}

		return groupForService(owningService), nil
	})
	if err != nil {
		return nil, err
	}

	fetchGroups := make([]*FetchGroup, 0, len(groupsByService))
	for _, fetchGroup := range groupsByService {
		fetchGroups = append(fetchGroups, fetchGroup)
	}
	sort.SliceStable(fetchGroups, func(i, j int) bool {
		a := fetchGroups[i]
		b := fetchGroups[j]
		return a.ServiceName < b.ServiceName
	})

	return fetchGroups, nil
}

func splitSubfields(ctx context.Context, qpctx *queryPlanningContext, path ast.Path, fields FieldSet, parentGroup *FetchGroup) error {
	return splitFields(ctx, qpctx, path, fields, func(field *Field) (*FetchGroup, error) {
		scope := field.Scope
		fieldNode := field.FieldNode
		fieldDef := field.FieldDef
		parentType := scope.parentType

		var baseService, owningService string
		// Treat abstract types as value types to replicate type explosion fix
		// XXX: this replicates the behavior of the Rust query planner implementation,
		// in order to get the tests passing before making further changes. But the
		// type explosion fix this depends on is fundamentally flawed and needs to
		// be replaced.
		var isValueType bool
		{
			metadata := qpctx.getFederationMetadataForType(parentType)
			if metadata != nil {
				isValueType = metadata.IsValueType
			}
		}
		if parentType.Kind != ast.Object || isValueType {
			baseService = parentGroup.ServiceName
			owningService = parentGroup.ServiceName
		} else {
			baseService = qpctx.getBaseService(ctx, parentType)
			owningService = qpctx.getOwningService(ctx, parentType, fieldDef)
		}

		if baseService == "" {
			return nil, gqlerror.ErrorPosf(
				fieldNode.Position,
				`couldn't find base service for type "%s"`,
				parentType.Name,
			)
		}

		if owningService == "" {
			return nil, gqlerror.ErrorPosf(
				fieldNode.Position,
				`couldn't find owning service for field "%s.%s"`,
				parentType.Name, fieldDef.Name,
			)
		}

		// Is the field defined on the base service?
		if owningService == baseService {
			// Can we fetch the field from the parent group?
			var matchSomeField bool
			for _, providedField := range parentGroup.ProvidedFields {
				b := matchesField(providedField, field)
				if b {
					matchSomeField = true
					break
				}
			}
			if owningService == parentGroup.ServiceName || matchSomeField {
				return parentGroup, nil
			}

			// We need to fetch the key fields from the parent group first, and then
			// use a dependent fetch from the owning service.
			keyFields, err := qpctx.getKeyFields(ctx, scope, parentGroup.ServiceName, false)
			if err != nil {
				return nil, err
			}
			if len(keyFields) == 0 ||
				(len(keyFields) == 1 &&
					keyFields[0].FieldDef.Name == "__typename") {
				// Only __typename key found.
				// In some cases, the parent group does not have any @key directives.
				// Fall back to owning group's keys
				keyFields, err = qpctx.getKeyFields(ctx, scope, owningService, false)
				if err != nil {
					return nil, err
				}
			}

			return parentGroup.dependentGroupForService(owningService, keyFields)
		}

		// It's an extension field, so we need to fetch the required fields first.
		requiredFields, err := qpctx.getRequiredFields(ctx, scope, fieldDef, owningService)
		if err != nil {
			return nil, err
		}

		// Can we fetch the required fields from the parent group?
		satisfied := true
		for _, requiredField := range requiredFields {
			var found bool
			for _, providedField := range parentGroup.ProvidedFields {
				if matchesField(requiredField, providedField) {
					found = true
					break
				}
			}
			if !found {
				satisfied = false
				break
			}
		}
		if satisfied {
			if owningService == parentGroup.ServiceName {
				return parentGroup, nil
			}
			return parentGroup.dependentGroupForService(
				owningService,
				requiredFields,
			)
		}

		// We need to go through the base group first.
		keyFields, err := qpctx.getKeyFields(ctx, scope, parentGroup.ServiceName, false)
		if err != nil {
			return nil, err
		}

		if len(keyFields) == 0 {
			return nil, gqlerror.ErrorPosf(
				fieldNode.Position,
				`couldn't find keys for type "%s" in service "%s"`,
				parentType.Name, baseService,
			)
		}

		if baseService == parentGroup.ServiceName {
			return parentGroup.dependentGroupForService(owningService, requiredFields)
		}

		baseGroup, err := parentGroup.dependentGroupForService(baseService, keyFields)
		if err != nil {
			return nil, err
		}

		return baseGroup.dependentGroupForService(owningService, requiredFields)
	})
}

func executionNodeForGroup(ctx context.Context, qpctx *queryPlanningContext, fetchGroup *FetchGroup, parentType *ast.Definition) (plan.PlanNode, error) {
	serviceName := fetchGroup.ServiceName
	fields := fetchGroup.Fields
	requiredFields := fetchGroup.RequiredFields
	internalFragments := fetchGroup.InternalFragments
	mergeAt := fetchGroup.MergeAt
	dependentGroups := fetchGroup.dependentGroups()

	selectionSet := selectionSetFromFieldSet(qpctx.schema, fields, parentType)
	requires := selectionSetFromFieldSet(qpctx.schema, requiredFields, nil)
	variableUsages := qpctx.getVariableUsages(selectionSet, internalFragments)

	var operation *ast.QueryDocument
	var err error
	if len(requires) != 0 {
		operation, err = operationForEntitiesFetch(selectionSet, variableUsages, internalFragments)
		if err != nil {
			return nil, err
		}
	} else {
		operation, err = operationForRootFetch(selectionSet, variableUsages, internalFragments, qpctx.operation.Operation)
		if err != nil {
			return nil, err
		}
	}

	variableUsageNames := make([]string, 0, len(variableUsages))
	for _, variableUsage := range variableUsages {
		variableUsageNames = append(variableUsageNames, variableUsage.Variable)
	}

	var buf bytes.Buffer
	formatter.NewFormatter(&buf).FormatQueryDocument(operation)

	fetchNode := &plan.FetchNode{
		ServiceName:    serviceName,
		VariableUsages: variableUsageNames,
		Requires:       plan.TrimSelectionNodes(requires),
		Operation:      buf.String(),
	}

	var node plan.PlanNode
	if len(mergeAt) > 0 {
		node = &plan.FlattenNode{
			Path: mergeAt,
			Node: fetchNode,
		}
	} else {
		node = fetchNode
	}

	if len(dependentGroups) > 0 {
		var dependentNodes []plan.PlanNode
		for _, dependentGroup := range dependentGroups {
			node, err := executionNodeForGroup(ctx, qpctx, dependentGroup, nil)
			if err != nil {
				return nil, err
			}
			dependentNodes = append(dependentNodes, node)
		}

		dependentNode, err := flatWrapParallel(ctx, dependentNodes)
		if err != nil {
			return nil, err
		}
		return flatWrapSequence(ctx, []plan.PlanNode{node, dependentNode})
	}

	return node, nil
}

func operationForRootFetch(selectionSet ast.SelectionSet, variableUsages ast.VariableDefinitionList, internalFragments ast.FragmentDefinitionList, operation ast.Operation) (*ast.QueryDocument, error) {
	if operation == "" {
		operation = ast.Query
	}

	return &ast.QueryDocument{
		Operations: ast.OperationList{
			&ast.OperationDefinition{
				Operation:           operation,
				VariableDefinitions: variableUsages,
				SelectionSet:        selectionSet,
			},
		},
		Fragments: internalFragments,
	}, nil
}

func operationForEntitiesFetch(selectionSet ast.SelectionSet, variableUsages ast.VariableDefinitionList, internalFragments ast.FragmentDefinitionList) (*ast.QueryDocument, error) {
	representationsVariable := &ast.Value{
		Raw:  "representations",
		Kind: ast.Variable,
	}

	var variableDefinitions ast.VariableDefinitionList
	variableDefinitions = append(variableDefinitions, &ast.VariableDefinition{
		Variable: representationsVariable.Raw,
		Type: &ast.Type{
			Elem: &ast.Type{
				NamedType: "_Any",
				NonNull:   true,
			},
			NonNull: true,
		},
	})
	variableDefinitions = append(variableDefinitions, variableUsages...)

	return &ast.QueryDocument{
		Operations: ast.OperationList{
			&ast.OperationDefinition{
				Operation:           ast.Query,
				VariableDefinitions: variableDefinitions,
				SelectionSet: ast.SelectionSet{
					&ast.Field{
						Name: "_entities",
						Arguments: ast.ArgumentList{
							&ast.Argument{
								Name:  representationsVariable.Raw,
								Value: representationsVariable,
							},
						},
						SelectionSet: selectionSet,
					},
				},
			},
		},
		Fragments: internalFragments,
	}, nil
}

type FetchGroup struct {
	ServiceName       string
	Fields            FieldSet
	InternalFragments ast.FragmentDefinitionList

	RequiredFields FieldSet
	ProvidedFields FieldSet

	MergeAt ast.Path

	dependentGroupsByService map[string]*FetchGroup
	otherDependentGroups     []*FetchGroup
}

func (fg *FetchGroup) dependentGroupForService(serviceName string, requiredFields FieldSet) (*FetchGroup, error) {
	if fg.dependentGroupsByService == nil {
		fg.dependentGroupsByService = make(map[string]*FetchGroup)
	}

	group := fg.dependentGroupsByService[serviceName]

	if group == nil {
		group = &FetchGroup{
			ServiceName: serviceName,
			MergeAt:     fg.MergeAt,
		}
		fg.dependentGroupsByService[serviceName] = group
	}

	if len(requiredFields) != 0 {
		group.RequiredFields = append(group.RequiredFields, requiredFields...)
		fg.Fields = append(fg.Fields, requiredFields...)
	}

	return group, nil
}

func (fg *FetchGroup) dependentGroups() []*FetchGroup {
	serviceNames := make([]string, 0, len(fg.dependentGroupsByService))
	for serviceName := range fg.dependentGroupsByService {
		serviceNames = append(serviceNames, serviceName)
	}
	sort.Strings(serviceNames)

	var result []*FetchGroup
	for _, serviceName := range serviceNames {
		result = append(result, fg.dependentGroupsByService[serviceName])
	}
	result = append(result, fg.otherDependentGroups...)

	return result
}

func (fg *FetchGroup) mergeDependentGroups(that *FetchGroup) {
	for _, dependentGroup := range that.dependentGroups() {
		// In order to avoid duplicate fetches, we try to find existing dependent
		// groups with the same service and merge path first.
		var existingDependentGroup *FetchGroup
		for _, group := range fg.dependentGroups() {
			if group.ServiceName == dependentGroup.ServiceName &&
				group.MergeAt.String() == dependentGroup.MergeAt.String() {
				existingDependentGroup = group
				break
			}
		}
		if existingDependentGroup != nil {
			existingDependentGroup.mergeDependentGroups(dependentGroup)
		} else {
			fg.otherDependentGroups = append(fg.otherDependentGroups, dependentGroup)
		}
	}
}

func BuildOperationContext(ctx context.Context, cs *ComposedSchema, document *ast.QueryDocument, operationName string) (*OperationContext, error) {
	var operation *ast.OperationDefinition
	if operationName != "" {
		operation = document.Operations.ForName(operationName)
	} else if len(document.Operations) == 1 {
		operation = document.Operations[0]
	} else {
		return nil, gqlerror.Errorf("must provide operation name if query contains multiple operations")
	}
	if operation == nil {
		if operationName != "" {
			return nil, gqlerror.Errorf(`Unknown operation named "%s"`, operationName)
		} else {
			return nil, gqlerror.Errorf("must provide an operation")
		}
	}

	opctx := &OperationContext{
		QueryDocument: document,
		OperationName: operationName,
		Schema:        cs.Schema,
		Fragments:     document.Fragments,
		metadata:      cs,
	}

	return opctx, nil
}

func flatWrapSequence(ctx context.Context, nodes []plan.PlanNode) (plan.PlanNode, error) {
	if len(nodes) == 0 {
		return nil, errors.New("nodes is 0 length")
	} else if len(nodes) == 1 {
		return nodes[0], nil
	}

	var newNodes []plan.PlanNode
	for _, node := range nodes {
		switch node := node.(type) {
		case *plan.SequenceNode:
			newNodes = append(newNodes, node.Nodes...)
		default:
			newNodes = append(newNodes, node)
		}
	}

	node := &plan.SequenceNode{
		Nodes: newNodes,
	}

	return node, nil
}

func flatWrapParallel(ctx context.Context, nodes []plan.PlanNode) (plan.PlanNode, error) {
	if len(nodes) == 0 {
		return nil, errors.New("nodes is 0 length")
	} else if len(nodes) == 1 {
		return nodes[0], nil
	}

	var newNodes []plan.PlanNode
	for _, node := range nodes {
		switch node := node.(type) {
		case *plan.ParallelNode:
			newNodes = append(newNodes, node.Nodes...)
		default:
			newNodes = append(newNodes, node)
		}
	}

	node := &plan.ParallelNode{
		Nodes: newNodes,
	}

	return node, nil
}

func splitFields(ctx context.Context, qpctx *queryPlanningContext, path ast.Path, fields FieldSet, groupForField func(field *Field) (*FetchGroup, error)) error {
	logger := log.FromContext(ctx)

	fieldSets := groupByResponseName(fields)
	for _, fieldsForResponseName := range fieldSets {
		fieldsForScopes := groupByScope(fieldsForResponseName)
	SCOPE:
		for _, fieldsForScope := range fieldsForScopes {
			// Field nodes that share the same response name and scope are guaranteed to have the same field name and
			// arguments. We only need the other nodes when merging selection sets, to take node-specific subfields and
			// directives into account.

			logger.Info(
				"splitFields loop",
				"fieldsForScope", fieldsForScope.MarshalLog(),
			)

			if len(fieldsForScope) == 0 {
				panic("fieldsForScope length is 0")
			}

			// All the fields in fieldsForScope have the same scope, so that means the same parent type and possible runtime
			// types, so we effectively can just use the first one and ignore the rest.
			field := fieldsForScope[0]
			scope := field.Scope
			fieldDef := field.FieldDef
			parentType := scope.parentType

			if fieldDef.Name == graphql.TypeNameMetaFieldDef.Name {
				schema := qpctx.schema
				var rootTypes []*ast.Definition
				if schema.Query != nil {
					rootTypes = append(rootTypes, schema.Query)
				}
				if schema.Mutation != nil {
					rootTypes = append(rootTypes, schema.Mutation)
				}
				if schema.Subscription != nil {
					rootTypes = append(rootTypes, schema.Subscription)
				}
				for _, rootType := range rootTypes {
					if rootType.Name == parentType.Name {
						logger.Info("skipping __typename for root types")
						continue SCOPE
					}
				}
			}

			// We skip introspection fields like `__schema` and `__type`.
			if graphql.IsIntrospectionType(fieldDef.Type.Name()) {
				logger.Info("skipping introspection type", "type", fieldDef.Type.Name())
				continue
			}

			possibleRuntimeTypeNames := make([]string, 0, len(scope.possibleRuntimeTypes()))
			for _, possibleRuntimeType := range scope.possibleRuntimeTypes() {
				possibleRuntimeTypeNames = append(possibleRuntimeTypeNames, possibleRuntimeType.Name)
			}
			if parentType.Kind == ast.Object && containsType(scope.possibleRuntimeTypes(), parentType) {
				logger.Info(
					"parent type is object and included in scope's possible runtime types",
					"parentType", parentType.Name,
					"possibleRuntimeTypes", possibleRuntimeTypeNames,
				)

				// If parent type is an object type, we can directly look for the right
				// group.
				group, err := groupForField(field)
				if err != nil {
					return err
				}

				newField, err := completeField(ctx, qpctx, scope, group, path, fieldsForScope)
				if err != nil {
					return err
				}
				group.Fields = append(group.Fields, newField)

			} else {
				logger.Info(
					"parent type is not object or not included in scope's possible runtime types",
					"parentType", parentType.Name,
					"possibleRuntimeTypes", possibleRuntimeTypeNames,
				)

				// For interfaces however, we need to look at all possible runtime types.
				var possibleFieldDefs []*ast.FieldDefinition
				hasNoExtendingFieldDefs := true
				for _, typ := range scope.possibleRuntimeTypes() {
					fd, err := qpctx.getFieldDef(ctx, typ, field.FieldNode)
					if err != nil {
						return err
					}
					possibleFieldDefs = append(possibleFieldDefs, fd)
					meta := qpctx.getFederationMetadataForField(fd)
					if meta != nil && meta.GraphName != "" {
						hasNoExtendingFieldDefs = false
					}
				}

				if hasNoExtendingFieldDefs {
					logger.Info("no field of scope's possible runtime have federation directives, avoid type explosion")
					group, err := groupForField(field)
					if err != nil {
						return err
					}

					newField, err := completeField(ctx, qpctx, scope, group, path, fieldsForScope)
					if err != nil {
						return err
					}
					group.Fields = append(group.Fields, newField)
					continue
				}

				// We keep track of which possible runtime parent types can be fetched
				// from which group,
				groupsIndex := make([]*FetchGroup, 0)
				groupsByRuntimeParentTypes := make(map[*FetchGroup][]*ast.Definition)
				groupsByRuntimeParentNames := make(map[string][]string)

				logger.Info("computing fetch groups by runtime parent types")
				for _, runtimeParentType := range scope.possibleRuntimeTypes() {
					fieldDef, err := qpctx.getFieldDef(ctx, runtimeParentType, field.FieldNode)
					if err != nil {
						return err
					}

					newScope, err := scope.refine(ctx, runtimeParentType, nil)
					if err != nil {
						return err
					}

					key, err := groupForField(&Field{
						Scope:     newScope,
						FieldNode: field.FieldNode,
						FieldDef:  fieldDef,
					})
					if err != nil {
						return err
					}
					if len(groupsByRuntimeParentTypes[key]) == 0 {
						groupsIndex = append(groupsIndex, key)
					}
					groupsByRuntimeParentTypes[key] = append(groupsByRuntimeParentTypes[key], runtimeParentType)
					groupsByRuntimeParentNames[key.ServiceName] = append(groupsByRuntimeParentNames[key.ServiceName], runtimeParentType.Name)
				}

				logger.Info(
					"groups by runtime parent type",
					"groupsByRuntimeParentTypes", groupsByRuntimeParentNames,
				)
				for _, group := range groupsIndex {
					runtimeParentTypes := groupsByRuntimeParentTypes[group]

					for _, runtimeParentType := range runtimeParentTypes {
						fieldDef, err := qpctx.getFieldDef(ctx, runtimeParentType, field.FieldNode)
						if err != nil {
							return err
						}

						fieldsWithRuntimeParentType := make(FieldSet, 0, len(fieldsForScope))
						for _, field := range fieldsForScope {
							copied := *field
							copied.FieldDef = fieldDef
							fieldsWithRuntimeParentType = append(fieldsWithRuntimeParentType, &copied)
						}

						newScope, err := scope.refine(ctx, runtimeParentType, nil)
						if err != nil {
							return err
						}

						newField, err := completeField(ctx, qpctx, newScope, group, path, fieldsWithRuntimeParentType)
						if err != nil {
							return err
						}
						group.Fields = append(group.Fields, newField)
					}
				}
			}
		}
	}

	return nil
}

func completeField(ctx context.Context, qpctx *queryPlanningContext, scope *Scope, parentGroup *FetchGroup, path ast.Path, fields FieldSet) (*Field, error) {
	fieldNode := fields[0].FieldNode
	fieldDef := fields[0].FieldDef
	returnType := qpctx.schema.Types[fieldDef.Type.Name()]

	if !isCompositeType(returnType) {
		// FIXME: We should look at all field nodes to make sure we take directives
		// into account (or remove directives for the time being).
		return &Field{
			Scope:     scope,
			FieldNode: fieldNode,
			FieldDef:  fieldDef,
		}, nil
	}

	// For composite types, we need to recurse.

	fieldPath := addPath(path, getResponseName(fieldNode), fieldDef.Type)

	subGroup := &FetchGroup{
		ServiceName: parentGroup.ServiceName,
	}
	subGroup.MergeAt = fieldPath

	providedFields, err := qpctx.getProvidedFields(ctx, fieldDef, parentGroup.ServiceName)
	if err != nil {
		return nil, err
	}
	subGroup.ProvidedFields = providedFields

	// For abstract types, we always need to request `__typename`
	if utils.IsAbstractType(returnType) {
		newScope, err := newScope(qpctx, returnType, nil, nil)
		if err != nil {
			return nil, err
		}
		subGroup.Fields = append(subGroup.Fields, &Field{
			Scope:     newScope,
			FieldNode: typenameField,
			FieldDef:  graphql.TypeNameMetaFieldDef,
		})
	}

	subfields, err := collectSubfields(ctx, qpctx, returnType, fields)
	if err != nil {
		return nil, err
	}

	err = splitSubfields(ctx, qpctx, fieldPath, subfields, subGroup)
	if err != nil {
		return nil, err
	}

	// Because of the way we split fields, we may have created multiple
	// dependent groups to the same subgraph for the same path. We therefore
	// attempt to merge dependent groups of the subgroup and of the parent group
	// to avoid duplicate fetches.
	parentGroup.mergeDependentGroups(subGroup)

	var definition *ast.FragmentDefinition
	selectionSet := selectionSetFromFieldSet(qpctx.schema, subGroup.Fields, returnType)

	if qpctx.autoFragmentation && len(subGroup.Fields) > 2 {
		definition, selectionSet, err = getInternalFragment(selectionSet, returnType, qpctx)
		if err != nil {
			return nil, err
		}
		parentGroup.InternalFragments = append(parentGroup.InternalFragments, definition)
	}

	// "Hoist" internalFragments of the subGroup into the parentGroup so all
	// fragments can be included in the final request for the root FetchGroup
	for _, fragment := range subGroup.InternalFragments {
		parentGroup.InternalFragments = append(parentGroup.InternalFragments, fragment)
	}

	copied := *fieldNode
	copied.SelectionSet = selectionSet
	return &Field{
		Scope:     scope,
		FieldNode: &copied,
		FieldDef:  fieldDef,
	}, nil
}

func getInternalFragment(selectionSet ast.SelectionSet, returnType *ast.Definition, qpctx *queryPlanningContext) (*ast.FragmentDefinition, ast.SelectionSet, error) {
	key, err := json.Marshal(selectionSet)
	if err != nil {
		return nil, nil, err
	}
	fragment := qpctx.internalFragments[string(key)]
	if fragment == nil {
		name := fmt.Sprintf("__QueryPlanFragment_%d__", len(qpctx.internalFragments))

		definition := &ast.FragmentDefinition{
			Name:          name,
			TypeCondition: returnType.Name,
			SelectionSet:  selectionSet,
		}
		fragmentSelection := ast.SelectionSet{
			&ast.FragmentSpread{
				Name: name,
			},
		}

		fragment = &internalFragment{
			Name:         name,
			Definition:   definition,
			SelectionSet: fragmentSelection,
		}
		qpctx.internalFragments[string(key)] = fragment
	}

	return fragment.Definition, fragment.SelectionSet, nil
}

func collectSubfields(ctx context.Context, qpctx *queryPlanningContext, returnType *ast.Definition, fields FieldSet) (FieldSet, error) {
	var subfields FieldSet

	for _, field := range fields {
		selectionSet := field.FieldNode.SelectionSet

		if len(selectionSet) != 0 {
			newScope, err := newScope(qpctx, returnType, nil, nil)
			if err != nil {
				return nil, err
			}
			newFields, err := qpctx.collectFields(ctx, newScope, selectionSet)
			if err != nil {
				return nil, err
			}

			subfields = append(subfields, newFields...)
		}
	}

	return subfields, nil
}

func addPath(path ast.Path, name string, typ *ast.Type) ast.Path {
	path = append(ast.Path{}, path...)
	path = append(path, ast.PathName(name))
	for typ != nil && typ.NamedType == "" {
		if typ.Elem != nil {
			path = append(path, ast.PathName("@"))
		}

		typ = typ.Elem
	}

	return path
}
