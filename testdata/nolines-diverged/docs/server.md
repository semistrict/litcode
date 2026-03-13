# Server

```go file=src/server.go
func StartServer() {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthCheck)
	mux.HandleFunc("/api/v1/users", listUsers)
	mux.HandleFunc("/api/v1/posts", listPosts)
	http.ListenAndServe(":8080", mux)
}
```
