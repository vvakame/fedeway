#!/bin/bash -eux

cd "$(dirname "$0")"

GRAPHQLJS_REVISION=2df59f18dd3f3c415eaba57d744131a674079ddf
FEDERATION_REVISION=325c4c9455b16d5443c8d95dd730a1de51cf2972

rm -rf implementations
mkdir implementations

cd implementations

git clone git@github.com:graphql/graphql-js.git
( cd graphql-js && git checkout $GRAPHQLJS_REVISION && npm ci )

git clone git@github.com:apollographql/federation.git
( cd federation && git checkout $FEDERATION_REVISION && npm ci )
