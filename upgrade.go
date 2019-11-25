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

  // make sure version update in the database always comes last.
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

    "1.8.0": []string {
      `alter table qf_document_structures add (public char(1) default 'f');`,
      `update qf_version set version = '1.9.0' where id = 1;`,
    },

    "1.9.0": []string {
      `
      create table qf_btns_and_roles (
        id int not null auto_increment,
        roleid int not null,
        buttonid int not null,
        primary key (id),
        unique(roleid, buttonid),
        foreign key (roleid) references qf_roles (id),
        foreign key (buttonid) references qf_buttons (id)
        )
      `,
      `update qf_version set version = '1.10.0' where id = 1;`,
    },
  }

  upgradeProgression := []string{"1.7.0", "1.8.0", "1.9.0",}

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

  var currentVersion string
  err = SQLDB.QueryRow("select version from qf_version where id = 1").Scan(&currentVersion)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  startIndex := 0
  for i, ver := range upgradeProgression {
    if ver == currentVersion {
      startIndex = i
    }
  }

  stmts := make([]string, 0)

  for i, ver := range upgradeProgression[startIndex: ] {
    if i == len(upgradeProgression[startIndex: ]) -1 {
      stmts = append(stmts, upgradeStmts[ver]...)
    } else {
      stmts = append(stmts, upgradeStmts[ver][: len(upgradeStmts[ver]) - 1]...)
    }
  }

  for _, stmt := range stmts {
    _, err := SQLDB.Exec(stmt)
    if err != nil {
      errorPage(w, err.Error())
      return
    }
  }

  fmt.Fprintf(w, "All done.")
}
