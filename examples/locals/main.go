package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/go-logr/logr"
	"github.com/go-logr/stdr"
	"github.com/vvakame/fedeway/gateway"
	"github.com/vvakame/fedeway/internal/engine/subgraphs/accounts"
	"github.com/vvakame/fedeway/internal/engine/subgraphs/books"
	"github.com/vvakame/fedeway/internal/engine/subgraphs/documents"
	"github.com/vvakame/fedeway/internal/engine/subgraphs/inventory"
	"github.com/vvakame/fedeway/internal/engine/subgraphs/product"
	"github.com/vvakame/fedeway/internal/engine/subgraphs/reviews"
)

func main() {
	err := realMain()
	if err != nil {
		log.Fatal(err)
	}
}

func realMain() error {
	ctx := context.Background()

	logger := stdr.New(log.Default())
	ctx = logr.NewContext(ctx, logger)

	type DemoSchema interface {
		Name() string
		ExecutableSchema() graphql.ExecutableSchema
	}
	schemas := []DemoSchema{
		accounts.NewExecutableSchema(),
		books.NewExecutableSchema(),
		documents.NewExecutableSchema(),
		inventory.NewExecutableSchema(),
		product.NewExecutableSchema(),
		reviews.NewExecutableSchema(),
	}
	serviceDefs := make([]*gateway.ServiceDefinition, 0, len(schemas))
	for _, schema := range schemas {
		serviceDefs = append(serviceDefs, gateway.NewLocalServiceDefinition(
			schema.Name(),
			schema.ExecutableSchema(),
		))
	}

	gw, err := gateway.NewGateway(ctx, &gateway.GatewayConfig{
		ServiceDefinitions: serviceDefs,
	})
	if err != nil {
		logger.Error(err, "failed to execute NewGateway")
		return err
	}

	srv := handler.NewDefaultServer(gw)
	mux := http.NewServeMux()
	mux.Handle("/", playground.Handler("fedeway - locals", "/query"))
	mux.Handle("/query", srv)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := fmt.Sprintf(":%s", port)

	logger.Info("listening server", "addr", addr)

	err = http.ListenAndServe(addr, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r = r.WithContext(logr.NewContext(r.Context(), logger))
		mux.ServeHTTP(w, r)
	}))
	if err != nil {
		return err
	}

	return nil
}
