package qf

import (
  // "github.com/gorilla/mux"
  "net/http"
  "fmt"
  "path/filepath"
  "html/template"
  "sort"
)


func RolesView(w http.ResponseWriter, r *http.Request) {
  strSlice := make([]string, 0)
  var str string
  rows, err := SQLDB.Query("select role from qf_roles order by role asc")
  if err != nil {
    fmt.Fprintf(w, "Error occured while trying to collect roles. Exact Error: " + err.Error())
    return
  }
  defer rows.Close()
  for rows.Next() {
    err := rows.Scan(&str)
    if err != nil {
      fmt.Fprintf(w, "Error occured while trying to collect role. Exact Error: " + err.Error())
      return
    }
    strSlice = append(strSlice, str)
  }
  if err = rows.Err(); err != nil {
    fmt.Fprintf(w, "An error occured: " + err.Error())
    return
  }

  if r.Method == http.MethodGet {
    type Context struct {
      Roles []string
      NumberOfRoles int
    }

    ctx := Context{strSlice, len(strSlice)}
    tmpl := template.Must(template.ParseFiles(filepath.Join(getProjectPath(), "templates/roles-view.html")))
    tmpl.Execute(w, ctx)

  } else if r.Method == http.MethodPost {

    role := r.FormValue("role")
    if len(strSlice) == 0 {
      sort.Strings(strSlice)
      i := sort.SearchStrings(strSlice, role)
      if i != len(strSlice) {
        fmt.Fprintf(w, "The role \"%s\" already exists.", role)
        return
      }
    }

    _, err := SQLDB.Exec("insert into qf_roles(role) values(?)", role)
    if err != nil {
      fmt.Fprintf(w, "Error creating role. Exact Error: " + err.Error())
      return
    }

    fmt.Fprintf(w, "Successfully create role.")
  }
}
