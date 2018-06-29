package qf

import (
  "fmt"
  "net/http"
  "database/sql"
  _ "github.com/go-sql-driver/mysql"
)


var SQLDB *sql.DB
var SiteDB string
var UsersTable string
var Admins []uint64
var GetCurrentUser func(r *http.Request) (uint64, error)
var BaseTemplate string


func QFSetup(w http.ResponseWriter, r *http.Request) {
  if SQLDB == nil {
    fmt.Fprintf(w, "You have not set the \"qf.SQLDB\". Initialize a connection to the database and set the result to this value.")
    return
  } else {
    if err := SQLDB.Ping(); err != nil {
      fmt.Fprintf(w, "DB Ping failed. Exact Error: %s", err.Error())
      return
    }
  }

  if SiteDB == "" {
    fmt.Fprintf(w, "You have not set the \"qf.SiteDB\". Create your database for your site and set this to it.")
    return
  } else {
    var dbCount int
    err := SQLDB.QueryRow("select count(*) from information_schema.schemata where schema_name = ?", SiteDB).Scan(&dbCount)
    if err != nil {
      fmt.Fprintf(w, "Error checking if the database exists. Exact Error: %s", err.Error())
      return
    }
    if dbCount == 0 {
      fmt.Fprintf(w, "Your SiteDB \"%s\" does not exists.", SiteDB)
      return
    }
  }

  if UsersTable == "" {
    fmt.Fprintf(w, "You have not set the \"qf.UsersTable\". Create your users table and set this variable to it.")
    return
  } else {
    var tblCount int
    err := SQLDB.QueryRow("select count(*) from information_schema.tables where table_schema=? and table_name=?", SiteDB, UsersTable).Scan(&tblCount)
    if err != nil {
      fmt.Fprintf(w, "Error checking if the table exists. Exact Error: %s", err.Error())
      return
    }
    if tblCount == 0 {
      fmt.Fprintf(w, "Your UsersTable \"%s\" does not exists.", UsersTable)
      return
    }
  }

  if Admins == nil {
    fmt.Fprintf(w, "You have not set the \"qf.Admins\". Please set this to a list of ids (in uint64) of the Admins of this site.")
    return
  }

  if GetCurrentUser == nil {
    fmt.Fprintf(w, "You must set the \"qf.GetCurrentUser\". Please set this variable to a function with signature func(r *http.Request) (uint64, err).")
    return
  }

  var count int
  err := SQLDB.QueryRow(`select count(*) as count from information_schema.tables
  where table_schema=? and table_name=?`, SiteDB, "qf_document_structures").Scan(&count)
  if err != nil {
    fmt.Fprintf(w, "An error occured: %s", err.Error())
    return
  }

  if count == 1 {
    fmt.Fprintf(w, "This setup has been executed.")
    return

  } else {
    // do setup
    tx, err := SQLDB.Begin()
    if err != nil {
      fmt.Fprintf(w, "An error occured: %s", err.Error())
      return
    }
    // create forms general table
    _, err = tx.Exec(`create table qf_document_structures (
      id int not null auto_increment,
      name varchar(100) not null,
      approval_steps varchar(255),
      primary key (id),
      unique (name)
      )`)
    if err != nil {
      tx.Rollback()
      fmt.Fprintf(w, "An error occured: %s", err.Error())
      return
    }

    // create fields table
    _, err = tx.Exec(`create table qf_fields (
      id int not null auto_increment,
      dsid int not null,
      label varchar(100) not null,
      name varchar(100) not null,
      type varchar(100) not null,
      options varchar(255),
      other_options varchar(255),
      primary key (id),
      foreign key (dsid) references qf_document_structures(id),
      unique (dsid, name)
      )`)
    if err != nil {
      tx.Rollback()
      fmt.Fprintf(w, "An error occured: %s", err.Error())
      return
    }

    _, err = tx.Exec(`create table qf_roles (
      id int not null auto_increment,
      role varchar(50) not null,
      primary key(id),
      unique (role)
      )`)
    if err != nil {
      tx.Rollback()
      fmt.Fprintf(w, "An error occured: %s", err.Error())
      return
    }

    _, err = tx.Exec(`create table qf_permissions (
      id int not null auto_increment,
      roleid int not null,
      object varchar(255) not null,
      permissions varchar(255) not null,
      primary key (id),
      unique(roleid, object),
      foreign key (roleid) references qf_roles (id)
      )`)
    if err != nil {
      tx.Rollback()
      fmt.Fprintf(w, "An error occured: %s", err.Error())
      return
    }

    sqlStmt := "create table qf_user_roles ("
    sqlStmt += "id bigint unsigned not null auto_increment,"
    sqlStmt += "userid bigint unsigned not null,"
    sqlStmt += "roleid int not null, primary key(id), unique(userid, roleid),"
    sqlStmt += "foreign key (roleid) references qf_roles (id),"
    sqlStmt += fmt.Sprintf("foreign key (userid) references `%s`(id))", UsersTable)
    _, err = tx.Exec(sqlStmt)
    if err != nil {
      tx.Rollback()
      fmt.Fprintf(w, "An error occured: %s", err.Error())
      return
    }

    tx.Commit()
    fmt.Fprintf(w, "Setup Completed.")

  }
}
