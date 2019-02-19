package qf

import (
  "net/http"
  "fmt"
  "github.com/gorilla/mux"
  "html/template"
  "strings"
  "database/sql"
  "html"
  "math"
  "strconv"
)


func innerListDocuments(w http.ResponseWriter, r *http.Request, readSqlStmt, totalSqlStmt, listType string) {
  useridUint64, err := GetCurrentUser(r)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  vars := mux.Vars(r)
  ds := vars["document-structure"]
  page := vars["page"]
  var pageI uint64
  if page != "" {
    pageI, err = strconv.ParseUint(page, 10, 64)
    if err != nil {
      errorPage(w, err.Error())
      return
    }
  } else {
    pageI = 1
  }

  detv, err := docExists(ds)
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  if detv == false {
    errorPage(w, fmt.Sprintf("The document structure %s does not exists.", ds))
    return
  }

  tv1, err := DoesCurrentUserHavePerm(r, ds, "read")
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  tv2, err := DoesCurrentUserHavePerm(r, ds, "read-only-created")
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  if ! tv1 && ! tv2 {
    errorPage(w, "You don't have the read permission for this document structure.")
    return
  }

  var count uint64
  err = SQLDB.QueryRow(totalSqlStmt).Scan(&count)
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  if count == 0 {
    errorPage(w, "There are no documents to display.")
    return
  }

  var id int
  err = SQLDB.QueryRow("select id from qf_document_structures where fullname = ?", ds).Scan(&id)
  if err != nil {
    errorPage(w, err.Error())
  }

  colNames, err := getColumnNames(ds)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  var itemsPerPage uint64 = 50
  startIndex := (pageI - 1) * itemsPerPage
  totalItems := count
  totalPages := math.Ceil( float64(totalItems) / float64(itemsPerPage) )

  ids := make([]uint64, 0)
  var idd uint64

  rows, err := SQLDB.Query(readSqlStmt, startIndex, itemsPerPage)
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  defer rows.Close()
  for rows.Next() {
    err := rows.Scan(&idd)
    if err != nil {
      errorPage(w, err.Error())
      return
    }
    ids = append(ids, idd)
  }
  if err = rows.Err(); err != nil {
    errorPage(w, err.Error())
    return
  }

  uocPerm, err1 := DoesCurrentUserHavePerm(r, ds, "update-only-created")
  docPerm, err2 := DoesCurrentUserHavePerm(r, ds, "delete-only-created")
  if err1 != nil || err2 != nil {
    errorPage(w, "Error occured while determining if the user have read permission for this page.")
    return
  }

  tblName, err := tableName(ds)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  myRows := make([]Row, 0)
  for _, id := range ids {
    colAndDatas := make([]ColAndData, 0)
    for _, colLabel := range colNames {
      var data string
      var dataFromDB sql.NullString
      sqlStmt := fmt.Sprintf("select %s from `%s` where id = %d", colLabel.Col, tblName, id)
      err := SQLDB.QueryRow(sqlStmt).Scan(&dataFromDB)
      if err != nil {
        errorPage(w, err.Error())
        return
      }
      if dataFromDB.Valid {
        data = html.UnescapeString(dataFromDB.String)
      } else {
        data = ""
      }
      colAndDatas = append(colAndDatas, ColAndData{colLabel.Label, data})
    }

    var createdBy uint64
    sqlStmt := fmt.Sprintf("select created_by from `%s` where id = %d", tblName, id)
    err := SQLDB.QueryRow(sqlStmt).Scan(&createdBy)
    if err != nil {
      errorPage(w, err.Error())
      return
    }
    rup := false
    rdp := false
    if createdBy == useridUint64 && uocPerm {
      rup = true
    }
    if createdBy == useridUint64 && docPerm {
      rdp = true
    }
    myRows = append(myRows, Row{id, colAndDatas, rup, rdp})
  }


  type Context struct {
    DocumentStructure string
    ColNames []string
    MyRows []Row
    CurrentPage uint64
    Pages []uint64
    CreatePerm bool
    UpdatePerm bool
    DeletePerm bool
    HasApprovals bool
    Approver bool
    ListType string
    OptionalDate string
  }

  pages := make([]uint64, 0)
  for i := uint64(0); i < uint64(totalPages); i++ {
    pages = append(pages, i+1)
  }

  tv1, err1 = DoesCurrentUserHavePerm(r, ds, "create")
  tv2, err2 = DoesCurrentUserHavePerm(r, ds, "update")
  tv3, err3 := DoesCurrentUserHavePerm(r, ds, "delete")
  if err1 != nil || err2 != nil || err3 != nil {
    errorPage(w, "An error occurred when getting permissions of this document structure for this user.")
    return
  }

  hasApprovals, err := isApprovalFrameworkInstalled(ds)
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  approver, err := isApprover(r, ds)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  var date string
  if listType == "date-list" {
    date = vars["date"]
  } else {
    date = ""
  }

  colNamesList := make([]string, 0)
  for _, colLabel := range colNames {
    colNamesList = append(colNamesList, colLabel.Label)
  }
  ctx := Context{ds, colNamesList, myRows, pageI, pages, tv1, tv2, tv3, hasApprovals,
    approver, listType, date}
  tmpl := template.Must(template.ParseFiles(getBaseTemplate(), "qffiles/list-documents.html"))
  tmpl.Execute(w, ctx)
}


func listDocuments(w http.ResponseWriter, r *http.Request) {
  useridUint64, err := GetCurrentUser(r)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  vars := mux.Vars(r)
  ds := vars["document-structure"]

  detv, err := docExists(ds)
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  if detv == false {
    errorPage(w, fmt.Sprintf("The document structure %s does not exists.", ds))
    return
  }

  tv1, err := DoesCurrentUserHavePerm(r, ds, "read")
  if err != nil {
    errorPage(w, err.Error())
  }
  tv2, err := DoesCurrentUserHavePerm(r, ds, "read-only-created")
  if err != nil {
    errorPage(w, err.Error())
  }

  tblName, err := tableName(ds)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  var readSqlStmt string
  var totalSqlStmt string

  if tv1 {
    readSqlStmt = fmt.Sprintf("select id from `%s` order by created desc limit ?, ?", tblName)
    totalSqlStmt = fmt.Sprintf("select count(*) from `%s`", tblName)
  } else if tv2 {
    readSqlStmt = fmt.Sprintf("select id from `%s` where created_by = %d order by created desc limit ?, ?", tblName, useridUint64 )
    totalSqlStmt = fmt.Sprintf("select count(*) from `%s` where created_by = %d", tblName, useridUint64)
  }

  innerListDocuments(w, r, readSqlStmt, totalSqlStmt, "true-list")
  return
}


func searchDocuments(w http.ResponseWriter, r *http.Request) {
  _, err := GetCurrentUser(r)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  vars := mux.Vars(r)
  ds := vars["document-structure"]

  detv, err := docExists(ds)
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  if detv == false {
    errorPage(w, fmt.Sprintf("The document structure %s does not exists.", ds))
    return
  }

  tv1, err := DoesCurrentUserHavePerm(r, ds, "read")
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  tv2, err := DoesCurrentUserHavePerm(r, ds, "read-only-created")
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  if ! tv1 && ! tv2 {
    errorPage(w, "You don't have the read permission for this document structure.")
    return
  }

  dds, err := GetDocData(ds)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  type Context struct {
    DocumentStructure string
    DDs []DocData
    UserHasLimitedReadPermission bool
  }
  ctx := Context{ds, dds, tv2 }
  tmpl := template.Must(template.ParseFiles(getBaseTemplate(), "qffiles/search-documents.html"))
  tmpl.Execute(w, ctx)
}


func searchResults(w http.ResponseWriter, r *http.Request) {
  useridUint64, err := GetCurrentUser(r)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  vars := mux.Vars(r)
  ds := vars["document-structure"]

  detv, err := docExists(ds)
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  if detv == false {
    errorPage(w, fmt.Sprintf("The document structure %s does not exists.", ds))
    return
  }

  tv1, err := DoesCurrentUserHavePerm(r, ds, "read")
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  tv2, err := DoesCurrentUserHavePerm(r, ds, "read-only-created")
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  if ! tv1 && ! tv2 {
    errorPage(w, "You don't have the read permission for this document structure.")
    return
  }

  dds, err := GetDocData(ds)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  endSqlStmt := make([]string, 0)
  for _, dd := range dds {
    if dd.Type == "Section Break" || dd.Type == "Image" || dd.Type == "File" {
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

  tblName, err := tableName(ds)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  var readSqlStmt string
  var totalSqlStmt string

  if tv1 {
    if r.FormValue("created_by") != "" {
      endSqlStmt = append(endSqlStmt, "created_by = " + html.EscapeString(r.FormValue("created_by")))
    }
    if len(endSqlStmt) == 0 {
      errorPage(w, "Your query is empty.")
      return
    } else {
      readSqlStmt = fmt.Sprintf("select id from `%s` where ", tblName) + strings.Join(endSqlStmt, " and ")
      readSqlStmt += " order by created desc limit ?, ?"
      totalSqlStmt = fmt.Sprintf("select count(*) from `%s` where ", tblName) + strings.Join(endSqlStmt, " and ")
    }
  } else if tv2 {
    endSqlStmt = append(endSqlStmt, fmt.Sprintf("created_by = %d", useridUint64))
    readSqlStmt = fmt.Sprintf("select id from `%s` where ", tblName) + strings.Join(endSqlStmt, " and ")
    readSqlStmt += " order by created desc limit ?, ?"
    totalSqlStmt = fmt.Sprintf("select count(*) from `%s` where ", tblName) + strings.Join(endSqlStmt, " and ")
  }

  innerListDocuments(w, r, readSqlStmt, totalSqlStmt, "search-list")
  return
}

func dateLists(w http.ResponseWriter, r *http.Request) {
  useridUint64, err := GetCurrentUser(r)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  vars := mux.Vars(r)
  ds := vars["document-structure"]

  detv, err := docExists(ds)
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  if detv == false {
    errorPage(w, fmt.Sprintf("The document structure %s does not exists.", ds))
    return
  }

  tv1, err := DoesCurrentUserHavePerm(r, ds, "read")
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  tv2, err := DoesCurrentUserHavePerm(r, ds, "read-only-created")
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  if ! tv1 && ! tv2 {
    errorPage(w, "You don't have the read permission for this document structure.")
    return
  }

  tblName, err := tableName(ds)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  var count uint64
  sqlStmt := fmt.Sprintf("select count(*) from `%s`", tblName)
  err = SQLDB.QueryRow(sqlStmt).Scan(&count)
  if count == 0 {
    errorPage(w, "There are no documents to display.")
    return
  }

  if tv1 {
    sqlStmt = fmt.Sprintf("select distinct date(created) as dc from `%s` order by dc desc", tblName)
  } else if tv2 {
    sqlStmt = fmt.Sprintf("select distinct date(created) as dc from `%s` where created_by = %d order by dc desc",
      tblName, useridUint64)
  }

  dates := make([]string, 0)
  var date string
  rows, err := SQLDB.Query(sqlStmt)
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  defer rows.Close()
  for rows.Next() {
    err := rows.Scan(&date)
    if err != nil {
      errorPage(w, err.Error())
      return
    }
    dates = append(dates, date)
  }
  if err = rows.Err(); err != nil {
    errorPage(w, err.Error())
    return
  }

  type DateAndCount struct {
    Date string
    Count uint64
  }
  dacs := make([]DateAndCount, 0)
  for _, date := range dates {
    var count uint64
    if tv1 {
      sqlStmt = fmt.Sprintf("select count(*) from `%s` where date(created) = ?", tblName)
    } else if tv2 {
      sqlStmt = fmt.Sprintf("select count(*) from `%s` where date(created) = ? and created_by = %d",
        tblName, useridUint64)
    }

    err = SQLDB.QueryRow(sqlStmt, date).Scan(&count)
    if err != nil {
      errorPage(w, err.Error())
      return
    }
    dacs = append(dacs, DateAndCount{date, count})
  }

  type Context struct {
    DACs []DateAndCount
    DocumentStructure string
  }

  ctx := Context{dacs, ds}
  tmpl := template.Must(template.ParseFiles(getBaseTemplate(), "qffiles/date-lists.html"))
  tmpl.Execute(w, ctx)
}


func dateList(w http.ResponseWriter, r *http.Request) {
  useridUint64, err := GetCurrentUser(r)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  vars := mux.Vars(r)
  ds := vars["document-structure"]
  date := vars["date"]


  tv1, err := DoesCurrentUserHavePerm(r, ds, "read")
  if err != nil {
    errorPage(w, err.Error())
  }
  tv2, err := DoesCurrentUserHavePerm(r, ds, "read-only-created")
  if err != nil {
    errorPage(w, err.Error())
  }

  tblName, err := tableName(ds)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  var readSqlStmt string
  var totalSqlStmt string

  if tv1 {
    readSqlStmt = fmt.Sprintf("select id from `%s` where date(created) = '%s' order by created desc limit ?, ?",
      tblName, html.EscapeString(date))
    totalSqlStmt = fmt.Sprintf("select count(*) from `%s` where date(created) = '%s'",
      tblName, html.EscapeString(date))
  } else if tv2 {
    readSqlStmt = fmt.Sprintf("select id from `%s` where date(created) = '%s' and created_by = %d order by created desc limit ?, ?",
      tblName, html.EscapeString(date), useridUint64)
    totalSqlStmt = fmt.Sprintf("select count(*) from `%s` where date(created) = '%s' and created_by = %d",
      tblName, html.EscapeString(date), useridUint64)
  }

  innerListDocuments(w, r, readSqlStmt, totalSqlStmt, "date-list")
  return
}
