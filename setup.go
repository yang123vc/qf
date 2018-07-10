package qf

import (
  "fmt"
  "net/http"
  "database/sql"
  _ "github.com/go-sql-driver/mysql"
  "github.com/gorilla/mux"
)


var SQLDB *sql.DB
var SiteDB string
var UsersTable string
var Admins []uint64
var GetCurrentUser func(r *http.Request) (uint64, error)
var BaseTemplate string

type ExtraCode struct {
  DSNo int
  ValidationFn func(jsonData string) string
  AfterCreateFn func(id uint64)
  AfterUpdateFn func(id uint64)
  AfterDeleteFn func(jsonData string)
}

var ExtraCodeList []ExtraCode

func qfSetup(w http.ResponseWriter, r *http.Request) {
  if SQLDB == nil {
    errorPage(w, r, "You have not set the \"qf.SQLDB\". Initialize a connection to the database and set the result to this value.", nil)
    return
  } else {
    if err := SQLDB.Ping(); err != nil {
      errorPage(w, r, "DB Ping failed.", err)
      return
    }
  }

  if SiteDB == "" {
    errorPage(w, r, "You have not set the \"qf.SiteDB\". Create your database for your site and set this to it.", nil)
    return
  } else {
    var dbCount int
    err := SQLDB.QueryRow("select count(*) from information_schema.schemata where schema_name = ?", SiteDB).Scan(&dbCount)
    if err != nil {
      errorPage(w, r, "Error checking if the database exists. ", err)
      return
    }
    if dbCount == 0 {
      errorPage(w, r, fmt.Sprintf("Your SiteDB \"%s\" does not exists.", SiteDB), nil)
      return
    }
  }

  if UsersTable == "" {
    errorPage(w, r, "You have not set the \"qf.UsersTable\". Create your users table and set this variable to it.", nil)
    return
  } else {
    var tblCount int
    err := SQLDB.QueryRow("select count(*) from information_schema.tables where table_schema=? and table_name=?", SiteDB, UsersTable).Scan(&tblCount)
    if err != nil {
      errorPage(w, r, "Error checking if the table exists. ", err)
      return
    }
    if tblCount == 0 {
      errorPage(w, r, fmt.Sprintf("Your UsersTable \"%s\" does not exists.", UsersTable), nil)
      return
    }
  }

  if Admins == nil {
    errorPage(w, r, "You have not set the \"qf.Admins\". Please set this to a list of ids (in uint64) of the Admins of this site.", nil)
    return
  }

  if GetCurrentUser == nil {
    errorPage(w, r, "You must set the \"qf.GetCurrentUser\". Please set this variable to a function with signature func(r *http.Request) (uint64, err).", nil)
    return
  }

  var count int
  err := SQLDB.QueryRow(`select count(*) as count from information_schema.tables
  where table_schema=? and table_name=?`, SiteDB, "qf_document_structures").Scan(&count)
  if err != nil {
    errorPage(w, r, "An internal error occured.", err)
    return
  }

  if count == 1 {
    errorPage(w, r, "This setup has been executed.", nil)
    return

  } else {
    // do setup
    tx, err := SQLDB.Begin()
    if err != nil {
      errorPage(w, r, "An internal error occured", err)
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
      errorPage(w, r, "An error occured while creating the document structures table.", err)
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
      errorPage(w, r, "An error occured while creating the fields table", err)
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
      errorPage(w, r, "An error occured while creating the roles table.", err)
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
      errorPage(w, r, "An error occured while creating permissions table.", err)
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
      errorPage(w, r, "An error occured while creating user to roles table.", err)
      return
    }

    tx.Commit()
    fmt.Fprintf(w, "Setup Completed.")

  }
}


func AddQFHandlers(r *mux.Router) {
  // Please don't change the paths.

  // Please call this link first to do your setup.
  r.HandleFunc("/qf-setup/", qfSetup)

  // r.HandleFunc("/jquery/", qf.JQuery)
  r.HandleFunc("/serve-js/{library}/", serveJS)

  // document structure links
  r.HandleFunc("/new-document-structure/", newDocumentStructure)
  r.HandleFunc("/list-document-structures/", listDocumentStructures)
  r.HandleFunc("/delete-document-structure/{document-structure}/", deleteDocumentStructure)
  r.HandleFunc("/view-document-structure/{document-structure}/", viewDocumentStructure)
  r.HandleFunc("/edit-document-structure-permissions/{document-structure}/", editDocumentStructurePermissions)

  // roles links
  r.HandleFunc("/roles-view/", rolesView)
  r.HandleFunc("/delete-role/{role}/", deleteRole)
  r.HandleFunc("/users-to-roles-list/", usersToRolesList)
  r.HandleFunc("/edit-user-roles/{userid}/", editUserRoles)
  r.HandleFunc("/remove-role-from-user/{userid}/{role}/", removeRoleFromUser)
  r.HandleFunc("/delete-role-permissions/{document-structure}/{role}/", deleteRolePermissions)

  // document links
  r.HandleFunc("/doc/{document-structure}/create/", createDocument)
  r.HandleFunc("/doc/{document-structure}/update/{id:[0-9]+}/", updateDocument)
  r.HandleFunc("/doc/{document-structure}/list/", listDocuments)
  r.HandleFunc("/doc/{document-structure}/list/{page:[0-9]+}/", listDocuments)
  r.HandleFunc("/doc/{document-structure}/delete/{id:[0-9]+}/", deleteDocument)
  r.HandleFunc("/doc/{document-structure}/search/", searchDocuments)
  r.HandleFunc("/doc/{document-structure}/date-lists/", dateLists)
  r.HandleFunc("/doc/{document-structure}/date-list/{date}/", dateList)
  r.HandleFunc("/doc/{document-structure}/date-list/{date}/{page:[0-9]+}/", dateList)

  // Approvals
  r.HandleFunc("/add-approvals-to-document-structure/{document-structure}/", addApprovals)
  r.HandleFunc("/remove-approvals-from-document-structure/{document-structure}/", removeApprovals)
  r.HandleFunc("/approvals/{document-structure}/{id:[0-9]+}/", viewOrUpdateApprovals)
}
