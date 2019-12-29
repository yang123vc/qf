package qf

import (
  "net/http"
  "html/template"
  "github.com/gorilla/mux"
  "fmt"
  "database/sql"
  "strings"
)


func newDSFromTemplate(w http.ResponseWriter, r *http.Request) {
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

  docDatas, err := GetDocData(ds)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  var childTableStr string
  var helpText sql.NullString
  err = SQLDB.QueryRow("select child_table, help_text from qf_document_structures where fullname = ?", ds).Scan(&childTableStr, &helpText)
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  var childTableBool bool
  if childTableStr == "t" {
    childTableBool = true
  } else {
    childTableBool = false
  }
  var helpTextStr string
  if helpText.Valid {
    helpTextStr = helpText.String
  }

  var ctdsl sql.NullString
  err = SQLDB.QueryRow("select group_concat(fullname separator ',,,') from qf_document_structures where child_table = 't'").Scan(&ctdsl)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  dsList, err := GetDocumentStructureList()
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  type Context struct {
    DocDatas []DocData
    DocumentStructure string
    Add func(x, y int) int
    IsChildTable bool
    HelpText string
    FormatOtherOptions func([]string) string
    ChildTableDocumentStructures string
    DocumentStructures string
  }

  add := func(x, y int) int {
    return x + y
  }

  ffunc := func(x []string) string {
    return strings.Join(x, "\n")
  }

  ctx := Context{docDatas, ds, add, childTableBool, helpTextStr, ffunc, ctdsl.String, strings.Join(dsList, ",,,")}
  tmpl := template.Must(template.ParseFiles(getBaseTemplate(), "qffiles/new-ds-from-template.html"))
  tmpl.Execute(w, ctx)
}
