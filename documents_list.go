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

  rperm, err := DoesCurrentUserHavePerm(r, ds, "read")
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  if ! rperm {
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
    cperm, err := DoesCurrentUserHavePerm(r, ds, "create")
    if err != nil {
      errorPage(w, err.Error())
      return
    }
    type Context struct {
      DocumentStructure string
      CreatePerm bool
      ListType string
    }
    tmpl := template.Must(template.ParseFiles(getBaseTemplate(), "qffiles/suggest-create-document.html"))
    tmpl.Execute(w, Context{ds, cperm, listType})
    return
  }

  var id int
  err = SQLDB.QueryRow("select id from qf_document_structures where fullname = ?", ds).Scan(&id)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  type ColLabel struct {
    Col string
    Label string
  }

  colNames := make([]ColLabel, 0)
  var dsid int
  isAlias, ptdsid, err := DSIdAliasPointsTo(ds)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  if isAlias {
    dsid = ptdsid
  } else {
    err := SQLDB.QueryRow("select id from qf_document_structures where fullname = ?", ds).Scan(&dsid)
    if err != nil {
      errorPage(w, err.Error())
      return
    }
  }

  var colName string
  var label string
  rows, err := SQLDB.Query(`select name, label from qf_fields where dsid = ? and  type != "Table"
    and type != "Section Break" and type != "File" and type != "Image" order by view_order asc limit 3`, dsid)
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  defer rows.Close()
  for rows.Next() {
    err := rows.Scan(&colName, &label)
    if err != nil {
      errorPage(w, err.Error())
      return
    }
    colNames = append(colNames, ColLabel{colName, label})
  }
  if err = rows.Err(); err != nil {
    errorPage(w, err.Error())
    return
  }
  colNames = append(colNames, ColLabel{"created", "Creation DateTime"}, ColLabel{"created_by", "Created By"})

  var itemsPerPage uint64 = 50
  startIndex := (pageI - 1) * itemsPerPage
  totalItems := count
  totalPages := math.Ceil( float64(totalItems) / float64(itemsPerPage) )


  if r.FormValue("order_by") != "" {
    // get db name of order_by
    var dbName string
    orderBy := html.EscapeString( r.FormValue("order_by") )
    if orderBy == "Created By" {
      dbName = "created_by"
    } else if orderBy == "Creation DateTime" {
      dbName = "created"
    } else if orderBy == "Modification DateTime" {
      dbName = "modified"
    } else {
      err = SQLDB.QueryRow("select name from qf_fields where dsid = ? and label = ?", dsid, orderBy).Scan(&dbName)
      if err != nil {
        errorPage(w, err.Error())
        return
      }
    }

    var direction string
    if r.FormValue("direction") == "Ascending" {
      direction = "asc"
    } else {
      direction = "desc"
    }

    readSqlStmt += fmt.Sprintf(" order by %s %s limit ?, ?", dbName, direction)
  } else {
    readSqlStmt += " order by created desc limit ?, ?"
  }

  uocPerm, err1 := DoesCurrentUserHavePerm(r, ds, "update-only-created")
  docPerm, err2 := DoesCurrentUserHavePerm(r, ds, "delete-only-created")
  if err1 != nil || err2 != nil {
    errorPage(w, "Error occured while determining if the user have read permission for this page.")
    return
  }


  type ColAndData struct {
    ColName string
    Data string
  }

  type Row struct {
    Id uint64
    ColAndDatas []ColAndData
    RowUpdatePerm bool
    RowDeletePerm bool
  }

  myRows := make([]Row, 0)

  rows, err = SQLDB.Query(readSqlStmt, startIndex, itemsPerPage)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  columns, err := rows.Columns()
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  // Make a slice for the values
  values := make([]sql.RawBytes, len(columns))

  // rows.Scan wants '[]interface{}' as an argument, so we must copy the
  // references into such a slice
  // See http://code.google.com/p/go-wiki/wiki/InterfaceSlice for details
  scanArgs := make([]interface{}, len(values))
  for i := range values {
    scanArgs[i] = &values[i]
  }

  allRowsMap := make([]map[string]string, 0)

  for rows.Next() {
    err = rows.Scan(scanArgs...)
    if err != nil {
      errorPage(w, err.Error())
      return
    }

    rowMap := make(map[string]string)
    var value string
    for i, col := range values {
      if col == nil {
        value = ""
      } else {
        value = html.UnescapeString(string(col))
      }
      rowMap[ columns[i] ] = value
    }

    allRowsMap = append(allRowsMap, rowMap)
  }
  if err = rows.Err(); err != nil {
    errorPage(w, err.Error())
    return
  }

  for _, rowMapItem := range allRowsMap {
    colAndDatas := make([]ColAndData, 0)
    for _, colLabel := range colNames {
      data := rowMapItem[ colLabel.Col ]
      colAndDatas = append(colAndDatas, ColAndData{colLabel.Label, data})
    }

    rup := false
    rdp := false
    createdBy, err := strconv.ParseUint(rowMapItem["created_by"], 10, 64)
    if err != nil {
      errorPage(w, err.Error())
      return
    }
    if createdBy == useridUint64 && uocPerm {
      rup = true
    }
    if createdBy == useridUint64 && docPerm {
      rdp = true
    }
    rid, err := strconv.ParseUint(rowMapItem["id"], 10, 64)
    if err != nil {
      errorPage(w, err.Error())
      return
    }
    myRows = append(myRows, Row{rid, colAndDatas, rup, rdp})

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
    OrderColumns []string
  }

  pages := make([]uint64, 0)
  for i := uint64(0); i < uint64(totalPages); i++ {
    pages = append(pages, i+1)
  }

  tv1, err1 := DoesCurrentUserHavePerm(r, ds, "create")
  tv2, err2 := DoesCurrentUserHavePerm(r, ds, "update")
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

  var allColumnLabels sql.NullString
  err = SQLDB.QueryRow(`select group_concat(label separator ",,,") from qf_fields where dsid = ? and  type != "Table"
    and type != "Section Break" and type != "File" and type != "Image" order by view_order asc`, dsid).Scan(&allColumnLabels)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  ctx := Context{ds, colNamesList, myRows, pageI, pages, tv1, tv2, tv3, hasApprovals,
    approver, listType, date, strings.Split(allColumnLabels.String, ",,,")}
  tmpl := template.Must(template.ParseFiles(getBaseTemplate(), "qffiles/list-documents.html"))
  tmpl.Execute(w, ctx)
}


func listDocuments(w http.ResponseWriter, r *http.Request) {
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
  if ! tv1 {
    errorPage(w, "You don't have read permission for this document structure.")
    return
  }

  tblName, err := tableName(ds)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  readSqlStmt := fmt.Sprintf("select * from `%s` ", tblName)
  totalSqlStmt := fmt.Sprintf("select count(*) from `%s`", tblName)
  innerListDocuments(w, r, readSqlStmt, totalSqlStmt, "true-list")
  return
}


func searchDocuments(w http.ResponseWriter, r *http.Request) {
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

  if ! tv1 {
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
  }
  ctx := Context{ds, dds}
  tmpl := template.Must(template.ParseFiles(getBaseTemplate(), "qffiles/search-documents.html"))
  tmpl.Execute(w, ctx)
}


func parseSearchVariables(r *http.Request) ([]string, error) {
  vars := mux.Vars(r)
  ds := vars["document-structure"]

  dds, err := GetDocData(ds)
  if err != nil {
    return nil, err
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

  if r.FormValue("created_by") != "" {
    endSqlStmt = append(endSqlStmt, "created_by = " + html.EscapeString(r.FormValue("created_by")))
  }
  if r.FormValue("creation-year") != "" {
    data := fmt.Sprintf("\"%s\"", html.EscapeString(r.FormValue("creation-year")))
    endSqlStmt = append(endSqlStmt, "extract(year from created) = " + data)
  }
  if r.FormValue("creation-month") != "" {
    data := fmt.Sprintf("\"%s\"", html.EscapeString(r.FormValue("creation-month")))
    endSqlStmt = append(endSqlStmt, "extract(month from created) = " + data)
  }

  return endSqlStmt, nil
}


func searchResults(w http.ResponseWriter, r *http.Request) {
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
  if ! tv1 {
    errorPage(w, "You don't have the read permission for this document structure.")
    return
  }

  endSqlStmt, err := parseSearchVariables(r)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  if len(endSqlStmt) == 0 {
    errorPage(w, "Your query is empty.")
    return
  }

  tblName, err := tableName(ds)
  if err != nil {
    errorPage(w, err.Error())
    return
  }


  readSqlStmt := fmt.Sprintf("select * from `%s` where ", tblName) + strings.Join(endSqlStmt, " and ")
  // readSqlStmt += " order by created desc limit ?, ?"
  totalSqlStmt := fmt.Sprintf("select count(*) from `%s` where ", tblName) + strings.Join(endSqlStmt, " and ")

  innerListDocuments(w, r, readSqlStmt, totalSqlStmt, "search-list")
  return
}


func dateLists(w http.ResponseWriter, r *http.Request) {
  _, err := GetCurrentUser(r)
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
  if ! tv1 {
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
    cperm, err := DoesCurrentUserHavePerm(r, ds, "create")
    if err != nil {
      errorPage(w, err.Error())
      return
    }
    type Context struct {
      DocumentStructure string
      CreatePerm bool
    }
    tmpl := template.Must(template.ParseFiles(getBaseTemplate(), "qffiles/suggest-create-document.html"))
    tmpl.Execute(w, Context{ds, cperm})
    return
  }

  var itemsPerPage uint64 = 50
  startIndex := (pageI - 1) * itemsPerPage
  totalItems := count
  totalPages := math.Ceil( float64(totalItems) / float64(itemsPerPage) )

  sqlStmt = fmt.Sprintf("select distinct date(created) as dc from `%s` order by dc desc limit ?, ?", tblName)

  dates := make([]string, 0)
  var date string
  rows, err := SQLDB.Query(sqlStmt, startIndex, itemsPerPage)
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
    sqlStmt = fmt.Sprintf("select count(*) from `%s` where date(created) = ?", tblName)

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
    CurrentPage uint64
    Pages []uint64
  }

  pages := make([]uint64, 0)
  for i := uint64(0); i < uint64(totalPages); i++ {
    pages = append(pages, i+1)
  }

  ctx := Context{dacs, ds, pageI, pages}
  tmpl := template.Must(template.ParseFiles(getBaseTemplate(), "qffiles/date-lists.html"))
  tmpl.Execute(w, ctx)
}


func dateList(w http.ResponseWriter, r *http.Request) {
  vars := mux.Vars(r)
  ds := vars["document-structure"]
  date := vars["date"]

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
  if ! tv1 {
    errorPage(w, "You don't have the read permission for this document structure.")
    return
  }

  tblName, err := tableName(ds)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  readSqlStmt := fmt.Sprintf("select * from `%s` where date(created) = '%s' ",
    tblName, html.EscapeString(date))
  totalSqlStmt := fmt.Sprintf("select count(*) from `%s` where date(created) = '%s'",
    tblName, html.EscapeString(date))

  innerListDocuments(w, r, readSqlStmt, totalSqlStmt, "date-list")
  return
}
