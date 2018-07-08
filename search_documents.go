package qf

import (
  "net/http"
  "fmt"
  "github.com/gorilla/mux"
  "path/filepath"
  "html/template"
  "strings"
  "database/sql"
)


func SearchDocuments(w http.ResponseWriter, r *http.Request) {
  _, err := GetCurrentUser(r)
  if err != nil {
    fmt.Fprintf(w, "You need to be logged in to continue. Exact Error: " + err.Error())
    return
  }

  vars := mux.Vars(r)
  ds := vars["document-structure"]

  detv, err := docExists(ds)
  if err != nil {
    fmt.Fprintf(w, "Error occurred while determining if this document exists. Exact Error: " + err.Error())
    return
  }
  if detv == false {
    fmt.Fprintf(w, "The document structure %s does not exists.", ds)
    return
  }

  tv1, err1 := DoesCurrentUserHavePerm(r, ds, "read")
  if err1 != nil {
    fmt.Fprintf(w, "Error occured while determining if the user have read permission for this page. Exact Error: " + err1.Error())
    return
  }
  if ! tv1 {
    fmt.Fprintf(w, "You don't have the read permission for this document structure.")
    return
  }

  var id int
  err = SQLDB.QueryRow("select id from qf_document_structures where name = ?", ds).Scan(&id)
  if err != nil {
    panic(err)
  }

  dds := GetDocData(id)

  if r.Method == http.MethodGet {
    type Context struct {
      DocName string
      DDs []DocData
    }
    ctx := Context{ds, dds}
    fullTemplatePath := filepath.Join(getProjectPath(), "templates/search-documents.html")
    tmpl := template.Must(template.ParseFiles(getBaseTemplate(), fullTemplatePath))
    tmpl.Execute(w, ctx)

  } else if r.Method == http.MethodPost {

    colNames := make([]string, 0)
    var colName string
    rows, err := SQLDB.Query("select name from qf_fields where dsid = ? and type != \"Section Break\" order by id asc limit 3", id)
    if err != nil {
      fmt.Fprintf(w, "Error reading column names. Exact Error: " + err.Error())
      return
    }
    defer rows.Close()
    for rows.Next() {
      err := rows.Scan(&colName)
      if err != nil {
        fmt.Fprintf(w, "Error reading a column name. Exact Error: " + err.Error())
        return
      }
      colNames = append(colNames, colName)
    }
    if err = rows.Err(); err != nil {
      fmt.Fprintf(w, "Extra Error reading column names. Exact Error: " + err.Error())
      return
    }
    colNames = append(colNames, "created", "created_by")

    endSqlStmt := make([]string, 0)
    sqlStmt := fmt.Sprintf("select id from `%s` where ", tableName(ds))
    for _, dd := range dds {
      if dd.Type == "Section Break" {
        continue
      }
      if r.FormValue(dd.Name) == "" {
        continue
      }

      switch dd.Type {
      case "Text", "Data", "Email", "Read Only", "URL", "Select", "Date", "Datetime":
        var data string
        if r.FormValue(dd.Name) == "" {
          data = "null"
        } else {
          data = fmt.Sprintf("\"%s\"", template.HTMLEscapeString(r.FormValue(dd.Name)))
        }
        endSqlStmt = append(endSqlStmt, dd.Name + " = " + data)
      case "Check":
        var data string
        if r.FormValue(dd.Name) == "on" {
          data = "\"t\""
        } else {
          data = "\"f\""
        }
        endSqlStmt = append(endSqlStmt, dd.Name + " = " + data)
      default:
        var data string
        if r.FormValue(dd.Name) == "" {
          data = "null"
        } else {
          data = template.HTMLEscapeString(r.FormValue(dd.Name))
        }
        endSqlStmt = append(endSqlStmt, dd.Name + " = " + data)
      }
    }

    ids := make([]uint64, 0)
    var idd uint64
    if r.FormValue("created_by") != "" && len(endSqlStmt) != 0 {
      sqlStmt += "created_by = " + template.HTMLEscapeString(r.FormValue("created_by"))
      sqlStmt += strings.Join(endSqlStmt, ", ")
    } else if r.FormValue("created_by") != "" {
      sqlStmt += "created_by = " + template.HTMLEscapeString(r.FormValue("created_by"))
    } else if r.FormValue("created_by") == "" && len(endSqlStmt) != 0 {
      sqlStmt += strings.Join(endSqlStmt, ", ")
    } else if len(endSqlStmt) == 0 && r.FormValue("created_by") == "" {
      fmt.Fprintf(w, "Your query is empty.")
      return
    }
    rows, err = SQLDB.Query(sqlStmt)
    if err != nil {
      fmt.Fprintf(w, "Error reading this document structure data. Exact Error: " + err.Error())
      return
    }
    defer rows.Close()
    for rows.Next() {
      err := rows.Scan(&idd)
      if err != nil {
        fmt.Fprintf(w, "Error reading a row of data for this document structure. Exact Error: " + err.Error())
        return
      }
      ids = append(ids, idd)
    }
    if err = rows.Err(); err != nil {
      fmt.Fprintf(w, "Extra error occurred while reading this document structure data. Exact Error: " + err.Error())
      return
    }

    if len(ids) == 0 {
      fmt.Fprintf(w, "Your query returned no results.")
      return
    }
    
    myRows := make([]Row, 0)
    for _, id := range ids {
      colAndDatas := make([]ColAndData, 0)
      for _, col := range colNames {
        var data string
        var dataFromDB sql.NullString
        sqlStmt := fmt.Sprintf("select %s from `%s` where id = %d", col, tableName(ds), id)
        err := SQLDB.QueryRow(sqlStmt).Scan(&dataFromDB)
        if err != nil {
          fmt.Fprintf(w, "An internal error occured. Exact Error: " + err.Error())
          return
        }
        if dataFromDB.Valid {
          data = dataFromDB.String
        } else {
          data = ""
        }
        colAndDatas = append(colAndDatas, ColAndData{col, data})
      }

      var createdBy uint64
      sqlStmt := fmt.Sprintf("select created_by from `%s` where id = %d", tableName(ds), id)
      err := SQLDB.QueryRow(sqlStmt).Scan(&createdBy)
      if err != nil {
        fmt.Fprintf(w, "An internal error occured. Exact Error: " + err.Error())
        return
      }
      myRows = append(myRows, Row{id, colAndDatas, false, false})
    }

    type Context struct {
      DocumentStructure string
      ColNames []string
      MyRows []Row
    }

    ctx := Context{ds, colNames, myRows}
    fullTemplatePath := filepath.Join(getProjectPath(), "templates/search-results.html")
    tmpl := template.Must(template.ParseFiles(getBaseTemplate(), fullTemplatePath))
    tmpl.Execute(w, ctx)


  }
}
