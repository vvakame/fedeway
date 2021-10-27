# fedeway

Apollo Federation Gateway implementations by Go.

## Known issue

### gqlgen

* nested `@requires` is not supported [#1138](https://github.com/99designs/gqlgen/issues/1138)
* `_service` is not present when SDL doesn't have subgraph-like syntax.
* doesn't support renamed root type likes `schema { query: RootQuery }`.
* multiple `@key` is not supported [#1031](https://github.com/99designs/gqlgen/issues/1031)
