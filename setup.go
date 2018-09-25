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
  ValidationFn func(jsonData string) string
  AfterCreateFn func(id uint64)
  AfterUpdateFn func(id uint64)
  BeforeDeleteFn func(id uint64)
}

var ExtraCodeMap = make(map[int]ExtraCode)

var ApprovalFrameworkMailsFn func(docid uint64, role, status, message string)


func qfSetup(w http.ResponseWriter, r *http.Request) {
  if SQLDB == nil {
    errorPage(w, "You have not set the \"qf.SQLDB\". Initialize a connection to the database and set the result to this value.", nil)
    return
  } else {
    if err := SQLDB.Ping(); err != nil {
      errorPage(w, "DB Ping failed.", err)
      return
    }
  }

  if SiteDB == "" {
    errorPage(w, "You have not set the \"qf.SiteDB\". Create your database for your site and set this to it.", nil)
    return
  } else {
    var dbCount int
    err := SQLDB.QueryRow("select count(*) from information_schema.schemata where schema_name = ?", SiteDB).Scan(&dbCount)
    if err != nil {
      errorPage(w, "Error checking if the database exists. ", err)
      return
    }
    if dbCount == 0 {
      errorPage(w, fmt.Sprintf("Your SiteDB \"%s\" does not exists.", SiteDB), nil)
      return
    }
  }

  if UsersTable == "" {
    errorPage(w, "You have not set the \"qf.UsersTable\". Create your users table and set this variable to it.", nil)
    return
  } else {
    var tblCount int
    err := SQLDB.QueryRow("select count(*) from information_schema.tables where table_schema=? and table_name=?", SiteDB, UsersTable).Scan(&tblCount)
    if err != nil {
      errorPage(w, "Error checking if the table exists. ", err)
      return
    }
    if tblCount == 0 {
      errorPage(w, fmt.Sprintf("Your UsersTable \"%s\" does not exists.", UsersTable), nil)
      return
    }
  }

  if Admins == nil {
    errorPage(w, "You have not set the \"qf.Admins\". Please set this to a list of ids (in uint64) of the Admins of this site.", nil)
    return
  }

  if GetCurrentUser == nil {
    errorPage(w, "You must set the \"qf.GetCurrentUser\". Please set this variable to a function with signature func(r *http.Request) (uint64, err).", nil)
    return
  }

  var count int
  err := SQLDB.QueryRow(`select count(*) as count from information_schema.tables
  where table_schema=? and table_name=?`, SiteDB, "qf_document_structures").Scan(&count)
  if err != nil {
    errorPage(w, "An internal error occured.", err)
    return
  }

  if count == 1 {
    errorPage(w, "This setup has been executed.", nil)
    return

  } else {
    // do setup

    // create forms general table
    _, err = SQLDB.Exec(`create table qf_document_structures (
      id int not null auto_increment,
      fullname varchar(255) not null,
      tbl_name varchar(64) not null,
      child_table varchar(1) default 'f',
      approval_steps varchar(255),
      help_text text,
      primary key (id),
      unique (fullname),
      unique (tbl_name)
      )`)
    if err != nil {
      errorPage(w, "An error occured while creating the document structures table.", err)
      return
    }

    // create fields table
    _, err = SQLDB.Exec(`create table qf_fields (
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
      errorPage(w, "An error occured while creating the fields table", err)
      return
    }

    _, err = SQLDB.Exec(`create table qf_approvals_tables (
      id int not null auto_increment,
      document_structure varchar(255) not null,
      role varchar(255) not null,
      tbl_name varchar(64) not null,
      primary key (id),
      unique(tbl_name),
      index (document_structure, role)
      )`)
    if err != nil {
      errorPage(w, "An error ocurred while creating the store for approvals tables", err)
      return
    }

    _, err = SQLDB.Exec(`create table qf_roles (
      id int not null auto_increment,
      role varchar(50) not null,
      primary key(id),
      unique (role)
      )`)
    if err != nil {
      errorPage(w, "An error occured while creating the roles table.", err)
      return
    }

    _, err = SQLDB.Exec(`create table qf_permissions (
      id int not null auto_increment,
      roleid int not null,
      object varchar(255) not null,
      permissions varchar(255) not null,
      primary key (id),
      unique(roleid, object),
      foreign key (roleid) references qf_roles (id)
      )`)
    if err != nil {
      errorPage(w, "An error occured while creating permissions table.", err)
      return
    }

    sqlStmt := "create table qf_user_roles ("
    sqlStmt += "id bigint unsigned not null auto_increment,"
    sqlStmt += "userid bigint unsigned not null,"
    sqlStmt += "roleid int not null, primary key(id), unique(userid, roleid),"
    sqlStmt += "foreign key (roleid) references qf_roles (id),"
    sqlStmt += fmt.Sprintf("foreign key (userid) references `%s`(id))", UsersTable)
    _, err = SQLDB.Exec(sqlStmt)
    if err != nil {
      errorPage(w, "An error occured while creating user to roles table.", err)
      return
    }

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
  r.HandleFunc("/new-role/", newRole)
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
  r.HandleFunc("/doc/{document-structure}/search-results/", searchResults)
  r.HandleFunc("/doc/{document-structure}/search-results/{page:[0-9]+}/", searchResults)
  r.HandleFunc("/doc/{document-structure}/date-lists/", dateLists)
  r.HandleFunc("/doc/{document-structure}/date-list/{date}/", dateList)
  r.HandleFunc("/doc/{document-structure}/date-list/{date}/{page:[0-9]+}/", dateList)
  r.HandleFunc("/doc/{document-structure}/approved-list/", approvedList)
  r.HandleFunc("/doc/{document-structure}/approved-list/{page:[0-9]+}/", approvedList)
  r.HandleFunc("/doc/{document-structure}/unapproved-list/", unapprovedList)
  r.HandleFunc("/doc/{document-structure}/unapproved-list/{page:[0-9]+}/", unapprovedList)

  // Approvals
  r.HandleFunc("/add-approvals-to-document-structure/{document-structure}/", addApprovals)
  r.HandleFunc("/remove-approvals-from-document-structure/{document-structure}/", removeApprovals)
  r.HandleFunc("/approvals/{document-structure}/{id:[0-9]+}/", viewOrUpdateApprovals)
}
