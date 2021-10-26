import gql from "graphql-tag";
import { GraphQLSchemaValidationError } from "apollo-graphql";
import { composeAndValidate, compositionHasErrors } from "@apollo/federation";
import { PossibleFragmentSpreadsRule, printSchema } from "graphql";

export const typeDefsAccount = gql`
  extend type Query {
    me: User
  }
  type User @key(fields: "id") {
    id: ID!
    name: String
    username: String
    birthDate: String
  }
`;
export const typeDefsInventory = gql`
  extend type Product @key(fields: "upc") {
    upc: String! @external
    inStock: Boolean
    # quantity: Int
  }
`;
export const typeDefsProducts = gql`
  extend type Query {
    topProducts(first: Int): [Product]
  }
  type Product @key(fields: "upc") {
    upc: String!
    sku: String!
    name: String
    price: String
  }
`;
export const typeDefsReviews = gql`
  type Review @key(fields: "id") {
    id: ID!
    body: String
    author: User
    product: Product
  }

  extend type User @key(fields: "id") {
    id: ID! @external
    reviews: [Review]
  }
  extend type Product @key(fields: "upc") {
    upc: String! @external
    reviews: [Review]
  }
`;

const services = [
  {
    name: "accounts",
    url: "http://accounts.example.com/query",
    typeDefs: typeDefsAccount,
  },
  {
    name: "inventory",
    url: "http://inventory.example.com/query",
    typeDefs: typeDefsInventory,
  },
  {
    name: "products",
    url: "http://products.example.com/query",
    typeDefs: typeDefsProducts,
  },
  {
    name: "reviews",
    url: "http://reviews.example.com/query",
    typeDefs: typeDefsReviews,
  },
];

const compositionResult = composeAndValidate(services);

if (compositionHasErrors(compositionResult)) {
  throw new GraphQLSchemaValidationError(compositionResult.errors);
}

console.log(compositionResult.supergraphSdl);

console.log("---------------------");

console.log(printSchema(compositionResult.schema));
PossibleFragmentSpreadsRule;
