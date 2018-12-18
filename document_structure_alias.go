package qf

import (
  "net/http"
  "html/template"
  "path/filepath"
  "strings"
  "strconv"
  "fmt"
  "database/sql"
)

func newDocumentStructureAlias(w http.ResponseWriter, r *http.Request) {
  truthValue, err := isUserAdmin(r)
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  if ! truthValue {
    errorPage(w, "You are not an admin here. You don't have permissions to view this page.")
    return
  }

  var notAliasDSList sql.NullString
  err = SQLDB.QueryRow("select group_concat(fullname separator ',,,') from qf_document_structures where dsid is null").Scan(&notAliasDSList)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  dsList, err := GetDocumentStructureList()
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  if r.Method == http.MethodGet {
    type Context struct {
      DocumentStructureList []string
      DocumentStructures string
    }
    ctx := Context{strings.Split(notAliasDSList.String, ",,,"), strings.Join(dsList, ",,,") }

    fullTemplatePath := filepath.Join(getProjectPath(), "templates/new-document-structure-alias.html")
    tmpl := template.Must(template.ParseFiles(getBaseTemplate(), fullTemplatePath))
    tmpl.Execute(w, ctx)


  } else {

    ds := r.FormValue("template-document-structure")
    aliases := make([]string, 0)
    for i:= 1; i < 100; i++ {
      iStr := strconv.Itoa(i)
      if r.FormValue("alias-" + iStr) == "" {
        break
      }
      aliases = append(aliases, r.FormValue("alias-" + iStr))
    }

    dsid, err := getDocumentStructureID(ds)
    if err != nil {
      errorPage(w, err.Error())
      return
    }

    dstblName, err := tableName(ds)
    if err != nil {
      errorPage(w, err.Error())
      return
    }
    for _, alias := range aliases {

      atblName, err := newTableName()
      if err != nil {
        errorPage(w, err.Error())
        return
      }
      _, err = SQLDB.Exec(`insert into qf_document_structures(fullname, tbl_name, dsid) values(?, ?, ?)`,
        alias, atblName, dsid)
      if err != nil {
        errorPage(w, err.Error())
        return
      }

      sqlStmt := fmt.Sprintf("create table `%s` like `%s`", atblName, dstblName)
      _, err = SQLDB.Exec(sqlStmt)
      if err != nil {
        errorPage(w, err.Error())
        return
      }
    }

    redirectURL := fmt.Sprintf("/view-document-structure/%s/", ds)
    http.Redirect(w, r, redirectURL, 307)
  }


}
