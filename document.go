package qf

import (
  "net/http"
  "fmt"
  "path/filepath"
  "strings"
  "html/template"
  "github.com/gorilla/mux"
)


type DocData struct {
  Label string
  Name string
  Type string
  Required bool
  Unique bool
  OtherOptions []string
}


func getDocData(formId int) []DocData{
  var label, name, type_, options, otherOptions string

  dds := make([]DocData, 0)
  rows, err := SQLDB.Query("select label, name, type, options, other_options from qf_fields where formid = ? order by id asc", formId)
  if err != nil {
    panic(err)
  }
  defer rows.Close()
  for rows.Next() {
    err := rows.Scan(&label, &name, &type_, &options, &otherOptions)
    if err != nil {
      panic(err)
    }
    var required, unique bool
    if optionSearch(options, "required") {
      required = true
    }
    if optionSearch(options, "unique") {
      unique = true
    }
    dd := DocData{label, name, type_, required, unique, strings.Split(otherOptions, "\n")}
    dds = append(dds, dd)
  }
  err = rows.Err()
  if err != nil {
    panic(err)
  }

  return dds
}


func CreateDocument(w http.ResponseWriter, r *http.Request) {
  vars := mux.Vars(r)
  doc := vars["document-schema"]

  if ! docExists(doc, w) {
    fmt.Fprintf(w, "The document schema %s does not exists.", doc)
    return
  }

  var id int
  err := SQLDB.QueryRow("select id from qf_forms where doc_name = ?", doc).Scan(&id)
  if err != nil {
    panic(err)
  }

  dds := getDocData(id)

  if r.Method == http.MethodGet {
    type Context struct {
      DocName string
      DDs []DocData
    }
    ctx := Context{doc, dds}
    tmpl := template.Must(template.ParseFiles(filepath.Join(getProjectPath(), "templates/create-document.html")))
    tmpl.Execute(w, ctx)

  } else if r.Method == http.MethodPost {
    colNames := make([]string, 0)
    formData := make([]string, 0)
    for _, dd := range dds {
      colNames = append(colNames, dd.Name)
      switch dd.Type {
      case "Text", "Data", "Email", "Read Only", "URL", "Select", "Date", "Datetime":
        data := fmt.Sprintf("\"%s\"", r.FormValue(dd.Name))
        formData = append(formData, data)
      case "Check":
        var data string
        if r.FormValue(dd.Name) == "on" {
          data = "\"t\""
        } else {
          data = "\"f\""
        }
        formData = append(formData, data)
      default:
        formData = append(formData, r.FormValue(dd.Name))
      }
    }
    colNamesStr := strings.Join(colNames, ", ")
    formDataStr := strings.Join(formData, ", ")
    sql := fmt.Sprintf("insert into `%s`(created, modified, %s) values(now(), now(), %s)", tableName(doc), colNamesStr, formDataStr)
    _, err := SQLDB.Exec(sql)
    if err != nil {
      fmt.Fprintf(w, "An error occured while saving: " + err.Error())
      return
    }

    fmt.Fprintln(w, "Successfully inserted values.")
  }

}


func EditDocument(w http.ResponseWriter, r *http.Request) {
  vars := mux.Vars(r)
  doc := vars["document-schema"]
  docid := vars["id"]

  if ! docExists(doc, w) {
    fmt.Fprintf(w, "The document schema %s does not exists.", doc)
    return
  }

  var count uint64
  sql := fmt.Sprintf("select count(*) from `%s` where id = %s", tableName(doc), docid)
  err := SQLDB.QueryRow(sql).Scan(&count)
  if count == 0 {
    fmt.Fprintf(w, "The document with id %s do not exists", docid)
    return
  }

  var id int
  err = SQLDB.QueryRow("select id from qf_forms where doc_name = ?", doc).Scan(&id)
  if err != nil {
    panic(err)
  }

  docDatas := getDocData(id)

  type docAndSchema struct {
    DocData
    Data string
  }

  docAndSchemaSlice := make([]docAndSchema, 0)
  for _, docData := range docDatas {
    var data string
    sql := fmt.Sprintf("select %s from `%s` where id = %s", docData.Name, tableName(doc), docid)
    err := SQLDB.QueryRow(sql).Scan(&data)
    if err != nil {
      panic(err)
    }
    docAndSchemaSlice = append(docAndSchemaSlice, docAndSchema{docData, data})
  }

  var created, modified string
  sql = fmt.Sprintf("select created, modified from `%s` where id = %s", tableName(doc), docid)
  err = SQLDB.QueryRow(sql).Scan(&created, &modified)
  if err != nil {
    panic(err)
  }

  if r.Method == http.MethodGet {
    type Context struct {
      Created string
      Modified string
      DocName string
      DocAndSchemas []docAndSchema
      Id string
    }

    ctx := Context{created, modified, doc, docAndSchemaSlice, docid}
    tmpl := template.Must(template.ParseFiles(filepath.Join(getProjectPath(), "templates/edit-document.html")))
    tmpl.Execute(w, ctx)

  } else if r.Method == http.MethodPost {

    colNames := make([]string, 0)
    formData := make([]string, 0)
    for _, docAndSchema := range docAndSchemaSlice {
      if docAndSchema.Data != r.FormValue(docAndSchema.DocData.Name) {
        colNames = append(colNames, docAndSchema.DocData.Name)
        switch docAndSchema.DocData.Type {
        case "Text", "Data", "Email", "Read Only", "URL", "Select", "Date", "Datetime":
          data := fmt.Sprintf("\"%s\"", r.FormValue(docAndSchema.DocData.Name))
          formData = append(formData, data)
        case "Check":
          var data string
          if r.FormValue(docAndSchema.DocData.Name) == "on" {
            data = "\"t\""
          } else {
            data = "\"f\""
          }
          formData = append(formData, data)
        default:
          formData = append(formData, r.FormValue(docAndSchema.DocData.Name))
        }
      }
    }

    updatePartStmt := make([]string, 0)
    updatePartStmt = append(updatePartStmt, "modified = now()")
    for i := 0; i < len(colNames); i++ {
      stmt1 := fmt.Sprintf("%s = %s", colNames[i], formData[i])
      updatePartStmt = append(updatePartStmt, stmt1)
    }

    sql := fmt.Sprintf("update `%s` set %s where id = %s", tableName(doc), strings.Join(updatePartStmt, ", "), docid)
    fmt.Println(sql)
    _, err := SQLDB.Exec(sql)
    if err != nil {
      fmt.Fprintf(w, "An error occured while saving: " + err.Error())
      return
    }

    fmt.Fprintln(w, "Successfully updated.")
  }

}


func ListDocuments(w http.ResponseWriter, r *http.Request) {
  vars := mux.Vars(r)
  doc := vars["document-schema"]

  if ! docExists(doc, w) {
    fmt.Fprintf(w, "The document schema %s does not exists.", doc)
    return
  }

  var count uint64
  sql := fmt.Sprintf("select count(*) from `%s`", tableName(doc))
  err := SQLDB.QueryRow(sql).Scan(&count)
  if count == 0 {
    fmt.Fprintf(w, "There are no documents to display.")
    return
  }

  var id int
  err = SQLDB.QueryRow("select id from qf_forms where doc_name = ?", doc).Scan(&id)
  if err != nil {
    panic(err)
  }

  colNames := make([]string, 0)
  var colName string
  rows, err := SQLDB.Query("select name from qf_fields where formid = ? order by id asc limit 3", id)
  if err != nil {
    panic(err)
  }
  defer rows.Close()
  for rows.Next() {
    err := rows.Scan(&colName)
    if err != nil {
      panic(err)
    }
    colNames = append(colNames, colName)
  }
  if err = rows.Err(); err != nil {
    panic(err)
  }

  ids := make([]uint64, 0)
  var idd uint64
  sql = fmt.Sprintf("select id from `%s` order by id asc limit 50", tableName(doc))
  rows, err = SQLDB.Query(sql)
  if err != nil {
    panic(err)
  }
  defer rows.Close()
  for rows.Next() {
    err := rows.Scan(&idd)
    if err != nil {
      panic(err)
    }
    ids = append(ids, idd)
  }
  if err = rows.Err(); err != nil {
    panic(err)
  }

  type ColAndData struct {
    ColName string
    Data string
  }

  type Row struct {
    Id uint64
    ColAndDatas []ColAndData
  }
  myRows := make([]Row, 0)
  for _, id := range ids {
    colAndDatas := make([]ColAndData, 0)
    for _, col := range colNames {
      var data string
      sql := fmt.Sprintf("select %s from `%s` where id = %d order by id asc limit 50", col, tableName(doc), id)
      err := SQLDB.QueryRow(sql).Scan(&data)
      if err != nil {
        panic(err)
      }
      colAndDatas = append(colAndDatas, ColAndData{col, data})
    }
    myRows = append(myRows, Row{id, colAndDatas})
  }


  // fmt.Fprintln(w, myRows)
  type Context struct {
    DocName string
    ColNames []string
    MyRows []Row
  }

  ctx := Context{doc, colNames, myRows}
  tmpl := template.Must(template.ParseFiles(filepath.Join(getProjectPath(), "templates/list-documents.html")))
  tmpl.Execute(w, ctx)
}


func DeleteDocument(w http.ResponseWriter, r *http.Request) {
  vars := mux.Vars(r)
  doc := vars["document-schema"]
  docid := vars["id"]

  if ! docExists(doc, w) {
    fmt.Fprintf(w, "The document schema %s does not exists.", doc)
    return
  }

  var count uint64
  sql := fmt.Sprintf("select count(*) from `%s` where id = %s", tableName(doc), docid)
  err := SQLDB.QueryRow(sql).Scan(&count)
  if count == 0 {
    fmt.Fprintf(w, "The document with id %s do not exists", docid)
    return
  }

  sql = fmt.Sprintf("delete from `%s` where id = %s", tableName(doc), docid)
  _, err = SQLDB.Exec(sql)
  if err != nil {
    fmt.Fprintf(w, "An error occured: " + err.Error())
    return
  }

  redirectURL := fmt.Sprintf("/doc/%s/list/", doc)
  http.Redirect(w, r, redirectURL, 307)
}
