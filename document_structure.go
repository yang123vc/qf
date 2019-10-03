package qf

import (
  "net/http"
  "fmt"
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
    err = SQLDB.QueryRow("select group_concat(fullname separator ',,,') from qf_document_structures where child_table = 't'").Scan(&ctdsl)
    if err != nil {
      errorPage(w, err.Error())
      return
    }
    ctx := Context{strings.Join(dsList, ",,,"), ctdsl.String }

    tmpl := template.Must(template.ParseFiles(getBaseTemplate(), "qffiles/new-document-structure.html"))
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

    tblName, err := newTableName()
    if err != nil {
      errorPage(w, err.Error())
      return
    }
    res, err := SQLDB.Exec(`insert into qf_document_structures(fullname, tbl_name, help_text) values(?, ?, ?)`,
      r.FormValue("ds-name"), tblName, r.FormValue("help-text"))
    if err != nil {
      errorPage(w, err.Error())
      return
    }

    dsid, _:= res.LastInsertId()

    if r.FormValue("child-table") == "on" {
      _, err = SQLDB.Exec("update qf_document_structures set child_table = 't' where id = ?", dsid)
      if err != nil {
        errorPage(w, err.Error())
        return
      }
    }

    stmt, err := SQLDB.Prepare(`insert into qf_fields(dsid, label, name, type, options, other_options, view_order)
      values(?, ?, ?, ?, ?, ?, ?)`)
    if err != nil {
      errorPage(w, err.Error())
      return
    }
    for i, o := range(qffs) {
      _, err := stmt.Exec(dsid, o.label, o.name, o.type_, o.options, o.other_options, i + 1)
      if err != nil {
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
      sql += "edit_log varchar(255), "
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

      if ! optionSearch(qff.options, "unique") && qff.type_ != "Text" && qff.type_ != "Table" {
        sqlEnding += fmt.Sprintf(", index(%s)", qff.name)
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
      sql += ", index(created), index(modified)"
      sql += "," + fmt.Sprintf("foreign key (created_by) references `%s`(id)", UsersTable) + sqlEnding + ")"
    }

    _, err1 := SQLDB.Exec(sql)
    if err1 != nil {
      errorPage(w, err1.Error())
      return
    }

    redirectURL := fmt.Sprintf("/edit-document-structure-permissions/%s/", r.FormValue("ds-name"))
    http.Redirect(w, r, redirectURL, 307)
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

  var str, ctStr string
  var ctBool bool
  rows, err := SQLDB.Query("select fullname, child_table from qf_document_structures order by fullname")
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  defer rows.Close()
  for rows.Next() {
    err := rows.Scan(&str, &ctStr)
    if err != nil {
      errorPage(w, err.Error())
      return
    }
    if ctStr == "f" {
      ctBool = false
    } else if ctStr == "t" {
      ctBool = true
    }
    structDSList = append(structDSList, DS{str, ctBool})
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

  tmpl := template.Must(template.ParseFiles(getBaseTemplate(), "qffiles/list-document-structures.html"))
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

  var ctStr string
  err = SQLDB.QueryRow("select child_table from qf_document_structures where fullname = ?", ds).Scan(&ctStr)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  if ctStr == "t" {
    var dsid string
    dsidsUsingThisCT := make([]string, 0)

    rows, err := SQLDB.Query("select dsid from qf_fields where other_options = ?", ds)
    if err != nil {
      errorPage(w, err.Error())
      return
    }
    defer rows.Close()
    for rows.Next() {
      err := rows.Scan(&dsid)
      if err != nil {
        errorPage(w, err.Error())
        return
      }

      dsidsUsingThisCT = append(dsidsUsingThisCT, dsid)
    }
    err = rows.Err()
    if err != nil {
      errorPage(w, err.Error())
      return
    }

    if len(dsidsUsingThisCT) > 0 {
      m := fmt.Sprintf("This Child Table is in use by the following document structures with id: %s",
        dsidsUsingThisCT)
      errorPage(w, m)
      return
    }
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

  tblName, err := tableName(ds)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  hasForm, err := documentStructureHasForm(ds)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  if hasForm {
    dds, err := GetDocData(ds)
    if err != nil {
      errorPage(w, err.Error())
      return
    }

    useridUint64, err := GetCurrentUser(r)
    if err != nil {
      errorPage(w, err.Error())
      return
    }

    for _, dd := range dds {
      if dd.Type == "File" || dd.Type == "Image" {
        var fps sql.NullString
        qStmt := fmt.Sprintf("select group_concat(%s separator ',,,') from `%s` where created_by = ?", dd.Name, tblName)
        err = SQLDB.QueryRow(qStmt, useridUint64).Scan(&fps)
        if err != nil {
          errorPage(w, err.Error())
          return
        }

        filepaths := strings.Split(fps.String, ",,,")
        for _, fp := range filepaths {
          qStmt = "insert into qf_files_for_delete (created_by, filepath) values (?, ?)"
          _, err = SQLDB.Exec(qStmt, useridUint64, fp)
          if err != nil {
            panic(err)
          }
        }        
      }
    }

  }

  dsid, err := getDocumentStructureID(ds)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  _, err = SQLDB.Exec("delete from qf_fields where dsid = ?", dsid)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  _, err = SQLDB.Exec("delete from qf_permissions where dsid = ?", dsid)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  _, err = SQLDB.Exec("delete from qf_buttons where dsid = ?", dsid)
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

  var redirectURL string
  if hasForm {
    redirectURL = "/complete-files-delete/?n=l"
  } else {
    redirectURL = "/list-document-structures/"
  }
  http.Redirect(w, r, redirectURL, 307)
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
  var tblNameStr string
  var publicStr string
  err = SQLDB.QueryRow("select id, tbl_name, public from qf_document_structures where fullname = ?", ds).Scan(&id, &tblNameStr, &publicStr)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  docDatas, err := GetDocData(ds)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  var childTableStr string
  err = SQLDB.QueryRow("select child_table from qf_document_structures where fullname = ?", ds).Scan(&childTableStr)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  childTableBool := charToBool(childTableStr)
  publicBool := charToBool(publicStr)

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
    Public bool
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
    childTableBool, tblNameStr, publicBool}
  tmpl := template.Must(template.ParseFiles(getBaseTemplate(), "qffiles/view-document-structure.html"))
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
    tmpl := template.Must(template.ParseFiles(getBaseTemplate(), "qffiles/edit-document-structure-permissions.html"))
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
