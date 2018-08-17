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


func addApprovals(w http.ResponseWriter, r *http.Request) {
  truthValue, err := isUserAdmin(r)
  if err != nil {
    errorPage(w, "Error occurred while trying to ascertain if the user is admin.", err)
    return
  }
  if ! truthValue {
    errorPage(w, "You are not an admin here. You don't have permissions to view this page.", nil)
    return
  }

  vars := mux.Vars(r)
  ds := vars["document-structure"]

  detv, err := docExists(ds)
  if err != nil {
    errorPage(w, "Error occurred while determining if this document exists.", err)
    return
  }
  if detv == false {
    errorPage(w, fmt.Sprintf("The document structure %s does not exists.", ds), nil)
    return
  }

  if r.Method == http.MethodGet {
    type Context struct {
      Roles []string
      DocumentStructure string
    }

    roles, err := GetRoles()
    if err != nil {
      errorPage(w, "Error getting roles.", err)
      return
    }

    ctx := Context{roles, ds}
    fullTemplatePath := filepath.Join(getProjectPath(), "templates/add-approvals.html")
    tmpl := template.Must(template.ParseFiles(getBaseTemplate(), fullTemplatePath))
    tmpl.Execute(w, ctx)

  } else if r.Method == http.MethodPost {

    // verify if this document structure already has the approval framework.
    var stepsStr sql.NullString
    err = SQLDB.QueryRow("select approval_steps from qf_document_structures where name = ?", ds).Scan(&stepsStr)
    if err != nil {
      errorPage(w, "Error occured when getting approval steps of this document structure.", err)
      return
    }
    if stepsStr.Valid {
      errorPage(w, "This document structure already has approval steps.", nil)
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
      errorPage(w, "Error occurred when trying to get document structure id.", err)
      return
    }
    _, err := SQLDB.Exec("update qf_document_structures set approval_steps = ? where id = ?", strings.Join(steps, ","), dsid)
    if err != nil {
      errorPage(w, "Error occurred when updating the document structure.", err)
      return
    }

    for _, step := range steps {
      sqlStmt := fmt.Sprintf("create table `%s` ( id bigint unsigned not null auto_increment, ", getApprovalTable(ds, step))
      sqlStmt += "created datetime not null,"
      sqlStmt += "modified datetime not null,"
      sqlStmt += "created_by bigint unsigned not null,"
      sqlStmt += "docid bigint unsigned not null,"
      sqlStmt += "status varchar(20) not null,"
      sqlStmt += "message text, primary key (id), unique(docid),"
      sqlStmt += fmt.Sprintf("foreign key (created_by) references `%s`(id),", UsersTable)
      sqlStmt += fmt.Sprintf("foreign key (docid) references `%s`(id) )", tableName(ds))

      _, err = SQLDB.Exec(sqlStmt)
      if err != nil {
        errorPage(w, "An error occured while creating approvals table.", err)
        return
      }
    }

    redirectURL := fmt.Sprintf("/view-document-structure/%s/", ds)
    http.Redirect(w, r, redirectURL, 307)
  }
}


func removeApprovals(w http.ResponseWriter, r *http.Request) {
  truthValue, err := isUserAdmin(r)
  if err != nil {
    errorPage(w, "Error occurred while trying to ascertain if the user is admin.", err)
    return
  }
  if ! truthValue {
    errorPage(w, "You are not an admin here. You don't have permissions to view this page.", nil)
    return
  }

  vars := mux.Vars(r)
  ds := vars["document-structure"]

  detv, err := docExists(ds)
  if err != nil {
    errorPage(w, "Error occurred while determining if this document exists.", err)
    return
  }
  if detv == false {
    errorPage(w, fmt.Sprintf("The document structure %s does not exists.", ds), nil)
    return
  }

  approvers, err := getApprovers(ds)
  if err != nil {
    errorPage(w, "Error occurred getting approvers.", err)
    return
  }

  for _, step := range approvers {
    _, err = SQLDB.Exec(fmt.Sprintf("drop table `%s`", getApprovalTable(ds, step)) )
    if err != nil {
      errorPage(w, "An error occured while deleting an approvals table.", err)
      return
    }
  }

  _, err = SQLDB.Exec("update qf_document_structures set approval_steps = null where name = ?", ds)
  if err != nil {
    errorPage(w, "An error occured while clearing approval steps.", err)
    return
  }

  redirectURL := fmt.Sprintf("/view-document-structure/%s/", ds)
  http.Redirect(w, r, redirectURL, 307)
}


func viewOrUpdateApprovals(w http.ResponseWriter, r *http.Request) {
  useridUint64, err := GetCurrentUser(r)
  if err != nil {
    errorPage(w, "You need to be logged in to continue.", err)
    return
  }

  vars := mux.Vars(r)
  ds := vars["document-structure"]
  docid := vars["id"]
  docidUint64, err := strconv.ParseUint(docid, 10, 64)
  if err != nil {
    errorPage(w, "Document ID is invalid.", err)
  }

  detv, err := docExists(ds)
  if err != nil {
    errorPage(w, "Error occurred while determining if this document exists.", err)
    return
  }
  if detv == false {
    errorPage(w, fmt.Sprintf("The document structure %s does not exists.", ds), nil)
    return
  }

  readPerm, err := DoesCurrentUserHavePerm(r, ds, "read")
  if err != nil {
    errorPage(w, "Error occured while determining if the user have read permission for this document structure. " , err)
    return
  }
  rocPerm, err := DoesCurrentUserHavePerm(r, ds, "read-only-created")
  if err != nil {
    errorPage(w, "Error occured while determining if the user have read-only-created permission for this document. " , err)
    return
  }

  var createdBy uint64
  sqlStmt := fmt.Sprintf("select created_by from `%s` where id = %s", tableName(ds), docid)
  err = SQLDB.QueryRow(sqlStmt).Scan(&createdBy)
  if err != nil {
    errorPage(w, "An internal error occured. " , err)
    return
  }

  if ! readPerm {
    if rocPerm {
      if createdBy != useridUint64 {
        errorPage(w, "You are not the owner of this document so can't read it.", nil)
        return
      }
    } else {
      errorPage(w, "You don't have the read permission for this document structure.", nil)
      return
    }
  }

  approvers, err := getApprovers(ds)
  if err != nil {
    errorPage(w, "Error getting approvers for this document structure. " , err)
    return
  }
  if len(approvers) == 0 {
    errorPage(w, "This document structure doesn't have the approval framework on it.", nil)
  }

  var count uint64
  sqlStmt = fmt.Sprintf("select count(*) from `%s` where id = %s", tableName(ds), docid)
  err = SQLDB.QueryRow(sqlStmt).Scan(&count)
  if count == 0 {
    errorPage(w, fmt.Sprintf("The document with id %s do not exists", docid), nil)
    return
  }

  userRoles, err := GetCurrentUserRoles(r)
  if err != nil {
    errorPage(w, "Error occured when getting current user roles. " , err)
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
          break
        }
      }
      var approvalDataCount uint64
      sqlStmt = fmt.Sprintf("select count(*) from `%s` where docid = ?", getApprovalTable(ds, role))
      err = SQLDB.QueryRow(sqlStmt, docid).Scan(&approvalDataCount)
      if err != nil {
        errorPage(w, "Error occurred while checking for approval data. " , err)
        return
      }
      if approvalDataCount == 0 {
        ads = append(ads, ApprovalData{role, "", "", cuhtr})
      } else if approvalDataCount == 1 {
        var status, message sql.NullString
        sqlStmt = fmt.Sprintf("select status, message from `%s` where docid = ?", getApprovalTable(ds, role))
        err = SQLDB.QueryRow(sqlStmt, docid).Scan(&status, &message)
        if err != nil {
          errorPage(w, "Error occurred while checking for approval data. " , err)
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
      DocID string
    }


    ctx := Context{ads, ds, docid}
    fullTemplatePath := filepath.Join(getProjectPath(), "templates/view-update-approvals.html")
    tmpl := template.Must(template.ParseFiles(getBaseTemplate(), fullTemplatePath))
    tmpl.Execute(w, ctx)

  } else if r.Method == http.MethodPost {

    role := r.FormValue("role")
    status := r.FormValue("status")
    message := r.FormValue("message")

    var approvalDataCount uint64
    sqlStmt = fmt.Sprintf("select count(*) from `%s` where id = ?", getApprovalTable(ds, role))
    err = SQLDB.QueryRow(sqlStmt, docid).Scan(&approvalDataCount)
    if err != nil {
      errorPage(w, "Error occurred while checking for approval data. " , err)
      return
    }
    if approvalDataCount == 0 {
      sqlStmt = fmt.Sprintf("insert into `%s` (created, modified, created_by, status, message, docid)", getApprovalTable(ds, role) )
      sqlStmt += " values(now(), now(), ?, ?, ?, ?)"
      _, err = SQLDB.Exec(sqlStmt, useridUint64, status, message, docid)
      if err != nil {
        errorPage(w, "Error occurred while saving approval data. " , err)
        return
      }
    } else if approvalDataCount == 1 {
      sqlStmt = fmt.Sprintf("update `%s` set modified = now(), status = ?, message = ? where docid = ?", getApprovalTable(ds, role))
      _, err = SQLDB.Exec(sqlStmt, status, message, docid)
      if err != nil {
        errorPage(w, "Error occurred while updating approval data. " , err)
        return
      }
    }

    if ApprovalFrameworkMailsFn != nil {
      ApprovalFrameworkMailsFn(docidUint64, role, status, message)
    }

    redirectURL := fmt.Sprintf("/doc/%s/list/", ds)
    http.Redirect(w, r, redirectURL, 307)
  }
}
