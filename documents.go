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
  "html"
)


func createDocument(w http.ResponseWriter, r *http.Request) {
  useridUint64, err := GetCurrentUser(r)
  if err != nil {
    errorPage(w, "You need to be logged in to continue.", err)
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

  truthValue, err := DoesCurrentUserHavePerm(r, ds, "create")
  if err != nil {
    errorPage(w, "Error occured while determining if the user have permission for this page.  " , err)
    return
  }
  if ! truthValue {
    errorPage(w, "You don't have the create permission for this document structure.", nil)
    return
  }


  var id int
  var helpText sql.NullString
  err = SQLDB.QueryRow("select id, help_text from qf_document_structures where fullname = ?", ds).Scan(&id, &helpText)
  if err != nil {
    errorPage(w, "An internal error occured.", err)
    return
  }

  var htStr string
  if helpText.Valid {
    htStr = strings.Replace(helpText.String, "\n", "<br>", -1)
  } else {
    htStr = ""
  }

  ue := func(s string) template.HTML {
    return template.HTML(s)
  }

  dds := GetDocData(id)

  tableFields := make(map[string][]DocData)
  for _, dd := range dds {
    if dd.Type != "Table" {
      continue
    }
    ct := dd.OtherOptions[0]
    var ctid int
    err = SQLDB.QueryRow("select id from qf_document_structures where name = ?", ct).Scan(&ctid)
    if err != nil {
      errorPage(w, "An error occurred while getting child table form structure.", err)
      return
    }
    tableFields[ct] = GetDocData(ctid)
  }

  if r.Method == http.MethodGet {
    type Context struct {
      DocumentStructure string
      DDs []DocData
      HelpText string
      UndoEscape func(s string) template.HTML
      TableFields map[string][]DocData
    }

    ctx := Context{ds, dds, htStr, ue, tableFields}
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
        errorPage(w, "Exra Code Validation Error: " + outString, nil)
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
          data = fmt.Sprintf("\"%s\"", html.EscapeString(r.FormValue(dd.Name)))
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
      case "Table":
        childTableName := dd.OtherOptions[0]
        ddsCT := tableFields[childTableName]
        rowCount := r.FormValue("rows-count-for-" + dd.Name)
        rowCountInt, _ := strconv.Atoi(rowCount)
        rowIds := make([]string, 0)
        for j := 1; j < rowCountInt + 1; j++ {
          colNamesCT := make([]string, 0)
          formDataCT := make([]string, 0)
          jStr := strconv.Itoa(j)
          for _, ddCT := range ddsCT {
            colNamesCT = append(colNamesCT, ddCT.Name)
            tempData := r.FormValue(ddCT.Name + "-" + jStr)
            switch ddCT.Type {
            case "Text", "Data", "Email", "Read Only", "URL", "Select", "Date", "Datetime":
              var data string
              if tempData == "" {
                data = "null"
              } else {
                data = fmt.Sprintf("\"%s\"", html.EscapeString(tempData))
              }
              formDataCT = append(formDataCT, data)
            case "Check":
              var data string
              if tempData == "on" {
                data = "\"t\""
              } else {
                data = "\"f\""
              }
              formDataCT = append(formDataCT, data)
            default:
              var data string
              if tempData == "" {
                data = "null"
              } else {
                data = html.EscapeString(tempData)
              }
              formDataCT = append(formDataCT, data)
            }
          }
          ctblName, err := tableName(childTableName)
          if err != nil {
            errorPage(w, "Error getting child table db name", err)
            return
          }
          sqlStmt := fmt.Sprintf("insert into `%s`(%s) values (%s)", ctblName,
            strings.Join(colNamesCT, ", "), strings.Join(formDataCT, ", "))
          res, err := SQLDB.Exec(sqlStmt)
          if err != nil {
            errorPage(w, "An error occured while saving a child table row." , err)
            return
          }
          lastid, err := res.LastInsertId()
          if err != nil {
            errorPage(w, "An error occured while trying to run extra code: " , err)
            return
          }

          rowIds = append(rowIds, strconv.FormatInt(lastid, 10))
        }
        formData = append(formData, fmt.Sprintf("\"%s\"", strings.Join(rowIds, ",")))
      default:
        var data string
        if r.FormValue(dd.Name) == "" {
          data = "null"
        } else {
          data = html.EscapeString(r.FormValue(dd.Name))
        }
        formData = append(formData, data)
      }
    }

    tblName, err := tableName(ds)
    if err != nil {
      errorPage(w, "Error getting document structure's table name.", err)
      return
    }

    colNamesStr := strings.Join(colNames, ", ")
    formDataStr := strings.Join(formData, ", ")
    sqlStmt := fmt.Sprintf("insert into `%s`(created, modified, created_by, %s) values(now(), now(), %d, %s)",
      tblName, colNamesStr, useridUint64, formDataStr)
    res, err := SQLDB.Exec(sqlStmt)
    if err != nil {
      errorPage(w, "An error occured while saving." , err)
      return
    }

    // new document extra code
    lastid, err := res.LastInsertId()
    if err != nil {
      errorPage(w, "An error occured while trying to run extra code: " , err)
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
    errorPage(w, "You need to be logged in to continue.", err)
    return
  }

  vars := mux.Vars(r)
  ds := vars["document-structure"]
  docid := vars["id"]
  _, err = strconv.ParseUint(docid, 10, 64)
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
    errorPage(w, "Error occured while determining if the user have read permission for this document structure.  " , err)
    return
  }
  rocPerm, err := DoesCurrentUserHavePerm(r, ds, "read-only-created")
  if err != nil {
    errorPage(w, "Error occured while determining if the user have read-only-created permission for this document.  " , err)
    return
  }

  tblName, err := tableName(ds)
  if err != nil {
    errorPage(w, "Error getting document structure's table name.", err)
    return
  }

  var createdBy uint64
  sqlStmt := fmt.Sprintf("select created_by from `%s` where id = %s", tblName, docid)
  err = SQLDB.QueryRow(sqlStmt).Scan(&createdBy)
  if err != nil {
    errorPage(w, "An internal error occured.  " , err)
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

  var count uint64
  sqlStmt = fmt.Sprintf("select count(*) from `%s` where id = %s", tblName, docid)
  err = SQLDB.QueryRow(sqlStmt).Scan(&count)
  if count == 0 {
    errorPage(w, fmt.Sprintf("The document with id %s do not exists", docid), nil)
    return
  }

  var id int
  var helpText sql.NullString
  err = SQLDB.QueryRow("select id, help_text from qf_document_structures where name = ?", ds).Scan(&id, &helpText)
  if err != nil {
    errorPage(w, "An error occurred when reading document structure. Exact Error" , err)
    return
  }

  var htStr string
  if helpText.Valid {
    htStr = strings.Replace(helpText.String, "\n", "<br>", -1)
  } else {
    htStr = ""
  }

  ue := func(s string) template.HTML {
    return template.HTML(s)
  }

  docDatas := GetDocData(id)

  type docAndStructure struct {
    DocData
    Data string
  }

  docAndStructureSlice := make([]docAndStructure, 0)
  tableData := make(map[string][][]docAndStructure)

  for _, docData := range docDatas {
    if docData.Type == "Section Break" {
      docAndStructureSlice = append(docAndStructureSlice, docAndStructure{docData, ""})
    } else {
      var data string
      var dataFromDB sql.NullString
      sqlStmt := fmt.Sprintf("select %s from `%s` where id = %s", docData.Name, tblName, docid)
      err := SQLDB.QueryRow(sqlStmt).Scan(&dataFromDB)
      if err != nil {
        errorPage(w, "Error occurred when getting edit data.  " , err)
        return
      }
      if dataFromDB.Valid {
        data = html.UnescapeString(dataFromDB.String)
      } else {
        data = ""
      }
      docAndStructureSlice = append(docAndStructureSlice, docAndStructure{docData, data})
      if docData.Type == "Table" {
        childTable := docData.OtherOptions[0]
        var ctid int
        err = SQLDB.QueryRow("select id from qf_document_structures where name = ?", childTable).Scan(&ctid)
        if err != nil {
          errorPage(w, "An error occurred while getting child table form structure.", err)
          return
        }
        ctdds := GetDocData(ctid)
        dASSuper := make([][]docAndStructure, 0)

        parts := strings.Split(data, ",")
        for _, part := range parts {
          docAndStructureSliceCT := make([]docAndStructure, 0)
          for _, ctdd := range ctdds {
            ctblName, err := tableName(childTable)
            if err != nil {
              errorPage(w, "Error getting child table db name", err)
              return
            }

            var data string
            var dataFromDB sql.NullString

            sqlStmt := fmt.Sprintf("select %s from `%s` where id = ?", ctdd.Name, ctblName)
            err = SQLDB.QueryRow(sqlStmt, part).Scan(&dataFromDB)
            if err != nil {
              errorPage(w, "Error occurred when getting edit child table data." , err)
              return
            }
            if dataFromDB.Valid {
              data = html.UnescapeString(dataFromDB.String)
            } else {
              data = ""
            }
            docAndStructureSliceCT = append(docAndStructureSliceCT, docAndStructure{ctdd, data})
          }
          dASSuper = append(dASSuper, docAndStructureSliceCT)
        }
        tableData[docData.Name] = dASSuper
      }
    }
  }

  var created, modified, firstname, surname string
  var created_by uint64
  sqlStmt = fmt.Sprintf("select `%[1]s`.created, `%[1]s`.modified, `%[2]s`.firstname, `%[2]s`.surname, `%[2]s`.id ", tblName, UsersTable)
  sqlStmt += fmt.Sprintf("from `%[1]s` inner join `%[2]s` on `%[1]s`.created_by = `%[2]s`.id where `%[1]s`.id = ?", tblName, UsersTable)
  err = SQLDB.QueryRow(sqlStmt, docid).Scan(&created, &modified, &firstname, &surname, &created_by)
  if err != nil {
    errorPage(w, "An error occured while getting extra read data of this document.", err)
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
      HelpText string
      UndoEscape func(s string) template.HTML
      TableData map[string][][]docAndStructure
      Add func(x,y int) int
    }

    add := func(x, y int) int {
      return x + y
    }

    updatePerm, err := DoesCurrentUserHavePerm(r, ds, "update")
    if err != nil {
      errorPage(w, "Error occured while determining if the user have update permission for this document structure.  " , err)
      return
    }
    deletePerm, err := DoesCurrentUserHavePerm(r, ds, "delete")
    if err != nil {
      errorPage(w, "Error occured while determining if the user have delete permission for this document structure.  " , err)
      return
    }
    uocPerm, err := DoesCurrentUserHavePerm(r, ds, "update-only-created")
    if err != nil {
      errorPage(w, "Error occured while determining if the user have update-only-created permission for this document.  " , err)
      return
    }

    if ! updatePerm {
      if uocPerm && createdBy == useridUint64 {
        updatePerm = true
      }
    }
    ctx := Context{created, modified, ds, docAndStructureSlice, docid, firstname, surname,
      created_by, updatePerm, deletePerm, htStr, ue, tableData, add}
    fullTemplatePath := filepath.Join(getProjectPath(), "templates/update-document.html")
    tmpl := template.Must(template.ParseFiles(getBaseTemplate(), fullTemplatePath))
    tmpl.Execute(w, ctx)

  } else if r.Method == http.MethodPost {
    tv2, err2 := DoesCurrentUserHavePerm(r, ds, "update")
    if err2 != nil {
      errorPage(w, "Error checking for permissions for this page.  " , err)
      return
    }
    uocPerm, err := DoesCurrentUserHavePerm(r, ds, "update-only-created")
    if err != nil {
      errorPage(w, "Error checking for permissions of this page.  " , err)
      return
    }

    if ! tv2 {
      if uocPerm && createdBy != useridUint64 {
        errorPage(w, "You are not the owner of this document. So can't update it.", nil)
        return
      } else if ! uocPerm {
        errorPage(w, "You don't have permissions to update this document.", nil)
        return
      }
    }

    r.ParseForm()

    // first check if it passes the extra code validation for this document.
    fData := make(map[string]string)
    for k := range r.PostForm {
      fData[k] = r.FormValue(k)
    }
    jsonString, err := json.Marshal(fData)

    ec, ectv := getEC(ds)
    if ectv && ec.ValidationFn != nil {
      outString := ec.ValidationFn(string(jsonString))
      if outString != "" {
        errorPage(w, "Exra Code Validation Error: " + outString, nil)
        return
      }
    }

    colNames := make([]string, 0)
    formData := make([]string, 0)
    for _, docAndStructure := range docAndStructureSlice {
      if docAndStructure.DocData.Type == "Table" {
        // delete old table data
        parts := strings.Split(docAndStructure.Data, ",")
        for _, part := range parts {
          ottblName, err := tableName(docAndStructure.DocData.OtherOptions[0])
          if err != nil {
            errorPage(w, "Error getting table name of the table in other options.", nil)
            return
          }

          sqlStmt = fmt.Sprintf("delete from `%s` where id = ?", ottblName)
          _, err = SQLDB.Exec(sqlStmt, part)
          if err != nil {
            errorPage(w, "An error occurred deleting child table data.", err)
            return
          }
        }

        // add new table data
        childTableName := docAndStructure.DocData.OtherOptions[0]
        var ctid int
        err = SQLDB.QueryRow("select id from qf_document_structures where name = ?", childTableName).Scan(&ctid)
        if err != nil {
          errorPage(w, "An error occurred while getting child table form structure.", err)
          return
        }
        ddsCT := GetDocData(ctid)

        rowCount := r.FormValue("rows-count-for-" + docAndStructure.DocData.Name)
        rowCountInt, _ := strconv.Atoi(rowCount)
        rowIds := make([]string, 0)
        for j := 1; j < rowCountInt + 1; j++ {
          colNamesCT := make([]string, 0)
          formDataCT := make([]string, 0)
          jStr := strconv.Itoa(j)
          for _, ddCT := range ddsCT {
            colNamesCT = append(colNamesCT, ddCT.Name)
            tempData := r.FormValue(ddCT.Name + "-" + jStr)
            switch ddCT.Type {
            case "Text", "Data", "Email", "Read Only", "URL", "Select", "Date", "Datetime":
              var data string
              if tempData == "" {
                data = "null"
              } else {
                data = fmt.Sprintf("\"%s\"", html.EscapeString(tempData))
              }
              formDataCT = append(formDataCT, data)
            case "Check":
              var data string
              if tempData == "on" {
                data = "\"t\""
              } else {
                data = "\"f\""
              }
              formDataCT = append(formDataCT, data)
            default:
              var data string
              if tempData == "" {
                data = "null"
              } else {
                data = html.EscapeString(tempData)
              }
              formDataCT = append(formDataCT, data)
            }
          }
          ctblName, err := tableName(childTableName)
          if err != nil {
            errorPage(w, "Error getting document structure's table name.", err)
            return
          }
          sqlStmt := fmt.Sprintf("insert into `%s`(%s) values (%s)", ctblName,
            strings.Join(colNamesCT, ", "), strings.Join(formDataCT, ", "))
          res, err := SQLDB.Exec(sqlStmt)
          if err != nil {
            errorPage(w, "An error occured while saving a child table row." , err)
            return
          }
          lastid, err := res.LastInsertId()
          if err != nil {
            errorPage(w, "An error occured while trying to run extra code: " , err)
            return
          }

          rowIds = append(rowIds, strconv.FormatInt(lastid, 10))
        }
        colNames = append(colNames, docAndStructure.DocData.Name)
        formData = append(formData, fmt.Sprintf("\"%s\"", strings.Join(rowIds, ",")))

      } else if docAndStructure.Data != html.EscapeString(r.FormValue(docAndStructure.DocData.Name)) {

        colNames = append(colNames, docAndStructure.DocData.Name)
        switch docAndStructure.DocData.Type {
        case "Text", "Data", "Email", "Read Only", "URL", "Select", "Date", "Datetime":
          data := fmt.Sprintf("\"%s\"", html.EscapeString(r.FormValue(docAndStructure.DocData.Name)))
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
          formData = append(formData, html.EscapeString(r.FormValue(docAndStructure.DocData.Name)))
        }
      }
    }

    updatePartStmt := make([]string, 0)
    updatePartStmt = append(updatePartStmt, "modified = now()")
    for i := 0; i < len(colNames); i++ {
      stmt1 := fmt.Sprintf("%s = %s", colNames[i], formData[i])
      updatePartStmt = append(updatePartStmt, stmt1)
    }

    sqlStmt := fmt.Sprintf("update `%s` set %s where id = %s", tblName, strings.Join(updatePartStmt, ", "), docid)
    _, err = SQLDB.Exec(sqlStmt)
    if err != nil {
      errorPage(w, "An error occured while saving: " , err)
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


func deleteApproversData(documentStructure string, dsid string) error {
  approvers, err := getApprovers(documentStructure)
  if err != nil {
    return err
  }

  for _, step := range approvers {
    atn, err := getApprovalTable(documentStructure, step)
    if err != nil {
      return err
    }

    _, err = SQLDB.Exec(fmt.Sprintf("delete from `%s` where docid = ?", atn), dsid)
    if err != nil {
      return err
    }
  }
  return nil
}


func deleteDocument(w http.ResponseWriter, r *http.Request) {
  useridUint64, err := GetCurrentUser(r)
  if err != nil {
    errorPage(w, "You need to be logged in to continue.", err)
    return
  }

  vars := mux.Vars(r)
  ds := vars["document-structure"]
  docid := vars["id"]
  _, err = strconv.ParseUint(docid, 10, 64)
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

  deletePerm, err := DoesCurrentUserHavePerm(r, ds, "delete")
  if err != nil {
    errorPage(w, "Error occured while determining if the user have delete permission.  " , err)
    return
  }
  docPerm, err := DoesCurrentUserHavePerm(r, ds, "delete-only-created")
  if err != nil {
    errorPage(w, "Error occurred while determining if the user have delete-only-created permission.  " , err)
  }

  var columns string
  err = SQLDB.QueryRow("select group_concat(name separator ',') from qf_fields where dsid=?").Scan(&columns)
  if err != nil {
    errorPage(w, "Error getting column names.", err)
    return
  }
  colNames := strings.Split(columns, ",")

  fData := make(map[string]string)
  for _, colName := range colNames {
    var data string
    var dataFromDB sql.NullString
    sqlStmt := fmt.Sprintf("select %s from `%s` where id = %s", colName, tblName, docid)
    err := SQLDB.QueryRow(sqlStmt).Scan(&dataFromDB)
    if err != nil {
      errorPage(w, "An internal error occured.  " , err)
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

  var id int
  err = SQLDB.QueryRow("select id from qf_document_structures where fullname = ?", ds).Scan(&id)
  if err != nil {
    errorPage(w, "An internal error occured.", err)
    return
  }

  dds := GetDocData(id)

  if deletePerm {
    err = deleteApproversData(ds, docid)
    if err != nil {
      errorPage(w, "Error deleting approval data for this document.  " , err)
      return
    }

    for _, dd := range dds {
      if dd.Type != "Table" {
        continue
      }

      parts := strings.Split(fData[dd.Name], ",")
      for _, part := range parts {
        ottblName, err := tableName(dd.OtherOptions[0])
        if err != nil {
          errorPage(w, "Error getting table name of the other options document structure", nil)
          return
        }

        sqlStmt = fmt.Sprintf("delete from `%s` where id = ?", ottblName)
        _, err = SQLDB.Exec(sqlStmt, part)
        if err != nil {
          errorPage(w, "An error occurred deleting child table data.", err)
          return
        }
      }
    }

    sqlStmt = fmt.Sprintf("delete from `%s` where id = %s", tblName, docid)
    _, err = SQLDB.Exec(sqlStmt)
    if err != nil {
      errorPage(w, "An error occured while deleting this document: " , err)
      return
    }

    if ectv && ec.AfterDeleteFn != nil {
      ec.AfterDeleteFn(string(jsonString))
    }

  } else if docPerm {

    var createdBy uint64
    sqlStmt := fmt.Sprintf("select created_by from `%s` where id = %s", tblName, docid)
    err := SQLDB.QueryRow(sqlStmt).Scan(&createdBy)
    if err != nil {
      errorPage(w, "An internal error occured.  " , err)
      return
    }

    if createdBy == useridUint64 {
      err = deleteApproversData(ds, docid)
      if err != nil {
        errorPage(w, "Error deleting approval data for this document.  " , err)
        return
      }

      for _, dd := range dds {
        if dd.Type != "Table" {
          continue
        }

        parts := strings.Split(fData[dd.Name], ",")
        for _, part := range parts {
          ottblName, err := tableName(dd.OtherOptions[0])
          if err != nil {
            errorPage(w, "Error occured getting the table name of the other options document structure.", err)
            return
          }
          sqlStmt = fmt.Sprintf("delete from `%s` where id = ?", ottblName)
          _, err = SQLDB.Exec(sqlStmt, part)
          if err != nil {
            errorPage(w, "An error occurred deleting child table data.", err)
            return
          }
        }
      }

      sqlStmt = fmt.Sprintf("delete from `%s` where id = %s", tblName, docid)
      _, err = SQLDB.Exec(sqlStmt)
      if err != nil {
        errorPage(w, "An error occured while deleting this document: " , err)
        return
      }

      if ectv && ec.AfterDeleteFn != nil {
        ec.AfterDeleteFn(string(jsonString))
      }

    } else {
      errorPage(w, "You don't have permissions to delete this document.", nil)
      return
    }

  } else {

    errorPage(w, "You don't have the delete permission for this document structure.", nil)
    return
  }

  redirectURL := fmt.Sprintf("/doc/%s/list/", ds)
  http.Redirect(w, r, redirectURL, 307)
}
