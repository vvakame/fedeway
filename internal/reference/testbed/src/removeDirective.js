"use strict";
var __makeTemplateObject = (this && this.__makeTemplateObject) || function (cooked, raw) {
    if (Object.defineProperty) { Object.defineProperty(cooked, "raw", { value: raw }); } else { cooked.raw = raw; }
    return cooked;
};
var __importDefault = (this && this.__importDefault) || function (mod) {
    return (mod && mod.__esModule) ? mod : { "default": mod };
};
Object.defineProperty(exports, "__esModule", { value: true });
var graphql_1 = require("graphql");
var graphql_tag_1 = __importDefault(require("graphql-tag"));
var source = (0, graphql_tag_1.default)(templateObject_1 || (templateObject_1 = __makeTemplateObject(["\n  \"directives at FIELDs are executable\"\n  directive @audit(risk: Int!) on FIELD\n\n  \"directives at FIELD_DEFINITIONs are for the type-system\"\n  directive @transparency(concealment: Int!) on FIELD_DEFINITION\n\n  type EarthConcern {\n    environmental: String! @transparency(concealment: 5)\n  }\n\n  extend type Query {\n    importantDirectives: [EarthConcern!]!\n  }\n"], ["\n  \"directives at FIELDs are executable\"\n  directive @audit(risk: Int!) on FIELD\n\n  \"directives at FIELD_DEFINITIONs are for the type-system\"\n  directive @transparency(concealment: Int!) on FIELD_DEFINITION\n\n  type EarthConcern {\n    environmental: String! @transparency(concealment: 5)\n  }\n\n  extend type Query {\n    importantDirectives: [EarthConcern!]!\n  }\n"])));
var modified = (0, graphql_1.visit)(source, {
    Directive: function (node) {
        return null;
    },
});
console.log((0, graphql_1.print)(modified));
var templateObject_1;
