import { visit, print } from "graphql";
import gql from "graphql-tag";

const source = gql`
  "directives at FIELDs are executable"
  directive @audit(risk: Int!) on FIELD

  "directives at FIELD_DEFINITIONs are for the type-system"
  directive @transparency(concealment: Int!) on FIELD_DEFINITION

  type EarthConcern {
    environmental: String! @transparency(concealment: 5)
  }

  extend type Query {
    importantDirectives: [EarthConcern!]!
  }
`;

const modified = visit(source, {
  Directive(node) {
    return null;
  },
});
console.log(print(modified));
