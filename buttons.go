package qf

import (
  "net/http"
  "path/filepath"
  "html/template"
)


func createButton(w http.ResponseWriter, r *http.Request) {
  truthValue, err := isUserAdmin(r)
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  if ! truthValue {
    errorPage(w, "You are not an admin here. You don't have permissions to view this page.")
    return
  }

  ndsList, err := notAliasDocumentStructureList()
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  if r.Method == http.MethodGet {
    type Context struct {
      DocumentStructureList []string
    }
    ctx := Context{ndsList}

    fullTemplatePath := filepath.Join(getProjectPath(), "templates/create-button.html")
    tmpl := template.Must(template.ParseFiles(getBaseTemplate(), fullTemplatePath))
    tmpl.Execute(w, ctx)


  } else {
    ds := r.FormValue("ds")
    dsid, err := getDocumentStructureID(ds)
    if err != nil {
      errorPage(w, err.Error())
      return
    }

    _, err = SQLDB.Exec("insert into qf_buttons (name, dsid, url_prefix) values (?,?,?)",
      r.FormValue("button_name"), dsid, r.FormValue("url_prefix"))
    if err != nil {
      errorPage(w, err.Error())
      return
    }

    http.Redirect(w, r, "/list-buttons/", 307)
  }

}


func listButtons(w http.ResponseWriter, r *http.Request) {
  truthValue, err := isUserAdmin(r)
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  if ! truthValue {
    errorPage(w, "You are not an admin here. You don't have permissions to view this page.")
    return
  }

  type QFButton struct {
    ButtonId int
    Name string
    DocumentStructure string
    URLPrefix string
  }
  qfbs := make([]QFButton, 0)

  var (
    buttonId int
    name string
    dsid int
    urlPrefix string
  )
  rows, err := SQLDB.Query("select id, name, dsid, url_prefix from qf_buttons")
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  defer rows.Close()
  for rows.Next() {
    err := rows.Scan(&buttonId, &name, &dsid, &urlPrefix)
    if err != nil {
      errorPage(w, err.Error())
      return
    }

    var dsName string
    err = SQLDB.QueryRow("select fullname from qf_document_structures where id = ?", dsid).Scan(&dsName)
    if err != nil {
      errorPage(w, err.Error())
      return
    }
    qfbs = append(qfbs, QFButton{buttonId, name, dsName, urlPrefix})
  }
  err = rows.Err()
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  type Context struct {
    QFBS []QFButton
  }

  ctx := Context{qfbs}
  fullTemplatePath := filepath.Join(getProjectPath(), "templates/list-buttons.html")
  tmpl := template.Must(template.ParseFiles(getBaseTemplate(), fullTemplatePath))
  tmpl.Execute(w, ctx)
}
