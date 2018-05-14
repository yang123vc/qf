package qf

import (
  "net/http"
  "fmt"
  "os/user"
  "path/filepath"
  "strconv"
  "strings"
  "html/template"
  // "database/sql"
  // _ "github.com/go-sql-driver/mysql"
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

  type QFField struct {
    label string
    name string
    type_ string
    options string
    default_value string
    other_options string
  }

  qffs := make([]QFField, 0)

  if r.Method == http.MethodPost {
    r.ParseForm()
    i := 1
    for i < 100 {
      iStr := strconv.Itoa(i)
      if r.FormValue("label-" + iStr) == "" {
        break
      } else {
        qff := QFField{
          label: r.FormValue("label-" + iStr),
          name: r.FormValue("name-" + iStr),
          type_: r.FormValue("type-" + iStr),
          options: strings.Join(r.PostForm["options-" + iStr], ","),
          default_value: r.FormValue("default-value-" + iStr),
          other_options: r.FormValue("other-options-" + iStr),
        }
        qffs = append(qffs, qff)
        i += 1
      }
    }

    tx, _ := SQLDB.Begin()
    var singleton string
    if r.FormValue("singleton") != "" {
      singleton = "t"
    } else {
      singleton = "f"
    }

    var childTable string
    if r.FormValue("child-table") != "" {
      childTable = "t"
    } else {
      childTable = "f"
    }

    res, err := tx.Exec(`insert into qf_forms(doc_name, child_table, singleton)
      values(?, ?, ?)`, r.FormValue("doc-name"), childTable, singleton)
    if err != nil {
      tx.Rollback()
      panic(err)
    }

    formId, _:= res.LastInsertId()
    stmt, err := tx.Prepare(`insert into qf_fields(formid, label, name, type, options, default_value, other_options)
      values(?, ?, ?, ?, ?, ?, ?)`)
    if err != nil {
      tx.Rollback()
      panic(err)
    }
    for i:= 0; i < len(qffs); i++ {
      o := qffs[i]
      _, err := stmt.Exec(formId, o.label, o.name, o.type_, o.options, o.default_value, o.other_options)
      if err != nil {
        tx.Rollback()
        panic(err)
      }
    }
    tx.Commit()
    fmt.Fprintf(w, "Document Schema saved.")

  } else {
    type Context struct {
      DocNames string
    }
    tempSlice := make([]string, 0)
    var str string
    rows, err := SQLDB.Query("select doc_name from qf_forms")
    if err != nil {
      fmt.Fprintf(w, "An error occured: " + err.Error())
      return
    }
    defer rows.Close()
    for rows.Next() {
      err := rows.Scan(&str)
      if err != nil {
        panic(err)
      }
      tempSlice = append(tempSlice, str)
    }
    err = rows.Err()
    if err != nil {
      fmt.Fprintf(w, "An error occured: " + err.Error())
      return
    }
    ctx := Context{strings.Join(tempSlice, ",")}
    tmpl := template.Must(template.ParseFiles(filepath.Join(getProjectPath(), "new-document-schema.html")))
    tmpl.Execute(w, ctx)
  }
}


func JQuery(w http.ResponseWriter, r *http.Request) {
  http.ServeFile(w, r, filepath.Join(getProjectPath(), "statics/jquery-3.3.1.min.js"))
}
