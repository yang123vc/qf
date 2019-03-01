package qf

import (
  "fmt"
  "net/http"
  "github.com/gorilla/mux"
)


func approvedList(w http.ResponseWriter, r *http.Request) {
  _, err := GetCurrentUser(r)
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
  if ! tv1 {
    errorPage(w, "You don't have the read permission for this document structure.")
    return
  }

  tblName, err := tableName(ds)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  readSqlStmt := fmt.Sprintf("select id from `%s` where fully_approved = 't' order by created desc limit ?, ?", tblName)
  totalSqlStmt := fmt.Sprintf("select count(*) from `%s` where fully_approved = 't' ", tblName)

  innerListDocuments(w, r, readSqlStmt, totalSqlStmt, "approved-list")
  return
}


func unapprovedList(w http.ResponseWriter, r *http.Request) {
  _, err := GetCurrentUser(r)
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
  if ! tv1 {
    errorPage(w, "You don't have the read permission for this document structure.")
    return
  }

  tblName, err := tableName(ds)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  readSqlStmt := fmt.Sprintf("select id from `%s` where fully_approved = 'f' order by created desc limit ?, ?", tblName)
  totalSqlStmt := fmt.Sprintf("select count(*) from `%s` where fully_approved = 'f' ", tblName)

  innerListDocuments(w, r, readSqlStmt, totalSqlStmt, "unapproved-list")
  return
}
