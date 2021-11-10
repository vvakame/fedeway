# simple fedeway demo

downstream service are copied from https://github.com/apollographql/federation-demo/tree/e24d9aca9d7a8fe9490ab53efeddc2ef8152f4a1

```shell
# shell A
$ git clone git@github.com:apollographql/federation-demo.git
$ cd federation-demo
$ git checkout e24d9aca9d7a8fe9490ab53efeddc2ef8152f4a1
$ npm ci
$ npm run start-services

# shell B
$ go run .
$ open http://localhost:8080/
```
