import { getIntrospectionQuery } from "graphql";

console.log(
  getIntrospectionQuery({
    descriptions: false,
  })
);
