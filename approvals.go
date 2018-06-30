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
      sqlStmt += "message text, primary key (id), unique(docid),"
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


func ViewOrUpdateApprovals(w http.ResponseWriter, r *http.Request) {
  useridUint64, err := GetCurrentUser(r)
  if err != nil {
    fmt.Fprintf(w, "You need to be logged in to continue. Exact Error: " + err.Error())
    return
  }

  vars := mux.Vars(r)
  ds := vars["document-structure"]
  docid := vars["id"]

  detv, err := docExists(ds)
  if err != nil {
    fmt.Fprintf(w, "Error occurred while determining if this document exists. Exact Error: " + err.Error())
    return
  }
  if detv == false {
    fmt.Fprintf(w, "The document structure %s does not exists.", ds)
    return
  }

  readPerm, err := doesCurrentUserHavePerm(r, ds, "read")
  if err != nil {
    fmt.Fprintf(w, "Error occured while determining if the user have read permission for this document structure. Exact Error: " + err.Error())
    return
  }
  rocPerm, err := doesCurrentUserHavePerm(r, ds, "read-only-created")
  if err != nil {
    fmt.Fprintf(w, "Error occured while determining if the user have read-only-created permission for this document. Exact Error: " + err.Error())
    return
  }

  var createdBy uint64
  sqlStmt := fmt.Sprintf("select created_by from `%s` where id = %s", tableName(ds), docid)
  err = SQLDB.QueryRow(sqlStmt).Scan(&createdBy)
  if err != nil {
    fmt.Fprintf(w, "An internal error occured. Exact Error: " + err.Error())
    return
  }

  if ! readPerm {
    if rocPerm {
      if createdBy != useridUint64 {
        fmt.Fprintf(w, "You are not the owner of this document so can't read it.")
        return
      }
    } else {
      fmt.Fprintf(w, "You don't have the read permission for this document structure.")
      return
    }
  }

  approvers, err := getApprovers(ds)
  if err != nil {
    fmt.Fprintf(w, "Error getting approvers for this document structure. Exact Error: " + err.Error())
    return
  }
  if len(approvers) == 0 {
    fmt.Fprintf(w, "This document structure doesn't have the approval framework on it.")
  }

  var count uint64
  sqlStmt = fmt.Sprintf("select count(*) from `%s` where id = %s", tableName(ds), docid)
  err = SQLDB.QueryRow(sqlStmt).Scan(&count)
  if count == 0 {
    fmt.Fprintf(w, "The document with id %s do not exists", docid)
    return
  }

  userRoles, err := GetCurrentUserRoles(r)
  if err != nil {
    fmt.Fprintf(w, "Error occured when getting current user roles. Exact Error: " + err.Error())
    return
  }
  
  if r.Method == http.MethodGet {
    type ApprovalData struct {
      Role string
      Status string
      Message string
      CurrentUserHasThisRole bool
    }
    ads := make([]ApprovalData, 0)
    for _, role := range approvers {
      var cuhtr bool
      for _, r := range userRoles {
        if r == role {
          cuhtr = true
        }
      }
      var approvalDataCount uint64
      sqlStmt = fmt.Sprintf("select count(*) from `%s` where id = ?", getApprovalTable(ds, role))
      err = SQLDB.QueryRow(sqlStmt, docid).Scan(&approvalDataCount)
      if err != nil {
        fmt.Fprintf(w, "Error occurred while checking for approval data. Exact Error: " + err.Error())
        return
      }
      if approvalDataCount == 0 {
        ads = append(ads, ApprovalData{role, "", "", cuhtr})
      } else if approvalDataCount == 1 {
        var status, message sql.NullString
        sqlStmt = fmt.Sprintf("select status, message from `%s` where id = ?", getApprovalTable(ds, role))
        err = SQLDB.QueryRow(sqlStmt, docid).Scan(&status, &message)
        if err != nil {
          fmt.Fprintf(w, "Error occurred while checking for approval data. Exact Error: " + err.Error())
          return
        }
        var actualMessage string
        if ! message.Valid {
          actualMessage = ""
        } else {
          actualMessage = message.String
        }
        ads = append(ads, ApprovalData{role, status.String, actualMessage, cuhtr})
      }
    }

    type Context struct {
      ApprovalDatas []ApprovalData
      DocumentStructure string
      Approver bool
      DocID string
      UserRoles []string
    }

    var approver bool
    outerLoop:
      for _, apr := range approvers {
        for _, role := range userRoles {
          if role == apr {
            approver = true
            break outerLoop
          }
        }
      }

    ctx := Context{ads, ds, approver, docid, userRoles}
    fullTemplatePath := filepath.Join(getProjectPath(), "templates/view-update-approvals.html")
    tmpl := template.Must(template.ParseFiles(getBaseTemplate(), fullTemplatePath))
    tmpl.Execute(w, ctx)
  }
}
