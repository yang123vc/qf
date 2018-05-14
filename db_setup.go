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
      is_child_table char(1),
      is_singleton char(1),
      primary key (id),
      unique (doc_name)
      )`)
    if err != nil {
      panic(err)
    }

    // create fields table
    _, err = SQLDB.Exec(`create table qf_fields (
      id int not null auto_increment,
      field_label varchar(100) not null,
      field_name varchar(100) not null,
      field_type varchar(100) not null,
      field_options varchar(255),
      default_value varchar(255),
      field_other_options varchar(255),
      primary key (id)
      )`)
    if err != nil {
      panic(err)
    }

  }
}
