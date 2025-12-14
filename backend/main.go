package main

import (
    "log"
    "net/http"
    "os"

    "github.com/go-chi/chi/v5"
)

func main() {
    r := chi.NewRouter()

    r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        w.Write([]byte("ok"))
    })

    r.Get("/airports", func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        w.Write([]byte("[]"))
    })

    addr := ":" + getPort()
    log.Println("backend listening on", addr)
    log.Fatal(http.ListenAndServe(addr, r))
}

func getPort() string {
    port := os.Getenv("PORT")
    if port == "" {
        port = "4000"
    }
    return port
}
