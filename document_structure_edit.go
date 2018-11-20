package qf

import (
  "net/http"
  // "fmt"
  "github.com/gorilla/mux"
  "fmt"
  "strings"
  "html/template"
  "path/filepath"
  "strconv"
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
    OldLabels []string
    NumberofFields int
    OldLabelsStr string
  }

  dsid, err := getDocumentStructureID(ds)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  var labels string
  err = SQLDB.QueryRow("select group_concat(label separator ',,,') from qf_fields where dsid = ?", dsid).Scan(&labels)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  labelsList := strings.Split(labels, ",,,")
  ctx := Context{ds, strings.Join(dsList, ",,,"), labelsList, len(labelsList), labels}
  fullTemplatePath := filepath.Join(getProjectPath(), "templates/edit-document-structure.html")
  tmpl := template.Must(template.ParseFiles(getBaseTemplate(), fullTemplatePath))
  tmpl.Execute(w, ctx)
}


func updateDocumentStructureName(w http.ResponseWriter, r *http.Request) {
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


func updateFieldLabels(w http.ResponseWriter, r *http.Request) {
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
    errorPage(w, fmt.Sprintf("The document structure %s does not exist.", ds))
    return
  }

  r.ParseForm()
  updateData := make(map[string]string)
  for i := 1; i < 100; i++ {
    p := strconv.Itoa(i)
    if r.FormValue("old-field-label-" + p) == "" {
      break
    } else {
      updateData[ r.FormValue("old-field-label-" + p) ] = r.FormValue("new-field-label-" + p)
    }
  }

  dsid, err := getDocumentStructureID(ds)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  for old, new := range updateData {
    sqlStmt := "update `qf_fields` set label = ? where dsid=? and label = ?"
    _, err = SQLDB.Exec(sqlStmt, new, dsid, old)
    if err != nil {
      errorPage(w, err.Error())
      return
    }
  }

  redirectURL := fmt.Sprintf("/view-document-structure/%s/", ds)
  http.Redirect(w, r, redirectURL, 307)
}
