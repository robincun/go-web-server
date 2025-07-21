package main

import (
	"fmt"
	"net/http"

	"github.com/robincun/go-web-server/server"
)

func customRouteHandler(w http.ResponseWriter, r *http.Request, s *server.Session) {
	s.Authorized = true
	w.Write([]byte("Now Authorized"))
}
func main() {
	customRoutes := make([]server.CustomRoute, 1)
	customRoute := server.CustomRoute{
		Path:                  "/custom/authorize",
		IsExpirable:           false,
		IsAuthorizationNeeded: false,
		Handler:               customRouteHandler,
	}
	customRoutes[0] = customRoute
	err := server.StartServer(customRoutes)
	if err != nil {
		panic(err)
	}
	fmt.Println("Server stopped")
}
