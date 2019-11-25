package qf

import (
  "net/http"
  "html/template"
  "github.com/gorilla/mux"
  "strings"
  "database/sql"
  "strconv"
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

  var dslStr sql.NullString
  err = SQLDB.QueryRow("select group_concat(fullname separator ',,,') from qf_document_structures where child_table = 'f'").Scan(&dslStr)
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  var dsList []string
  if dslStr.Valid {
    dsList = strings.Split(dslStr.String, ",,,")
  } else {
    dsList = make([]string, 0)
  }

  roles, err := GetRoles()
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  if r.Method == http.MethodGet {
    type Context struct {
      DocumentStructureList []string
      Roles []string
    }
    ctx := Context{dsList, roles}

    tmpl := template.Must(template.ParseFiles(getBaseTemplate(), "qffiles/create-button.html"))
    tmpl.Execute(w, ctx)


  } else {
    ds := r.FormValue("ds")
    dsid, err := getDocumentStructureID(ds)
    if err != nil {
      errorPage(w, err.Error())
      return
    }

    res, err := SQLDB.Exec("insert into qf_buttons (name, dsid, url_prefix) values (?,?,?)",
      r.FormValue("button_name"), dsid, r.FormValue("url_prefix"))
    if err != nil {
      errorPage(w, err.Error())
      return
    }

    btnid, err := res.LastInsertId()
    if err != nil {
      errorPage(w, err.Error())
      return
    }

    execRoles := make([]string, 0)
    for i := 1; i < 100 ; i++ {
      iStr := strconv.Itoa(i)
      if r.FormValue("role-" + iStr) == "" {
        break
      } else {
        execRoles = append(execRoles, r.FormValue("role-" + iStr))
      }
    }

    for _, r := range execRoles {
      roleid, err := getRoleId(r)
      if err != nil {
        errorPage(w, err.Error())
        return
      }
      _, err = SQLDB.Exec("insert into qf_btns_and_roles(roleid, buttonid) values(?,?)", roleid, btnid)
      if err != nil {
        errorPage(w, err.Error())
        return
      }
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
    Roles []string
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

    var roleIdsStr sql.NullString
    qStmt := `select group_concat(qf_roles.role separator ',,,') from qf_btns_and_roles inner join qf_roles
    on qf_btns_and_roles.roleid = qf_roles.id where qf_btns_and_roles.buttonid = ?`
    err = SQLDB.QueryRow(qStmt, buttonId).Scan(&roleIdsStr)
    if err != nil {
      errorPage(w, err.Error())
      return
    }

    qfbs = append(qfbs, QFButton{buttonId, name, dsName, urlPrefix, strings.Split(roleIdsStr.String, ",,,")})
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
  tmpl := template.Must(template.ParseFiles(getBaseTemplate(), "qffiles/list-buttons.html"))
  tmpl.Execute(w, ctx)
}


func deleteButton(w http.ResponseWriter, r *http.Request) {
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
  bid := vars["id"]

  _, err = SQLDB.Exec("delete from qf_buttons where id = ?", bid)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  http.Redirect(w, r, "/list-buttons/", 307)
}
