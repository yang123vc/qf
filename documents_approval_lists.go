package qf

import (
  "fmt"
  "net/http"
  "github.com/gorilla/mux"
)


func approvedList(w http.ResponseWriter, r *http.Request) {
  useridUint64, err := GetCurrentUser(r)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  vars := mux.Vars(r)
  ds := vars["document-structure"]

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
    readSqlStmt = fmt.Sprintf("select id from `%s` where fully_approved = 't' order by created desc limit ?, ?", tblName)
    totalSqlStmt = fmt.Sprintf("select count(*) from `%s` where fully_approved = 't' ", tblName)
  } else if tv2 {
    readSqlStmt = fmt.Sprintf("select id from `%s` where created_by = %d and fully_approved = 't' order by created desc limit ?, ?", tblName, useridUint64 )
    totalSqlStmt = fmt.Sprintf("select count(*) from `%s` where created_by = %d and fully_approved = 't' ", tblName, useridUint64)
  }

  innerListDocuments(w, r, readSqlStmt, totalSqlStmt, "approved-list")
  return
}


func unapprovedList(w http.ResponseWriter, r *http.Request) {
  useridUint64, err := GetCurrentUser(r)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  vars := mux.Vars(r)
  ds := vars["document-structure"]

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
    readSqlStmt = fmt.Sprintf("select id from `%s` where fully_approved = 'f' order by created desc limit ?, ?", tblName)
    totalSqlStmt = fmt.Sprintf("select count(*) from `%s` where fully_approved = 'f' ", tblName)
  } else if tv2 {
    readSqlStmt = fmt.Sprintf("select id from `%s` where created_by = %d and fully_approved = 'f' order by created desc limit ?, ?", tblName, useridUint64 )
    totalSqlStmt = fmt.Sprintf("select count(*) from `%s` where created_by = %d and fully_approved = 'f' ", tblName, useridUint64)
  }

  innerListDocuments(w, r, readSqlStmt, totalSqlStmt, "unapproved-list")
  return
}
