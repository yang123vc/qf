package qf

import (
	"net/http"
	"github.com/gorilla/mux"
	"fmt"
)


func makePublic(w http.ResponseWriter, r *http.Request) {
  truthValue, err := isUserAdmin(r)
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  if ! truthValue {
    errorPage(w, "You are not an admin here. You don't have permissions to view this page.")
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

  qStmt := "update qf_document_structures set public = 't' where fullname = ?"
  _, err = SQLDB.Exec(qStmt, ds)
  if err != nil {
  	errorPage(w, err.Error())
  	return
  }
  
  http.Redirect(w, r, fmt.Sprintf("/view-document-structure/%s/", ds), 307)
}


func undoMakePublic(w http.ResponseWriter, r *http.Request) {
	truthValue, err := isUserAdmin(r)
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  if ! truthValue {
    errorPage(w, "You are not an admin here. You don't have permissions to view this page.")
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

  qStmt := "update qf_document_structures set public = 'f' where fullname = ?"
  _, err = SQLDB.Exec(qStmt, ds)
  if err != nil {
  	errorPage(w, err.Error())
  	return
  }
  
  http.Redirect(w, r, fmt.Sprintf("/view-document-structure/%s/", ds), 307)
}


func publicState(documentStructure string) (bool, error) {
	qStmt := "select public from qf_document_structures where fullname = ?"
	var stateStr string
	err := SQLDB.QueryRow(qStmt, documentStructure).Scan(&stateStr)
	if err != nil {
		return false, err
	}
	return charToBool(stateStr), nil
}
