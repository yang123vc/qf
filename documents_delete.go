package qf

import (
  "net/http"
  "github.com/gorilla/mux"
  "strconv"
  "fmt"
  "database/sql"
  "golang.org/x/net/context"
  "cloud.google.com/go/storage"
  "strings"
)


func deleteDocument(w http.ResponseWriter, r *http.Request) {
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

  deletePerm, err := DoesCurrentUserHavePerm(r, ds, "delete")
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  docPerm, err := DoesCurrentUserHavePerm(r, ds, "delete-only-created")
  if err != nil {
    errorPage(w, err.Error())
  }

  var id int
  err = SQLDB.QueryRow("select id from qf_document_structures where fullname = ?", ds).Scan(&id)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  var columns string
  err = SQLDB.QueryRow("select group_concat(name separator ',') from qf_fields where dsid = ?", id).Scan(&columns)
  if err != nil {
    errorPage(w, err.Error())
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
      errorPage(w, err.Error())
      return
    }
    if dataFromDB.Valid {
      data = dataFromDB.String
    } else {
      data = ""
    }
    fData[colName] = data
  }

  ec, ectv := getEC(ds)

  dds, err := GetDocData(ds)
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

  if deletePerm || (docPerm && createdBy == useridUint64) {
    if ectv && ec.BeforeDeleteFn != nil {
      ec.BeforeDeleteFn(docidUint64)
    }

    approvers, err := getApprovers(ds)
    if err != nil {
      errorPage(w, err.Error())
      return
    }

    for _, step := range approvers {
      atn, err := getApprovalTable(ds, step)
      if err != nil {
        errorPage(w, err.Error())
        return
      }

      _, err = SQLDB.Exec(fmt.Sprintf("delete from `%s` where docid = ?", atn), docid)
      if err != nil {
        errorPage(w, err.Error())
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

    for _, dd := range dds {
      if dd.Type == "Table" {
        parts := strings.Split(fData[dd.Name], ",")
        for _, part := range parts {
          ottblName, err := tableName(dd.OtherOptions[0])
          if err != nil {
            errorPage(w, err.Error())
            return
          }

          sqlStmt = fmt.Sprintf("delete from `%s` where id = ?", ottblName)
          _, err = SQLDB.Exec(sqlStmt, part)
          if err != nil {
            errorPage(w, err.Error())
            return
          }
        }
      }

      if dd.Type == "File" || dd.Type == "Image" {
        client.Bucket(QFBucketName).Object(fData[dd.Name]).Delete(ctx)
      }

    }

    sqlStmt = fmt.Sprintf("delete from `%s` where id = %s", tblName, docid)
    _, err = SQLDB.Exec(sqlStmt)
    if err != nil {
      errorPage(w, err.Error())
      return
    }

  } else {
    errorPage(w, "You don't have the delete permission for this document.")
    return
  }

  redirectURL := fmt.Sprintf("/doc/%s/list/", ds)
  http.Redirect(w, r, redirectURL, 307)
}


func deleteFile(w http.ResponseWriter, r *http.Request) {
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

  deletePerm, err := DoesCurrentUserHavePerm(r, ds, "delete")
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  docPerm, err := DoesCurrentUserHavePerm(r, ds, "delete-only-created")
  if err != nil {
    errorPage(w, err.Error())
  }

  var createdBy uint64
  sqlStmt = fmt.Sprintf("select created_by from `%s` where id = %s", tblName, docid)
  err = SQLDB.QueryRow(sqlStmt).Scan(&createdBy)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  ctx := context.Background()
  client, err := storage.NewClient(ctx)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  if deletePerm || (docPerm && createdBy == useridUint64) {
    var toDeleteFileName string
    sqlStmt = fmt.Sprintf("select %s from `%s` where id = %s", vars["name"], tblName, docid)
    err = SQLDB.QueryRow(sqlStmt).Scan(&toDeleteFileName)
    if err != nil {
      errorPage(w, err.Error())
      return
    }
    err = client.Bucket(QFBucketName).Object(toDeleteFileName).Delete(ctx)
    if err != nil {
      errorPage(w, err.Error())
      return
    }
    sqlStmt = fmt.Sprintf("update `%s` set %s = null, modified = now() where id = %s",
      tblName, vars["name"], docid)
    _, err = SQLDB.Exec(sqlStmt)
    if err != nil {
      errorPage(w, err.Error())
      return
    }
  }

  redirectURL := fmt.Sprintf("/doc/%s/update/%s/", ds, docid)
  http.Redirect(w, r, redirectURL, 307)
}
