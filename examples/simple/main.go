package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/go-logr/logr"
	"github.com/go-logr/stdr"
	"github.com/vvakame/fedeway/internal/gateway"
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

	gw, err := gateway.NewGateway(ctx, &gateway.GatewayConfig{
		ServiceDefinitions: []*gateway.ServiceDefinition{
			{
				Name: "accounts",
				URL:  "http://localhost:4001/graphql",
			},
			{
				Name: "reviews",
				URL:  "http://localhost:4002/graphql",
			},
			{
				Name: "products",
				URL:  "http://localhost:4003/graphql",
			},
			{
				Name: "inventory",
				URL:  "http://localhost:4004/graphql",
			},
		},
	})
	if err != nil {
		logger.Error(err, "failed to execute NewGateway")
		return err
	}

	srv := handler.NewDefaultServer(gw)
	mux := http.NewServeMux()
	mux.Handle("/", playground.Handler("fedeway - simple", "/query"))
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
