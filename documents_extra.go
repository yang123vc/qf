package qf

import (
  "net/http"
  "fmt"
  "github.com/gorilla/mux"
  "path/filepath"
  "html/template"
  "strings"
  "database/sql"
  "html"
)


func searchDocuments(w http.ResponseWriter, r *http.Request) {
  _, err := GetCurrentUser(r)
  if err != nil {
    errorPage(w, r, "You need to be logged in to continue.", err)
    return
  }

  vars := mux.Vars(r)
  ds := vars["document-structure"]

  detv, err := docExists(ds)
  if err != nil {
    errorPage(w, r, "Error occurred while determining if this document exists.", err)
    return
  }
  if detv == false {
    errorPage(w, r, fmt.Sprintf("The document structure %s does not exists.", ds), nil)
    return
  }

  tv1, err1 := DoesCurrentUserHavePerm(r, ds, "read")
  if err1 != nil {
    errorPage(w, r, "Error occured while determining if the user have read permission for this page.", err1)
    return
  }
  if ! tv1 {
    errorPage(w, r, "You don't have the read permission for this document structure.", nil)
    return
  }

  var id int
  err = SQLDB.QueryRow("select id from qf_document_structures where name = ?", ds).Scan(&id)
  if err != nil {
    errorPage(w, r, "An internal error occurred.", err)
  }

  dds := GetDocData(id)

  if r.Method == http.MethodGet {
    type Context struct {
      DocumentStructure string
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
      errorPage(w, r, "Error reading column names.", err)
      return
    }
    defer rows.Close()
    for rows.Next() {
      err := rows.Scan(&colName)
      if err != nil {
        errorPage(w, r, "Error reading a column name.  ", err)
        return
      }
      colNames = append(colNames, colName)
    }
    if err = rows.Err(); err != nil {
      errorPage(w, r, "Extra Error reading column names.", err)
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
          data = fmt.Sprintf("\"%s\"", html.EscapeString(r.FormValue(dd.Name)))
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
          data = html.EscapeString(r.FormValue(dd.Name))
        }
        endSqlStmt = append(endSqlStmt, dd.Name + " = " + data)
      }
    }

    ids := make([]uint64, 0)
    var idd uint64
    if r.FormValue("created_by") != "" && len(endSqlStmt) != 0 {
      sqlStmt += "created_by = " + html.EscapeString(r.FormValue("created_by"))
      sqlStmt += strings.Join(endSqlStmt, ", ")
    } else if r.FormValue("created_by") != "" {
      sqlStmt += "created_by = " + html.EscapeString(r.FormValue("created_by"))
    } else if r.FormValue("created_by") == "" && len(endSqlStmt) != 0 {
      sqlStmt += strings.Join(endSqlStmt, ", ")
    } else if len(endSqlStmt) == 0 && r.FormValue("created_by") == "" {
      errorPage(w, r, "Your query is empty.", nil)
      return
    }
    rows, err = SQLDB.Query(sqlStmt)
    if err != nil {
      errorPage(w, r, "Error reading this document structure data.  " , err)
      return
    }
    defer rows.Close()
    for rows.Next() {
      err := rows.Scan(&idd)
      if err != nil {
        errorPage(w, r, "Error reading a row of data for this document structure.  " , err)
        return
      }
      ids = append(ids, idd)
    }
    if err = rows.Err(); err != nil {
      errorPage(w, r, "Extra error occurred while reading this document structure data.  " , err)
      return
    }

    if len(ids) == 0 {
      errorPage(w, r, "Your query returned no results.", nil)
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
          errorPage(w, r, "An internal error occured.  " , err)
          return
        }
        if dataFromDB.Valid {
          data = html.UnescapeString(dataFromDB.String)
        } else {
          data = ""
        }
        colAndDatas = append(colAndDatas, ColAndData{col, data})
      }

      var createdBy uint64
      sqlStmt := fmt.Sprintf("select created_by from `%s` where id = %d", tableName(ds), id)
      err := SQLDB.QueryRow(sqlStmt).Scan(&createdBy)
      if err != nil {
        errorPage(w, r, "An internal error occured.  " , err)
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


func dateLists(w http.ResponseWriter, r *http.Request) {
  useridUint64, err := GetCurrentUser(r)
  if err != nil {
    errorPage(w, r, "You need to be logged in to continue.", err)
    return
  }

  vars := mux.Vars(r)
  ds := vars["document-structure"]

  detv, err := docExists(ds)
  if err != nil {
    errorPage(w, r, "Error occurred while determining if this document exists.", err)
    return
  }
  if detv == false {
    errorPage(w, r, fmt.Sprintf("The document structure %s does not exists.", ds), nil)
    return
  }

  tv1, err1 := DoesCurrentUserHavePerm(r, ds, "read")
  tv2, err2 := DoesCurrentUserHavePerm(r, ds, "read-only-created")
  if err1 != nil || err2 != nil {
    errorPage(w, r, "Error occured while determining if the user have read permission for this page.", nil)
    return
  }

  if ! tv1 && ! tv2 {
    errorPage(w, r, "You don't have the read permission for this document structure.", nil)
    return
  }

  var count uint64
  sqlStmt := fmt.Sprintf("select count(*) from `%s`", tableName(ds))
  err = SQLDB.QueryRow(sqlStmt).Scan(&count)
  if count == 0 {
    errorPage(w, r, "There are no documents to display.", nil)
    return
  }

  if tv1 {
    sqlStmt = fmt.Sprintf("select distinct date(created) as dc from `%s` order by dc desc", tableName(ds))
  } else if tv2 {
    sqlStmt = fmt.Sprintf("select distinct date(created) as dc from `%s` where created_by = %d order by dc desc", tableName(ds), useridUint64)
  }

  dates := make([]string, 0)
  var date string
  rows, err := SQLDB.Query(sqlStmt)
  if err != nil {
    errorPage(w, r, "Error getting date data for this document structure.  " , err)
    return
  }
  defer rows.Close()
  for rows.Next() {
    err := rows.Scan(&date)
    if err != nil {
      errorPage(w, r, "Error retrieving single date.  " , err)
      return
    }
    dates = append(dates, date)
  }
  if err = rows.Err(); err != nil {
    errorPage(w, r, "Error after retrieving date data.  " , err)
    return
  }

  type DateAndCount struct {
    Date string
    Count uint64
  }
  dacs := make([]DateAndCount, 0)
  for _, date := range dates {
    var count uint64
    sqlStmt = fmt.Sprintf("select count(*) from `%s` where date(created) = ?", tableName(ds))
    err = SQLDB.QueryRow(sqlStmt, date).Scan(&count)
    if err != nil {
      errorPage(w, r, "Error reading count of a date list.  " , err)
      return
    }
    dacs = append(dacs, DateAndCount{date, count})
  }

  type Context struct {
    DACs []DateAndCount
    DocumentStructure string
  }

  ctx := Context{dacs, ds}
  fullTemplatePath := filepath.Join(getProjectPath(), "templates/date-lists.html")
  tmpl := template.Must(template.ParseFiles(getBaseTemplate(), fullTemplatePath))
  tmpl.Execute(w, ctx)
}


func dateList(w http.ResponseWriter, r *http.Request) {
  useridUint64, err := GetCurrentUser(r)
  if err != nil {
    errorPage(w, r, "You need to be logged in to continue.", err)
    return
  }

  vars := mux.Vars(r)
  ds := vars["document-structure"]
  date := vars["date"]

  readSqlStmt := fmt.Sprintf("select id from `%s` where date(created) = '%s' order by created desc limit ?, ?",
    tableName(ds), html.EscapeString(date))
  rocSqlStmt := fmt.Sprintf("select id from `%s` where date(created) = '%s' and created_by = %d order by created desc limit ?, ?",
    tableName(ds), html.EscapeString(date), useridUint64)
  readTotalSql := fmt.Sprintf("select count(*) from `%s` where date(created) = '%s'",
    tableName(ds), html.EscapeString(date))
  rocTotalSql := fmt.Sprintf("select count(*) from `%s` where date(created) = '%s' and created_by = %d",
    tableName(ds), html.EscapeString(date))
  innerListDocuments(w, r, readSqlStmt, rocSqlStmt, readTotalSql, rocTotalSql, "date-list")
  return
}
