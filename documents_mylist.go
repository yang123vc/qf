package qf

import (
  "net/http"
  "github.com/gorilla/mux"
  "html/template"
  "strings"
  "fmt"
)


func getMyListConfig(r *http.Request) (map[string][]string, error){
  vars := mux.Vars(r)
  ds := vars["document-structure"]

  userid, err := GetCurrentUser(r)
  if err != nil {
    return nil, err
  }

  dsid, err := getDocumentStructureID(ds)
  if err != nil {
    return nil, err
  }

  sqlStmt := "select field, data from qf_mylistoptions where userid = ? and dsid = ?"
  rows, err := SQLDB.Query(sqlStmt, userid, dsid)
  if err != nil {
    return nil, err
  }
  defer rows.Close()
  var field, data string

  ret := make(map[string][]string)
  for rows.Next() {
    err := rows.Scan(&field, &data)
    if err != nil {
      return nil, err
    }
    val, ok := ret[field]
    if ! ok {
      ret[field] = []string{data}
    } else {
      val = append(val, data)
      ret[field] = val
    }
  }
  if err = rows.Err(); err != nil {
    return nil, err
  }
  return ret, nil
}


func myListSetup(w http.ResponseWriter, r *http.Request) {
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

  tv1, err := DoesCurrentUserHavePerm(r, ds, "read")
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  if ! tv1 {
    errorPage(w, "You don't have the read permission for this document structure.")
    return
  }

  listConfigs, err := getMyListConfig(r)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  userid, err := GetCurrentUser(r)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  dsid, err := getDocumentStructureID(ds)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  var fields string
  err = SQLDB.QueryRow("select group_concat(name order by view_order asc separator ',,,') from qf_fields where dsid = ?", dsid).Scan(&fields)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  if r.Method == http.MethodGet {

    type Context struct {
      ListConfigs map[string][]string
      DocumentStructure string
      UserId uint64
      Fields []string
    }

    ctx := Context{listConfigs, ds, userid, strings.Split(fields, ",,,")}
    tmpl := template.Must(template.ParseFiles(getBaseTemplate(), "qffiles/mylist-config.html"))
    tmpl.Execute(w, ctx)

  } else {

    field := r.FormValue("field")
    data := r.FormValue("data")

    count := 0
    sqlStmt := "select count(*) from qf_mylistoptions where userid=? and dsid=? and field=? and data=?"
    err := SQLDB.QueryRow(sqlStmt, userid, dsid, field, data).Scan(&count)
    if err != nil {
      errorPage(w, err.Error())
      return
    }

    if count == 0 {
      sqlStmt = "insert into qf_mylistoptions (userid, dsid, field, data) values(?,?,?,?)"
      _, err := SQLDB.Exec(sqlStmt, userid, dsid, field, data)
      if err != nil {
        errorPage(w, err.Error())
        return
      }
    }

    http.Redirect(w, r, fmt.Sprintf("/mylist/%s/", ds), 307)
  }
}


func removeOneMylistConfig(w http.ResponseWriter, r *http.Request) {
  vars := mux.Vars(r)
  ds := vars["document-structure"]
  field := vars["field"]
  data := vars["data"]

  detv, err := docExists(ds)
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  if detv == false {
    errorPage(w, fmt.Sprintf("The document structure %s does not exists.", ds))
    return
  }

  tv1, err := DoesCurrentUserHavePerm(r, ds, "read")
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  if ! tv1 {
    errorPage(w, "You don't have the read permission for this document structure.")
    return
  }

  userid, err := GetCurrentUser(r)
  if err != nil {
    errorPage(w, "You don't have the read permission for this document structure.")
    return
  }

  dsid, err := getDocumentStructureID(ds)
  if err != nil {
    errorPage(w, "You don't have the read permission for this document structure.")
    return
  }

  sqlStmt := "delete from qf_mylistoptions where userid=? and dsid=? and field=? and data=?"
  _, err = SQLDB.Exec(sqlStmt, userid, dsid, field, data)
  if err != nil {
    errorPage(w, "You don't have the read permission for this document structure.")
    return
  }

  http.Redirect(w, r, fmt.Sprintf("/mylist-setup/%s/", ds), 307)
}
