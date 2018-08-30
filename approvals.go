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

  // verify if this document structure already has the approval framework.
  var stepsStr sql.NullString
  err = SQLDB.QueryRow("select approval_steps from qf_document_structures where fullname = ?", ds).Scan(&stepsStr)
  if err != nil {
    errorPage(w, "Error occured when getting approval steps of this document structure.", err)
    return
  }
  if stepsStr.Valid {
    errorPage(w, "This document structure already has approval steps.", nil)
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

    steps := make([]string, 0)
    for i := 1; i < 100 ; i++ {
      iStr := strconv.Itoa(i)
      if r.FormValue("step-" + iStr) == "" {
        break
      } else {
        steps = append(steps, r.FormValue("step-" + iStr))
      }
    }

    atns := make([]string, 0)
    for _, step := range steps {
      atn, err := newApprovalTableName(ds, step)
      if err != nil {
        errorPage(w, "Error creating approval's table name", err)
        return
      }
      atns = append(atns, atn)

      sqlStmt := fmt.Sprintf("create table `%s` ( id bigint unsigned not null auto_increment, ", atn)
      sqlStmt += "created datetime not null,"
      sqlStmt += "modified datetime not null,"
      sqlStmt += "created_by bigint unsigned not null,"
      sqlStmt += "docid bigint unsigned not null,"
      sqlStmt += "status varchar(20) not null,"
      sqlStmt += "message text, primary key (id), unique(docid),"
      sqlStmt += fmt.Sprintf("foreign key (created_by) references `%s`(id),", UsersTable)

      tblName, err := tableName(ds)
      if err != nil {
        errorPage(w, "Error getting document structure's table name.", err)
        return
      }

      sqlStmt += fmt.Sprintf("foreign key (docid) references `%s`(id) )", tblName)

      _, err = SQLDB.Exec(sqlStmt)
      if err != nil {
        errorPage(w, "An error occured while creating approvals table.", err)
        return
      }

      _, err = SQLDB.Exec("insert into qf_approvals_tables(document_structure, role, tbl_name) values (?,?,?)",
        ds, step, atn)
      if err != nil {
        errorPage(w, "Error occurred storing approval table name.", err)
        return
      }
    }

    var dsid int
    err = SQLDB.QueryRow("select id from qf_document_structures where fullname = ?", ds).Scan(&dsid)
    if err != nil {
      errorPage(w, "Error occurred when trying to get document structure id.", err)
      return
    }
    _, err := SQLDB.Exec("update qf_document_structures set approval_steps = ? where id = ?", strings.Join(steps, ","), dsid)
    if err != nil {
      errorPage(w, "Error occurred when updating the document structure.", err)
      return
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
    atn, err := getApprovalTable(ds, step)
    if err != nil {
      errorPage(w, "An error occurred getting approval table name.", nil)
      return
    }

    _, err = SQLDB.Exec(fmt.Sprintf("drop table `%s`", atn) )
    if err != nil {
      errorPage(w, "An error occured while deleting an approvals table.", err)
      return
    }

    _, err = SQLDB.Exec("delete from qf_approvals_tables where document_structure = ? and role = ?",
      ds, step)
    if err != nil {
      errorPage(w, "Error occurred removing record of approval table.", nil)
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

  tblName, err := tableName(ds)
  if err != nil {
    errorPage(w, "Error getting document structure's table name.", err)
    return
  }

  var count uint64
  sqlStmt := fmt.Sprintf("select count(*) from `%s` where id = %s", tblName, docid)
  err = SQLDB.QueryRow(sqlStmt).Scan(&count)
  if count == 0 {
    errorPage(w, fmt.Sprintf("The document with id %s do not exists", docid), nil)
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
  romPerm, err := DoesCurrentUserHavePerm(r, ds, "read-only-mentioned")
  if err != nil {
    errorPage(w, "Error while getting user's permissions.", err)
    return
  }

  var createdBy uint64
  sqlStmt = fmt.Sprintf("select created_by from `%s` where id = %s", tblName, docid)
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
    } else if romPerm {
      muColumn, err := getMentionedUserColumn(ds)
      if err != nil {
        errorPage(w, "Error getting MentionedUser column.", err)
        return
      }

      var muColumnData uint64
      sqlStmt = fmt.Sprintf("select %s from `%s` where id = %s", muColumn, tblName, docid)
      err = SQLDB.QueryRow(sqlStmt).Scan(&muColumnData)
      if err != nil {
        errorPage(w, "An error occurred while reading the mentioned user column.", err)
        return
      }

      if muColumnData != useridUint64 {
        errorPage(w, "You are not mentioned in this document so can't read it.", nil)
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

      atn, err := getApprovalTable(ds, role)
      if err != nil {
        errorPage(w, "An error occurred getting approval table name.", nil)
        return
      }

      var approvalDataCount uint64
      sqlStmt = fmt.Sprintf("select count(*) from `%s` where docid = ?", atn)
      err = SQLDB.QueryRow(sqlStmt, docid).Scan(&approvalDataCount)
      if err != nil {
        errorPage(w, "Error occurred while checking for approval data. " , err)
        return
      }
      if approvalDataCount == 0 {
        ads = append(ads, ApprovalData{role, "", "", cuhtr})
      } else if approvalDataCount == 1 {
        var status, message sql.NullString
        sqlStmt = fmt.Sprintf("select status, message from `%s` where docid = ?", atn)
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

    atn, err := getApprovalTable(ds, role)
    if err != nil {
      errorPage(w, "An error occurred getting approval table name.", nil)
      return
    }

    var approvalDataCount uint64
    sqlStmt = fmt.Sprintf("select count(*) from `%s` where id = ?", atn)
    err = SQLDB.QueryRow(sqlStmt, docid).Scan(&approvalDataCount)
    if err != nil {
      errorPage(w, "Error occurred while checking for approval data. " , err)
      return
    }
    if approvalDataCount == 0 {
      sqlStmt = fmt.Sprintf("insert into `%s` (created, modified, created_by, status, message, docid)", atn )
      sqlStmt += " values(now(), now(), ?, ?, ?, ?)"
      _, err = SQLDB.Exec(sqlStmt, useridUint64, status, message, docid)
      if err != nil {
        errorPage(w, "Error occurred while saving approval data. " , err)
        return
      }
    } else if approvalDataCount == 1 {
      sqlStmt = fmt.Sprintf("update `%s` set modified = now(), status = ?, message = ? where docid = ?", atn)
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
