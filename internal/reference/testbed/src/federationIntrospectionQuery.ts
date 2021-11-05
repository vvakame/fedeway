import * as accounts from "../../implementations/federation/federation-integration-testsuite-js/src/fixtures/accounts";
import * as books from "../../implementations/federation/federation-integration-testsuite-js/src/fixtures/books";
import * as documents from "../../implementations/federation/federation-integration-testsuite-js/src/fixtures/documents";
import * as inventory from "../../implementations/federation/federation-integration-testsuite-js/src/fixtures/inventory";
import * as product from "../../implementations/federation/federation-integration-testsuite-js/src/fixtures/product";
import * as reviews from "../../implementations/federation/federation-integration-testsuite-js/src/fixtures/reviews";

import { ApolloServer } from "apollo-server";
import { buildSubgraphSchema } from "@apollo/federation";
import { ApolloGateway, LocalGraphQLDataSource } from "@apollo/gateway";
import { DocumentNode, getIntrospectionQuery } from "graphql";
import { GraphQLResolverMap } from "apollo-graphql";
import gql from "graphql-tag";

export type ServiceDefinitionModule = ServiceDefinition & GraphQLSchemaModule;

export interface ServiceDefinition {
  typeDefs: DocumentNode;
  name: string;
  url?: string;
}

export interface GraphQLSchemaModule {
  typeDefs: DocumentNode;
  resolvers?: GraphQLResolverMap<any>;
}

const fixtures: ServiceDefinitionModule[] = [
  accounts,
  books,
  documents,
  inventory,
  product,
  reviews,
];

const gateway = new ApolloGateway({
  serviceList: fixtures,
  buildService: ({ url }) => {
    const service = fixtures.find((s) => s.url == url);
    return new LocalGraphQLDataSource(buildSubgraphSchema(service!));
  },
  debug: true,
});

const query = gql`
  ${getIntrospectionQuery()}
`;

(async () => {
  const { schema, executor } = await gateway.load();

  const server = new ApolloServer({
    schema,
    executor,
  });

  const result = await server.executeOperation({
    query,
  });

  console.log(
    JSON.stringify(result.data, null, 2),
    result.errors,
    result.extensions
  );
})();
