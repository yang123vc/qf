package qf

import (
  "net/http"
  "fmt"
  "path/filepath"
  "strings"
  "html/template"
  "github.com/gorilla/mux"
  "strconv"
  "database/sql"
  "html"
  "golang.org/x/net/context"
  "cloud.google.com/go/storage"
  "io"
  "io/ioutil"
  "time"
)


func createDocument(w http.ResponseWriter, r *http.Request) {
  useridUint64, err := GetCurrentUser(r)
  if err != nil {
    errorPage(w, err.Error())
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

  truthValue, err := DoesCurrentUserHavePerm(r, ds, "create")
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  if ! truthValue {
    errorPage(w, "You don't have the create permission for this document structure.")
    return
  }

  var aliasName string
  isAlias, ptdsid, err := DSIdAliasPointsTo(ds)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  if isAlias {
    aliasName = ds
    err = SQLDB.QueryRow("select fullname from qf_document_structures where id = ?", ptdsid).Scan(&ds)
    if err != nil {
      errorPage(w, err.Error())
      return
    }
  }

  var helpText sql.NullString
  err = SQLDB.QueryRow("select help_text from qf_document_structures where fullname = ?", ds).Scan(&helpText)
  if err != nil {
    errorPage(w, err.Error())
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

  dds, err := GetDocData(ds)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  tableFields := make(map[string][]DocData)
  for _, dd := range dds {
    if dd.Type != "Table" {
      continue
    }
    ct := dd.OtherOptions[0]
    tableFields[ct], err = GetDocData(ct)
    if err != nil {
      errorPage(w, err.Error())
      return
    }
  }

  if r.Method == http.MethodGet {
    type Context struct {
      DocumentStructure string
      DDs []DocData
      HelpText string
      UndoEscape func(s string) template.HTML
      TableFields map[string][]DocData
    }

    var ctx Context
    if isAlias {
      ctx = Context{aliasName, dds, htStr, ue, tableFields}
    } else {
      ctx = Context{ds, dds, htStr, ue, tableFields}
    }
    fullTemplatePath := filepath.Join(getProjectPath(), "templates/create-document.html")
    tmpl := template.Must(template.ParseFiles(getBaseTemplate(), fullTemplatePath))
    tmpl.Execute(w, ctx)

  } else if r.Method == http.MethodPost {

    r.FormValue("email")

    // first check if it passes the extra code validation for this document.
    ec, ectv := getEC(ds)
    if ectv && ec.ValidationFn != nil {
      outString := ec.ValidationFn(r.PostForm)
      if outString != "" {
        errorPage(w, "Exra Code Validation Error: " + outString)
        return
      }
    }

    var ctx context.Context
    var client *storage.Client

    hasForm, err := documentStructureHasForm(ds)
    if hasForm {
      ctx = context.Background()
      client, err = storage.NewClient(ctx)
      if err != nil {
        errorPage(w, err.Error())
        return
      }
    }

    var tblName string
    if isAlias {
      tblName, err = tableName(aliasName)
      if err != nil {
        errorPage(w, err.Error())
        return
      }
    } else {
      tblName, err = tableName(ds)
      if err != nil {
        errorPage(w, err.Error())
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
            errorPage(w, err.Error())
            return
          }
          sqlStmt := fmt.Sprintf("insert into `%s`(%s) values (%s)", ctblName,
            strings.Join(colNamesCT, ", "), strings.Join(formDataCT, ", "))
          res, err := SQLDB.Exec(sqlStmt)
          if err != nil {
            errorPage(w, err.Error())
            return
          }
          lastid, err := res.LastInsertId()
          if err != nil {
            errorPage(w, err.Error())
            return
          }

          rowIds = append(rowIds, strconv.FormatInt(lastid, 10))
        }
        formData = append(formData, fmt.Sprintf("\"%s\"", strings.Join(rowIds, ",")))
      case "File", "Image":
        file, handle, err := r.FormFile(dd.Name)
        if err != nil {
          formData = append(formData, "null")
          continue
        }
        defer file.Close()

        // ctx := context.Background()
        var newFileName string
        for {
          var extension string
          if strings.HasSuffix(handle.Filename, "tar.gz") {
            extension = ".tar.gz"
          } else if strings.HasSuffix(handle.Filename, "tar.xz") {
            extension = ".tar.xz"
          } else {
            extension = filepath.Ext(handle.Filename)
          }

          randomFileName := filepath.Join(tblName, untestedRandomString(100) + extension)
          objHandle := client.Bucket(QFBucketName).Object(randomFileName)
          _, err := objHandle.NewReader(ctx)
          if err == nil {
            continue
          }

          wc := objHandle.NewWriter(ctx)
          if _, err := io.Copy(wc, file); err != nil {
            errorPage(w, err.Error())
            return
          }
          if err := wc.Close(); err != nil {
            errorPage(w, err.Error())
            return
          }
          newFileName = randomFileName
          break
        }
        data := fmt.Sprintf("\"%s\"", newFileName)
        formData = append(formData, data)
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

    colNamesStr := strings.Join(colNames, ", ")
    formDataStr := strings.Join(formData, ", ")
    sqlStmt := fmt.Sprintf("insert into `%s`(created, modified, created_by, %s) values(now(), now(), %d, %s)",
      tblName, colNamesStr, useridUint64, formDataStr)
    res, err := SQLDB.Exec(sqlStmt)
    if err != nil {
      errorPage(w, err.Error())
      return
    }

    // new document extra code
    lastid, err := res.LastInsertId()
    if err != nil {
      errorPage(w, err.Error())
      return
    }

    if ectv && ec.AfterCreateFn != nil {
      ec.AfterCreateFn(uint64(lastid))
    }

    var redirectURL string
    if isAlias {
      redirectURL = fmt.Sprintf("/doc/%s/list/", aliasName)
    } else {
      redirectURL = fmt.Sprintf("/doc/%s/list/", ds)
    }
    http.Redirect(w, r, redirectURL, 307)
  }

}


func updateDocument(w http.ResponseWriter, r *http.Request) {
  useridUint64, err := GetCurrentUser(r)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  vars := mux.Vars(r)
  ds := vars["document-structure"]
  docid := vars["id"]
  _, err = strconv.ParseUint(docid, 10, 64)
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

  updatePerm, err := DoesCurrentUserHavePerm(r, ds, "update")
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  deletePerm, err := DoesCurrentUserHavePerm(r, ds, "delete")
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  uocPerm, err := DoesCurrentUserHavePerm(r, ds, "update-only-created")
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  var aliasName string
  isAlias, ptdsid, err := DSIdAliasPointsTo(ds)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  if isAlias {
    aliasName = ds
    err = SQLDB.QueryRow("select fullname from qf_document_structures where id = ?", ptdsid).Scan(&ds)
    if err != nil {
      errorPage(w, err.Error())
      return
    }
  }

  var tblName string
  if isAlias {
    tblName, err = tableName(aliasName)
    if err != nil {
      errorPage(w, err.Error())
      return
    }
  } else {
    tblName, err = tableName(ds)
    if err != nil {
      errorPage(w, err.Error())
      return
    }
  }

  var createdBy uint64
  sqlStmt := fmt.Sprintf("select created_by from `%s` where id = %s", tblName, docid)
  err = SQLDB.QueryRow(sqlStmt).Scan(&createdBy)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  if ! updatePerm {
    if uocPerm && createdBy == useridUint64 {
      updatePerm = true
    }
  }

  if ! readPerm {
    if rocPerm {
      if createdBy != useridUint64 {
        errorPage(w, "You are not the owner of this document so can't read it.")
        return
      }
    } else {
      errorPage(w, "You don't have the read permission for this document structure.")
      return
    }
  }

  var count uint64
  sqlStmt = fmt.Sprintf("select count(*) from `%s` where id = %s", tblName, docid)
  err = SQLDB.QueryRow(sqlStmt).Scan(&count)
  if count == 0 {
    errorPage(w, fmt.Sprintf("The document with id %s do not exists", docid))
    return
  }

  var helpText sql.NullString
  if isAlias {
    err = SQLDB.QueryRow("select help_text from qf_document_structures where fullname = ?", aliasName).Scan(&helpText)
    if err != nil {
      errorPage(w, err.Error())
      return
    }
  } else {
    err = SQLDB.QueryRow("select help_text from qf_document_structures where fullname = ?", ds).Scan(&helpText)
    if err != nil {
      errorPage(w, err.Error())
      return
    }
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

  docDatas, err := GetDocData(ds)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

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
        errorPage(w, err.Error())
        return
      }
      if dataFromDB.Valid {
        data = html.UnescapeString(dataFromDB.String)
      } else {
        data = ""
      }
      if data != "" && (docData.Type == "File" || docData.Type == "Image") {
        pkey, err := ioutil.ReadFile(KeyFilePath)
        if err != nil {
          errorPage(w, err.Error())
          return
        }
        opts := &storage.SignedURLOptions{
          GoogleAccessID: GoogleAccessID,
          PrivateKey: pkey,
          Method: "GET",
          Expires: time.Now().Add(1 * time.Hour),
        }
        viewableFilePath, err := storage.SignedURL(QFBucketName, data, opts)
        if err != nil {
          errorPage(w, err.Error())
          return
        }
        data = viewableFilePath
      }
      docAndStructureSlice = append(docAndStructureSlice, docAndStructure{docData, data})
      if docData.Type == "Table" {
        childTable := docData.OtherOptions[0]
        ctdds, err := GetDocData(childTable)
        if err != nil {
          errorPage(w, err.Error())
          return
        }
        dASSuper := make([][]docAndStructure, 0)

        parts := strings.Split(data, ",")
        for _, part := range parts {
          docAndStructureSliceCT := make([]docAndStructure, 0)
          for _, ctdd := range ctdds {
            ctblName, err := tableName(childTable)
            if err != nil {
              errorPage(w, err.Error())
              return
            }

            var data string
            var dataFromDB sql.NullString

            sqlStmt := fmt.Sprintf("select %s from `%s` where id = ?", ctdd.Name, ctblName)
            err = SQLDB.QueryRow(sqlStmt, part).Scan(&dataFromDB)
            if err != nil {
              errorPage(w, err.Error())
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
    errorPage(w, err.Error())
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
      HasApprovals bool
      Approver bool
    }

    add := func(x, y int) int {
      return x + y
    }

    hasApprovals, err := isApprovalFrameworkInstalled(ds)
    if err != nil {
      errorPage(w, err.Error())
      return
    }
    approver, err := isApprover(r, ds)
    if err != nil {
      errorPage(w, err.Error())
      return
    }

    ctx := Context{created, modified, ds, docAndStructureSlice, docid, firstname, surname,
      created_by, updatePerm, deletePerm, htStr, ue, tableData, add, hasApprovals,
      approver}
    fullTemplatePath := filepath.Join(getProjectPath(), "templates/update-document.html")
    tmpl := template.Must(template.ParseFiles(getBaseTemplate(), fullTemplatePath))
    tmpl.Execute(w, ctx)

  } else if r.Method == http.MethodPost {
    if ! updatePerm {
      errorPage(w, "You don't have permissions to update this document.")
      return
    }

    r.FormValue("email")

    // first check if it passes the extra code validation for this document.
    ec, ectv := getEC(ds)
    if ectv && ec.ValidationFn != nil {
      outString := ec.ValidationFn(r.PostForm)
      if outString != "" {
        errorPage(w, "Exra Code Validation Error: " + outString)
        return
      }
    }

    var ctx context.Context
    var client *storage.Client
    hasForm, err := documentStructureHasForm(ds)
    if hasForm {
      ctx = context.Background()
      client, err = storage.NewClient(ctx)
      if err != nil {
        errorPage(w, err.Error())
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
            errorPage(w, "Error getting table name of the table in other options.")
            return
          }

          sqlStmt = fmt.Sprintf("delete from `%s` where id = ?", ottblName)
          _, err = SQLDB.Exec(sqlStmt, part)
          if err != nil {
            errorPage(w, err.Error())
            return
          }
        }

        // add new table data
        childTableName := docAndStructure.DocData.OtherOptions[0]
        ddsCT, err := GetDocData(childTableName)
        if err != nil {
          errorPage(w, err.Error())
          return
        }

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
            errorPage(w, err.Error())
            return
          }
          sqlStmt := fmt.Sprintf("insert into `%s`(%s) values (%s)", ctblName,
            strings.Join(colNamesCT, ", "), strings.Join(formDataCT, ", "))
          res, err := SQLDB.Exec(sqlStmt)
          if err != nil {
            errorPage(w, err.Error())
            return
          }
          lastid, err := res.LastInsertId()
          if err != nil {
            errorPage(w, err.Error())
            return
          }

          rowIds = append(rowIds, strconv.FormatInt(lastid, 10))
        }
        colNames = append(colNames, docAndStructure.DocData.Name)
        formData = append(formData, fmt.Sprintf("\"%s\"", strings.Join(rowIds, ",")))

      } else if docAndStructure.Type == "Image" || docAndStructure.Type == "File" {
        file, handle, err := r.FormFile(docAndStructure.DocData.Name)
        if err != nil {
          continue
        }
        defer file.Close()

        var newFileName string
        for {
          var extension string
          if strings.HasSuffix(handle.Filename, "tar.gz") {
            extension = ".tar.gz"
          } else if strings.HasSuffix(handle.Filename, "tar.xz") {
            extension = ".tar.xz"
          } else {
            extension = filepath.Ext(handle.Filename)
          }

          randomFileName := filepath.Join(tblName, untestedRandomString(100) + extension)
          objHandle := client.Bucket(QFBucketName).Object(randomFileName)
          _, err := objHandle.NewReader(ctx)
          if err == nil {
            continue
          }

          wc := objHandle.NewWriter(ctx)
          if _, err := io.Copy(wc, file); err != nil {
            errorPage(w, err.Error())
            return
          }
          if err := wc.Close(); err != nil {
            errorPage(w, err.Error())
            return
          }
          newFileName = randomFileName

          // delete any file that was previously stored.
          var datum sql.NullString
          sqlStmt = fmt.Sprintf("select %s from `%s` where id = ?", docAndStructure.DocData.Name, tblName)
          SQLDB.QueryRow(sqlStmt, docid).Scan(&datum)
          if datum.Valid {
            client.Bucket(QFBucketName).Object(datum.String).Delete(ctx)
          }
          break
        }
        colNames = append(colNames, docAndStructure.DocData.Name)
        formData = append(formData, fmt.Sprintf("\"%s\"", newFileName))
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
      errorPage(w, err.Error())
      return
    }

    // post save extra code
    if ectv && ec.AfterUpdateFn != nil {
      docidUint64, _ := strconv.ParseUint(docid, 10, 64)
      ec.AfterUpdateFn(docidUint64)
    }

    var redirectURL string
    if isAlias {
      redirectURL = fmt.Sprintf("/doc/%s/list/", aliasName)
    } else {
      redirectURL = fmt.Sprintf("/doc/%s/list/", ds)
    }
    http.Redirect(w, r, redirectURL, 307)
  }

}
