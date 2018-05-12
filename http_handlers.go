package qf

import (
  "net/http"
  "fmt"
  "os/user"
  "path/filepath"
)

func getProjectPath() string {
  userStruct, err := user.Current()
  if err != nil {
    panic(err)
  }
  projectPath := filepath.Join(userStruct.HomeDir, "go/src/github.com/bankole7782/qf")
  return projectPath
}


func NewDocumentSchema(w http.ResponseWriter, r *http.Request) {
  if r.Method == http.MethodPost {
    r.ParseForm()
    fmt.Println(r.PostForm)
  } else {

    http.ServeFile(w, r, filepath.Join(getProjectPath(), "new-document-schema.html"))
  }
}


func JQuery(w http.ResponseWriter, r *http.Request) {
  http.ServeFile(w, r, filepath.Join(getProjectPath(), "statics/jquery-3.3.1.min.js"))
}
