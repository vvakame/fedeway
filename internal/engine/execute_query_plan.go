package engine

import (
	"bytes"
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

type executionContext struct {
	QueryPlan      *plan.QueryPlan
	Schema         *ast.Schema
	ServiceMap     ServiceMap
	RequestContext *graphql.OperationContext
	Errors         gqlerror.List
}

func ExecuteQueryPlan(ctx context.Context, queryPlan *plan.QueryPlan, serviceMap ServiceMap, schema *ast.Schema, requestContext *graphql.OperationContext) *graphql.Response {
	ec := &executionContext{
		QueryPlan:      queryPlan,
		Schema:         schema,
		ServiceMap:     serviceMap,
		RequestContext: requestContext,
	}

	var resultLock sync.Mutex
	data := make(map[string]interface{})

	if queryPlan.Node != nil {
		executeNode(ctx, ec, queryPlan.Node, &resultLock, data, nil)
	}

	resp := execute.Execute(ctx, &execute.ExecutionArgs{
		Schema:         schema,
		RawQuery:       requestContext.RawQuery,
		Document:       requestContext.Doc,
		RootValue:      data,
		VariableValues: requestContext.Variables,
		OperationName:  requestContext.OperationName,
		FieldResolver:  nil,
		TypeResolver:   nil,
	})
	if len(resp.Errors) != 0 {
		// ここではエラーが発生しないはず
		return &graphql.Response{Errors: resp.Errors}
	}
	if len(ec.Errors) != 0 {
		resp.Errors = ec.Errors
	}

	return resp
}

// Note: this function always returns a protobuf QueryPlanNode tree, even if
// we're going to ignore it, because it makes the code much simpler and more
// typesafe. However, it doesn't actually ask for traces from the backend
// service unless we are capturing traces for Studio.
// ... original comment said.
func executeNode(ctx context.Context, ec *executionContext, node plan.PlanNode, resultLock *sync.Mutex, results interface{}, path ast.Path) {
	// TODO capture panic & recover
	// 以下はgqlgenでの対応パターン
	//defer func() {
	//	if r := recover(); r != nil {
	//		ec.Error(ctx, ec.Recover(ctx, r))
	//		ret = graphql.Null
	//	}
	//}()

	switch node := node.(type) {
	case *plan.SequenceNode:
		for _, childNode := range node.Nodes {
			executeNode(ctx, ec, childNode, resultLock, results, path)
		}
	case *plan.ParallelNode:
		var wg sync.WaitGroup
		for _, childNode := range node.Nodes {
			wg.Add(1)
			childNode := childNode
			go func() {
				executeNode(ctx, ec, childNode, resultLock, results, path)
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
			resultLock,
			flattenResultsAtPath(resultLock, true, results, node.Path),
			newPath,
		)
	case *plan.FetchNode:
		gErr := executeFetch(
			ctx,
			ec,
			node,
			resultLock,
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

func executeFetch(ctx context.Context, ec *executionContext, fetch *plan.FetchNode, resultLock *sync.Mutex, results interface{}, path ast.Path) *gqlerror.Error {
	service := ec.ServiceMap[fetch.ServiceName]

	if service == nil {
		return gqlerror.Errorf(`couldn't find service with name "%s"`, fetch.ServiceName)
	}

	sendOperation := func(ec *executionContext, source string, variables map[string]interface{}) (map[string]interface{}, *gqlerror.Error) {
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
			RecoverFunc:          graphql.DefaultRecover, // TODO make configurable
			ResolverMiddleware: func(ctx context.Context, next graphql.Resolver) (res interface{}, err error) {
				return next(ctx)
			},
			Stats: graphql.Stats{}, // TODO support stats
		}

		response := service.Process(ctx, oc)

		if len(response.Errors) != 0 {
			for _, gErr := range response.Errors {
				gErr := downstreamServiceError(gErr, fetch.ServiceName, path)
				ec.Errors = append(ec.Errors, gErr)
			}
		}

		if len(response.Data) == 0 {
			return nil, nil
		}

		dec := json.NewDecoder(bytes.NewBuffer(response.Data))
		// UseNumber しないと 1 とかが float64 になってしまい UnmarshalInt とかがコケる
		dec.UseNumber()

		result := make(map[string]interface{})
		err := dec.Decode(&result)
		if err != nil {
			return nil, gqlerror.Errorf("json unmarshal error: %s", err)
		}

		return result, nil
	}

	resultLock.Lock()
	defer resultLock.Unlock()
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

		representations := make([]interface{}, 0, len(entities))
		representationToEntity := make([]int, 0, len(entities))

		for index, entity := range entities {
			if entity == nil {
				continue
			}
			originalEntity := entity
			entity, ok := originalEntity.(map[string]interface{})
			if !ok {
				return gqlerror.Errorf("unexpected entity type: %T", originalEntity)
			}
			representation, gErr := executeSelectionSet(ctx, ec, entity, requires)
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

		for i := range receivedEntities {
			utils.DeepMerge(entities[representationToEntity[i]], receivedEntities[i])
		}
	}

	return nil
}

func executeSelectionSet(ctx context.Context, ec *executionContext, source map[string]interface{}, selections []plan.QueryPlanSelectionNode) (map[string]interface{}, *gqlerror.Error) {
	// If the underlying service has returned null for the parent (source)
	// then there is no need to iterate through the parent's selection set
	if source == nil {
		return nil, nil
	}

	result := make(map[string]interface{})

	for _, selection := range selections {
		switch selection := selection.(type) {
		case *plan.QueryPlanFieldNode:
			responseName := selection.Name
			if selection.Alias != "" {
				responseName = selection.Alias
			}
			selections := selection.Selections

			if source, ok := source[responseName]; !ok {
				return nil, gqlerror.Errorf(`field "%s" was not found in response`, responseName)
			} else if sourceArray, ok := source.([]interface{}); ok {
				var resultArray []interface{}
				for _, source := range sourceArray {
					if len(selections) != 0 {
						nextValue, ok := source.(map[string]interface{})
						if !ok {
							return nil, gqlerror.Errorf("unexpected type: %T", source)
						}
						ss, gErr := executeSelectionSet(ctx, ec, nextValue, selections)
						if gErr != nil {
							return nil, gErr
						}
						resultArray = append(resultArray, ss)
					} else {
						resultArray = append(resultArray, source)
					}
				}
				result[responseName] = resultArray

			} else if sourceObject, ok := source.(map[string]interface{}); ok {
				subResult, gErr := executeSelectionSet(
					ctx,
					ec,
					sourceObject,
					selections,
				)
				if gErr != nil {
					return nil, gErr
				}
				result[responseName] = subResult

			} else {
				result[responseName] = source
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

			if doesTypeConditionMatch(ec.Schema, selection.TypeCondition, typename) {
				value, gErr := executeSelectionSet(ctx, ec, source, selection.Selections)
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

func flattenResultsAtPath(resultLock *sync.Mutex, shouldLock bool, value interface{}, path ast.Path) interface{} {
	// NOTE この関数の挙動についてメモしておく
	// subgraphにクエリを投げて、得られる結果は _entities 経由なので必ずarrayである
	// 得られた結果を deepMerge するとき、targetのarrayが手に入るとiterateしてdeepMergeするだけで済むので楽
	// この関数は executeNode に対して、targetのarrayを提供し、操作を簡単にするために存在している

	if len(path) == 0 {
		return value
	}
	if value == nil {
		return nil
	}
	if shouldLock {
		resultLock.Lock()
		defer resultLock.Unlock()
	}

	current := path[0]
	rest := path[1:]
	if current == ast.PathName("@") {
		values := value.([]interface{})
		newValues := make([]interface{}, 0, len(values))
		for _, element := range values {
			v := flattenResultsAtPath(resultLock, false, element, rest)
			if vs, ok := v.([]interface{}); ok {
				newValues = append(newValues, vs...)
			} else {
				newValues = append(newValues, v)
			}
		}
		return newValues
	} else {
		value := value.(map[string]interface{})
		return flattenResultsAtPath(resultLock, false, value[string(current.(ast.PathName))], rest)
	}
}

func downstreamServiceError(originalError *gqlerror.Error, serviceName string, path ast.Path) *gqlerror.Error {
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

	// TODO is this path correct? maybe wrong...?
	newErr := gqlerror.WrapPath(path, originalError)
	newErr.Message = message
	newErr.Extensions = extensions
	return newErr
}
