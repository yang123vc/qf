package qf

import (
  "net/http"
  "html/template"
  "database/sql"
  "fmt"
)

func newDSGroup(w http.ResponseWriter, r *http.Request) {
  truthValue, err := isUserAdmin(r)
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  if ! truthValue {
    errorPage(w, "You are not an admin here. You don't have permissions to view this page.")
    return
  }

  if r.Method == http.MethodGet {
    type Context struct {
      DSGroups string
    }
    var dsgl sql.NullString
    err = SQLDB.QueryRow("select group_concat(group_name separator ',,,') from qf_dsgroups").Scan(&dsgl)
    if err != nil {
      errorPage(w, err.Error())
      return
    }

    tmpl := template.Must(template.ParseFiles(getBaseTemplate(), "qffiles/new-dsgroup.html"))
    tmpl.Execute(w, Context{dsgl.String})
  } else {
    dsGroupName := r.FormValue("group-name")
    _, err = SQLDB.Exec("insert into qf_dsgroups (group_name) values (?)", dsGroupName)
    if err != nil {
      errorPage(w, err.Error())
      return
    }

    redirectURL := fmt.Sprintf("/compose-dsgroup/%s/", dsGroupName)
    http.Redirect(w, r, redirectURL, 307)
  }
}
