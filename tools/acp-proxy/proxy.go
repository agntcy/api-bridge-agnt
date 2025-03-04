package main

import (
	"acp-proxy/acp_api"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	middleware "github.com/oapi-codegen/nethttp-middleware"
)

func simpleLoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

func main() {
	ListenAddressPtr := flag.String("listen", "0.0.0.0:8080", "Listen address")
	flag.Parse()

	swagger, err := acp_api.GetSwagger()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading swagger spec\n: %s", err)
		os.Exit(1)
	}
	swagger.Servers = nil

	server := acp_api.NewACPServer()
	r := http.NewServeMux()

	h := acp_api.HandlerFromMux(server, r)
	h = middleware.OapiRequestValidator(swagger)(h)
	h = simpleLoggingMiddleware(h)

	s := &http.Server{
		Addr:    *ListenAddressPtr,
		Handler: h,
	}

	log.Fatal(s.ListenAndServe())
}
