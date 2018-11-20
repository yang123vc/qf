package qf

import (
  "net/http"
  // "fmt"
  "github.com/gorilla/mux"
  "fmt"
  "strings"
  "html/template"
  "path/filepath"
)


func editDocumentStructure(w http.ResponseWriter, r *http.Request) {
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

  dsList, err := GetDocumentStructureList()
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  type Context struct {
    DocumentStructure string
    DocumentStructures string
  }

  ctx := Context{ds, strings.Join(dsList, ",,,")}
  fullTemplatePath := filepath.Join(getProjectPath(), "templates/edit-document-structure.html")
  tmpl := template.Must(template.ParseFiles(getBaseTemplate(), fullTemplatePath))
  tmpl.Execute(w, ctx)
}


func changeDocumentStructureName(w http.ResponseWriter, r *http.Request) {
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

  sqlStmt := "update `qf_document_structures` set fullname= ? where fullname = ?"
  _, err = SQLDB.Exec(sqlStmt, r.FormValue("new-name"), ds)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  redirectURL := fmt.Sprintf("/view-document-structure/%s/", r.FormValue("new-name"))
  http.Redirect(w, r, redirectURL, 307)

}
