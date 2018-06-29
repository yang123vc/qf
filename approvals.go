package qf

import (
  "net/http"
  "html/template"
  "fmt"
  "path/filepath"
  "strconv"
  "strings"
  "github.com/gorilla/mux"
  "database/sql"
)


func AddApprovals(w http.ResponseWriter, r *http.Request) {
  truthValue, err := isUserAdmin(r)
  if err != nil {
    fmt.Fprintf(w, "Error occurred while trying to ascertain if the user is admin. Exact Error: " + err.Error())
    return
  }
  if ! truthValue {
    fmt.Fprintf(w, "You are not an admin here. You don't have permissions to view this page.")
    return
  }

  if r.Method == http.MethodGet {
    type Context struct {
      Roles []string
      DocumentStructures []string
    }

    roles, err := GetRoles()
    if err != nil {
      fmt.Fprintf(w, "Error getting roles. Exact Error: " + err.Error())
      return
    }
    dsList , err := GetDocumentStructureList()
    if err != nil {
      fmt.Fprintf(w, "Error occurred while getting document structure list. Exact Error: " + err.Error())
      return
    }

    ctx := Context{roles, dsList}
    fullTemplatePath := filepath.Join(getProjectPath(), "templates/add-approvals.html")
    tmpl := template.Must(template.ParseFiles(getBaseTemplate(), fullTemplatePath))
    tmpl.Execute(w, ctx)

  } else if r.Method == http.MethodPost {

    ds := r.FormValue("ds")

    // verify if this document structure already has the approval framework.
    var stepsStr sql.NullString
    err = SQLDB.QueryRow("select approval_steps from qf_document_structures where name = ?", ds).Scan(&stepsStr)
    if err != nil {
      fmt.Fprintf(w, "Error occured when getting approval steps of this document structure. Exact Error: " + err.Error())
      return
    }
    if stepsStr.Valid {
      fmt.Fprintf(w, "This document structure already has approval steps.")
      return
    }
    
    steps := make([]string, 0)
    for i := 1; i < 100 ; i++ {
      iStr := strconv.Itoa(i)
      if r.FormValue("step-" + iStr) == "" {
        break
      } else {
        steps = append(steps, r.FormValue("step-" + iStr))
      }
    }

    var dsid int
    err = SQLDB.QueryRow("select id from qf_document_structures where name = ?", ds).Scan(&dsid)
    if err != nil {
      fmt.Fprintf(w, "Error occurred when trying to get document structure id. Exact Error: " + err.Error())
      return
    }
    _, err := SQLDB.Exec("update qf_document_structures set approval_steps = ? where id = ?", strings.Join(steps, ","), dsid)
    if err != nil {
      fmt.Fprintf(w, "Error occurred when updating the document structure. Exact Error:" + err.Error())
      return
    }

    for _, step := range steps {
      sqlStmt := fmt.Sprintf("create table `%s` ( id bigint unsigned not null auto_increment, ", getApprovalTable(ds, step))
      sqlStmt += "created datetime not null,"
      sqlStmt += "modified datetime not null,"
      sqlStmt += "created_by bigint unsigned not null,"
      sqlStmt += "docid bigint unsigned not null,"
      sqlStmt += "status varchar(2) not null,"
      sqlStmt += "message text, primary key (id),"
      sqlStmt += fmt.Sprintf("foreign key (created_by) references `%s`(id),", UsersTable)
      sqlStmt += fmt.Sprintf("foreign key (docid) references `%s`(id) )", tableName(ds))

      _, err = SQLDB.Exec(sqlStmt)
      if err != nil {
        fmt.Fprintf(w, "An error occured while creating approvals table. Exact Error: %s", err.Error())
        return
      }
    }

    fmt.Fprintf(w, "Adding approval steps to document structure \"%s\" successful.", ds)
  }
}


func RemoveApprovals(w http.ResponseWriter, r *http.Request) {
  truthValue, err := isUserAdmin(r)
  if err != nil {
    fmt.Fprintf(w, "Error occurred while trying to ascertain if the user is admin. Exact Error: " + err.Error())
    return
  }
  if ! truthValue {
    fmt.Fprintf(w, "You are not an admin here. You don't have permissions to view this page.")
    return
  }

  vars := mux.Vars(r)
  ds := vars["document-structure"]

  detv, err := docExists(ds)
  if err != nil {
    fmt.Fprintf(w, "Error occurred while determining if this document exists. Exact Error: " + err.Error())
    return
  }
  if detv == false {
    fmt.Fprintf(w, "The document structure %s does not exists.", ds)
    return
  }

  var stepsStr sql.NullString
  err = SQLDB.QueryRow("select approval_steps from qf_document_structures where name = ?", ds).Scan(&stepsStr)
  if err != nil {
    fmt.Fprintf(w, "Error occured when getting approval steps of this document structure. Exact Error: " + err.Error())
    return
  }
  if ! stepsStr.Valid {
    fmt.Fprintf(w, "This document structure has no approval steps.")
    return
  }
  stepsList := strings.Split(stepsStr.String, ",")

  for _, step := range stepsList {
    _, err = SQLDB.Exec(fmt.Sprintf("drop table `%s`", getApprovalTable(ds, step)) )
    if err != nil {
      fmt.Fprintf(w, "An error occured while deleting an approvals table. Exact Error: " + err.Error())
      return
    }
  }

  _, err = SQLDB.Exec("update qf_document_structures set approval_steps = null where name = ?", ds)
  if err != nil {
    fmt.Fprintf(w, "An error occured while clearing approval steps. Exact Error: " + err.Error())
    return
  }

  fmt.Fprintf(w, "Successfully removed approval steps from this document structure.")
}
