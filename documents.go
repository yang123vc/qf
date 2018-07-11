package qf

import (
  "net/http"
  "fmt"
  "path/filepath"
  "strings"
  "html/template"
  "github.com/gorilla/mux"
  "encoding/json"
  "strconv"
  "database/sql"
  "math"
)


func createDocument(w http.ResponseWriter, r *http.Request) {
  useridUint64, err := GetCurrentUser(r)
  if err != nil {
    errorPage(w, r, "You need to be logged in to continue.", err)
    return
  }

  vars := mux.Vars(r)
  ds := vars["document-structure"]

  detv, err := docExists(ds)
  if err != nil {
    errorPage(w, r, "Error occurred while determining if this document exists.", err)
    return
  }
  if detv == false {
    errorPage(w, r, fmt.Sprintf("The document structure %s does not exists.", ds), nil)
    return
  }

  truthValue, err := DoesCurrentUserHavePerm(r, ds, "create")
  if err != nil {
    errorPage(w, r, "Error occured while determining if the user have permission for this page.  " , err)
    return
  }
  if ! truthValue {
    errorPage(w, r, "You don't have the create permission for this document structure.", nil)
    return
  }


  var id int
  err = SQLDB.QueryRow("select id from qf_document_structures where name = ?", ds).Scan(&id)
  if err != nil {
    errorPage(w, r, "An internal error occured.", err)
    return
  }

  dds := GetDocData(id)

  if r.Method == http.MethodGet {
    type Context struct {
      DocumentStructure string
      DDs []DocData
    }
    ctx := Context{ds, dds}
    fullTemplatePath := filepath.Join(getProjectPath(), "templates/create-document.html")
    tmpl := template.Must(template.ParseFiles(getBaseTemplate(), fullTemplatePath))
    tmpl.Execute(w, ctx)

  } else if r.Method == http.MethodPost {

    // first check if it passes the extra code validation for this document.
    r.ParseForm()
    fData := make(map[string]string)
    for k := range r.PostForm {
      fData[k] = r.FormValue(k)
    }
    jsonString, _ := json.Marshal(fData)

    ec, ectv := getEC(ds)
    if ectv && ec.ValidationFn != nil {
      outString := ec.ValidationFn(string(jsonString))
      if outString != "" {
        fmt.Fprintf(w, "Exra Code Validation Error: " + outString)
        return
      }
    }

    colNames := make([]string, 0)
    formData := make([]string, 0)
    for _, dd := range dds {
      if dd.Type == "Section Break" {
        continue
      }
      colNames = append(colNames, dd.Name)
      switch dd.Type {
      case "Text", "Data", "Email", "Read Only", "URL", "Select", "Date", "Datetime":
        var data string
        if r.FormValue(dd.Name) == "" {
          data = "null"
        } else {
          data = fmt.Sprintf("\"%s\"", template.HTMLEscapeString(r.FormValue(dd.Name)))
        }
        formData = append(formData, data)
      case "Check":
        var data string
        if r.FormValue(dd.Name) == "on" {
          data = "\"t\""
        } else {
          data = "\"f\""
        }
        formData = append(formData, data)
      default:
        var data string
        if r.FormValue(dd.Name) == "" {
          data = "null"
        } else {
          data = template.HTMLEscapeString(r.FormValue(dd.Name))
        }
        formData = append(formData, data)
      }
    }
    colNamesStr := strings.Join(colNames, ", ")
    formDataStr := strings.Join(formData, ", ")
    sqlStmt := fmt.Sprintf("insert into `%s`(created, modified, created_by, %s) values(now(), now(), %d, %s)", tableName(ds), colNamesStr, useridUint64, formDataStr)
    res, err := SQLDB.Exec(sqlStmt)
    if err != nil {
      errorPage(w, r, "An error occured while saving: " , err)
      return
    }

    // new document extra code
    lastid, err := res.LastInsertId()
    if err != nil {
      errorPage(w, r, "An error occured while trying to run extra code: " , err)
      return
    }

    if ectv && ec.AfterCreateFn != nil {
      ec.AfterCreateFn(uint64(lastid))
    }

    redirectURL := fmt.Sprintf("/doc/%s/list/", ds)
    http.Redirect(w, r, redirectURL, 307)
  }

}


func updateDocument(w http.ResponseWriter, r *http.Request) {
  useridUint64, err := GetCurrentUser(r)
  if err != nil {
    errorPage(w, r, "You need to be logged in to continue.", err)
    return
  }

  vars := mux.Vars(r)
  ds := vars["document-structure"]
  docid := vars["id"]
  _, err = strconv.ParseUint(docid, 10, 64)
  if err != nil {
    errorPage(w, r, "Document ID is invalid.", err)
  }

  detv, err := docExists(ds)
  if err != nil {
    errorPage(w, r, "Error occurred while determining if this document exists.", err)
    return
  }
  if detv == false {
    errorPage(w, r, fmt.Sprintf("The document structure %s does not exists.", ds), nil)
    return
  }

  readPerm, err := DoesCurrentUserHavePerm(r, ds, "read")
  if err != nil {
    errorPage(w, r, "Error occured while determining if the user have read permission for this document structure.  " , err)
    return
  }
  rocPerm, err := DoesCurrentUserHavePerm(r, ds, "read-only-created")
  if err != nil {
    errorPage(w, r, "Error occured while determining if the user have read-only-created permission for this document.  " , err)
    return
  }

  var createdBy uint64
  sqlStmt := fmt.Sprintf("select created_by from `%s` where id = %s", tableName(ds), docid)
  err = SQLDB.QueryRow(sqlStmt).Scan(&createdBy)
  if err != nil {
    errorPage(w, r, "An internal error occured.  " , err)
    return
  }

  if ! readPerm {
    if rocPerm {
      if createdBy != useridUint64 {
        errorPage(w, r, "You are not the owner of this document so can't read it.", nil)
        return
      }
    } else {
      errorPage(w, r, "You don't have the read permission for this document structure.", nil)
      return
    }
  }

  var count uint64
  sqlStmt = fmt.Sprintf("select count(*) from `%s` where id = %s", tableName(ds), docid)
  err = SQLDB.QueryRow(sqlStmt).Scan(&count)
  if count == 0 {
    errorPage(w, r, fmt.Sprintf("The document with id %s do not exists", docid), nil)
    return
  }

  var id int
  err = SQLDB.QueryRow("select id from qf_document_structures where name = ?", ds).Scan(&id)
  if err != nil {
    errorPage(w, r, "An error occurred when reading document structure. Exact Error" , err)
    return
  }

  docDatas := GetDocData(id)

  type docAndStructure struct {
    DocData
    Data string
  }

  docAndStructureSlice := make([]docAndStructure, 0)
  for _, docData := range docDatas {
    if docData.Type == "Section Break" {
      docAndStructureSlice = append(docAndStructureSlice, docAndStructure{docData, ""})
    } else {
      var data string
      var dataFromDB sql.NullString
      sqlStmt := fmt.Sprintf("select %s from `%s` where id = %s", docData.Name, tableName(ds), docid)
      err := SQLDB.QueryRow(sqlStmt).Scan(&dataFromDB)
      if err != nil {
        errorPage(w, r, "Error occurred when getting edit data.  " , err)
        return
      }
      if dataFromDB.Valid {
        data = dataFromDB.String
        } else {
          data = ""
        }
        docAndStructureSlice = append(docAndStructureSlice, docAndStructure{docData, data})
    }
  }

  var created, modified, firstname, surname string
  var created_by uint64
  sqlStmt = fmt.Sprintf("select `%[1]s`.created, `%[1]s`.modified, `%[2]s`.firstname, `%[2]s`.surname, `%[2]s`.id ", tableName(ds), UsersTable)
  sqlStmt += fmt.Sprintf("from `%[1]s` inner join `%[2]s` on `%[1]s`.created_by = `%[2]s`.id where `%[1]s`.id = ?", tableName(ds), UsersTable)
  err = SQLDB.QueryRow(sqlStmt, docid).Scan(&created, &modified, &firstname, &surname, &created_by)
  if err != nil {
    errorPage(w, r, "An error occured while getting extra read data of this document.", err)
    return
  }


  if r.Method == http.MethodGet {
    type Context struct {
      Created string
      Modified string
      DocumentStructure string
      DocAndStructures []docAndStructure
      Id string
      FirstName string
      Surname string
      CreatedBy uint64
      UpdatePerm bool
      DeletePerm bool
    }

    updatePerm, err := DoesCurrentUserHavePerm(r, ds, "update")
    if err != nil {
      errorPage(w, r, "Error occured while determining if the user have update permission for this document structure.  " , err)
      return
    }
    deletePerm, err := DoesCurrentUserHavePerm(r, ds, "delete")
    if err != nil {
      errorPage(w, r, "Error occured while determining if the user have delete permission for this document structure.  " , err)
      return
    }
    uocPerm, err := DoesCurrentUserHavePerm(r, ds, "update-only-created")
    if err != nil {
      errorPage(w, r, "Error occured while determining if the user have update-only-created permission for this document.  " , err)
      return
    }

    if ! updatePerm {
      if uocPerm && createdBy == useridUint64 {
        updatePerm = true
      }
    }
    ctx := Context{created, modified, ds, docAndStructureSlice, docid, firstname, surname, created_by, updatePerm, deletePerm}
    fullTemplatePath := filepath.Join(getProjectPath(), "templates/edit-document.html")
    tmpl := template.Must(template.ParseFiles(getBaseTemplate(), fullTemplatePath))
    tmpl.Execute(w, ctx)

  } else if r.Method == http.MethodPost {
    tv2, err2 := DoesCurrentUserHavePerm(r, ds, "update")
    if err2 != nil {
      errorPage(w, r, "Error checking for permissions for this page.  " , err)
      return
    }
    uocPerm, err := DoesCurrentUserHavePerm(r, ds, "update-only-created")
    if err != nil {
      errorPage(w, r, "Error checking for permissions of this page.  " , err)
      return
    }

    if ! tv2 {
      if uocPerm && createdBy != useridUint64 {
        errorPage(w, r, "You are not the owner of this document. So can't update it.", nil)
        return
      } else if ! uocPerm {
        errorPage(w, r, "You don't have permissions to update this document.", nil)
        return
      }
    }

    // first check if it passes the extra code validation for this document.
    r.ParseForm()
    fData := make(map[string]string)
    for k := range r.PostForm {
      fData[k] = r.FormValue(k)
    }
    jsonString, err := json.Marshal(fData)

    ec, ectv := getEC(ds)
    if ectv && ec.ValidationFn != nil {
      outString := ec.ValidationFn(string(jsonString))
      if outString != "" {
        fmt.Fprintf(w, "Exra Code Validation Error: " + outString)
        return
      }
    }

    colNames := make([]string, 0)
    formData := make([]string, 0)
    for _, docAndStructure := range docAndStructureSlice {
      if docAndStructure.Data != r.FormValue(docAndStructure.DocData.Name) {
        colNames = append(colNames, docAndStructure.DocData.Name)
        switch docAndStructure.DocData.Type {
        case "Text", "Data", "Email", "Read Only", "URL", "Select", "Date", "Datetime":
          data := fmt.Sprintf("\"%s\"", template.HTMLEscapeString(r.FormValue(docAndStructure.DocData.Name)))
          formData = append(formData, data)
        case "Check":
          var data string
          if r.FormValue(docAndStructure.DocData.Name) == "on" {
            data = "\"t\""
          } else {
            data = "\"f\""
          }
          formData = append(formData, data)
        default:
          formData = append(formData, template.HTMLEscapeString(r.FormValue(docAndStructure.DocData.Name)))
        }
      }
    }

    updatePartStmt := make([]string, 0)
    updatePartStmt = append(updatePartStmt, "modified = now()")
    for i := 0; i < len(colNames); i++ {
      stmt1 := fmt.Sprintf("%s = %s", colNames[i], formData[i])
      updatePartStmt = append(updatePartStmt, stmt1)
    }

    sqlStmt := fmt.Sprintf("update `%s` set %s where id = %s", tableName(ds), strings.Join(updatePartStmt, ", "), docid)
    _, err = SQLDB.Exec(sqlStmt)
    if err != nil {
      errorPage(w, r, "An error occured while saving: " , err)
      return
    }

    // post save extra code
    if ectv && ec.AfterUpdateFn != nil {
      docidUint64, _ := strconv.ParseUint(docid, 10, 64)
      ec.AfterUpdateFn(docidUint64)
    }

    redirectURL := fmt.Sprintf("/doc/%s/list/", ds)
    http.Redirect(w, r, redirectURL, 307)
  }

}


func innerListDocuments(w http.ResponseWriter, r *http.Request, readSqlStmt, rocSqlStmt, readTotalSql, rocTotalSql, listType string) {
  useridUint64, err := GetCurrentUser(r)
  if err != nil {
    errorPage(w, r, "You need to be logged in to continue.", err)
    return
  }

  vars := mux.Vars(r)
  ds := vars["document-structure"]
  page := vars["page"]
  var pageI uint64
  if page != "" {
    pageI, err = strconv.ParseUint(page, 10, 64)
    if err != nil {
      errorPage(w, r, "The page number is invalid.  " , err)
      return
    }
  } else {
    pageI = 1
  }

  detv, err := docExists(ds)
  if err != nil {
    errorPage(w, r, "Error occurred while determining if this document exists.", err)
    return
  }
  if detv == false {
    errorPage(w, r, fmt.Sprintf("The document structure %s does not exists.", ds), nil)
    return
  }

  tv1, err1 := DoesCurrentUserHavePerm(r, ds, "read")
  tv2, err2 := DoesCurrentUserHavePerm(r, ds, "read-only-created")
  if err1 != nil || err2 != nil {
    errorPage(w, r, "Error occured while determining if the user have read permission for this page.", nil)
    return
  }

  if ! tv1 && ! tv2 {
    errorPage(w, r, "You don't have the read permission for this document structure.", nil)
    return
  }

  var sqlStmt string
  if tv1 {
    sqlStmt = readTotalSql
  } else if tv2 {
    sqlStmt = rocTotalSql
  }

  var count uint64
  err = SQLDB.QueryRow(sqlStmt).Scan(&count)
  if count == 0 {
    errorPage(w, r, "There are no documents to display.", nil)
    return
  }

  var id int
  err = SQLDB.QueryRow("select id from qf_document_structures where name = ?", ds).Scan(&id)
  if err != nil {
    errorPage(w, r, "An internal error occured.  " , err)
  }

  colNames, err := getColumnNames(ds)
  if err != nil {
    errorPage(w, r, "Error getting column names.", err)
    return
  }

  var itemsPerPage uint64 = 50
  startIndex := (pageI - 1) * itemsPerPage
  totalItems := count
  totalPages := math.Ceil( float64(totalItems) / float64(itemsPerPage) )

  ids := make([]uint64, 0)
  var idd uint64

  // variables point
  if tv1 {
    sqlStmt = readSqlStmt
  } else if tv2 {
    sqlStmt = rocSqlStmt
  }

  rows, err := SQLDB.Query(sqlStmt, startIndex, itemsPerPage)
  if err != nil {
    errorPage(w, r, "Error reading this document structure data.  " , err)
    return
  }
  defer rows.Close()
  for rows.Next() {
    err := rows.Scan(&idd)
    if err != nil {
      errorPage(w, r, "Error reading a row of data for this document structure.  " , err)
      return
    }
    ids = append(ids, idd)
  }
  if err = rows.Err(); err != nil {
    errorPage(w, r, "Extra error occurred while reading this document structure data.  " , err)
    return
  }

  uocPerm, err1 := DoesCurrentUserHavePerm(r, ds, "update-only-created")
  docPerm, err2 := DoesCurrentUserHavePerm(r, ds, "delete-only-created")
  if err1 != nil || err2 != nil {
    errorPage(w, r, "Error occured while determining if the user have read permission for this page.", nil)
    return
  }

  myRows := make([]Row, 0)
  for _, id := range ids {
    colAndDatas := make([]ColAndData, 0)
    for _, col := range colNames {
      var data string
      var dataFromDB sql.NullString
      sqlStmt := fmt.Sprintf("select %s from `%s` where id = %d", col, tableName(ds), id)
      err := SQLDB.QueryRow(sqlStmt).Scan(&dataFromDB)
      if err != nil {
        errorPage(w, r, "An internal error occured.  " , err)
        return
      }
      if dataFromDB.Valid {
        data = dataFromDB.String
      } else {
        data = ""
      }
      colAndDatas = append(colAndDatas, ColAndData{col, data})
    }

    var createdBy uint64
    sqlStmt := fmt.Sprintf("select created_by from `%s` where id = %d", tableName(ds), id)
    err := SQLDB.QueryRow(sqlStmt).Scan(&createdBy)
    if err != nil {
      errorPage(w, r, "An internal error occured.  " , err)
      return
    }
    rup := false
    rdp := false
    if createdBy == useridUint64 && uocPerm {
      rup = true
    }
    if createdBy == useridUint64 && docPerm {
      rdp = true
    }
    myRows = append(myRows, Row{id, colAndDatas, rup, rdp})
  }


  type Context struct {
    DocumentStructure string
    ColNames []string
    MyRows []Row
    CurrentPage uint64
    Pages []uint64
    CreatePerm bool
    UpdatePerm bool
    DeletePerm bool
    HasApprovals bool
    Approver bool
    ListType string
    OptionalDate string
  }

  pages := make([]uint64, 0)
  for i := uint64(0); i < uint64(totalPages); i++ {
    pages = append(pages, i+1)
  }

  tv1, err1 = DoesCurrentUserHavePerm(r, ds, "create")
  tv2, err2 = DoesCurrentUserHavePerm(r, ds, "update")
  tv3, err3 := DoesCurrentUserHavePerm(r, ds, "delete")
  if err1 != nil || err2 != nil || err3 != nil {
    errorPage(w, r, "An error occurred when getting permissions of this object for this user.", nil)
    return
  }

  userRoles, err := GetCurrentUserRoles(r)
  if err != nil {
    errorPage(w, r, "Error occured when getting current user roles.  " , err)
    return
  }
  approvers, err := getApprovers(ds)
  if err != nil {
    errorPage(w, r, "Error occurred when getting approval list of this document stucture.  " , err)
    return
  }
  var hasApprovals, approver bool
  if len(approvers) == 0 {
    hasApprovals = false
  } else {
    hasApprovals = true
  }
  outerLoop:
    for _, apr := range approvers {
      for _, role := range userRoles {
        if role == apr {
          approver = true
          break outerLoop
        }
      }
    }

  var date string
  if listType == "date-list" {
    date = vars["date"]
  } else {
    date = ""
  }

  ctx := Context{ds, colNames, myRows, pageI, pages, tv1, tv2, tv3, hasApprovals,
    approver, listType, date}
  fullTemplatePath := filepath.Join(getProjectPath(), "templates/list-documents.html")
  tmpl := template.Must(template.ParseFiles(getBaseTemplate(), fullTemplatePath))
  tmpl.Execute(w, ctx)
}


func listDocuments(w http.ResponseWriter, r *http.Request) {
  useridUint64, err := GetCurrentUser(r)
  if err != nil {
    errorPage(w, r, "You need to be logged in to continue.", err)
    return
  }

  vars := mux.Vars(r)
  ds := vars["document-structure"]

  readSqlStmt := fmt.Sprintf("select id from `%s` order by created desc limit ?, ?", tableName(ds))
  rocSqlStmt := fmt.Sprintf("select id from `%s` where created_by = %d order by created desc limit ?, ?", tableName(ds), useridUint64 )
  readTotalSql := fmt.Sprintf("select count(*) from `%s`", tableName(ds))
  rocTotalSql := fmt.Sprintf("select count(*) from `%s` where created_by = %d", tableName(ds), useridUint64)
  innerListDocuments(w, r, readSqlStmt, rocSqlStmt, readTotalSql, rocTotalSql, "true-list")
  return
}


func deleteApproversData(documentStructure string, dsid string) error {
  approvers, err := getApprovers(documentStructure)
  if err != nil {
    return err
  }

  for _, step := range approvers {
    _, err = SQLDB.Exec(fmt.Sprintf("delete from `%s` where docid = ?", getApprovalTable(documentStructure, step)), dsid)
    if err != nil {
      return err
    }
  }
  return nil
}


func deleteDocument(w http.ResponseWriter, r *http.Request) {
  useridUint64, err := GetCurrentUser(r)
  if err != nil {
    errorPage(w, r, "You need to be logged in to continue.", err)
    return
  }

  vars := mux.Vars(r)
  ds := vars["document-structure"]
  docid := vars["id"]
  _, err = strconv.ParseUint(docid, 10, 64)
  if err != nil {
    errorPage(w, r, "Document ID is invalid.", err)
  }

  detv, err := docExists(ds)
  if err != nil {
    errorPage(w, r, "Error occurred while determining if this document exists.", err)
    return
  }
  if detv == false {
    errorPage(w, r, fmt.Sprintf("The document structure %s does not exists.", ds), nil)
    return
  }

  var count uint64
  sqlStmt := fmt.Sprintf("select count(*) from `%s` where id = %s", tableName(ds), docid)
  err = SQLDB.QueryRow(sqlStmt).Scan(&count)
  if count == 0 {
    errorPage(w, r, fmt.Sprintf("The document with id %s do not exists", docid), nil)
    return
  }

  deletePerm, err := DoesCurrentUserHavePerm(r, ds, "delete")
  if err != nil {
    errorPage(w, r, "Error occured while determining if the user have delete permission.  " , err)
    return
  }
  docPerm, err := DoesCurrentUserHavePerm(r, ds, "delete-only-created")
  if err != nil {
    errorPage(w, r, "Error occurred while determining if the user have delete-only-created permission.  " , err)
  }

  colNames, err := getColumnNames(ds)
  if err != nil {
    errorPage(w, r, "Error getting column names.", err)
    return
  }

  fData := make(map[string]string)
  for _, colName := range colNames {
    var data string
    var dataFromDB sql.NullString
    sqlStmt := fmt.Sprintf("select %s from `%s` where id = %s", colName, tableName(ds), docid)
    err := SQLDB.QueryRow(sqlStmt).Scan(&dataFromDB)
    if err != nil {
      errorPage(w, r, "An internal error occured.  " , err)
      return
    }
    if dataFromDB.Valid {
      data = dataFromDB.String
    } else {
      data = ""
    }
    fData[colName] = data
  }
  jsonString, _ := json.Marshal(fData)
  ec, ectv := getEC(ds)

  if deletePerm {
    err = deleteApproversData(ds, docid)
    if err != nil {
      errorPage(w, r, "Error deleting approval data for this document.  " , err)
      return
    }

    sqlStmt = fmt.Sprintf("delete from `%s` where id = %s", tableName(ds), docid)
    _, err = SQLDB.Exec(sqlStmt)
    if err != nil {
      errorPage(w, r, "An error occured while deleting this document: " , err)
      return
    }

    if ectv && ec.AfterDeleteFn != nil {
      ec.AfterDeleteFn(string(jsonString))
    }

  } else if docPerm {

    var createdBy uint64
    sqlStmt := fmt.Sprintf("select created_by from `%s` where id = %s", tableName(ds), docid)
    err := SQLDB.QueryRow(sqlStmt).Scan(&createdBy)
    if err != nil {
      errorPage(w, r, "An internal error occured.  " , err)
      return
    }

    if createdBy == useridUint64 {
      err = deleteApproversData(ds, docid)
      if err != nil {
        errorPage(w, r, "Error deleting approval data for this document.  " , err)
        return
      }

      sqlStmt = fmt.Sprintf("delete from `%s` where id = %s", tableName(ds), docid)
      _, err = SQLDB.Exec(sqlStmt)
      if err != nil {
        errorPage(w, r, "An error occured while deleting this document: " , err)
        return
      }

      if ectv && ec.AfterDeleteFn != nil {
        ec.AfterDeleteFn(string(jsonString))
      }

    } else {
      errorPage(w, r, "You don't have permissions to delete this document.", nil)
      return
    }

  } else {

    errorPage(w, r, "You don't have the delete permission for this document structure.", nil)
    return
  }

  redirectURL := fmt.Sprintf("/doc/%s/list/", ds)
  http.Redirect(w, r, redirectURL, 307)
}
