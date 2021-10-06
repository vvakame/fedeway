package execute

import (
	"bytes"
	"context"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"github.com/99designs/gqlgen/graphql"
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
	Schema         *ast.Schema
	Fragments      ast.FragmentDefinitionList
	RootValue      interface{}
	ContextValue   map[string]interface{}
	Operation      *ast.OperationDefinition
	VariableValues map[string]interface{}
	FieldResolver  FieldResolver
	TypeResolver   TypeResolver
	Errors         gqlerror.List
}

type ExecutionArgs struct {
	Schema         *ast.Schema
	Document       *ast.QueryDocument
	RootValue      interface{}            // optional
	ContextValue   map[string]interface{} // optional
	VariableValues map[string]interface{} // optional
	OperationName  string                 // optional
	FieldResolver  FieldResolver          // optional
	TypeResolver   TypeResolver           // optional
}

var _ FieldResolver = defaultFieldResolver

// TODO この定義正しい？
type FieldResolver func(ctx context.Context, source interface{}, args, contextValue map[string]interface{}) (interface{}, *gqlerror.Error)

var _ TypeResolver = defaultTypeResolver

// TODO この定義正しい？
type TypeResolver func(ctx context.Context, value interface{}, contextValue map[string]interface{}, schema *ast.Schema, abstractType *ast.Type) string

// Implements the "Executing requests" section of the GraphQL specification.
//
// Returns either a synchronous ExecutionResult (if all encountered resolvers
// are synchronous), or a Promise of an ExecutionResult that will eventually be
// resolved and never rejected.
//
// If the arguments to this function do not result in a legal execution context,
// a GraphQLError will be thrown immediately explaining the invalid input.
func Execute(ctx context.Context, args *ExecutionArgs) (*graphql.Response, *gqlerror.Error) {
	schema := args.Schema
	document := args.Document
	rootValue := args.RootValue
	contextValue := args.ContextValue
	variableValues := args.VariableValues
	operationName := args.OperationName
	fieldResolver := args.FieldResolver
	typeResolver := args.TypeResolver

	// If arguments are missing or incorrect, throw an error.
	gErr := assertValidExecutionArguments(schema, document, variableValues)
	if gErr != nil {
		return nil, gErr
	}

	// If a valid execution context cannot be created due to incorrect arguments,
	// a "Response" with only errors is returned.
	exeContext, gErrs := buildExecutionContext(
		schema,
		document,
		rootValue,
		contextValue,
		variableValues,
		operationName,
		fieldResolver,
		typeResolver,
	)

	// Return early errors if execution context failed.
	if len(gErrs) != 0 {
		return &graphql.Response{
			Errors: gErrs,
		}, nil
	}

	// Return a Promise that will eventually resolve to the data described by
	// The "Response" section of the GraphQL specification.
	//
	// If errors are encountered while executing a GraphQL field, only that
	// field and its descendants will be omitted, and sibling fields will still
	// be executed. An execution which encounters errors will still result in a
	// resolved Promise.
	data, gErr := executeOperation(ctx, exeContext, exeContext.Operation, rootValue)
	if gErr != nil {
		return nil, gErr
	}
	resp := buildResponse(ctx, exeContext, data)
	return resp, nil
}

func buildResponse(ctx context.Context, exeContext *ExecutionContext, data graphql.Marshaler) *graphql.Response {
	var buf bytes.Buffer
	data.MarshalGQL(&buf)

	resp := &graphql.Response{
		Errors:     exeContext.Errors,
		Data:       buf.Bytes(),
		Extensions: graphql.GetExtensions(ctx),
	}

	return resp
}

// Essential assertions before executing to provide developer feedback for
// improper use of the GraphQL library.
func assertValidExecutionArguments(schema *ast.Schema, document *ast.QueryDocument, rawVariableValues map[string]interface{}) *gqlerror.Error {
	if document == nil {
		return gqlerror.Errorf("must provide document")
	}

	// If the schema used for execution is invalid, throw an error.
	// TODO
	// assertValidSchema(schema)

	// Variables, if provided, must be an object.
	if rawVariableValues != nil && !utils.IsObjectLike(rawVariableValues) {
		return gqlerror.Errorf("variables must be provided as an Object where each property is a variable value. Perhaps look to see if an unparsed JSON string was provided")
	}

	return nil
}

func buildExecutionContext(schema *ast.Schema, document *ast.QueryDocument, rootValue interface{}, contextValue map[string]interface{}, rawVariableValues map[string]interface{}, operationName string, fieldResolver FieldResolver, typeResolver TypeResolver) (*ExecutionContext, gqlerror.List) {
	// TODO ここ厳密じゃないけどいいかな？ operationNameがで見つかったものがダブったとき fragments の定義
	operation := document.Operations.ForName(operationName)

	if operation == nil {
		if operationName != "" {
			return nil, gqlerror.List{gqlerror.Errorf(`unknown operation named "%s"`, operationName)}
		}
		return nil, gqlerror.List{gqlerror.Errorf("must provide an operation")}
	}

	// TODO この置き換えが正しいかわからん getVariableValues 相当
	coercedVariableValues, gErr := validator.VariableValues(schema, operation, rawVariableValues)
	if gErr != nil {
		return nil, gqlerror.List{gErr}
	}

	if fieldResolver == nil {
		fieldResolver = defaultFieldResolver
	}
	if typeResolver == nil {
		typeResolver = defaultTypeResolver
	}

	return &ExecutionContext{
		Schema:         schema,
		Fragments:      document.Fragments,
		RootValue:      rootValue,
		ContextValue:   contextValue,
		Operation:      operation,
		VariableValues: coercedVariableValues,
		FieldResolver:  fieldResolver,
		TypeResolver:   typeResolver,
	}, nil
}

// Implements the "Executing operations" section of the spec.
func executeOperation(ctx context.Context, exeContext *ExecutionContext, operation *ast.OperationDefinition, rootValue interface{}) (graphql.Marshaler, *gqlerror.Error) {
	if !graphql.HasOperationContext(ctx) {
		panic("ctx doesn't have OperationContext")
	}

	var typ *ast.Definition
	switch operation.Operation {
	case ast.Query:
		if queryType := exeContext.Schema.Query; queryType == nil {
			return nil, gqlerror.ErrorPosf(operation.Position, "schema does not define the required query root type")
		} else {
			typ = queryType
		}
	case ast.Mutation:
		if mutationType := exeContext.Schema.Mutation; mutationType == nil {
			return nil, gqlerror.ErrorPosf(operation.Position, "schema is not configured for mutations")
		} else {
			typ = mutationType
		}
	case ast.Subscription:
		if subscriptionType := exeContext.Schema.Subscription; subscriptionType == nil {
			return nil, gqlerror.ErrorPosf(operation.Position, "schema is not configured for subscriptions")
		} else {
			typ = subscriptionType
		}
		typ = exeContext.Schema.Subscription
	default:
		return nil, gqlerror.ErrorPosf(operation.Position, "can only have query, mutation and subscription operations")
	}

	fields := graphql.CollectFields(graphql.GetOperationContext(ctx), operation.SelectionSet, []string{typ.Name})
	ctx = graphql.WithFieldContext(ctx, &graphql.FieldContext{
		Object: typ.Name,
	})

	// NOTE original では inline fragment と絡めた同名fieldのmergeについて後続の処理になげているので Map<string, ReadonlyArray<FieldNode>> 的な型になる
	//      gqlgen では fragment の解決とmergeなどは適宜行われているため いわば Map<string, FieldNode> 相当の型になっている
	//  fields := collectFields(
	//	  exeContext.Schema,
	//	  exeContext.Fragments,
	//	  exeContext.VariableValues,
	//	  typ,
	//	  operation.SelectionSet,
	//	  &Fields{},
	//	  make(map[string]struct{}),
	//  )

	// Errors from sub-fields of a NonNull type may propagate to the top level,
	// at which point we still log the error and null the parent field, which
	// in this case is the entire response.
	var result graphql.Marshaler
	if operation.Operation == ast.Mutation {
		result = executeFieldsSerially(ctx, exeContext, typ, rootValue, fields)
	} else {
		result = executeFields(ctx, exeContext, typ, rootValue, fields)
	}

	return result, nil
}

// Implements the "Executing selection sets" section of the spec
// for fields that must be executed serially.
func executeFieldsSerially(ctx context.Context, exeContext *ExecutionContext, parentType *ast.Definition, sourceValue interface{}, fields []graphql.CollectedField) graphql.Marshaler {
	out := graphql.NewFieldSet(fields)
	var invalids uint32
	for i, field := range fields {
		fc := &graphql.FieldContext{
			Object: field.ObjectDefinition.Name,
			Field:  field,
		}
		ctx := graphql.WithFieldContext(ctx, fc)
		rawArgs := field.ArgumentMap(exeContext.VariableValues)
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
			rawArgs := field.ArgumentMap(exeContext.VariableValues)
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

	// Get the resolve function, regardless of if its result is normal or abrupt (error).

	// Build a JS object of arguments from the field.arguments AST, using the
	// variables scope to fulfill any variable references.
	// TODO: find a way to memoize, in case this field is within a List type.
	// oroginal: const args = getArgumentValues(...)
	args := fc.Args

	// The resolve function's optional third argument is a context value that
	// is provided to every resolve function within an execution. It is commonly
	// used to represent an authenticated user, or request-specific caches.
	contextValue := exeContext.ContextValue

	// TODO originalにあったinfoを引数から削りました
	result, gErr := resolveFn(ctx, source, args, contextValue)
	if gErr != nil {
		exeContext.Errors = append(exeContext.Errors, gErr)
		return graphql.Null
	}

	completed, gErr := completeValue(
		ctx,
		exeContext,
		returnType,
		fieldNode,
		result,
	)
	if gErr != nil {
		exeContext.Errors = append(exeContext.Errors, gErr)
		return graphql.Null
	}

	return completed
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
func completeValue(ctx context.Context, exeContext *ExecutionContext, returnType *ast.Type, fieldNode graphql.CollectedField, result interface{}) (graphql.Marshaler, *gqlerror.Error) {
	fc := graphql.GetFieldContext(ctx)

	// If result is an Error, throw a located error.
	if err, ok := result.(error); ok && err != nil {
		return graphql.Null, gqlerror.WrapPath(fc.Path(), err)
	}

	// If field type is NonNull, complete for inner type, and throw field error
	// if result is null.
	if returnType.NonNull {
		copied := *returnType
		copied.NonNull = false
		completed, gErr := completeValue(
			ctx,
			exeContext,
			&copied,
			fieldNode,
			result,
		)
		if gErr != nil {
			return graphql.Null, gErr
		}
		if completed == graphql.Null {
			return graphql.Null, gqlerror.ErrorPathf(fc.Path(), "cannot return null for non-nullable field %s.%s", fc.Object, fc.Field.Name)
		}
		return completed, nil
	}

	// If result value is null or undefined then return null.
	if result == nil {
		return graphql.Null, nil
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
	if utils.IsLeadType(exeContext.Schema.Types[returnType.NamedType]) {
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
	return graphql.Null, gqlerror.ErrorPathf(fc.Path(), "cannot complete value of unexpected output type: %s", returnType.String())
}

// Complete a list value by completing each item in the list with the
// inner type
func completeListValue(ctx context.Context, exeContext *ExecutionContext, returnType *ast.Type, fieldNode graphql.CollectedField, result interface{}) (graphql.Marshaler, *gqlerror.Error) {
	fc := graphql.GetFieldContext(ctx)

	resultRV := reflect.ValueOf(result)
	if resultRV.Kind() != reflect.Slice {
		return graphql.Null, gqlerror.ErrorPathf(fc.Path(), `expected slice, but did not find one for field "%s.%s"`, fc.Object, fc.Field.Name)
	}

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

			completedItem, gErr := completeValue(
				ctx,
				exeContext,
				itemType,
				fieldNode,
				item,
			)
			if gErr != nil {
				// TODO ctx 経由であれこれする形にここに限らず全体的に変えないといけない
				panic(gErr)
			}

			ret[index] = completedItem
			wg.Done()
		}()
	}

	wg.Wait()

	return ret, nil
}

// Complete a Scalar or Enum by serializing to a valid value, returning
// null if serialization is not possible.
func completeLeafValue(ctx context.Context, returnType *ast.Type, result interface{}) (graphql.Marshaler, *gqlerror.Error) {
	fc := graphql.GetFieldContext(ctx)

	if result == nil {
		return graphql.Null, nil
	}

	switch result := result.(type) {
	case bool:
		return graphql.MarshalBoolean(result), nil
	case float64:
		return graphql.MarshalFloat(result), nil
	case int:
		return graphql.MarshalInt(result), nil
	case int64:
		return graphql.MarshalInt64(result), nil
	case int32:
		return graphql.MarshalInt32(result), nil
	case string:
		return graphql.MarshalString(result), nil
	case time.Time:
		return graphql.MarshalTime(result), nil
	default:
		panic(gqlerror.ErrorPathf(fc.Path(), "unsupported leaf type: %T", result))
	}
}

// Complete a value of an abstract type by determining the runtime object type
// of that value, then complete the value for that type.
func completeAbstractValue(ctx context.Context, exeContext *ExecutionContext, returnType *ast.Type, fieldNode graphql.CollectedField, result interface{}) (graphql.Marshaler, *gqlerror.Error) {
	resolveTypeFn := exeContext.TypeResolver
	contextValue := exeContext.ContextValue

	runtimeType, gErr := ensureValidRuntimeType(
		ctx,
		resolveTypeFn(ctx, result, contextValue, exeContext.Schema, returnType),
		exeContext,
		returnType,
		fieldNode,
		result,
	)
	if gErr != nil {
		return graphql.Null, gErr
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
func completeObjectValue(ctx context.Context, exeContext *ExecutionContext, returnType *ast.Type, fieldNode graphql.CollectedField, result interface{}) (graphql.Marshaler, *gqlerror.Error) {
	// Collect sub-fields to execute to complete this value.
	subFieldNodes := graphql.CollectFields(graphql.GetOperationContext(ctx), fieldNode.SelectionSet, []string{returnType.Name()})

	// If there is an isTypeOf predicate function, call it with the
	// current result. If isTypeOf returns false, then raise an error rather
	// than continuing execution.
	// NOTE: original では promise 周りの処理が色々あったけど多分省いてよいはず

	// TODO returnType じゃなくて types から引いた definition を渡しているけど List とか NonNull の情報が落ちてるのでまずいのではないか…？ あとで確認する
	return executeFields(ctx, exeContext, exeContext.Schema.Types[returnType.Name()], result, subFieldNodes), nil
}

// If a resolveType function is not given, then a default resolve behavior is
// used which attempts two strategies:
//
// First, See if the provided value has a `__typename` field defined, if so, use
// that value as name of the resolved type.
//
// Otherwise, test each possible type for the abstract type by calling
// isTypeOf for the object being coerced, returning the first type that matches.
func defaultTypeResolver(ctx context.Context, value interface{}, contextValue map[string]interface{}, schema *ast.Schema, abstractType *ast.Type) string {
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
func defaultFieldResolver(ctx context.Context, source interface{}, args, contextValue map[string]interface{}) (interface{}, *gqlerror.Error) {
	fc := graphql.GetFieldContext(ctx)
	if fc == nil {
		panic("ctx doesn't have FieldContext")
	}

	// ensure source is a value for which property access is acceptable.
	if utils.IsObjectLike(source) {
		source := source.(map[string]interface{})
		property := source[fc.Field.Alias]
		if f, ok := property.(func() (graphql.Marshaler, *gqlerror.Error)); ok {
			// TODO このままだとまずくない
			return f()
		}

		return property, nil
	}

	return nil, nil
}
