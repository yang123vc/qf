package qf 

import (
	"net/http"
	"github.com/gorilla/mux"
	"io/ioutil"
  "golang.org/x/net/context"
  "cloud.google.com/go/storage"
  "strings"
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
