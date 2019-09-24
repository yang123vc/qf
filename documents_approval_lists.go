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

  if ! tv1 && ! tv2 {
    errorPage(w, "You don't have the read permission for this document structure.")
    return
  }

  tblName, err := tableName(ds)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  var readSqlStmt string
  var totalSqlStmt string
  if tv1 {
    readSqlStmt = fmt.Sprintf("select * from `%s` where fully_approved = 't' ", tblName)
    totalSqlStmt = fmt.Sprintf("select count(*) from `%s` where fully_approved = 't' ", tblName)  
  } else if tv2 {
    readSqlStmt = fmt.Sprintf("select * from `%s` where fully_approved = 't' and created_by = %d ", tblName, useridUint64)
    totalSqlStmt = fmt.Sprintf("select count(*) from `%s` where fully_approved = 't' and created_by = %d ", tblName, useridUint64)  
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

  if ! tv1 && ! tv2 {
    errorPage(w, "You don't have the read permission for this document structure.")
    return
  }

  tblName, err := tableName(ds)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  var readSqlStmt string
  var totalSqlStmt string

  if tv1 {
    readSqlStmt = fmt.Sprintf("select * from `%s` where fully_approved = 'f' ", tblName)
    totalSqlStmt = fmt.Sprintf("select count(*) from `%s` where fully_approved = 'f' ", tblName)    
  } else {
    readSqlStmt = fmt.Sprintf("select * from `%s` where fully_approved = 'f' and created_by = %d ", tblName, useridUint64)
    totalSqlStmt = fmt.Sprintf("select count(*) from `%s` where fully_approved = 'f' and created_by = %d ", tblName, useridUint64)    
  }


  innerListDocuments(w, r, readSqlStmt, totalSqlStmt, "unapproved-list")
  return
}
