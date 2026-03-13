package server

import "net/http"

func StartServer() {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/api/v2/users", usersHandler)
	mux.HandleFunc("/api/v2/posts", postsHandler)
	http.ListenAndServe(":9090", mux)
}
