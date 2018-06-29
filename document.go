package qf

import (
  "net/http"
  "fmt"
  "path/filepath"
  "strings"
  "html/template"
  "github.com/gorilla/mux"
  "encoding/json"
  "os/exec"
  "strconv"
  "database/sql"
  "math"
)


func CreateDocument(w http.ResponseWriter, r *http.Request) {
  useridUint64, err := GetCurrentUser(r)
  if err != nil {
    fmt.Fprintf(w, "You need to be logged in to continue. Exact Error: " + err.Error())
    return
  }

  vars := mux.Vars(r)
  doc := vars["document-structure"]

  detv, err := docExists(doc)
  if err != nil {
    fmt.Fprintf(w, "Error occurred while determining if this document exists. Exact Error: " + err.Error())
    return
  }
  if detv == false {
    fmt.Fprintf(w, "The document structure %s does not exists.", doc)
    return
  }

  truthValue, err := doesCurrentUserHavePerm(r, doc, "create")
  if err != nil {
    fmt.Fprintf(w, "Error occured while determining if the user have permission for this page. Exact Error: " + err.Error())
    return
  }
  if ! truthValue {
    fmt.Fprintf(w, "You don't have the create permission for this document structure.")
    return
  }


  var id int
  err = SQLDB.QueryRow("select id from qf_document_structures where name = ?", doc).Scan(&id)
  if err != nil {
    panic(err)
  }
  cmdString := fmt.Sprintf("qfec%d", id)

  dds := GetDocData(id)

  if r.Method == http.MethodGet {
    type Context struct {
      DocName string
      DDs []DocData
    }
    ctx := Context{doc, dds}
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
    jsonString, err := json.Marshal(fData)

    _, err = exec.LookPath(cmdString)
    if err == nil {
      out, err := exec.Command(cmdString, "v", string(jsonString)).Output()
      if err == nil && string(out) != "" {
        fmt.Fprintln(w, "Extra Code Validation Error: " + string(out))
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
          data = fmt.Sprintf("\"%s\"", r.FormValue(dd.Name))
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
          data = r.FormValue(dd.Name)
        }
        formData = append(formData, data)
      }
    }
    colNamesStr := strings.Join(colNames, ", ")
    formDataStr := strings.Join(formData, ", ")
    sqlStmt := fmt.Sprintf("insert into `%s`(created, modified, created_by, %s) values(now(), now(), %d, %s)", tableName(doc), colNamesStr, useridUint64, formDataStr)
    res, err := SQLDB.Exec(sqlStmt)
    if err != nil {
      fmt.Fprintf(w, "An error occured while saving: " + err.Error())
      return
    }

    // new document extra code
    lastid, err := res.LastInsertId()
    if err != nil {
      fmt.Fprintf(w, "An error occured while trying to run extra code: " + err.Error())
    }
    _, err = exec.LookPath(cmdString)
    if err == nil {
      exec.Command(cmdString, "n", strconv.FormatInt(lastid, 10)).Run()
    }

    redirectURL := fmt.Sprintf("/doc/%s/list/", doc)
    http.Redirect(w, r, redirectURL, 307)
  }

}


func UpdateDocument(w http.ResponseWriter, r *http.Request) {
  useridUint64, err := GetCurrentUser(r)
  if err != nil {
    fmt.Fprintf(w, "You need to be logged in to continue. Exact Error: " + err.Error())
    return
  }

  vars := mux.Vars(r)
  doc := vars["document-structure"]
  docid := vars["id"]

  detv, err := docExists(doc)
  if err != nil {
    fmt.Fprintf(w, "Error occurred while determining if this document exists. Exact Error: " + err.Error())
    return
  }
  if detv == false {
    fmt.Fprintf(w, "The document structure %s does not exists.", doc)
    return
  }

  readPerm, err := doesCurrentUserHavePerm(r, doc, "read")
  if err != nil {
    fmt.Fprintf(w, "Error occured while determining if the user have read permission for this document structure. Exact Error: " + err.Error())
    return
  }
  rocPerm, err := doesCurrentUserHavePerm(r, doc, "read-only-created")
  if err != nil {
    fmt.Fprintf(w, "Error occured while determining if the user have read-only-created permission for this document. Exact Error: " + err.Error())
    return
  }

  var createdBy uint64
  sqlStmt := fmt.Sprintf("select created_by from `%s` where id = %s", tableName(doc), docid)
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
      fmt.Fprintf(w, "You don't have the update permission for this document structure.")
      return
    }
  }

  var count uint64
  sqlStmt = fmt.Sprintf("select count(*) from `%s` where id = %s", tableName(doc), docid)
  err = SQLDB.QueryRow(sqlStmt).Scan(&count)
  if count == 0 {
    fmt.Fprintf(w, "The document with id %s do not exists", docid)
    return
  }

  var id int
  err = SQLDB.QueryRow("select id from qf_document_structures where name = ?", doc).Scan(&id)
  if err != nil {
    fmt.Fprintf(w, "An error occurred when reading document structure. Exact Error" + err.Error())
    return
  }
  cmdString := fmt.Sprintf("qfec%d", id)

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
      sqlStmt := fmt.Sprintf("select %s from `%s` where id = %s", docData.Name, tableName(doc), docid)
      err := SQLDB.QueryRow(sqlStmt).Scan(&dataFromDB)
      if err != nil {
        fmt.Fprintf(w, "Error occurred when getting edit data. Exact Error: " + err.Error())
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
  sqlStmt = fmt.Sprintf("select `%[1]s`.created, `%[1]s`.modified, `%[2]s`.firstname, `%[2]s`.surname, `%[2]s`.id ", tableName(doc), UsersTable)
  sqlStmt += fmt.Sprintf("from `%[1]s` inner join `%[2]s` on `%[1]s`.created_by = `%[2]s`.id where `%[1]s`.id = ?", tableName(doc), UsersTable)
  err = SQLDB.QueryRow(sqlStmt, docid).Scan(&created, &modified, &firstname, &surname, &created_by)
  if err != nil {
    panic(err)
  }


  if r.Method == http.MethodGet {
    type Context struct {
      Created string
      Modified string
      DocName string
      DocAndStructures []docAndStructure
      Id string
      FirstName string
      Surname string
      CreatedBy uint64
      UpdatePerm bool
      DeletePerm bool
    }

    updatePerm, err := doesCurrentUserHavePerm(r, doc, "update")
    if err != nil {
      fmt.Fprintf(w, "Error occured while determining if the user have update permission for this document structure. Exact Error: " + err.Error())
      return
    }
    deletePerm, err := doesCurrentUserHavePerm(r, doc, "delete")
    if err != nil {
      fmt.Fprintf(w, "Error occured while determining if the user have delete permission for this document structure. Exact Error: " + err.Error())
      return
    }
    uocPerm, err := doesCurrentUserHavePerm(r, doc, "update-only-created")
    if err != nil {
      fmt.Fprintf(w, "Error occured while determining if the user have update-only-created permission for this document. Exact Error: " + err.Error())
      return
    }

    if ! updatePerm {
      if uocPerm && createdBy == useridUint64 {
        updatePerm = true
      }
    }
    ctx := Context{created, modified, doc, docAndStructureSlice, docid, firstname, surname, created_by, updatePerm, deletePerm}
    fullTemplatePath := filepath.Join(getProjectPath(), "templates/edit-document.html")
    tmpl := template.Must(template.ParseFiles(getBaseTemplate(), fullTemplatePath))
    tmpl.Execute(w, ctx)

  } else if r.Method == http.MethodPost {
    tv2, err2 := doesCurrentUserHavePerm(r, doc, "update")
    if err2 != nil {
      fmt.Fprintf(w, "Error checking for permissions for this page. Exact Error: " + err.Error())
      return
    }
    uocPerm, err := doesCurrentUserHavePerm(r, doc, "update-only-created")
    if err != nil {
      fmt.Fprintf(w, "Error checking for permissions of this page. Exact Error: " + err.Error())
      return
    }

    if ! tv2 {
      if uocPerm && createdBy != useridUint64 {
        fmt.Fprintf(w, "You are not the owner of this document. So can't update it.")
        return
      } else if ! uocPerm {
        fmt.Fprintf(w, "You don't have permissions to update this document.")
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

    _, err = exec.LookPath(cmdString)
    if err == nil {
      out, err := exec.Command(cmdString, "v", string(jsonString)).Output()
      if err == nil && string(out) != "" {
        fmt.Fprintln(w, "Extra Code Validation Error: " + string(out))
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
          data := fmt.Sprintf("\"%s\"", r.FormValue(docAndStructure.DocData.Name))
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
          formData = append(formData, r.FormValue(docAndStructure.DocData.Name))
        }
      }
    }

    updatePartStmt := make([]string, 0)
    updatePartStmt = append(updatePartStmt, "modified = now()")
    for i := 0; i < len(colNames); i++ {
      stmt1 := fmt.Sprintf("%s = %s", colNames[i], formData[i])
      updatePartStmt = append(updatePartStmt, stmt1)
    }

    sqlStmt := fmt.Sprintf("update `%s` set %s where id = %s", tableName(doc), strings.Join(updatePartStmt, ", "), docid)
    _, err = SQLDB.Exec(sqlStmt)
    if err != nil {
      fmt.Fprintf(w, "An error occured while saving: " + err.Error())
      return
    }

    // post save extra code
    _, err = exec.LookPath(cmdString)
    if err == nil {
      exec.Command(cmdString, "u", docid).Run()
    }

    redirectURL := fmt.Sprintf("/doc/%s/list/", doc)
    http.Redirect(w, r, redirectURL, 307)
  }

}


func ListDocuments(w http.ResponseWriter, r *http.Request) {
  useridUint64, err := GetCurrentUser(r)
  if err != nil {
    fmt.Fprintf(w, "You need to be logged in to continue. Exact Error: " + err.Error())
    return
  }

  vars := mux.Vars(r)
  doc := vars["document-structure"]
  page := vars["page"]
  var pageI uint64
  if page != "" {
    pageI, err = strconv.ParseUint(page, 10, 64)
    if err != nil {
      fmt.Fprintf(w, "The page number is invalid. Exact Error: " + err.Error())
      return
    }
  } else {
    pageI = 1
  }

  detv, err := docExists(doc)
  if err != nil {
    fmt.Fprintf(w, "Error occurred while determining if this document exists. Exact Error: " + err.Error())
    return
  }
  if detv == false {
    fmt.Fprintf(w, "The document structure %s does not exists.", doc)
    return
  }

  tv1, err1 := doesCurrentUserHavePerm(r, doc, "read")
  tv2, err2 := doesCurrentUserHavePerm(r, doc, "read-only-created")
  if err1 != nil || err2 != nil {
    fmt.Fprintf(w, "Error occured while determining if the user have read permission for this page.")
    return
  }

  if ! tv1 && ! tv2 {
    fmt.Fprintf(w, "You don't have the read permission for this document structure.")
    return
  }

  var count uint64
  sqlStmt := fmt.Sprintf("select count(*) from `%s`", tableName(doc))
  err = SQLDB.QueryRow(sqlStmt).Scan(&count)
  if count == 0 {
    fmt.Fprintf(w, "There are no documents to display.")
    return
  }

  var id int
  err = SQLDB.QueryRow("select id from qf_document_structures where name = ?", doc).Scan(&id)
  if err != nil {
    fmt.Fprintf(w, "An internal error occured. Exact Error: " + err.Error())
  }

  colNames := make([]string, 0)
  var colName string
  rows, err := SQLDB.Query("select name from qf_fields where dsid = ? and type != \"Section Break\" order by id asc limit 3", id)
  if err != nil {
    panic(err)
  }
  defer rows.Close()
  for rows.Next() {
    err := rows.Scan(&colName)
    if err != nil {
      panic(err)
    }
    colNames = append(colNames, colName)
  }
  if err = rows.Err(); err != nil {
    panic(err)
  }
  colNames = append(colNames, "created", "created_by")

  var itemsPerPage uint64 = 50
  startIndex := (pageI - 1) * itemsPerPage
  totalItems := count
  totalPages := math.Ceil( float64(totalItems) / float64(itemsPerPage) )

  ids := make([]uint64, 0)
  var idd uint64
  if tv2 && tv1 {
    sqlStmt = fmt.Sprintf("select id from `%s` order by created desc limit ?, ?", tableName(doc))
  } else if tv2 {
    sqlStmt = fmt.Sprintf("select id from `%s` where created_by = %d order by created desc limit ?, ?", tableName(doc), useridUint64 )
  } else {
    sqlStmt = fmt.Sprintf("select id from `%s` order by created desc limit ?, ?", tableName(doc))
  }
  rows, err = SQLDB.Query(sqlStmt, startIndex, itemsPerPage)
  if err != nil {
    panic(err)
  }
  defer rows.Close()
  for rows.Next() {
    err := rows.Scan(&idd)
    if err != nil {
      panic(err)
    }
    ids = append(ids, idd)
  }
  if err = rows.Err(); err != nil {
    panic(err)
  }

  type ColAndData struct {
    ColName string
    Data string
  }

  type Row struct {
    Id uint64
    ColAndDatas []ColAndData
    RowUpdatePerm bool
    RowDeletePerm bool
  }

  uocPerm, err1 := doesCurrentUserHavePerm(r, doc, "update-only-created")
  docPerm, err2 := doesCurrentUserHavePerm(r, doc, "delete-only-created")
  if err1 != nil || err2 != nil {
    fmt.Fprintf(w, "Error occured while determining if the user have read permission for this page.")
    return
  }

  myRows := make([]Row, 0)
  for _, id := range ids {
    colAndDatas := make([]ColAndData, 0)
    for _, col := range colNames {
      var data string
      var dataFromDB sql.NullString
      sqlStmt := fmt.Sprintf("select %s from `%s` where id = %d", col, tableName(doc), id)
      err := SQLDB.QueryRow(sqlStmt).Scan(&dataFromDB)
      if err != nil {
        fmt.Fprintf(w, "An internal error occured. Exact Error: " + err.Error())
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
    sqlStmt := fmt.Sprintf("select created_by from `%s` where id = %d", tableName(doc), id)
    err := SQLDB.QueryRow(sqlStmt).Scan(&createdBy)
    if err != nil {
      fmt.Fprintf(w, "An internal error occured. Exact Error: " + err.Error())
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
    DocName string
    ColNames []string
    MyRows []Row
    CurrentPage uint64
    Pages []uint64
    CreatePerm bool
    UpdatePerm bool
    DeletePerm bool
    HasApprovals bool
    Approver bool
  }
  pages := make([]uint64, 0)
  for i := uint64(0); i < uint64(totalPages); i++ {
    pages = append(pages, i+1)
  }

  tv1, err1 = doesCurrentUserHavePerm(r, doc, "create")
  tv2, err2 = doesCurrentUserHavePerm(r, doc, "update")
  tv3, err3 := doesCurrentUserHavePerm(r, doc, "delete")
  if err1 != nil || err2 != nil || err3 != nil {
    fmt.Fprintf(w, "An error occurred when getting permissions of this object for this user.")
    return
  }

  userRoles, err := GetCurrentUserRoles(r)
  if err != nil {
    fmt.Fprintf(w, "Error occured when getting user roles. Exact Error: " + err.Error())
    return
  }
  approvers, err := getApprovers(doc)
  if err != nil {
    fmt.Fprintf(w, "Error occurred when getting approval list of this document stucture. Exact Error: " + err.Error())
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

  ctx := Context{doc, colNames, myRows, pageI, pages, tv1, tv2, tv3, hasApprovals, approver}
  fullTemplatePath := filepath.Join(getProjectPath(), "templates/list-documents.html")
  tmpl := template.Must(template.ParseFiles(getBaseTemplate(), fullTemplatePath))
  tmpl.Execute(w, ctx)
}


func DeleteDocument(w http.ResponseWriter, r *http.Request) {
  useridUint64, err := GetCurrentUser(r)
  if err != nil {
    fmt.Fprintf(w, "You need to be logged in to continue. Exact Error: " + err.Error())
    return
  }

  vars := mux.Vars(r)
  doc := vars["document-structure"]
  docid := vars["id"]

  detv, err := docExists(doc)
  if err != nil {
    fmt.Fprintf(w, "Error occurred while determining if this document exists. Exact Error: " + err.Error())
    return
  }
  if detv == false {
    fmt.Fprintf(w, "The document structure %s does not exists.", doc)
    return
  }

  var count uint64
  sqlStmt := fmt.Sprintf("select count(*) from `%s` where id = %s", tableName(doc), docid)
  err = SQLDB.QueryRow(sqlStmt).Scan(&count)
  if count == 0 {
    fmt.Fprintf(w, "The document with id %s do not exists", docid)
    return
  }

  deletePerm, err := doesCurrentUserHavePerm(r, doc, "delete")
  if err != nil {
    fmt.Fprintf(w, "Error occured while determining if the user have delete permission. Exact Error: " + err.Error())
    return
  }
  docPerm, err := doesCurrentUserHavePerm(r, doc, "delete-only-created")
  if err != nil {
    fmt.Fprintf(w, "Error occurred while determining if the user have delete-only-created permission. Exact Error: " + err.Error())
  }

  if deletePerm {
    sqlStmt = fmt.Sprintf("delete from `%s` where id = %s", tableName(doc), docid)
    _, err = SQLDB.Exec(sqlStmt)
    if err != nil {
      fmt.Fprintf(w, "An error occured while deleting this document: " + err.Error())
      return
    }

  } else if docPerm {

    var createdBy uint64
    sqlStmt := fmt.Sprintf("select created_by from `%s` where id = %s", tableName(doc), docid)
    err := SQLDB.QueryRow(sqlStmt).Scan(&createdBy)
    if err != nil {
      fmt.Fprintf(w, "An internal error occured. Exact Error: " + err.Error())
      return
    }

    if createdBy == useridUint64 {
      sqlStmt = fmt.Sprintf("delete from `%s` where id = %s", tableName(doc), docid)
      _, err = SQLDB.Exec(sqlStmt)
      if err != nil {
        fmt.Fprintf(w, "An error occured while deleting this document: " + err.Error())
        return
      }
    } else {
      fmt.Fprintf(w, "You don't have permissions to delete this document.")
      return
    }

  } else {

    fmt.Fprintf(w, "You don't have the delete permission for this document structure.")
    return
  }

  redirectURL := fmt.Sprintf("/doc/%s/list/", doc)
  http.Redirect(w, r, redirectURL, 307)
}
