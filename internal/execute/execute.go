package execute

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/introspection"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"
	"github.com/vektah/gqlparser/v2/validator"
	"github.com/vvakame/fedeway/internal/utils"
)

// NOTE: これ何？
// federation の downstream service から得た merge 済データをQueryの要求にあわせて出力順を整える必要がある
// 要するに graphql(JS版) の execute 相当のものが必要らしいな… となったもの

// TODO gqlgenのほうの executionContext と互換性取るか考える
type ExecutionContext struct {
	Schema        *ast.Schema
	RootValue     interface{}
	FieldResolver FieldResolver
	TypeResolver  TypeResolver
}

type ExecutionArgs struct {
	Schema         *ast.Schema
	RawQuery       string
	Document       *ast.QueryDocument
	RootValue      interface{}            // optional
	VariableValues map[string]interface{} // optional
	OperationName  string                 // optional
	FieldResolver  FieldResolver          // optional
	TypeResolver   TypeResolver           // optional
}

var _ FieldResolver = defaultFieldResolver
var _ TypeResolver = defaultTypeResolver

type FieldResolver func(ctx context.Context, source interface{}, args map[string]interface{}) (interface{}, *gqlerror.Error)
type TypeResolver func(ctx context.Context, value interface{}, schema *ast.Schema, abstractType *ast.Type) string

// Implements the "Executing requests" section of the GraphQL specification.
//
// Returns either a synchronous ExecutionResult (if all encountered resolvers
// are synchronous), or a Promise of an ExecutionResult that will eventually be
// resolved and never rejected.
//
// If the arguments to this function do not result in a legal execution context,
// a GraphQLError will be thrown immediately explaining the invalid input.
func Execute(ctx context.Context, args *ExecutionArgs) *graphql.Response {
	schema := args.Schema
	rawQuery := args.RawQuery
	document := args.Document
	rootValue := args.RootValue
	variableValues := args.VariableValues
	operationName := args.OperationName
	fieldResolver := args.FieldResolver
	typeResolver := args.TypeResolver

	oc := &graphql.OperationContext{
		RawQuery:           rawQuery,
		Variables:          variableValues,
		Doc:                document,
		Operation:          document.Operations.ForName(operationName),
		RecoverFunc:        nil, // TODO
		ResolverMiddleware: nil, // TODO
	}
	var gErr *gqlerror.Error
	oc.Variables, gErr = validator.VariableValues(schema, oc.Operation, variableValues)
	if gErr != nil {
		return &graphql.Response{
			Errors: gqlerror.List{gErr},
		}
	}
	ctx = graphql.WithOperationContext(ctx, oc)
	// TODO default 使うのをやめる
	ctx = graphql.WithResponseContext(ctx, graphql.DefaultErrorPresenter, graphql.DefaultRecover)

	// If a valid execution context cannot be created due to incorrect arguments,
	// a "Response" with only errors is returned.
	exeContext := buildExecutionContext(schema, rootValue, fieldResolver, typeResolver)

	return execute(ctx, exeContext)
}

func execute(ctx context.Context, exeContext *ExecutionContext) *graphql.Response {
	oc := graphql.GetOperationContext(ctx)

	// Return a Promise that will eventually resolve to the data described by
	// The "Response" section of the GraphQL specification.
	//
	// If errors are encountered while executing a GraphQL field, only that
	// field and its descendants will be omitted, and sibling fields will still
	// be executed. An execution which encounters errors will still result in a
	// resolved Promise.
	data := executeOperation(ctx, exeContext, oc.Operation, exeContext.RootValue)
	return buildResponse(ctx, data)
}

func buildResponse(ctx context.Context, data graphql.Marshaler) *graphql.Response {
	var buf bytes.Buffer
	data.MarshalGQL(&buf)

	resp := &graphql.Response{
		Errors:     graphql.GetErrors(ctx),
		Data:       buf.Bytes(),
		Extensions: graphql.GetExtensions(ctx),
	}

	return resp
}

func buildExecutionContext(schema *ast.Schema, rootValue interface{}, fieldResolver FieldResolver, typeResolver TypeResolver) *ExecutionContext {
	if fieldResolver == nil {
		fieldResolver = defaultFieldResolver
	}
	if typeResolver == nil {
		typeResolver = defaultTypeResolver
	}

	return &ExecutionContext{
		Schema:        schema,
		RootValue:     rootValue,
		FieldResolver: fieldResolver,
		TypeResolver:  typeResolver,
	}
}

// Implements the "Executing operations" section of the spec.
func executeOperation(ctx context.Context, exeContext *ExecutionContext, operation *ast.OperationDefinition, rootValue interface{}) graphql.Marshaler {
	if !graphql.HasOperationContext(ctx) {
		panic("ctx doesn't have OperationContext")
	}

	var typ *ast.Definition
	switch operation.Operation {
	case ast.Query:
		if queryType := exeContext.Schema.Query; queryType == nil {
			graphql.AddError(ctx, gqlerror.ErrorPosf(operation.Position, "schema does not define the required query root type"))
			return graphql.Null
		} else {
			typ = queryType
		}
	case ast.Mutation:
		if mutationType := exeContext.Schema.Mutation; mutationType == nil {
			graphql.AddError(ctx, gqlerror.ErrorPosf(operation.Position, "schema is not configured for mutations"))
			return graphql.Null
		} else {
			typ = mutationType
		}
	case ast.Subscription:
		if subscriptionType := exeContext.Schema.Subscription; subscriptionType == nil {
			graphql.AddError(ctx, gqlerror.ErrorPosf(operation.Position, "schema is not configured for subscriptions"))
			return graphql.Null
		} else {
			typ = subscriptionType
		}
		typ = exeContext.Schema.Subscription
	default:
		graphql.AddError(ctx, gqlerror.ErrorPosf(operation.Position, "can only have query, mutation and subscription operations"))
		return graphql.Null
	}

	fields := graphql.CollectFields(graphql.GetOperationContext(ctx), operation.SelectionSet, []string{typ.Name})
	ctx = graphql.WithFieldContext(ctx, &graphql.FieldContext{
		Object: typ.Name,
	})

	// IntrospectionQueryの対応を入れる
	// js版だとSchema自体がIntrospectionQueryに対して応答的だがGo実装ではそうじゃないので
	for _, field := range fields {
		if field.Name == "__schema" {
			rootValueMap, ok := rootValue.(map[string]interface{})
			if !ok {
				graphql.AddErrorf(ctx, "unexpected rootValue type: %T", rootValue)
				return graphql.Null
			}
			rootValueMap["__schema"] = introspection.WrapSchema(exeContext.Schema)
		}
	}

	// NOTE original では inline fragment と絡めた同名fieldのmergeについて後続の処理になげているので Map<string, ReadonlyArray<FieldNode>> 的な型になる
	//      gqlgen では fragment の解決とmergeなどは適宜行われているため いわば Map<string, FieldNode> 相当の型になっている

	// Errors from sub-fields of a NonNull type may propagate to the top level,
	// at which point we still log the error and null the parent field, which
	// in this case is the entire response.
	var result graphql.Marshaler
	if operation.Operation == ast.Mutation {
		result = executeFieldsSerially(ctx, exeContext, typ, rootValue, fields)
	} else {
		result = executeFields(ctx, exeContext, typ, rootValue, fields)
	}

	return result
}

// Implements the "Executing selection sets" section of the spec
// for fields that must be executed serially.
func executeFieldsSerially(ctx context.Context, exeContext *ExecutionContext, parentType *ast.Definition, sourceValue interface{}, fields []graphql.CollectedField) graphql.Marshaler {
	oc := graphql.GetOperationContext(ctx)

	out := graphql.NewFieldSet(fields)
	var invalids uint32
	for i, field := range fields {
		fc := &graphql.FieldContext{
			Object: field.ObjectDefinition.Name,
			Field:  field,
		}
		ctx := graphql.WithFieldContext(ctx, fc)
		rawArgs := field.ArgumentMap(oc.Variables)
		// TODO rawArgs から args への変換処理必要？ Unmarshal しない前提ならいらないはずだが
		fc.Args = rawArgs

		data := executeField(
			ctx,
			exeContext,
			parentType,
			sourceValue,
			field,
		)

		if len(graphql.GetFieldErrors(ctx, fc)) != 0 {
			invalids++
		}

		out.Values[i] = data
	}
	out.Dispatch()

	var result graphql.Marshaler
	if invalids > 0 {
		result = graphql.Null
	} else {
		result = out
	}

	return result
}

// Implements the "Executing selection sets" section of the spec
// for fields that may be executed in parallel.
func executeFields(ctx context.Context, exeContext *ExecutionContext, parentType *ast.Definition, sourceValue interface{}, fields []graphql.CollectedField) graphql.Marshaler {
	oc := graphql.GetOperationContext(ctx)

	out := graphql.NewFieldSet(fields)
	var invalids uint32
	for i, field := range fields {
		field := field
		out.Concurrently(i, func() graphql.Marshaler {
			fc := &graphql.FieldContext{
				Object: field.ObjectDefinition.Name,
				Field:  field,
			}
			ctx := graphql.WithFieldContext(ctx, fc)
			rawArgs := field.ArgumentMap(oc.Variables)
			// TODO rawArgs から args への変換処理必要？ Unmarshal しない前提ならいらないはずだが
			fc.Args = rawArgs

			data := executeField(
				ctx,
				exeContext,
				parentType,
				sourceValue,
				field,
			)

			if len(graphql.GetFieldErrors(ctx, fc)) != 0 {
				atomic.AddUint32(&invalids, 1)
			}

			return data
		})
	}
	out.Dispatch()

	var result graphql.Marshaler
	if invalids > 0 {
		result = graphql.Null
	} else {
		result = out
	}

	return result
}

// Implements the "Executing field" section of the spec
// In particular, this function figures out the value that the field returns by
// calling its resolve function, then calls completeValue to complete promises,
// serialize scalars, or execute the sub-selection-set for objects.
func executeField(ctx context.Context, exeContext *ExecutionContext, parentType *ast.Definition, source interface{}, fieldNode graphql.CollectedField) graphql.Marshaler {
	fieldDef := fieldNode.Definition
	if fieldDef == nil {
		// TODO return graphql.Null を返すべき？
		panic("fieldDef is nil")
	}

	returnType := fieldDef.Type
	resolveFn := exeContext.FieldResolver

	fc := graphql.GetFieldContext(ctx)
	// NOTE originalでは buildResolveInfo とかしてた

	// Build a JS object of arguments from the field.arguments AST, using the
	// variables scope to fulfill any variable references.
	// TODO: find a way to memoize, in case this field is within a List type.
	// oroginal: const args = getArgumentValues(...)
	args := fc.Args

	result, gErr := resolveFn(ctx, source, args)
	if gErr != nil {
		graphql.AddError(ctx, gErr)
		return graphql.Null
	}

	return completeValue(
		ctx,
		exeContext,
		returnType,
		fieldNode,
		result,
	)
}

// Implements the instructions for completeValue as defined in the
// "Field entries" section of the spec.
//
// If the field type is Non-Null, then this recursively completes the value
// for the inner type. It throws a field error if that completion returns null,
// as per the "Nullability" section of the spec.
//
// If the field type is a List, then this recursively completes the value
// for the inner type on each item in the list.
//
// If the field type is a Scalar or Enum, ensures the completed value is a legal
// value of the type by calling the `serialize` method of GraphQL type
// definition.
//
// If the field is an abstract type, determine the runtime type of the value
// and then complete based on that type
//
// Otherwise, the field type expects a sub-selection set, and will complete the
// value by executing all sub-selections.
func completeValue(ctx context.Context, exeContext *ExecutionContext, returnType *ast.Type, fieldNode graphql.CollectedField, result interface{}) graphql.Marshaler {
	fc := graphql.GetFieldContext(ctx)

	// If result is an Error, throw a located error.
	if err, ok := result.(error); ok && err != nil {
		graphql.AddError(ctx, err)
		return graphql.Null
	}

	// If field type is NonNull, complete for inner type, and throw field error
	// if result is null.
	if returnType.NonNull {
		copied := *returnType
		copied.NonNull = false
		completed := completeValue(
			ctx,
			exeContext,
			&copied,
			fieldNode,
			result,
		)
		if len(graphql.GetFieldErrors(ctx, fc)) != 0 {
			return graphql.Null
		}
		if completed == graphql.Null {
			graphql.AddErrorf(ctx, "cannot return null for non-nullable field %s.%s", fieldNode.ObjectDefinition.Name, fieldNode.Name)
			return graphql.Null
		}
		return completed
	}

	// If result value is null or undefined then return null.
	if result == nil {
		return graphql.Null
	}

	// If field type is List, complete each item in the list with the inner type
	if returnType.Elem != nil {
		return completeListValue(
			ctx,
			exeContext,
			returnType,
			fieldNode,
			result,
		)
	}

	// If field type is a leaf type, Scalar or Enum, serialize to a valid value,
	// returning null if serialization is not possible.
	if utils.IsLeafType(exeContext.Schema.Types[returnType.NamedType]) {
		return completeLeafValue(ctx, returnType, result)
	}

	// If field type is an abstract type, Interface or Union, determine the
	// runtime Object type and complete for that type.
	if utils.IsAbstractType(exeContext.Schema.Types[returnType.NamedType]) {
		return completeAbstractValue(
			ctx,
			exeContext,
			returnType,
			fieldNode,
			result,
		)
	}

	// If field type is Object, execute and complete all sub-selections.
	// istanbul ignore else (See: 'https://github.com/graphql/graphql-js/issues/2618')
	if utils.IsObjectType(exeContext.Schema.Types[returnType.NamedType]) {
		return completeObjectValue(
			ctx,
			exeContext,
			returnType,
			fieldNode,
			result,
		)
	}

	// istanbul ignore next (Not reachable. All possible output types have been considered)
	graphql.AddErrorf(ctx, "cannot complete value of unexpected output type: %s", returnType.String())
	return graphql.Null
}

// Complete a list value by completing each item in the list with the
// inner type
func completeListValue(ctx context.Context, exeContext *ExecutionContext, returnType *ast.Type, fieldNode graphql.CollectedField, result interface{}) graphql.Marshaler {
	fc := graphql.GetFieldContext(ctx)

	resultRV := reflect.ValueOf(result)
	if resultRV.Kind() != reflect.Slice && resultRV.Kind() != reflect.Array {
		graphql.AddErrorf(ctx, `expected slice or array, but did not find one for field "%s.%s"`, fc.Object, fc.Field.Name)
		return graphql.Null
	}

	// TODO goroutine内でのpanicをrecoverする必要がある
	// 以下はgqlgenでの対応パターン
	//defer func() {
	//	if r := recover(); r != nil {
	//		ec.Error(ctx, ec.Recover(ctx, r))
	//		ret = graphql.Null
	//	}
	//}()

	// This is specified as a simple map, however we're optimizing the path
	// where the list contains no Promises by avoiding creating another Promise.
	itemType := returnType.Elem

	ret := make(graphql.Array, resultRV.Len())
	var wg sync.WaitGroup
	wg.Add(resultRV.Len())
	for index := 0; index < resultRV.Len(); index++ {
		index := index
		item := resultRV.Index(index).Interface()

		go func() {
			fc := &graphql.FieldContext{
				Index:  &index,
				Result: item,
			}
			ctx := graphql.WithFieldContext(ctx, fc)

			ret[index] = completeValue(
				ctx,
				exeContext,
				itemType,
				fieldNode,
				item,
			)

			wg.Done()
		}()
	}

	wg.Wait()

	return ret
}

// Complete a Scalar or Enum by serializing to a valid value, returning
// null if serialization is not possible.
func completeLeafValue(ctx context.Context, returnType *ast.Type, result interface{}) graphql.Marshaler {
	fc := graphql.GetFieldContext(ctx)

	if result == nil {
		return graphql.Null
	}

	switch result := result.(type) {
	case bool:
		return graphql.MarshalBoolean(result)
	case float64:
		return graphql.MarshalFloat(result)
	case int:
		return graphql.MarshalInt(result)
	case int64:
		return graphql.MarshalInt64(result)
	case int32:
		return graphql.MarshalInt32(result)
	case string:
		return graphql.MarshalString(result)
	case *string:
		if result == nil {
			return graphql.Null
		} else {
			return graphql.MarshalString(*result)
		}
	case time.Time:
		return graphql.MarshalTime(result)
	case json.Number:
		switch returnType.NamedType {
		case "Int":
			v, err := result.Int64()
			if err != nil {
				graphql.AddError(ctx, err)
				return graphql.Null
			}
			return graphql.MarshalInt64(v)
		case "Float":
			v, err := result.Float64()
			if err != nil {
				graphql.AddError(ctx, err)
				return graphql.Null
			}
			return graphql.MarshalFloat(v)
		default:
			// for custom scalar
			{
				v, err := result.Int64()
				if err == nil {
					return graphql.MarshalInt64(v)
				}
			}
			{
				v, err := result.Float64()
				if err == nil {
					return graphql.MarshalFloat(v)
				}
			}
			graphql.AddErrorf(ctx, "unsupported return type for json.Number: %s", returnType.NamedType)
			return graphql.Null
		}
	case graphql.Marshaler:
		return result
	default:
		panic(gqlerror.ErrorPathf(fc.Path(), "unsupported leaf type: %T", result))
	}
}

// Complete a value of an abstract type by determining the runtime object type
// of that value, then complete the value for that type.
func completeAbstractValue(ctx context.Context, exeContext *ExecutionContext, returnType *ast.Type, fieldNode graphql.CollectedField, result interface{}) graphql.Marshaler {
	resolveTypeFn := exeContext.TypeResolver

	runtimeType, gErr := ensureValidRuntimeType(
		ctx,
		resolveTypeFn(ctx, result, exeContext.Schema, returnType),
		exeContext,
		returnType,
		fieldNode,
		result,
	)
	if gErr != nil {
		graphql.AddError(ctx, gErr)
		return graphql.Null
	}

	return completeObjectValue(
		ctx,
		exeContext,
		runtimeType,
		fieldNode,
		result,
	)
}

func ensureValidRuntimeType(ctx context.Context, runtimeTypeName string, exeContext *ExecutionContext, returnType *ast.Type, fieldNode graphql.CollectedField, result interface{}) (*ast.Type, *gqlerror.Error) {
	fc := graphql.GetFieldContext(ctx)

	if runtimeTypeName == "" {
		return nil, gqlerror.ErrorPathf(
			fc.Path(),
			`abstract type "%s" must resolve to an Object type at runtime for field "%s.%s"`,
			returnType.Name(),
			fc.Object,
			fc.Field.Name,
		)
	}

	runtimeType := exeContext.Schema.Types[runtimeTypeName]
	if runtimeType == nil {
		return nil, gqlerror.ErrorPathf(
			fc.Path(),
			`abstract type "%s" was resolved to a type "%s" that does not exist inside the schema`,
			returnType.Name(),
			runtimeTypeName,
		)
	}

	if runtimeType.Kind != ast.Object {
		return nil, gqlerror.ErrorPathf(
			fc.Path(),
			`abstract type "%s" was resolved to a non-object type "%s"`,
			returnType.Name(),
			runtimeTypeName,
		)
	}

	// TODO 本来は schema.isSubType(returnType, runtimeType) だったんだけどこれでいいのかしら？
	if !utils.IsTypeDefSubTypeOf(exeContext.Schema, runtimeType, exeContext.Schema.Types[returnType.Name()]) {
		return nil, gqlerror.ErrorPathf(
			fc.Path(),
			`runtime Object type "%s" is not a possible type for "%s"`,
			runtimeType.Name,
			returnType.Name(),
		)
	}

	// TODO returnType.Position は嘘じゃないか？
	return ast.NamedType(runtimeType.Name, returnType.Position), nil
}

// Complete an Object value by executing all sub-selections.
func completeObjectValue(ctx context.Context, exeContext *ExecutionContext, returnType *ast.Type, fieldNode graphql.CollectedField, result interface{}) graphql.Marshaler {
	// Collect sub-fields to execute to complete this value.
	oc := graphql.GetOperationContext(ctx)
	returnTypeDef := exeContext.Schema.Types[returnType.Name()]
	implementDefs := exeContext.Schema.GetImplements(returnTypeDef)
	satisfies := make([]string, 0, len(implementDefs)+1)
	satisfies = append(satisfies, returnTypeDef.Name)
	for _, implementDef := range implementDefs {
		satisfies = append(satisfies, implementDef.Name)
	}
	subFieldNodes := graphql.CollectFields(oc, fieldNode.SelectionSet, satisfies)

	// If there is an isTypeOf predicate function, call it with the
	// current result. If isTypeOf returns false, then raise an error rather
	// than continuing execution.
	// NOTE: original では promise 周りの処理が色々あったけど多分省いてよいはず

	// TODO returnType じゃなくて types から引いた definition を渡しているけど List とか NonNull の情報が落ちてるのでまずいのではないか…？ あとで確認する
	return executeFields(ctx, exeContext, returnTypeDef, result, subFieldNodes)
}

// If a resolveType function is not given, then a default resolve behavior is
// used which attempts two strategies:
//
// First, See if the provided value has a `__typename` field defined, if so, use
// that value as name of the resolved type.
//
// Otherwise, test each possible type for the abstract type by calling
// isTypeOf for the object being coerced, returning the first type that matches.
func defaultTypeResolver(ctx context.Context, value interface{}, schema *ast.Schema, abstractType *ast.Type) string {
	// First, look for `__typename`.
	if utils.IsObjectLike(value) {
		value := value.(map[string]interface{})
		typename, ok := value["__typename"].(string)
		if ok {
			return typename
		}
	}

	// NOTE originalでは isTypeOf を呼んでるんだけど、 value instanceof Dog みたいなJS特有の処理なのでここでは実装しない

	return ""
}

// If a resolve function is not given, then a default resolve behavior is used
// which takes the property of the source object of the same name as the field
// and returns it as the result, or if it's a function, returns the result
// of calling that function while passing along args and context value.
func defaultFieldResolver(ctx context.Context, source interface{}, args map[string]interface{}) (interface{}, *gqlerror.Error) {
	fc := graphql.GetFieldContext(ctx)
	if fc == nil {
		panic("ctx doesn't have FieldContext")
	}

	// ensure source is a value for which property access is acceptable.
	if source == nil {
		return nil, nil
	} else if sourceV := reflect.ValueOf(source); sourceV.Kind() == reflect.Ptr && sourceV.IsNil() {
		// trap typed nil
		return nil, nil
	}
	if source, ok := source.(map[string]interface{}); ok {
		property := source[fc.Field.Alias]
		if f, ok := property.(func() (graphql.Marshaler, *gqlerror.Error)); ok {
			// TODO このままだとまずくない
			return f()
		}

		return property, nil
	}
	{
		lowerFieldName := strings.ToLower(fc.Field.Name)

		rv := reflect.ValueOf(source)
		if rv.Kind() == reflect.Struct && rv.Type().PkgPath() == "github.com/99designs/gqlgen/graphql/introspection" {
			// []Type becomes Type. but several methods exist on *Type.
			rvPtr := reflect.New(rv.Type())
			rvPtr.Elem().Set(rv)
			rv = rvPtr
		}
		rt := rv.Type()
		for i := 0; i < rt.NumMethod(); i++ {
			mv := rt.Method(i)
			if !mv.IsExported() {
				continue
			}
			if lowerFieldName != strings.ToLower(mv.Name) {
				continue
			}
			reqVs := []reflect.Value{rv}
			var respVs []reflect.Value
			// TODO improve build of function call parameters
			if mv.Type.NumIn() == 1 {
				respVs = mv.Func.Call(reqVs)
			} else if mv.Type.NumIn() == 2 {
				if len(args) == 1 {
					for _, argValue := range args {
						reqVs = append(reqVs, reflect.ValueOf(argValue))
					}
				} else {
					graphql.AddErrorf(ctx, "unsupported method signature: %s", mv.Func.String())
					return nil, nil
				}
				respVs = mv.Func.Call(reqVs)
			} else {
				graphql.AddErrorf(ctx, "unsupported method signature: %s", mv.Func.String())
				return nil, nil
			}
			if len(respVs) == 1 {
				if respVs[0].Kind() == reflect.Ptr && respVs[0].IsNil() {
					return nil, nil
				}
				resp := respVs[0].Interface()
				return resp, nil
			} else {
				graphql.AddErrorf(ctx, "unsupported method return values: %s", mv.Func.String())
				return nil, nil
			}
		}
		if rv.Kind() == reflect.Ptr {
			rv = rv.Elem()
		}
		if rv.Kind() == reflect.Struct {
			for i := 0; i < rv.NumField(); i++ {
				ft := rv.Type().Field(i)
				if !ft.IsExported() {
					continue
				}
				if lowerFieldName != strings.ToLower(ft.Name) {
					continue
				}

				return rv.Field(i).Interface(), nil
			}
		}
	}

	return nil, nil
}
