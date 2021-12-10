# fedeway

Apollo Federation Gateway v1 implementations by Go.

:warning: This product is under development. don't use in production. :warning:

## TODO

* implements validation rules on...
  * `validateServicesBeforeNormalization`
  * `validateComposedSchema`
* remove all of `option:skip: true` from test cases
* capture `panic` by recover func on ExecuteQueryPlan
* improve logging settings & implementations
* low priority
  * make configurable about `graphql.DefaultErrorPresenter` and `graphql.DefaultRecover`
  * use `DisableIntrospection` value
  * support `graphql.Stats`
  * observability. support OpenCensus or OpenTelemetry

## Issue from gqlgen

* nested `@requires` is not supported [#1138](https://github.com/99designs/gqlgen/issues/1138)
* multiple `@key` is not supported [#1031](https://github.com/99designs/gqlgen/issues/1031)
* `collectFields` return values bug [#1311](https://github.com/99designs/gqlgen/issues/1311) [#1329](https://github.com/99designs/gqlgen/issues/1329)
* `_service` is not present when SDL doesn't have subgraph-like syntax.
* doesn't support renamed root type likes `schema { query: RootQuery }`.
