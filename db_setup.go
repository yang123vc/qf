package qf

import (
  "os"
  "database/sql"
  _ "github.com/go-sql-driver/mysql"
)

var SQLDB *sql.DB

func DBSetup() {
  var count int
  err := SQLDB.QueryRow(`select count(*) as count from information_schema.tables
  where table_schema=? and table_name=?`, os.Getenv("QF_MYSQL_DATABASE"), "qf_forms").Scan(&count)
  if err != nil {
    panic(err)
  }
  if count == 1 {
    return
  } else { // do setup

    // create forms general table
    _, err := SQLDB.Exec(`create table qf_forms (
      id int not null auto_increment,
      doc_name varchar(100) not null,
      child_table char(1),
      singleton char(1),
      primary key (id),
      unique (doc_name)
      )`)
    if err != nil {
      panic(err)
    }

    // create fields table
    _, err = SQLDB.Exec(`create table qf_fields (
      id int not null auto_increment,
      formid int not null,
      label varchar(100) not null,
      name varchar(100) not null,
      type varchar(100) not null,
      options varchar(255),
      other_options varchar(255),
      primary key (id),
      foreign key (formid) references qf_forms(id),
      unique (formid, name)
      )`)
    if err != nil {
      panic(err)
    }

  }
}
