package qf

import (
  "fmt"
  "net/http"
  "github.com/gorilla/mux"
)


func approvedList(w http.ResponseWriter, r *http.Request) {
  useridUint64, err := GetCurrentUser(r)
  if err != nil {
    errorPage(w, "You need to be logged in to continue.", err)
    return
  }

  vars := mux.Vars(r)
  ds := vars["document-structure"]

  tv1, err := DoesCurrentUserHavePerm(r, ds, "read")
  if err != nil {
    errorPage(w, "Error reading permissions.", err)
  }
  tv2, err := DoesCurrentUserHavePerm(r, ds, "read-only-created")
  if err != nil {
    errorPage(w, "Error reading permissions.", err)
  }
  tv3, err := DoesCurrentUserHavePerm(r, ds, "read-only-mentioned")
  if err != nil {
    errorPage(w, "Error reading permissions.", err)
  }

  tblName, err := tableName(ds)
  if err != nil {
    errorPage(w, "Error getting document structure's table name.", err)
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
  } else if tv3 {
    muColumn, err := getMentionedUserColumn(ds)
    if err != nil {
      errorPage(w, "Error getting MentionedUser column.", err)
      return
    }
    readSqlStmt = fmt.Sprintf("select id from `%s` where %s = %d and fully_approved = 't' order by created desc limit ?, ?",
      tblName, muColumn, useridUint64 )
    totalSqlStmt = fmt.Sprintf("select count(*) from `%s` where %s = %d and fully_approved = 't' ", tblName, muColumn, useridUint64)
  }

  innerListDocuments(w, r, readSqlStmt, totalSqlStmt, "approved-list")
  return
}
