package qf

import (
  "net/http"
  "fmt"
  "path/filepath"
  "strconv"
  "strings"
  "html/template"
  "github.com/gorilla/mux"
  "database/sql"
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
    for i := 1; i < 100; i++ {
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
      }
    }

    tx, _ := SQLDB.Begin()

    res, err := tx.Exec(`insert into qf_document_structures(name) values(?)`, r.FormValue("ds-name"))
    if err != nil {
      tx.Rollback()
      fmt.Fprintf(w, "An error ocurred while saving this document structure. Exact Error: " + err.Error())
      return
    }

    formId, _:= res.LastInsertId()
    stmt, err := tx.Prepare(`insert into qf_fields(dsid, label, name, type, options, other_options)
      values(?, ?, ?, ?, ?, ?)`)
    if err != nil {
      tx.Rollback()
      fmt.Fprintf(w, "An internal error occured. Exact Error: " + err.Error())
    }
    for _, o := range(qffs) {
      _, err := stmt.Exec(formId, o.label, o.name, o.type_, o.options, o.other_options)
      if err != nil {
        tx.Rollback()
        fmt.Fprintf(w, "An error occured while saving fields data. Exact Error: " + err.Error())
        return
      }
    }

    // create actual form data tables, we've only stored the form structure to the database
    tbl := tableName(r.FormValue("ds-name"))
    sql := fmt.Sprintf("create table `%s` (", tbl)
    sql += "id bigint unsigned not null auto_increment,"
    sql += "created datetime not null,"
    sql += "modified datetime not null,"
    sql += "created_by bigint unsigned not null,"

    sqlEnding := ""
    for _, qff := range qffs {
      if qff.type_ == "Section Break" {
        continue
      }
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
      } else if qff.type_ == "Data" || qff.type_ == "Email" || qff.type_ == "URL" || qff.type_ == "Select" || qff.type_ == "Read Only" {
        sql += "varchar(255)"
      } else if qff.type_ == "Text" {
        sql += "text"
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
      fmt.Fprintf(w, "Error occured when creating document structure mysql table. Exact Error: " + err1.Error())
      return
    }

    for _, qff := range qffs {
      if optionSearch(qff.options, "index") && ! optionSearch(qff.options, "unique") {
        indexSql := fmt.Sprintf("create index idx_%s on `%s`(%s)", qff.name, tbl, qff.name)
        _, err := tx.Exec(indexSql)
        if err != nil {
          tx.Rollback()
          fmt.Fprintf(w, "An error occured while creating indexes on the document structure table. Exact Error: " + err.Error())
          return
        }
      }
    }
    tx.Commit()
    redirectURL := fmt.Sprintf("/edit-document-structure-permissions/%s/", r.FormValue("ds-name"))
    http.Redirect(w, r, redirectURL, 307)

  } else {
    type Context struct {
      DocumentStructures string
    }
    dsList, err := GetDocumentStructureList()
    if err != nil {
      fmt.Fprintf(w, "An error occured when trying to get the document structure list. Exact Error " + err.Error())
      return
    }
    ctx := Context{strings.Join(dsList, ",")}

    fullTemplatePath := filepath.Join(getProjectPath(), "templates/new-document-structure.html")
    tmpl := template.Must(template.ParseFiles(getBaseTemplate(), fullTemplatePath))
    tmpl.Execute(w, ctx)
  }
}


func ServeJS(w http.ResponseWriter, r *http.Request) {
  vars := mux.Vars(r)
  lib := vars["library"]

  if lib == "jquery" {
    http.ServeFile(w, r, filepath.Join(getProjectPath(), "statics/jquery-3.3.1.min.js"))
  } else if lib == "autosize" {
    http.ServeFile(w, r, filepath.Join(getProjectPath(), "statics/autosize.min.js"))
  }
}


func ListDocumentStructures(w http.ResponseWriter, r *http.Request) {
  truthValue, err := isUserAdmin(r)
  if err != nil {
    fmt.Fprintf(w, "Error occurred while trying to ascertain if the user is admin. Exact Error: " + err.Error())
    return
  }
  if ! truthValue {
    fmt.Fprintf(w, "You are not an admin here. You don't have permissions to view this page.")
    return
  }

  type Context struct {
    DocumentStructures []string
  }
  dsList, err := GetDocumentStructureList()
  if err != nil {
    fmt.Fprintf(w, "An error occured when trying to get the document structure list. Exact Error " + err.Error())
    return
  }
  ctx := Context{DocumentStructures: dsList}

  fullTemplatePath := filepath.Join(getProjectPath(), "templates/list-document-structures.html")
  tmpl := template.Must(template.ParseFiles(getBaseTemplate(), fullTemplatePath))
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
  ds := vars["document-structure"]

  detv, err := docExists(ds)
  if err != nil {
    fmt.Fprintf(w, "Error occurred while determining if this document exists. Exact Error: " + err.Error())
    return
  }
  if detv == false {
    fmt.Fprintf(w, "The document structure %s does not exists.", ds)
    return
  }

  var stepsStr sql.NullString
  err = SQLDB.QueryRow("select approval_steps from qf_document_structures where name = ?", ds).Scan(&stepsStr)
  if err != nil {
    fmt.Fprintf(w, "Error occured when getting approval steps of this document structure. Exact Error: " + err.Error())
    return
  }
  if ! stepsStr.Valid {
    fmt.Fprintf(w, "This document structure has no approval steps.")
    return
  }
  stepsList := strings.Split(stepsStr.String, ",")

  for _, step := range stepsList {
    _, err = SQLDB.Exec(fmt.Sprintf("drop table `%s`", getApprovalTable(ds, step)) )
    if err != nil {
      fmt.Fprintf(w, "An error occured while deleting an approvals table. Exact Error: " + err.Error())
      return
    }
  }

  tx, _ := SQLDB.Begin()
  var id int
  err = tx.QueryRow("select id from qf_document_structures where name = ?", ds).Scan(&id)
  if err != nil {
    tx.Rollback()
    fmt.Fprintf(w, "Error occurred when trying to get document structure id. Exact Error: " + err.Error())
    return
  }

  _, err = tx.Exec("delete from qf_fields where dsid = ?", id)
  if err != nil {
    tx.Rollback()
    fmt.Fprintf(w, "Error occurred when deleting document structure fields. Exact Error: " + err.Error())
    return
  }

  _, err = tx.Exec("delete from qf_document_structures where name = ?", ds)
  if err != nil {
    tx.Rollback()
    fmt.Fprintf(w, "Error occurred when deleting document structure. Exact Error: " + err.Error())
    return
  }

  _, err = tx.Exec("delete from qf_permissions where object = ?", ds)
  if err != nil {
    tx.Rollback()
    fmt.Fprintf(w, "Error occurred when deleting document permissions. Exact Error: " + err.Error())
    return
  }

  sql := fmt.Sprintf("drop table `%s`", tableName(ds))
  _, err = tx.Exec(sql)
  if err != nil {
    tx.Rollback()
    fmt.Fprintf(w, "Error occurred when dropping document structure table. Exact Error: " + err.Error())
    return
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
  ds := vars["document-structure"]

  detv, err := docExists(ds)
  if err != nil {
    fmt.Fprintf(w, "Error occurred while determining if this document exists. Exact Error: " + err.Error())
    return
  }
  if detv == false {
    fmt.Fprintf(w, "The document structure %s does not exists.", ds)
    return
  }

  var id int
  err = SQLDB.QueryRow("select id from qf_document_structures where name = ?", ds).Scan(&id)
  if err != nil {
    fmt.Fprintf(w, "An error occured when trying to get the document structure id. Exact Error: " + err.Error())
    return
  }

  docDatas := GetDocData(id)
  type Context struct {
    DocumentStructure string
    DocDatas []DocData
    Id int
    Add func(x, y int) int
    RPS []RolePermissions
    ApproversStr string
    HasApprovers bool
  }

  add := func(x, y int) int {
    return x + y
  }

  rps, err := getRolePermissions(ds)
  if err != nil {
    fmt.Fprintf(w, "An error occured when trying to get the permissions on roles for this document. Exact error: " + err.Error())
    return
  }

  approvers, err := getApprovers(ds)
  if err != nil {
    fmt.Fprintf(w, "Error getting approval framework data. Exact Error: " + err.Error())
    return
  }
  var hasApprovers bool
  if len(approvers) == 0 {
    hasApprovers = false
  } else {
    hasApprovers = true
  }

  ctx := Context{ds, docDatas, id, add, rps, strings.Join(approvers, ", "), hasApprovers}
  fullTemplatePath := filepath.Join(getProjectPath(), "templates/view-document-structure.html")
  tmpl := template.Must(template.ParseFiles(getBaseTemplate(), fullTemplatePath))
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

  detv, err := docExists(doc)
  if err != nil {
    fmt.Fprintf(w, "Error occurred while determining if this document exists. Exact Error: " + err.Error())
    return
  }
  if detv == false {
    fmt.Fprintf(w, "The document structure %s does not exists.", doc)
    return
  }

  if r.Method == http.MethodGet {

    type Context struct {
      DocumentStructure string
      RPS []RolePermissions
      LenRPS int
      Roles []string
    }

    roles, err := GetRoles()
    if err != nil {
      fmt.Fprintf(w, "Error getting roles. Exact Error: " + err.Error())
      return
    }

    rps, err := getRolePermissions(doc)
    if err != nil {
      fmt.Fprintf(w, "An error occured when trying to get the permissions on roles for this document. Exact error: " + err.Error())
      return
    }

    ctx := Context{doc, rps, len(rps), roles}
    fullTemplatePath := filepath.Join(getProjectPath(), "templates/edit-document-structure-permissions.html")
    tmpl := template.Must(template.ParseFiles(getBaseTemplate(), fullTemplatePath))
    tmpl.Execute(w, ctx)

  } else if r.Method == http.MethodPost {
    r.ParseForm()
    nrps := make([]RolePermissions, 0)
    for i := 1; i < 1000; i++ {
      p := strconv.Itoa(i)
      if r.FormValue("role-" + p) == "" {
        break
      } else {
        if len(r.PostForm["perms-" + p]) == 0 {
          continue
        }
        rp := RolePermissions{r.FormValue("role-" + p), strings.Join(r.PostForm["perms-" + p], ",")}
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
