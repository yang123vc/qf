package qf

import (
  "net/http"
  "html/template"
  "database/sql"
  "fmt"
  "github.com/gorilla/mux"
  "time"
)


var MYSQL_FORMAT = "2006-01-02 15:04:05"


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
    sqlStmt += "comment text, primary key (id),"
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


func fillRoster(w http.ResponseWriter, r *http.Request) {
  useridUint64, err := GetCurrentUser(r)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  vars := mux.Vars(r)
  roster_name := vars["roster"]

  retv, err := rosterExists(roster_name)
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  if retv == false {
    errorPage(w, fmt.Sprintf("The roster %s does not exists.", roster_name))
    return
  }

  sqlStmt := "select qf_document_structures.fullname, qf_roster.sheet_tbl, qf_roster.details_tbl, qf_roster.frequency "
  sqlStmt += "from qf_document_structures inner join qf_roster on "
  sqlStmt += "qf_document_structures.id = qf_roster.dsid where qf_roster.name = ?"
  var ds, sheetTable, detailsTable, frequency string
  err = SQLDB.QueryRow(sqlStmt, roster_name).Scan(&ds, &sheetTable, &detailsTable, &frequency)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  truthValue, err := DoesCurrentUserHavePerm(r, ds, "create")
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  if ! truthValue {
    errorPage(w, fmt.Sprintf("You don't have the create permission for the underlying document structure of this roster: %s.", roster_name))
    return
  }

  var endPeriod, startPeriod time.Time
  todaysDate := time.Now()
  if frequency == "daily" {
    startPeriod = time.Date( todaysDate.Year(), todaysDate.Month(), todaysDate.Day() ,0,0,0,0, todaysDate.Location())
    endPeriod = time.Date( todaysDate.Year(), todaysDate.Month(), todaysDate.Day(), 23, 59, 59, 0, todaysDate.Location())
  } else if frequency == "weekly" {
    tmpPeriod := todaysDate
    for tmpPeriod.Weekday() != time.Monday {
      tmpPeriod = tmpPeriod.AddDate(0, 0, -1)
    }
    startPeriod = time.Date( tmpPeriod.Year(), tmpPeriod.Month(), tmpPeriod.Day(), 0, 0, 0, 0, todaysDate.Location())

    tmpPeriod = todaysDate
    for tmpPeriod.Weekday() != time.Sunday {
      tmpPeriod = tmpPeriod.AddDate(0, 0, 1)
    }
    endPeriod = time.Date( tmpPeriod.Year(), tmpPeriod.Month(), tmpPeriod.Day(), 23, 59, 59, 0, tmpPeriod.Location())
  } else if frequency == "monthly" {
    startPeriod = time.Date( todaysDate.Year(), todaysDate.Month(), 1, 0, 0, 0, 0, todaysDate.Location())
    tmpPeriod := startPeriod.AddDate(0, 1, -1)
    endPeriod = time.Date( tmpPeriod.Year(), tmpPeriod.Month(), tmpPeriod.Day(), 23, 59, 59, 0, tmpPeriod.Location())
  }

  sqlStmt = fmt.Sprintf("select count(*) from %s where start_period = ? and end_period = ?", sheetTable)
  var count uint64
  err = SQLDB.QueryRow(sqlStmt, startPeriod, endPeriod).Scan(&count)
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  if count == 0 {
    sqlStmt = fmt.Sprintf("insert into %s(start_period, end_period) values(?, ?)", sheetTable)
    _, err = SQLDB.Exec(sqlStmt, startPeriod, endPeriod)
    if err != nil {
      errorPage(w, err.Error())
      return
    }
  }

  if r.Method == http.MethodGet {
    type Context struct {
      DocumentStructure string
      Roster string
      TodaysDate time.Time
      StartPeriod time.Time
      EndPeriod time.Time
    }

    ctx := Context{ds, roster_name, todaysDate, startPeriod, endPeriod}
    tmpl := template.Must(template.ParseFiles(getBaseTemplate(), "qffiles/fill-roster.html"))
    tmpl.Execute(w, ctx)
  } else {
    tblName, err := tableName(ds)
    if err != nil {
      errorPage(w, err.Error())
      return
    }

    docid := r.FormValue("docid")
    var count uint64
    sqlStmt = fmt.Sprintf("select count(*) from `%s` where id = %s", tblName, docid)
    err = SQLDB.QueryRow(sqlStmt).Scan(&count)
    if err != nil {
      errorPage(w, err.Error())
      return
    }
    if count == 0 {
      errorPage(w, fmt.Sprintf("The document with id %s do not exists", docid))
      return
    }

    sqlStmt = fmt.Sprintf("select count(*) from %s where docid = ? and created < ? and created > ?", detailsTable)
    err = SQLDB.QueryRow(sqlStmt, docid, endPeriod.Format(MYSQL_FORMAT), startPeriod.Format(MYSQL_FORMAT)).Scan(&count)
    if err != nil {
      errorPage(w, err.Error())
      return
    }
    if count != 0 {
      errorPage(w, "There is an entry for the current period.")
      return
    }

    sqlStmt = fmt.Sprintf("insert into %s(created, created_by, docid, status, comment) values(now(),?,?,?,?)", detailsTable)
    _, err = SQLDB.Exec(sqlStmt, useridUint64, docid, r.FormValue("status"), r.FormValue("comment"))
    if err != nil {
      errorPage(w, err.Error())
      return
    }

    redirectURL := fmt.Sprintf("/all-roster-fillings/%s/", roster_name)
    http.Redirect(w, r, redirectURL, 307)
  }
}


func allRosterFillings(w http.ResponseWriter, r *http.Request) {
  // useridUint64, err := GetCurrentUser(r)
  _, err := GetCurrentUser(r)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  vars := mux.Vars(r)
  roster_name := vars["roster"]

  retv, err := rosterExists(roster_name)
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  if retv == false {
    errorPage(w, fmt.Sprintf("The roster %s does not exists.", roster_name))
    return
  }

  var sheetTable string
  err = SQLDB.QueryRow("select sheet_tbl from qf_roster where name = ?", roster_name).Scan(&sheetTable)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  type Sheet struct {
    StartPeriod string
    EndPeriod string
  }
  sheets := make([]Sheet, 0)
  sqlStmt := fmt.Sprintf("select start_period, end_period from %s order by id desc", sheetTable)
  var startPeriod, endPeriod string
  rows, err := SQLDB.Query(sqlStmt)
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  defer rows.Close()
  for rows.Next() {
    err := rows.Scan(&startPeriod, &endPeriod)
    if err != nil {
      errorPage(w, err.Error())
      return
    }
    sheets = append(sheets, Sheet{startPeriod, endPeriod})
  }
  if err = rows.Err(); err != nil {
    errorPage(w, err.Error())
    return
  }

  type Context struct {
    Roster string
    Sheets []Sheet
  }

  ctx := Context{roster_name, sheets}
  tmpl := template.Must(template.ParseFiles(getBaseTemplate(), "qffiles/all-roster-fillings.html"))
  tmpl.Execute(w, ctx)
}
