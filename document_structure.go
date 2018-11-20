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


func newDocumentStructure(w http.ResponseWriter, r *http.Request) {
  truthValue, err := isUserAdmin(r)
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  if ! truthValue {
    errorPage(w, "You are not an admin here. You don't have permissions to view this page.")
    return
  }

  if r.Method == http.MethodGet {

    type Context struct {
      DocumentStructures string
      ChildTableDocumentStructures string
    }
    dsList, err := GetDocumentStructureList()
    if err != nil {
      errorPage(w, err.Error())
      return
    }

    var ctdsl sql.NullString
    err = SQLDB.QueryRow("select group_concat(fullname separator ',') from qf_document_structures where child_table = 't'").Scan(&ctdsl)
    if err != nil {
      errorPage(w, err.Error())
      return
    }
    ctx := Context{strings.Join(dsList, ","), ctdsl.String }

    fullTemplatePath := filepath.Join(getProjectPath(), "templates/new-document-structure.html")
    tmpl := template.Must(template.ParseFiles(getBaseTemplate(), fullTemplatePath))
    tmpl.Execute(w, ctx)

  } else {
    type QFField struct {
      label string
      name string
      type_ string
      options string
      other_options string
    }

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

    tblName, err := newTableName(r.FormValue("ds-name"))
    if err != nil {
      errorPage(w, err.Error())
      return
    }
    res, err := tx.Exec(`insert into qf_document_structures(fullname, tbl_name, help_text) values(?, ?, ?)`,
      r.FormValue("ds-name"), tblName, r.FormValue("help-text"))
    if err != nil {
      tx.Rollback()
      errorPage(w, err.Error())
      return
    }

    dsid, _:= res.LastInsertId()

    if r.FormValue("child-table") == "on" {
      _, err = tx.Exec("update qf_document_structures set child_table = 't' where id = ?", dsid)
      if err != nil {
        tx.Rollback()
        errorPage(w, err.Error())
        return
      }
    }

    stmt, err := tx.Prepare(`insert into qf_fields(dsid, label, name, type, options, other_options)
      values(?, ?, ?, ?, ?, ?)`)
    if err != nil {
      tx.Rollback()
      errorPage(w, err.Error())
    }
    for _, o := range(qffs) {
      _, err := stmt.Exec(dsid, o.label, o.name, o.type_, o.options, o.other_options)
      if err != nil {
        tx.Rollback()
        errorPage(w, err.Error())
        return
      }
    }

    // create actual form data tables, we've only stored the form structure to the database
    sql := fmt.Sprintf("create table `%s` (", tblName)
    sql += "id bigint unsigned not null auto_increment,"
    if r.FormValue("child-table") != "on" {
      sql += "created datetime not null,"
      sql += "modified datetime not null,"
      sql += "created_by bigint unsigned not null,"
      sql += "fully_approved varchar(1) default 'f',"
    }

    sqlEnding := ""
    for _, qff := range qffs {
      if qff.type_ == "Section Break" {
        continue
      }
      sql += qff.name + " "
      if qff.type_ == "Big Number" {
        sql += "bigint unsigned"
      } else if qff.type_ == "Check" {
        sql += "char(1) default 'f'"
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
      } else if qff.type_ == "Text" || qff.type_ == "Table" {
        sql += "text"
      } else if qff.type_ == "File" || qff.type_ == "Image" {
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
        ottblName, err := tableName(qff.other_options)
        if err != nil {
          errorPage(w, err.Error())
          return
        }
        sqlEnding += fmt.Sprintf(", foreign key (%s) references `%s`(id)", qff.name, ottblName)
      }
    }
    sql += "primary key (id) "
    if r.FormValue("child-table") == "on" {
      sql += ")"
    } else {
      sql += "," + fmt.Sprintf("foreign key (created_by) references `%s`(id)", UsersTable) + sqlEnding + ")"
    }

    _, err1 := tx.Exec(sql)
    if err1 != nil {
      tx.Rollback()
      errorPage(w, err1.Error())
      return
    }

    for _, qff := range qffs {
      if optionSearch(qff.options, "index") && ! optionSearch(qff.options, "unique") {
        indexSql := fmt.Sprintf("create index idx_%s on `%s`(%s)", qff.name, tblName, qff.name)
        _, err := tx.Exec(indexSql)
        if err != nil {
          tx.Rollback()
          errorPage(w, err.Error())
          return
        }
      }
    }
    tx.Commit()
    redirectURL := fmt.Sprintf("/edit-document-structure-permissions/%s/", r.FormValue("ds-name"))
    http.Redirect(w, r, redirectURL, 307)

  }

}


func serveJS(w http.ResponseWriter, r *http.Request) {
  vars := mux.Vars(r)
  lib := vars["library"]

  if lib == "jquery" {
    http.ServeFile(w, r, filepath.Join(getProjectPath(), "statics/jquery-3.3.1.min.js"))
  } else if lib == "autosize" {
    http.ServeFile(w, r, filepath.Join(getProjectPath(), "statics/autosize.min.js"))
  }
}


func listDocumentStructures(w http.ResponseWriter, r *http.Request) {
  truthValue, err := isUserAdmin(r)
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  if ! truthValue {
    errorPage(w, "You are not an admin here. You don't have permissions to view this page.")
    return
  }

  type DS struct{
    DSName string
    ChildTable bool
  }

  structDSList := make([]DS, 0)
  var str string
  var ct string
  rows, err := SQLDB.Query("select fullname, child_table from qf_document_structures")
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  defer rows.Close()
  for rows.Next() {
    err := rows.Scan(&str, &ct)
    if err != nil {
      errorPage(w, err.Error())
      return
    }

    var b bool
    if ct == "t" {
      b = true
    } else {
      b = false
    }
    structDSList = append(structDSList, DS{str,b})
  }
  err = rows.Err()
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  type Context struct {
    DocumentStructures []DS
  }

  ctx := Context{DocumentStructures: structDSList}

  fullTemplatePath := filepath.Join(getProjectPath(), "templates/list-document-structures.html")
  tmpl := template.Must(template.ParseFiles(getBaseTemplate(), fullTemplatePath))
  tmpl.Execute(w, ctx)
}


func deleteDocumentStructure(w http.ResponseWriter, r *http.Request) {
  truthValue, err := isUserAdmin(r)
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  if ! truthValue {
    errorPage(w, "You are not an admin here. You don't have permissions to view this page.")
    return
  }

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

  approvers, err := getApprovers(ds)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  for _, step := range approvers {
    atn, err := getApprovalTable(ds, step)
    if err != nil {
      errorPage(w, "An error occurred getting approval table name.")
      return
    }

    _, err = SQLDB.Exec(fmt.Sprintf("drop table `%s`", atn) )
    if err != nil {
      errorPage(w, err.Error())
      return
    }

    _, err = SQLDB.Exec("delete from qf_approvals_tables where document_structure = ? and role = ?",
      ds, step)
    if err != nil {
      errorPage(w, "Error occurred removing record of approval table.")
      return
    }

  }

  var id int
  err = SQLDB.QueryRow("select id from qf_document_structures where fullname = ?", ds).Scan(&id)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  _, err = SQLDB.Exec("delete from qf_fields where dsid = ?", id)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  dsid, err := getDocumentStructureID(ds)
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  _, err = SQLDB.Exec("delete from qf_permissions where dsid = ?", dsid)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  tblName, err := tableName(ds)
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  sql := fmt.Sprintf("drop table `%s`", tblName)
  _, err = SQLDB.Exec(sql)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  _, err = SQLDB.Exec("delete from qf_document_structures where fullname = ?", ds)
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  
  http.Redirect(w, r, "/list-document-structures/", 307)
}


type RolePermissions struct {
  Role string
  Permissions string
}


func viewDocumentStructure(w http.ResponseWriter, r *http.Request) {
  truthValue, err := isUserAdmin(r)
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  if ! truthValue {
    errorPage(w, "You are not an admin here. You don't have permissions to view this page.")
    return
  }

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

  var id int
  var childTableStr string
  var tblNameStr string
  err = SQLDB.QueryRow("select id, child_table, tbl_name from qf_document_structures where fullname = ?", ds).Scan(&id, &childTableStr, &tblNameStr)
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  var childTableBool bool
  if childTableStr == "t" {
    childTableBool = true
  } else {
    childTableBool = false
  }

  docDatas, err := GetDocData(ds)
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  type Context struct {
    DocumentStructure string
    DocDatas []DocData
    Id int
    Add func(x, y int) int
    RPS []RolePermissions
    ApproversStr string
    HasApprovers bool
    ChildTable bool
    TableName string
  }

  add := func(x, y int) int {
    return x + y
  }

  rps, err := getRolePermissions(ds)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  approvers, err := getApprovers(ds)
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  var hasApprovers bool
  if len(approvers) == 0 {
    hasApprovers = false
  } else {
    hasApprovers = true
  }

  ctx := Context{ds, docDatas, id, add, rps, strings.Join(approvers, ", "), hasApprovers,
    childTableBool, tblNameStr}
  fullTemplatePath := filepath.Join(getProjectPath(), "templates/view-document-structure.html")
  tmpl := template.Must(template.ParseFiles(getBaseTemplate(), fullTemplatePath))
  tmpl.Execute(w, ctx)
}


func editDocumentStructurePermissions(w http.ResponseWriter, r *http.Request) {
  truthValue, err := isUserAdmin(r)
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  if ! truthValue {
    errorPage(w, "You are not an admin here. You don't have permissions to view this page.")
    return
  }

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

  if r.Method == http.MethodGet {

    type Context struct {
      DocumentStructure string
      RPS []RolePermissions
      LenRPS int
      Roles []string
    }

    roles, err := GetRoles()
    if err != nil {
      errorPage(w, err.Error())
      return
    }

    rps, err := getRolePermissions(ds)
    if err != nil {
      errorPage(w, err.Error())
      return
    }

    ctx := Context{ds, rps, len(rps), roles}
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

    dsid, err := getDocumentStructureID(ds)
    if err != nil {
      errorPage(w, err.Error())
      return
    }

    for _, rp := range nrps {
      roleid, err := getRoleId(rp.Role)
      if err != nil {
        errorPage(w, err.Error())
        return
      }
      _, err = SQLDB.Exec("insert into qf_permissions(roleid, dsid, permissions) values(?,?,?)", roleid, dsid, rp.Permissions)
      if err != nil {
        errorPage(w, err.Error())
        return
      }
    }

    redirectURL := fmt.Sprintf("/view-document-structure/%s/", ds)
    http.Redirect(w, r, redirectURL, 307)
  }

}
