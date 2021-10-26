import gql from "graphql-tag";
import { GraphQLSchemaValidationError } from "apollo-graphql";
import { composeAndValidate, compositionHasErrors } from "@apollo/federation";
import { extendSchema, GraphQLSchema, printSchema } from "graphql";

export const typeDefsA = gql`
  input NewProductInput {
    sku: ID!
    type: String
  }
`;
export const typeDefsB = gql`
  input NewProductInput {
    sku: ID!
    type: String
  }

  type Query {
    filler: String
  }
`;

const services = [
  {
    name: "A",
    url: undefined,
    typeDefs: typeDefsA,
  },
  {
    name: "B",
    url: undefined,
    typeDefs: typeDefsB,
  },
];

let schema = new GraphQLSchema({});
services.forEach((service) => {
  schema = extendSchema(schema, service.typeDefs, { assumeValidSDL: true });
});
console.log(printSchema(schema));

console.log("--------------");

const compositionResult = composeAndValidate(services);

if (compositionHasErrors(compositionResult)) {
  throw new GraphQLSchemaValidationError(compositionResult.errors);
}

console.log(compositionResult.supergraphSdl);
