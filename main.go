package main

import (
  "net/http"
  // "github.com/gorilla/mux"
  // "html/template"
)

func main() {
  http.HandleFunc("/new-document-schema/", func(w http.ResponseWriter, r *http.Request) {
    http.ServeFile(w, r, "new-document-schema.html")
  })

  fs := http.FileServer(http.Dir("statics/"))
  http.Handle("/static/", http.StripPrefix("/static/", fs))

  http.ListenAndServe(":3001", nil)
}
