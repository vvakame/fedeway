# Note

Apollo Federation Gateway js版の実装見ていく

* QueryPlanner
* QueryPlan
  * これもAST的構造
    * `QueryPlan`
    * 計画の種類 `PlanNode`
      * `Sequence`
      * `Parallel`
      * `Fetch`
      * `Flatten`
    * なんかの種類 `QueryPlanSelectionNode`
      * それぞれ FragmentSpreadNode を持てるっぽいけど requires が指定されてる場合 FetchNode 内にこれを持つことはできない？らしい
      * `Field`
      * `InlineFragment`
* QueryPlanningContext
  * `internalFragments`
  * `variableDefinitions`
  * 常にschema持ってるのでポインタ係っぽさ
  * key fieldsとか調べられるっぽい
  * TODO `getFragmentCondition` まで読んだ
* Scope
  * 名前空間的な… [見たほうが早い](https://github.com/apollographql/federation/blob/9d881a4180e8983e2a8507d5566a7bb07eb8cbe0/query-planner-js/src/Scope.ts) か
  * 基本は型(というか1つのQuery内のネスト)が1つのScopeを成す
  * Scopeは親Scopeを持つ(場合がほとんど)
  * `possibleRuntimeTypes` ?
  * なんか色々書いてある…
  * だいたい要するに、plan考える時にどのserviceが1アクセスの範囲内であるかの判定を行うときに便利… ってことなのかなぁ？違う？
* OperationContext
  * 


* `__typename` はだいたい必要っぽい？
* `serialize` は単に stringify の意味っぽい
* Mutationの場合、 `Sequence` になるらしい `Parallel` 不可。まぁわかる気はする。
  * 複数の操作を行う場合、fieldの記述順通りに上から順に実行されないといけない！なるほどね
* buildQueryPlan.ts の実装が詳細部分っぽい
* わりと効率とか気にせず愚直に分割してるだけっぽいことがわかってきた…
* buildComposedSchema …
* Planner系だけ先に実装できるか見たかったけどなかなか厳しい
  * ApolloGateway.createSchemaFromServiceList 相当の処理が必要
  * buildComposedSchemaだけでもなんとかなる？

* `autoFragmentization` て何？
* base service と owning service という表現があるっぽい
* possible runtime type って何？ `possibleRuntimeTypes`
* fragment の condition って何？
* 
