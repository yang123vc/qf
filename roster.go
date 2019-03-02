package qf

import (
  "net/http"
  "html/template"
  "database/sql"
  "fmt"
)


func newRoster(w http.ResponseWriter, r *http.Request) {
  truthValue, err := isUserAdmin(r)
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  if ! truthValue {
    errorPage(w, "You are not an admin here. You don't have permissions to view this page.")
    return
  }

  dsList, err := GetDocumentStructureList()
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  var rosterListStr sql.NullString
  err = SQLDB.QueryRow("select group_concat(name separator ',,,') from qf_roster").Scan(&rosterListStr)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  if r.Method == http.MethodGet {
    type Context struct {
      DocumentStructureList []string
      RosterListStr string
    }
    ctx := Context{dsList, rosterListStr.String}
    tmpl := template.Must(template.ParseFiles(getBaseTemplate(), "qffiles/new-roster.html"))
    tmpl.Execute(w, ctx)

  } else {

    dsid, err := getDocumentStructureID(r.FormValue("document_structure"))
    if err != nil {
      errorPage(w, err.Error())
      return
    }

    rstn, err := newRosterSheetTableName()
    if err != nil {
      errorPage(w, err.Error())
      return
    }

    _, err = SQLDB.Exec("insert into qf_roster(name, dsid, description, frequency, tbl_name) values(?,?,?,?,?)",
      r.FormValue("roster_name"), dsid, r.FormValue("roster_description"), r.FormValue("frequency"), rstn)
    if err != nil {
      errorPage(w, err.Error())
      return
    }

    sqlStmt := fmt.Sprintf("create table `%s` ( id bigint unsigned not null auto_increment, ", rstn)
    sqlStmt += "created datetime not null,"
    sqlStmt += "created_by bigint unsigned not null,"
    sqlStmt += "docid bigint unsigned not null,"
    sqlStmt += "status varchar(10) not null default 'undone',"
    sqlStmt += "comment text, primary key (id), unique(docid),"
    sqlStmt += fmt.Sprintf("foreign key (created_by) references `%s`(id),", UsersTable)

    tblName, err := tableName(r.FormValue("document_structure"))
    if err != nil {
      errorPage(w, err.Error())
      return
    }

    sqlStmt += fmt.Sprintf("foreign key (docid) references `%s`(id) )", tblName)

    _, err = SQLDB.Exec(sqlStmt)
    if err != nil {
      errorPage(w, err.Error())
      return
    }

    redirectURL := fmt.Sprintf("/view-roster/%s/", r.FormValue("roster_name"))
    http.Redirect(w, r, redirectURL, 307)
  }

}
