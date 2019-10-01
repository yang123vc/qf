package qf 

import (
	"net/http"
	"github.com/gorilla/mux"
	"io/ioutil"
  "golang.org/x/net/context"
  "cloud.google.com/go/storage"
  "strings"
  "fmt"
  "strconv"
  "database/sql"
  "html/template"
)


func serveFileForQF(w http.ResponseWriter, r *http.Request) {
	_, err := GetCurrentUser(r)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

	filePath := r.FormValue("p")
	parts := strings.Split(filePath, "/")
	tableName := parts[0]

	var ds string
	qStmt := "select fullname from qf_document_structures where tbl_name = ?"
	err = SQLDB.QueryRow(qStmt, tableName).Scan(&ds)
	if err != nil {
		errorPage(w, err.Error())
		return
	}

	truthValue, err := DoesCurrentUserHavePerm(r, ds, "read")
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  if ! truthValue {
    errorPage(w, "You don't have the read permission for this document structure.")
    return
  }

  ctx := context.Background()
  client, err := storage.NewClient(ctx)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  
  fr, err := client.Bucket(QFBucketName).Object(filePath).NewReader(ctx)
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  defer fr.Close()

  data, err := ioutil.ReadAll(fr)
	if err != nil {
		errorPage(w, err.Error())
		return
	}

	parts = strings.Split(filePath, FILENAME_SEPARATOR)

	w.Header().Set("Content-Disposition", "attachment; filename=" + parts[1])
	contentType := http.DetectContentType(data)
	w.Header().Set("Content-Type", contentType)
	w.Write(data)
}


func serveJS(w http.ResponseWriter, r *http.Request) {
  vars := mux.Vars(r)
  lib := vars["library"]

  if lib == "jquery" {
    http.ServeFile(w, r, "qffiles/jquery-3.3.1.min.js")
  } else if lib == "autosize" {
    http.ServeFile(w, r, "qffiles/autosize.min.js")
  }
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

  redirectURL := fmt.Sprintf("/update/%s/%s/", ds, docid)
  http.Redirect(w, r, redirectURL, 307)
}


func completeFilesDelete(w http.ResponseWriter, r *http.Request) {
  useridUint64, err := GetCurrentUser(r)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  var count uint64
  err = SQLDB.QueryRow("select count(*) from qf_files_for_delete where created_by = ? ", useridUint64).Scan(&count)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  if count == 0 {
    errorPage(w, "You have nothing to delete.")
    return
  }

  var fps sql.NullString
  err = SQLDB.QueryRow("select group_concat(filePath separator ',,,') from qf_files_for_delete where created_by = ?", useridUint64).Scan(&fps)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  type Context struct {
    FilePaths []string
  }

  ctx := Context{strings.Split(fps.String, ",,,")}
  tmpl := template.Must(template.ParseFiles(getBaseTemplate(), "qffiles/complete-files-delete.html"))
  tmpl.Execute(w, ctx)
}


func deleteFileFromBrowser(w http.ResponseWriter, r *http.Request) {
  _, err := GetCurrentUser(r)
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

  fp := r.FormValue("p")
  err = client.Bucket(QFBucketName).Object(fp).Delete(ctx)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  _, err = SQLDB.Exec("delete from qf_files_for_delete where filepath = ?", fp)
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  
  fmt.Fprintf(w, "ok")
}