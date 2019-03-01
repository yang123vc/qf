package qf

import (
  "net/http"
  "html/template"
  "fmt"
  "strconv"
  "strings"
  "github.com/gorilla/mux"
  "database/sql"
)


func addApprovals(w http.ResponseWriter, r *http.Request) {
  truthValue, err := isUserAdmin(r)
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  if ! truthValue {
    errorPage(w, "You are not an admin here. You don't have permissions to view this page.")
    return
  }

  vars := mux.Vars(r)
  ds := vars["document-structure"]

  detv, err := docExists(ds)
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  if detv == false {
    errorPage(w, fmt.Sprintf("The document structure %s does not exists.", ds))
    return
  }

  // verify if this document structure already has the approval framework.
  var stepsStr sql.NullString
  err = SQLDB.QueryRow("select approval_steps from qf_document_structures where fullname = ?", ds).Scan(&stepsStr)
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  if stepsStr.Valid {
    errorPage(w, "This document structure already has approval steps.")
    return
  }

  if r.Method == http.MethodGet {
    type Context struct {
      Roles []string
      DocumentStructure string
    }

    roles, err := GetRoles()
    if err != nil {
      errorPage(w, err.Error())
      return
    }

    ctx := Context{roles, ds}
    tmpl := template.Must(template.ParseFiles(getBaseTemplate(), "qffiles/add-approvals.html"))
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

    var dsid int
    err = SQLDB.QueryRow("select id from qf_document_structures where fullname = ?", ds).Scan(&dsid)
    if err != nil {
      errorPage(w, err.Error())
      return
    }

    atns := make([]string, 0)
    for _, step := range steps {
      atn, err := newApprovalTableName()
      if err != nil {
        errorPage(w, err.Error())
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
        errorPage(w, err.Error())
        return
      }

      sqlStmt += fmt.Sprintf("foreign key (docid) references `%s`(id) )", tblName)

      _, err = SQLDB.Exec(sqlStmt)
      if err != nil {
        errorPage(w, err.Error())
        return
      }

      roleid, err := getRoleId(step)
      if err != nil {
        errorPage(w, err.Error())
        return
      }

      _, err = SQLDB.Exec("insert into qf_approvals_tables(dsid, roleid, tbl_name) values (?,?,?)",
        dsid, roleid, atn)
      if err != nil {
        errorPage(w, err.Error())
        return
      }
    }

    _, err := SQLDB.Exec("update qf_document_structures set approval_steps = ? where id = ?", strings.Join(steps, ","), dsid)
    if err != nil {
      errorPage(w, err.Error())
      return
    }

    redirectURL := fmt.Sprintf("/view-document-structure/%s/", ds)
    http.Redirect(w, r, redirectURL, 307)
  }
}


func removeApprovals(w http.ResponseWriter, r *http.Request) {
  truthValue, err := isUserAdmin(r)
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  if ! truthValue {
    errorPage(w, "You are not an admin here. You don't have permissions to view this page.")
    return
  }

  vars := mux.Vars(r)
  ds := vars["document-structure"]

  detv, err := docExists(ds)
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  if detv == false {
    errorPage(w, fmt.Sprintf("The document structure %s does not exists.", ds))
    return
  }

  approvers, err := getApprovers(ds)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  for _, step := range approvers {
    atn, err := getApprovalTable(ds, step)
    if err != nil {
      errorPage(w, "An error occurred getting approval table name.")
      return
    }

    _, err = SQLDB.Exec(fmt.Sprintf("drop table `%s`", atn) )
    if err != nil {
      errorPage(w, err.Error())
      return
    }

    _, err = SQLDB.Exec("delete from qf_approvals_tables where document_structure = ? and role = ?",
      ds, step)
    if err != nil {
      errorPage(w, "Error occurred removing record of approval table.")
      return
    }

  }

  _, err = SQLDB.Exec("update qf_document_structures set approval_steps = null where fullname = ?", ds)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  redirectURL := fmt.Sprintf("/view-document-structure/%s/", ds)
  http.Redirect(w, r, redirectURL, 307)
}


func viewOrUpdateApprovals(w http.ResponseWriter, r *http.Request) {
  useridUint64, err := GetCurrentUser(r)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  vars := mux.Vars(r)
  ds := vars["document-structure"]
  docid := vars["id"]
  docidUint64, err := strconv.ParseUint(docid, 10, 64)
  if err != nil {
    errorPage(w, err.Error())
  }

  detv, err := docExists(ds)
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  if detv == false {
    errorPage(w, fmt.Sprintf("The document structure %s does not exists.", ds))
    return
  }

  tblName, err := tableName(ds)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  var count uint64
  sqlStmt := fmt.Sprintf("select count(*) from `%s` where id = %s", tblName, docid)
  err = SQLDB.QueryRow(sqlStmt).Scan(&count)
  if count == 0 {
    errorPage(w, fmt.Sprintf("The document with id %s do not exists", docid))
    return
  }

  readPerm, err := DoesCurrentUserHavePerm(r, ds, "read")
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  rocPerm, err := DoesCurrentUserHavePerm(r, ds, "read-only-created")
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  romPerm, err := DoesCurrentUserHavePerm(r, ds, "read-only-mentioned")
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  var createdBy uint64
  sqlStmt = fmt.Sprintf("select created_by from `%s` where id = %s", tblName, docid)
  err = SQLDB.QueryRow(sqlStmt).Scan(&createdBy)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  if ! readPerm {
    if rocPerm {
      if createdBy != useridUint64 {
        errorPage(w, "You are not the owner of this document so can't read it.")
        return
      }
    } else if romPerm {
      muColumn, err := getMentionedUserColumn(ds)
      if err != nil {
        errorPage(w, err.Error())
        return
      }

      var muColumnData uint64
      sqlStmt = fmt.Sprintf("select %s from `%s` where id = %s", muColumn, tblName, docid)
      err = SQLDB.QueryRow(sqlStmt).Scan(&muColumnData)
      if err != nil {
        errorPage(w, err.Error())
        return
      }

      if muColumnData != useridUint64 {
        errorPage(w, "You are not mentioned in this document so can't read it.")
        return
      }
    } else {
      errorPage(w, "You don't have the read permission for this document structure.")
      return
    }
  }

  approvers, err := getApprovers(ds)
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  if len(approvers) == 0 {
    errorPage(w, "This document structure doesn't have the approval framework on it.")
  }


  userRoles, err := GetCurrentUserRoles(r)
  if err != nil {
    errorPage(w, err.Error())
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
        errorPage(w, "An error occurred getting approval table name.")
        return
      }

      var approvalDataCount uint64
      sqlStmt = fmt.Sprintf("select count(*) from `%s` where docid = ?", atn)
      err = SQLDB.QueryRow(sqlStmt, docid).Scan(&approvalDataCount)
      if err != nil {
        errorPage(w, err.Error())
        return
      }
      if approvalDataCount == 0 {
        ads = append(ads, ApprovalData{role, "", "", cuhtr})
      } else if approvalDataCount == 1 {
        var status, message sql.NullString
        sqlStmt = fmt.Sprintf("select status, message from `%s` where docid = ?", atn)
        err = SQLDB.QueryRow(sqlStmt, docid).Scan(&status, &message)
        if err != nil {
          errorPage(w, err.Error())
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
    tmpl := template.Must(template.ParseFiles(getBaseTemplate(), "qffiles/view-update-approvals.html"))
    tmpl.Execute(w, ctx)

  } else if r.Method == http.MethodPost {

    role := r.FormValue("role")
    status := r.FormValue("status")
    message := r.FormValue("message")

    atn, err := getApprovalTable(ds, role)
    if err != nil {
      errorPage(w, "An error occurred getting approval table name.")
      return
    }

    var approvalDataCount uint64
    sqlStmt = fmt.Sprintf("select count(*) from `%s` where id = ?", atn)
    err = SQLDB.QueryRow(sqlStmt, docid).Scan(&approvalDataCount)
    if err != nil {
      errorPage(w, err.Error())
      return
    }
    if approvalDataCount == 0 {
      sqlStmt = fmt.Sprintf("insert into `%s` (created, modified, created_by, status, message, docid)", atn )
      sqlStmt += " values(now(), now(), ?, ?, ?, ?)"
      _, err = SQLDB.Exec(sqlStmt, useridUint64, status, message, docid)
      if err != nil {
        errorPage(w, err.Error())
        return
      }
    } else if approvalDataCount == 1 {
      sqlStmt = fmt.Sprintf("update `%s` set modified = now(), status = ?, message = ? where docid = ?", atn)
      _, err = SQLDB.Exec(sqlStmt, status, message, docid)
      if err != nil {
        errorPage(w, err.Error())
        return
      }
    }

    approvalSummary, err := isApproved(ds, docidUint64)
    if err != nil {
      errorPage(w, err.Error())
      return
    }

    dbValue := "f"
    if approvalSummary {
      dbValue = "t"
    }
    sqlStmt = fmt.Sprintf("update `%s` set fully_approved = ? where id = ?", tblName)
    _, err = SQLDB.Exec(sqlStmt, dbValue, docidUint64)
    if err != nil {
      errorPage(w, err.Error())
      return
    }

    if ApprovalFrameworkMailsFn != nil {
      ApprovalFrameworkMailsFn(docidUint64, role, status, message)
    }

    redirectURL := fmt.Sprintf("/doc/%s/list/", ds)
    http.Redirect(w, r, redirectURL, 307)
  }
}
