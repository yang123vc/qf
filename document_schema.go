package qf

import (
  "net/http"
  "fmt"
  "path/filepath"
  "strconv"
  "strings"
  "html/template"
  "github.com/gorilla/mux"
)


func NewDocumentSchema(w http.ResponseWriter, r *http.Request) {

  type QFField struct {
    label string
    name string
    type_ string
    options string
    other_options string
  }


  if r.Method == http.MethodPost {
    qffs := make([]QFField, 0)
    r.ParseForm()
    i := 1
    for i < 100 {
      iStr := strconv.Itoa(i)
      if r.FormValue("label-" + iStr) == "" {
        break
      } else {
        qff := QFField{
          label: r.FormValue("label-" + iStr),
          name: r.FormValue("name-" + iStr),
          type_: r.FormValue("type-" + iStr),
          options: strings.Join(r.PostForm["options-" + iStr], ","),
          other_options: r.FormValue("other-options-" + iStr),
        }
        qffs = append(qffs, qff)
        i += 1
      }
    }

    tx, _ := SQLDB.Begin()
    var singleton string
    if r.FormValue("singleton") != "" {
      singleton = "t"
    } else {
      singleton = "f"
    }

    var childTable string
    if r.FormValue("child-table") != "" {
      childTable = "t"
    } else {
      childTable = "f"
    }

    res, err := tx.Exec(`insert into qf_forms(doc_name, child_table, singleton)
      values(?, ?, ?)`, r.FormValue("doc-name"), childTable, singleton)
    if err != nil {
      tx.Rollback()
      panic(err)
    }

    formId, _:= res.LastInsertId()
    stmt, err := tx.Prepare(`insert into qf_fields(formid, label, name, type, options, other_options)
      values(?, ?, ?, ?, ?, ?)`)
    if err != nil {
      tx.Rollback()
      panic(err)
    }
    for _, o := range(qffs) {
      _, err := stmt.Exec(formId, o.label, o.name, o.type_, o.options, o.other_options)
      if err != nil {
        tx.Rollback()
        panic(err)
      }
    }

    // create actual form data tables, we've only stored the form schema to the database
    tbl := tableName(r.FormValue("doc-name"))
    sql := fmt.Sprintf("create table `%s` (", tbl)
    sql += "id bigint unsigned not null auto_increment,"
    sql += "created datetime not null,"
    sql += "modified datetime not null,"

    sqlEnding := ""
    for _, qff := range qffs {
      sql += qff.name + " "
      if qff.type_ == "Check" {
        sql += "char(1)"
      } else if qff.type_ == "Date" {
        sql += "date"
      } else if qff.type_ == "Date and Time" {
        sql += "datetime"
      } else if qff.type_ == "Float" {
        sql += "float"
      } else if qff.type_ == "Int" {
        sql += "int"
      } else if qff.type_ == "Link" {
        sql += "bigint unsigned"
      } else if qff.type_ == "Data" || qff.type_ == "Email" || qff.type_ == "URL" || qff.type_ == "Section Break" {
        sql += "varchar(255)"
      } else if qff.type_ == "Text" || qff.type_ == "Table" {
        sql += "text"
      } else if qff.type_ == "Select" || qff.type_ == "Read Only"{
        sql += "varchar(255)"
      }
      if optionSearch(qff.options, "required") {
        sql += " not null"
      }
      sql += ", "

      if optionSearch(qff.options, "unique") {
        sqlEnding += fmt.Sprintf(", unique(%s)", qff.name)
      }
      if qff.type_ == "Link" {
        sqlEnding += fmt.Sprintf(", foreign key (%s) references `%s`(id)", qff.name, tableName(qff.other_options))
      }
    }
    sql += "primary key (id)" + sqlEnding + ")"
    _, err1 := tx.Exec(sql)
    if err1 != nil {
      tx.Rollback()
      panic(err1)
    }

    for _, qff := range qffs {
      if optionSearch(qff.options, "index") && ! optionSearch(qff.options, "unique") {
        indexSql := fmt.Sprintf("create index idx_%s on `%s`(%s)", qff.name, tbl, qff.name)
        _, err := tx.Exec(indexSql)
        if err != nil {
          tx.Rollback()
          panic(err)
        }
      }
    }
    tx.Commit()
    http.Redirect(w, r, "/list-document-schemas/", 307)

  } else {
    type Context struct {
      DocNames string
    }
    ctx := Context{strings.Join(getDocNames(w), ",")}
    tmpl := template.Must(template.ParseFiles(filepath.Join(getProjectPath(), "templates/new-document-schema.html")))
    tmpl.Execute(w, ctx)
  }
}


func JQuery(w http.ResponseWriter, r *http.Request) {
  http.ServeFile(w, r, filepath.Join(getProjectPath(), "statics/jquery-3.3.1.min.js"))
}


func ListDocumentSchemas(w http.ResponseWriter, r *http.Request) {
  type Context struct {
    DocNames []string
  }
  ctx := Context{DocNames: getDocNames(w)}
  tmpl := template.Must(template.ParseFiles(filepath.Join(getProjectPath(), "templates/list-document-schemas.html")))
  tmpl.Execute(w, ctx)
}


func DeleteDocumentSchema(w http.ResponseWriter, r *http.Request) {
  vars := mux.Vars(r)
  doc := vars["document-schema"]

  if ! docExists(doc, w) {
    fmt.Fprintf(w, "The document schema %s does not exists.", doc)
    return
  }

  tx, _ := SQLDB.Begin()
  var id int
  err := tx.QueryRow("select id from qf_forms where doc_name = ?", doc).Scan(&id)
  if err != nil {
    tx.Rollback()
    panic(err)
  }

  _, err = tx.Exec("delete from qf_fields where formid = ?", id)
  if err != nil {
    tx.Rollback()
    panic(err)
  }

  _, err = tx.Exec("delete from qf_forms where doc_name = ?", doc)
  if err != nil {
    tx.Rollback()
    panic(err)
  }

  sql := fmt.Sprintf("drop table `%s`", tableName(doc))
  _, err = tx.Exec(sql)
  if err != nil {
    tx.Rollback()
    panic(err)
  }

  http.Redirect(w, r, "/list-document-schemas/", 307)
}
