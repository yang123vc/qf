package qf

import (
  "net/http"
  "html/template"
  "database/sql"
  "fmt"
  "github.com/gorilla/mux"
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

    rstn, err := newRosterObjectTableName("sheet")
    if err != nil {
      errorPage(w, err.Error())
      return
    }

    rdtn, err := newRosterObjectTableName("details")
    if err != nil {
      errorPage(w, err.Error())
      return
    }

    _, err = SQLDB.Exec("insert into qf_roster(name, dsid, description, frequency, sheet_tbl,  details_tbl) values(?,?,?,?,?,?)",
      r.FormValue("roster_name"), dsid, r.FormValue("roster_description"), r.FormValue("frequency"), rstn, rdtn)
    if err != nil {
      errorPage(w, err.Error())
      return
    }

    sqlStmt := fmt.Sprintf("create table `%s` ( id bigint unsigned not null auto_increment, ", rdtn)
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

    sqlStmt = fmt.Sprintf("create table `%s` ( id bigint unsigned not null auto_increment, ", rstn)
    sqlStmt += "start_period datetime not null, "
    sqlStmt += "end_period datetime not null, "
    sqlStmt += "details text, "
    sqlStmt += "primary key (id), unique(start_period, end_period) )"

    _, err = SQLDB.Exec(sqlStmt)
    if err != nil {
      errorPage(w, err.Error())
      return
    }

    redirectURL := fmt.Sprintf("/view-roster/%s/", r.FormValue("roster_name"))
    http.Redirect(w, r, redirectURL, 307)
  }

}


func listRosters(w http.ResponseWriter, r *http.Request) {
  truthValue, err := isUserAdmin(r)
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  if ! truthValue {
    errorPage(w, "You are not an admin here. You don't have permissions to view this page.")
    return
  }

  rns := make([]string, 0)
  var name string

  rows, err := SQLDB.Query("select name from qf_roster order by name asc")
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  defer rows.Close()
  for rows.Next() {
    err := rows.Scan(&name)
    if err != nil {
      errorPage(w, err.Error())
      return
    }
    rns = append(rns, name)
  }
  err = rows.Err()
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  type Context struct {
    RosterNames []string
  }
  ctx := Context{rns}

  tmpl := template.Must(template.ParseFiles(getBaseTemplate(), "qffiles/list-rosters.html"))
  tmpl.Execute(w, ctx)
}


func viewRoster(w http.ResponseWriter, r *http.Request) {
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
  roster_name := vars["roster"]

  retv, err := rosterExists(roster_name)
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  if !retv {
    errorPage(w, "Roster with that name does not exist.")
    return
  }

  type Context struct {
    Name string
    Description string
    Frequency string
    SheetTable string
    DetailsTable string
  }

  var description, frequency, sheetTable, detailsTable string
  err = SQLDB.QueryRow("select description, frequency, sheet_tbl, details_tbl from qf_roster where name = ? ", roster_name).Scan(&description,
    &frequency, &sheetTable, &detailsTable)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  ctx := Context{roster_name, description, frequency, sheetTable, detailsTable}
  tmpl := template.Must(template.ParseFiles(getBaseTemplate(), "qffiles/view-roster.html"))
  tmpl.Execute(w, ctx)
}


func deleteRoster(w http.ResponseWriter, r *http.Request) {
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
  roster_name := vars["roster"]

  retv, err := rosterExists(roster_name)
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  if !retv {
    errorPage(w, "Roster with that name does not exist.")
    return
  }

  var sheetTable, detailsTable string
  err = SQLDB.QueryRow("select sheet_tbl, details_tbl from qf_roster where name = ? ", roster_name).Scan(&sheetTable, &detailsTable)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  _, err = SQLDB.Exec(fmt.Sprintf("drop table %s", sheetTable))
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  _, err = SQLDB.Exec(fmt.Sprintf("drop table %s", detailsTable))
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  _, err = SQLDB.Exec("delete from qf_roster where name = ?", roster_name)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  redirectURL := "/list-rosters/"
  http.Redirect(w, r, redirectURL, 307)
}
