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
  "errors"
)


func innerDeleteDocument(r *http.Request, docid string, deleteFile bool) error {
  useridUint64, err := GetCurrentUser(r)
  if err != nil {
    return err
  }

  vars := mux.Vars(r)
  ds := vars["document-structure"]

  docidUint64, err := strconv.ParseUint(docid, 10, 64)
  if err != nil {
    return err
  }

  detv, err := docExists(ds)
  if err != nil {
    return err
  }
  if detv == false {
    return errors.New(fmt.Sprintf("The document structure %s does not exists.", ds))
  }

  tblName, err := tableName(ds)
  if err != nil {
    return err
  }

  var count uint64
  sqlStmt := fmt.Sprintf("select count(*) from `%s` where id = %s", tblName, docid)
  err = SQLDB.QueryRow(sqlStmt).Scan(&count)
  if count == 0 {
    return errors.New(fmt.Sprintf("The document with id %s do not exists", docid))
  }

  deletePerm, err := DoesCurrentUserHavePerm(r, ds, "delete")
  if err != nil {
    return err
  }
  docPerm, err := DoesCurrentUserHavePerm(r, ds, "delete-only-created")
  if err != nil {
    return err
  }

  var id int
  err = SQLDB.QueryRow("select id from qf_document_structures where fullname = ?", ds).Scan(&id)
  if err != nil {
    return err
  }

  ec, ectv := getEC(ds)

  dds, err := GetDocData(ds)
  if err != nil {
    return err
  }

  var colNames []string
  for _, dd := range dds {
    colNames = append(colNames, dd.Name)
  }

  fData := make(map[string]string)
  for _, colName := range colNames {
    var data string
    var dataFromDB sql.NullString
    sqlStmt := fmt.Sprintf("select %s from `%s` where id = %s", colName, tblName, docid)
    err := SQLDB.QueryRow(sqlStmt).Scan(&dataFromDB)
    if err != nil {
      return err
    }
    if dataFromDB.Valid {
      data = dataFromDB.String
    } else {
      data = ""
    }
    fData[colName] = data
  }

  var createdBy uint64
  sqlStmt = fmt.Sprintf("select created_by from `%s` where id = %s", tblName, docid)
  err = SQLDB.QueryRow(sqlStmt).Scan(&createdBy)
  if err != nil {
    return err
  }

  if deletePerm || (docPerm && createdBy == useridUint64) {
    if ectv && ec.BeforeDeleteFn != nil {
      ec.BeforeDeleteFn(docidUint64)
    }

    approvers, err := getApprovers(ds)
    if err != nil {
      return err
    }

    for _, step := range approvers {
      atn, err := getApprovalTable(ds, step)
      if err != nil {
        return err
      }

      _, err = SQLDB.Exec(fmt.Sprintf("delete from `%s` where docid = ?", atn), docid)
      if err != nil {
        return err
      }
    }

    var ctx context.Context
    var client *storage.Client

    hasForm, err := documentStructureHasForm(ds)
    if hasForm {
      ctx = context.Background()
      client, err = storage.NewClient(ctx)
      if err != nil {
        return err
      }
    }

    for _, dd := range dds {
      if dd.Type == "Table" {
        parts := strings.Split(fData[dd.Name], ",")
        for _, part := range parts {
          ottblName, err := tableName(dd.OtherOptions[0])
          if err != nil {
            return err
          }

          sqlStmt = fmt.Sprintf("delete from `%s` where id = ?", ottblName)
          _, err = SQLDB.Exec(sqlStmt, part)
          if err != nil {
            return err
          }
        }
      }

      if (deleteFile) {
        if dd.Type == "File" || dd.Type == "Image" {
          client.Bucket(QFBucketName).Object(fData[dd.Name]).Delete(ctx)
        }        
      } else {
        if dd.Type == "File" || dd.Type == "Image" {
          sqlStmt = "insert into `qf_files_for_delete` (created_by, filepath) values (?, ?)"
          _, err = SQLDB.Exec(sqlStmt, useridUint64, fData[dd.Name])
          if err != nil {
            return err
          }
        }
      }

    }

    sqlStmt = fmt.Sprintf("delete from `%s` where id = %s", tblName, docid)
    _, err = SQLDB.Exec(sqlStmt)
    if err != nil {
      return err
    }

  } else {
    return errors.New("You don't have the delete permission for this document.")
  }

  return nil
}


func deleteDocument(w http.ResponseWriter, r *http.Request) {
  vars := mux.Vars(r)
  docid := vars["id"]
  ds := vars["document-structure"]

  err := innerDeleteDocument(r, docid, true)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  redirectURL := fmt.Sprintf("/list/%s/", ds)
  http.Redirect(w, r, redirectURL, 307)
}


func deleteSearchResults(w http.ResponseWriter, r *http.Request) {
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


  endSqlStmt, err := parseSearchVariables(r)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  if len(endSqlStmt) == 0 {
    errorPage(w, "Your query is empty.")
    return
  }

  tblName, err := tableName(ds)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  var toDeleteDocId string
  sqlStmt := fmt.Sprintf("select id from `%s` where ", tblName) + strings.Join(endSqlStmt, " and ")
  rows, err := SQLDB.Query(sqlStmt)
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  defer rows.Close()
  for rows.Next() {
    err := rows.Scan(&toDeleteDocId)
    if err != nil {
      errorPage(w, err.Error())
      return
    }

    err = innerDeleteDocument(r, toDeleteDocId, false)
    if err != nil {
      errorPage(w, err.Error())
      return
    }
  }
  err = rows.Err()
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  hasForm, err := documentStructureHasForm(ds)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  var redirectURL string
  if hasForm {
    redirectURL = "/complete-files-delete/"
  } else {
    redirectURL = fmt.Sprintf("/list/%s/", ds)    
  }
  
  http.Redirect(w, r, redirectURL, 307)
}
