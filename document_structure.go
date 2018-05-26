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


func NewDocumentStructure(w http.ResponseWriter, r *http.Request) {
  truthValue, err := isUserAdmin(r)
  if err != nil {
    fmt.Fprintf(w, "Error occurred while trying to ascertain if the user is admin. Exact Error: " + err.Error())
    return
  }
  if ! truthValue {
    fmt.Fprintf(w, "You are not an admin here. You don't have permissions to view this page.")
    return
  }

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

    res, err := tx.Exec(`insert into qf_document_structures(doc_name, child_table, singleton)
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

    // create actual form data tables, we've only stored the form structure to the database
    tbl := tableName(r.FormValue("doc-name"))
    sql := fmt.Sprintf("create table `%s` (", tbl)
    sql += "id bigint unsigned not null auto_increment,"
    sql += "created datetime not null,"
    sql += "modified datetime not null,"
    sql += "created_by bigint unsigned not null,"

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
    sql += "primary key (id), " + fmt.Sprintf("foreign key (created_by) references `%s`(id)", UsersTable) + sqlEnding + ")"
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
    redirectURL := fmt.Sprintf("/edit-document-structure-permissions/%s/", r.FormValue("doc-name"))
    http.Redirect(w, r, redirectURL, 307)

  } else {
    type Context struct {
      DocNames string
    }
    ctx := Context{strings.Join(getDocNames(w), ",")}
    tmpl := template.Must(template.ParseFiles(filepath.Join(getProjectPath(), "templates/new-document-structure.html")))
    tmpl.Execute(w, ctx)
  }
}


func JQuery(w http.ResponseWriter, r *http.Request) {
  http.ServeFile(w, r, filepath.Join(getProjectPath(), "statics/jquery-3.3.1.min.js"))
}


func ListDocumentStructures(w http.ResponseWriter, r *http.Request) {
  type Context struct {
    DocNames []string
  }
  ctx := Context{DocNames: getDocNames(w)}
  tmpl := template.Must(template.ParseFiles(filepath.Join(getProjectPath(), "templates/list-document-structures.html")))
  tmpl.Execute(w, ctx)
}


func DeleteDocumentStructure(w http.ResponseWriter, r *http.Request) {
  truthValue, err := isUserAdmin(r)
  if err != nil {
    fmt.Fprintf(w, "Error occurred while trying to ascertain if the user is admin. Exact Error: " + err.Error())
    return
  }
  if ! truthValue {
    fmt.Fprintf(w, "You are not an admin here. You don't have permissions to view this page.")
    return
  }

  vars := mux.Vars(r)
  doc := vars["document-structure"]

  if ! docExists(doc, w) {
    fmt.Fprintf(w, "The document structure %s does not exists.", doc)
    return
  }

  tx, _ := SQLDB.Begin()
  var id int
  err = tx.QueryRow("select id from qf_document_structures where doc_name = ?", doc).Scan(&id)
  if err != nil {
    tx.Rollback()
    panic(err)
  }

  _, err = tx.Exec("delete from qf_fields where formid = ?", id)
  if err != nil {
    tx.Rollback()
    panic(err)
  }

  _, err = tx.Exec("delete from qf_document_structures where doc_name = ?", doc)
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

  http.Redirect(w, r, "/list-document-structures/", 307)
}


type RolePermissions struct {
  Role string
  Permissions string
}


func getRolePermissions(documentStructure string) ([]RolePermissions, error) {
  var role, permissions string
  rps := make([]RolePermissions, 0)
  rows, err := SQLDB.Query(`select qf_roles.role, qf_permissions.permissions
    from qf_roles inner join qf_permissions on qf_roles.id = qf_permissions.roleid
    where qf_permissions.object = ?`, documentStructure)
  if err != nil {
    return rps, err
  }
  defer rows.Close()
  for rows.Next() {
    err := rows.Scan(&role, &permissions)
    if err != nil {
      return rps, err
    }
    rps = append(rps, RolePermissions{role, permissions})
  }
  if err = rows.Err(); err != nil {
    return rps, err
  }
  return rps, nil
}


func ViewDocumentStructure(w http.ResponseWriter, r *http.Request) {
  truthValue, err := isUserAdmin(r)
  if err != nil {
    fmt.Fprintf(w, "Error occurred while trying to ascertain if the user is admin. Exact Error: " + err.Error())
    return
  }
  if ! truthValue {
    fmt.Fprintf(w, "You are not an admin here. You don't have permissions to view this page.")
    return
  }

  vars := mux.Vars(r)
  doc := vars["document-structure"]

  if ! docExists(doc, w) {
    fmt.Fprintf(w, "The document structure %s does not exists.", doc)
    return
  }

  var id int
  err = SQLDB.QueryRow("select id from qf_document_structures where doc_name = ?", doc).Scan(&id)
  if err != nil {
    panic(err)
  }

  var childTable, singleton string
  err = SQLDB.QueryRow("select child_table, singleton from qf_document_structures where id = ?", id).Scan(&childTable, &singleton)
  if err != nil {
    panic(err)
  }

  docDatas := getDocData(id)
  type Context struct {
    DocName string
    DocDatas []DocData
    Id int
    Add func(x, y int) int
    RPS []RolePermissions
  }

  add := func(x, y int) int {
    return x + y
  }

  rps, err := getRolePermissions(doc)
  if err != nil {
    fmt.Fprintf(w, "An error occured when trying to get the permissions on roles for this document. Exact error: " + err.Error())
    return
  }

  ctx := Context{doc, docDatas, id, add, rps}
  tmpl := template.Must(template.ParseFiles(filepath.Join(getProjectPath(), "templates/view-document-structure.html")))
  tmpl.Execute(w, ctx)
}


func EditDocumentStructurePermissions(w http.ResponseWriter, r *http.Request) {

  truthValue, err := isUserAdmin(r)
  if err != nil {
    fmt.Fprintf(w, "Error occurred while trying to ascertain if the user is admin. Exact Error: " + err.Error())
    return
  }
  if ! truthValue {
    fmt.Fprintf(w, "You are not an admin here. You don't have permissions to view this page.")
    return
  }

  vars := mux.Vars(r)
  doc := vars["document-structure"]

  if ! docExists(doc, w) {
    fmt.Fprintf(w, "The document structure %s does not exists.", doc)
    return
  }

  if r.Method == http.MethodGet {

    type Context struct {
      DocName string
      RPS []RolePermissions
      LenRPS int
      Roles []string
    }

    roles, ok := getRoles(w)
    if ! ok {
      return
    }

    rps, err := getRolePermissions(doc)
    if err != nil {
      fmt.Fprintf(w, "An error occured when trying to get the permissions on roles for this document. Exact error: " + err.Error())
      return
    }

    ctx := Context{doc, rps, len(rps), roles}
    tmpl := template.Must(template.ParseFiles(filepath.Join(getProjectPath(), "templates/edit-document-structure-permissions.html")))
    tmpl.Execute(w, ctx)

  } else if r.Method == http.MethodPost {

    nrps := make([]RolePermissions, 0)
    for i := 1; i < 1000; i++ {
      p := strconv.Itoa(i)
      if r.FormValue("role-" + p) == "" {
        break
      } else {
        rp := RolePermissions{r.FormValue("role-" + p), r.FormValue("permissions-" + p)}
        nrps = append(nrps, rp)
      }
    }

    for _, rp := range nrps {
      roleid, err := getRoleId(rp.Role)
      if err != nil {
        fmt.Fprintf(w, "Error occured while getting role id. Exact Error: " + err.Error())
        return
      }
      _, err = SQLDB.Exec("insert into qf_permissions(roleid, object, permissions) values(?,?,?)", roleid, doc, rp.Permissions)
      if err != nil {
        fmt.Fprintf(w, "Error storing role permissions. Exact error: " + err.Error())
        return
      }
    }

    redirectURL := fmt.Sprintf("/view-document-structure/%s/", doc)
    http.Redirect(w, r, redirectURL, 307)
  }

}
