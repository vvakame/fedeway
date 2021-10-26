import {
  buildASTSchema,
  extendSchema,
  GraphQLSchema,
  printSchema,
} from "graphql";
import gql from "graphql-tag";

const source = gql`
  directive @sample(text: String) on OBJECT

  interface IfA {
    IfA: String
  }
  interface IfB {
    IfB: Int
  }
  interface IfC {
    IfC: Boolean
  }

  type Foo implements IfA & IfB @sample(text: "Foo1") {
    id: ID
    name: String @deprecated(reason: "A")
    age: Int
  }

  extend type Foo {
    bar2: Boolean @deprecated(reason: "B")
  }

  type Foo implements IfA @sample(text: "Foo2") {
    bar: Boolean
  }

  extend type Foo implements IfC {
    bar1: Boolean @deprecated(reason: "C")
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
