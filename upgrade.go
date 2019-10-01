package qf

import (
  "net/http"
  "fmt"
)


func qfUpgrade(w http.ResponseWriter, r *http.Request) {
  truthValue, err := isUserAdmin(r)
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  if ! truthValue {
    errorPage(w, "You are not an admin here. You don't have permissions to view this page.")
    return
  }  

  var count int
  err = SQLDB.QueryRow(`select count(*) as count from information_schema.tables
  where table_schema=? and table_name=?`, SiteDB, "qf_document_structures").Scan(&count)
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  if count == 0 {
    errorPage(w, "Go to '/qf-setup/' for the setup. This page is for existing installations.")
    return
  }

  upgradeStmts := map[string][]string{
    "1.7.0": []string{
      `create table qf_files_for_delete (
      id bigint unsigned not null auto_increment,
      created_by bigint unsigned not null,
      filepath varchar(255) not null,
      primary key(id),
      index(filepath),
      index(created_by)
      )`,

      `update qf_version set version = '1.8.0' where id = 1;`,
    },
  }
  err = SQLDB.QueryRow(`select count(*) as count from information_schema.tables
  where table_schema=? and table_name=?`, SiteDB, "qf_version").Scan(&count)
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  if count == 0 {
    errorPage(w, "Upgrading a qf installation started from version 1.7.0. Sorry for any inconvenience.")
    return
  }

  runUpgradeQueries := func(stmts []string) error {
    for _, stmt := range stmts {
      _, err := SQLDB.Exec(stmt)
      if err != nil {
        return err
      }
    }
    return nil
  }

  var currentVersion string
  err = SQLDB.QueryRow("select version from qf_version where id = 1").Scan(&currentVersion)
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  if currentVersion == "1.7.0" {
    stmts := upgradeStmts[currentVersion]
    err = runUpgradeQueries(stmts)
    if err != nil {
      errorPage(w, err.Error())
      return
    }
  }

  fmt.Fprintf(w, "All done.")
}