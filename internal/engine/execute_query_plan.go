package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/vvakame/fedeway/internal/execute"
	"sync"

	"github.com/99designs/gqlgen/graphql"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"
	"github.com/vektah/gqlparser/v2/parser"
	"github.com/vvakame/fedeway/internal/plan"
	"github.com/vvakame/fedeway/internal/utils"
)

type ServiceMap map[string]DataSource

type ExecutionContext struct {
	QueryPlan        *plan.QueryPlan
	OperationContext *OperationContext
	ServiceMap       ServiceMap
	RequestContext   *graphql.OperationContext
	Errors           gqlerror.List
}

type ResultMap map[string]interface{}

func ExecuteQueryPlan(ctx context.Context, queryPlan *plan.QueryPlan, serviceMap ServiceMap, requestContext *graphql.OperationContext, operationContext *OperationContext) *graphql.Response {
	ec := &ExecutionContext{
		QueryPlan:        queryPlan,
		OperationContext: operationContext,
		ServiceMap:       serviceMap,
		RequestContext:   requestContext,
	}

	// TODO ResultMap みたいな独自の型にしてLockをちゃんとやらないといけない
	data := make(ResultMap)

	if queryPlan.Node != nil {
		executeNode(ctx, ec, queryPlan.Node, data, nil)
	}

	return execute.Execute(ctx, &execute.ExecutionArgs{
		Schema:         operationContext.Schema,
		RawQuery:       requestContext.RawQuery,
		Document:       requestContext.Doc,
		RootValue:      data,
		VariableValues: requestContext.Variables,
		OperationName:  requestContext.OperationName,
		FieldResolver:  nil, // TODO
		TypeResolver:   nil, // TODO
	})
}

// Note: this function always returns a protobuf QueryPlanNode tree, even if
// we're going to ignore it, because it makes the code much simpler and more
// typesafe. However, it doesn't actually ask for traces from the backend
// service unless we are capturing traces for Studio.
// ... original comment said.
func executeNode(ctx context.Context, ec *ExecutionContext, node plan.PlanNode, results ResultMap, path ast.Path) {
	// TODO 明日用のメモ results の型が ResultMap だとまずい
	// JSの実装を読むとResultMapでよさそうに見えるけど、実際は flattenResultsAtPath の処理結果が array になることがある
	// さらに面倒なことに、arrayの型がわからないのだよなぁ…

	switch node := node.(type) {
	case *plan.SequenceNode:
		for _, childNode := range node.Nodes {
			executeNode(ctx, ec, childNode, results, path)
		}
	case *plan.ParallelNode:
		var wg sync.WaitGroup
		for _, childNode := range node.Nodes {
			wg.Add(1)
			childNode := childNode
			go func() {
				executeNode(ctx, ec, childNode, results, path)
				wg.Done()
			}()
		}
		wg.Wait()
	case *plan.FlattenNode:
		newPath := make(ast.Path, 0, len(path)+len(node.Path))
		newPath = append(newPath, path...)
		newPath = append(newPath, node.Path...)
		executeNode(
			ctx,
			ec,
			node.Node,
			flattenResultsAtPath(results, node.Path).(map[string]interface{}),
			newPath,
		)
	case *plan.FetchNode:
		gErr := executeFetch(
			ctx,
			ec,
			node,
			results,
			path,
		)
		if gErr != nil {
			ec.Errors = append(ec.Errors, gErr)
		}

	default:
		// ignore
	}
}

func executeFetch(ctx context.Context, ec *ExecutionContext, fetch *plan.FetchNode, results interface{}, path ast.Path) *gqlerror.Error {
	service := ec.ServiceMap[fetch.ServiceName]

	if service == nil {
		return gqlerror.Errorf(`couldn't find service with name "%s"`, fetch.ServiceName)
	}

	sendOperation := func(context *ExecutionContext, source string, variables map[string]interface{}) (ResultMap, *gqlerror.Error) {
		doc, gErr := parser.ParseQuery(&ast.Source{Input: source})
		if gErr != nil {
			return nil, gErr
		}
		oc := &graphql.OperationContext{
			RawQuery:             source,
			Variables:            variables,
			Doc:                  doc,
			Operation:            doc.Operations.ForName(""),
			DisableIntrospection: true,
			RecoverFunc:          graphql.DefaultRecover, // TODO configurable
			Stats:                graphql.Stats{},        // TODO
		}
		ctx = graphql.WithOperationContext(ctx, oc)
		response := service.Process(ctx, oc)

		if len(response.Errors) != 0 {
			for _, gErr := range response.Errors {
				gErr := downstreamServiceError(gErr, fetch.ServiceName)
				context.Errors = append(context.Errors, gErr)
			}
		}

		result := ResultMap{}
		err := json.Unmarshal(response.Data, &result)
		if err != nil {
			return nil, gqlerror.Errorf("json unmarshal error: %w", err)
		}

		return result, nil
	}

	entities := make([]interface{}, 0)
	if v, ok := results.([]interface{}); ok {
		if v != nil {
			entities = append(entities, v...)
		}
	} else {
		entities = []interface{}{results}
	}

	if len(entities) < 1 {
		return nil
	}

	variables := make(map[string]interface{})
	if len(ec.RequestContext.Variables) != 0 {
		for _, variableName := range fetch.VariableUsages {
			providedVariable, ok := ec.RequestContext.Variables[variableName]
			if ok {
				variables[variableName] = providedVariable
			}
		}
	}

	if len(fetch.Requires) == 0 {
		dataReceivedFromService, gErr := sendOperation(ec, fetch.Operation, variables)
		if gErr != nil {
			return gErr
		}
		for _, entity := range entities {
			utils.DeepMerge(entity, dataReceivedFromService)
		}
	} else {
		requires := fetch.Requires

		var representations []interface{}
		var representationToEntity []int

		for index, entity := range entities {
			originalEntity := entity
			entity, ok := originalEntity.(map[string]interface{})
			if !ok {
				return gqlerror.Errorf("unexpected entity type: %T", originalEntity)
			}
			representation, gErr := executeSelectionSet(ctx, ec.OperationContext, entity, requires)
			if gErr != nil {
				return gErr
			}
			if representation != nil && representation["__typename"] != nil {
				representations = append(representations, representation)
				representationToEntity = append(representationToEntity, index)
			}
		}

		// If there are no representations, that means the type conditions in
		// the requires don't match any entities.
		if len(representations) < 1 {
			return nil
		}

		if _, ok := variables["representations"]; ok {
			return gqlerror.Errorf(`variables cannot contain key "representations"`)
		}

		newVariables := make(map[string]interface{}, len(variables)+1)
		for k, v := range variables {
			newVariables[k] = v
		}
		newVariables["representations"] = representations
		dataReceivedFromService, gErr := sendOperation(ec, fetch.Operation, newVariables)
		if gErr != nil {
			return gErr
		}

		if dataReceivedFromService == nil {
			return nil
		}

		var receivedEntities []interface{}
		if v, ok := dataReceivedFromService["_entities"]; !ok {
			return gqlerror.Errorf(`expected "data._entities" in response to be an array`)
		} else if v, ok := v.([]interface{}); !ok {
			return gqlerror.Errorf(`expected "data._entities" in response to be an array`)
		} else {
			receivedEntities = v
		}

		if len(receivedEntities) != len(representations) {
			return gqlerror.Errorf(`expected "data._entities" to contain %d elements`, len(representations))
		}

		for i := range entities {
			utils.DeepMerge(entities[representationToEntity[i]], receivedEntities[i])
		}
	}

	return nil
}

func executeSelectionSet(ctx context.Context, operationContext *OperationContext, source map[string]interface{}, selections []plan.QueryPlanSelectionNode) (map[string]interface{}, *gqlerror.Error) {
	// If the underlying service has returned null for the parent (source)
	// then there is no need to iterate through the parent's selection set
	if source == nil {
		return nil, nil
	}

	var queryPlanSelectionNodesToSelectionSet func(baseSelections []plan.QueryPlanSelectionNode) ast.SelectionSet
	queryPlanSelectionNodesToSelectionSet = func(baseSelections []plan.QueryPlanSelectionNode) ast.SelectionSet {
		selections := make(ast.SelectionSet, 0, len(baseSelections))
		for _, baseSelection := range baseSelections {
			switch baseSelection := baseSelection.(type) {
			case *plan.QueryPlanFieldNode:
				selections = append(selections, &ast.Field{
					Alias: baseSelection.Alias,
					Name:  baseSelection.Name,
				})
			case *plan.QueryPlanInlineFragmentNode:
				selections = append(selections, &ast.InlineFragment{
					TypeCondition: baseSelection.TypeCondition,
					SelectionSet:  queryPlanSelectionNodesToSelectionSet(baseSelection.Selections),
				})
			default:
				panic("nil selection found")
			}
		}
		return selections
	}

	result := make(map[string]interface{})

	for _, selection := range selections {
		switch selection := selection.(type) {
		case *plan.QueryPlanFieldNode:
			responseName := selection.Name
			if selection.Alias != "" {
				responseName = selection.Alias
			}
			baseSelections := selection.Selections
			selections := queryPlanSelectionNodesToSelectionSet(baseSelections)

			if source, ok := source[responseName]; !ok {
				return nil, gqlerror.Errorf(`field "%s" was not found in response`, responseName)
			} else if source, ok := source.([]interface{}); ok {
				for _, value := range source {
					if len(selections) != 0 {
						nextValue, ok := value.(map[string]interface{})
						if !ok {
							return nil, gqlerror.Errorf("unexpected type: %T", value)
						}
						ss, gErr := executeSelectionSet(ctx, operationContext, nextValue, baseSelections)
						if gErr != nil {
							return nil, gErr
						}
						result[responseName] = ss
					} else {
						result[responseName] = value
					}
				}
			}

		case *plan.QueryPlanInlineFragmentNode:
			if selection.TypeCondition == "" {
				continue
			}

			if source == nil {
				continue
			}
			typename, ok := source["__typename"].(string)
			if !ok {
				return nil, gqlerror.Errorf("unexpected type: %T", source["__typename"])
			}

			if doesTypeConditionMatch(operationContext.Schema, selection.TypeCondition, typename) {
				value, gErr := executeSelectionSet(ctx, operationContext, source, selection.Selections)
				if gErr != nil {
					return nil, gErr
				}
				utils.DeepMerge(result, value)
			}

		default:
			panic("nil selection found")
		}
	}

	return result, nil
}

func doesTypeConditionMatch(schema *ast.Schema, typeCondition string, typename string) bool {
	if typeCondition == typename {
		return true
	}

	typ := schema.Types[typename]
	if typ == nil {
		return false
	}

	conditionalType := schema.Types[typeCondition]
	if conditionalType == nil {
		return false
	}

	if utils.IsAbstractType(conditionalType) {
		// NOTE original: schema.isSubType(conditionalType, type)
		return utils.IsTypeDefSubTypeOf(schema, typ, conditionalType)
	}

	return false
}

func flattenResultsAtPath(value interface{}, path ast.Path) interface{} {
	if len(path) == 0 {
		return value
	}
	if value == nil {
		return nil
	}

	current := path[0]
	rest := path[1:]
	if current == ast.PathName("@") {
		values := value.([]interface{})
		var newValues []interface{}
		for _, element := range values {
			newValues = append(newValues, flattenResultsAtPath(element, rest))
		}
		return newValues
	} else {
		value := value.(map[string]interface{})
		newElement := flattenResultsAtPath(value[string(current.(ast.PathName))], rest)
		value[string(current.(ast.PathName))] = newElement
		return newElement
	}
}

func downstreamServiceError(originalError *gqlerror.Error, serviceName string) *gqlerror.Error {
	message := originalError.Message
	extensions := originalError.Extensions

	if message == "" {
		message = fmt.Sprintf(`error while fetching subquery from service "%s"`, serviceName)
	}

	newExtensions := map[string]interface{}{
		"code": "DOWNSTREAM_SERVICE_ERROR",
		// XXX The presence of a serviceName in extensions is used to
		// determine if this error should be captured for metrics reporting.
		"serviceName": serviceName,
	}
	for k, v := range extensions {
		newExtensions[k] = v
	}
	extensions = newExtensions

	newErr := gqlerror.WrapPath(nil, originalError)
	newErr.Message = message
	newErr.Extensions = extensions
	return newErr
}
