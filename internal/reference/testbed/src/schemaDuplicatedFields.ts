import {
  buildASTSchema,
  extendSchema,
  GraphQLSchema,
  printSchema,
} from "graphql";
import gql from "graphql-tag";

const source = gql`
  type Foo {
    id: ID
  }

  extend type Foo {
    id: ID
  }
`;

try {
  // raise error
  // Field "Foo.id" can only be defined once.
  buildASTSchema(source);
} catch (e) {
  console.error(e);
}

{
  const schema = extendSchema(new GraphQLSchema({}), source, {
    assumeValidSDL: true,
  });
  console.log(printSchema(schema));
}
