package qf

import (
  "net/http"
  "html/template"
  "path/filepath"
  "strings"
  "strconv"
  "fmt"
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

  ndsList, err := notAliasDocumentStructureList()
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
    ctx := Context{ndsList, strings.Join(dsList, ",,,") }

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


func createMultipleAliases(w http.ResponseWriter, r *http.Request) {
  truthValue, err := isUserAdmin(r)
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  if ! truthValue {
    errorPage(w, "You are not an admin here. You don't have permissions to view this page.")
    return
  }

  ndsList, err := notAliasDocumentStructureList()
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
    ctx := Context{ndsList, strings.Join(dsList, ",,,") }

    fullTemplatePath := filepath.Join(getProjectPath(), "templates/create-multiple-aliases.html")
    tmpl := template.Must(template.ParseFiles(getBaseTemplate(), fullTemplatePath))
    tmpl.Execute(w, ctx)

  } else {

    addText := r.FormValue("additional-text")
    pos := r.FormValue("position")

    for _, dsToCreateAlias := range r.PostForm["ds"] {
      dstblName, err := tableName(dsToCreateAlias)
      if err != nil {
        errorPage(w, err.Error())
        return
      }
      dsid, err := getDocumentStructureID(dsToCreateAlias)
      if err != nil {
        errorPage(w, err.Error())
        return
      }

      atblName, err := newTableName()
      if err != nil {
        errorPage(w, err.Error())
        return
      }

      var alias string
      if pos == "prefix" {
        alias = addText + dsToCreateAlias
      } else if pos == "suffix" {
        alias = dsToCreateAlias + alias
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
  }

  http.Redirect(w, r, "/list-document-structures/", 307)
}
