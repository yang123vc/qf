package qf

import (
  "os/user"
  "path/filepath"
  "strings"
  "net/http"
  "fmt"
  "database/sql"
  "strconv"
  "os"
  "html/template"
)


func getProjectPath() string {
  userStruct, err := user.Current()
  if err != nil {
    panic(err)
  }
  gp := os.Getenv("GOPATH")
  if gp == "" {
    gp = filepath.Join(userStruct.HomeDir, "go")
  }

  projectPath := filepath.Join(gp, "src/github.com/bankole7782/qf")
  return projectPath
}


func optionSearch(commaSeperatedOptions, option string) bool {
  if commaSeperatedOptions == "" {
    return false
  } else {
    options := strings.Split(commaSeperatedOptions, ",")
    optionsTrimmed := make([]string, 0)
    for _, opt := range options {
      optionsTrimmed = append(optionsTrimmed, strings.TrimSpace(opt))
    }
    for _, value := range optionsTrimmed {
      if option == value {
        return true
      }
    }
    return false
  }
}


func tableName(documentStructure string) string {
  return fmt.Sprintf("qf%s", documentStructure)
}


func docExists(documentName string) (bool, error) {
  dsList, err := GetDocumentStructureList()
  if err != nil {
    return false, err
  }

  for _, value := range dsList {
    if value == documentName {
      return true, nil
    }
  }
  return false, nil
}


func GetDocumentStructureList() ([]string, error) {
  tempSlice := make([]string, 0)
  var str string
  rows, err := SQLDB.Query("select name from qf_document_structures")
  if err != nil {
    return tempSlice, err
  }
  defer rows.Close()
  for rows.Next() {
    err := rows.Scan(&str)
    if err != nil {
      return tempSlice, err
    }
    tempSlice = append(tempSlice, str)
  }
  err = rows.Err()
  if err != nil {
    return tempSlice, err
  }
  return tempSlice, nil
}


func getRoleId(role string) (int, error) {
  var roleid int
  err := SQLDB.QueryRow("select id from qf_roles where role = ? ", role).Scan(&roleid)
  return roleid, err
}


func isUserAdmin(r *http.Request) (bool, error) {
  userid, err := GetCurrentUser(r)
  if err != nil {
    return false, err
  }
  for _, id := range Admins {
    if userid == id {
      return true, nil
    }
  }
  return false, nil
}


func DoesCurrentUserHavePerm(r *http.Request, object, permission string) (bool, error) {
  adminTruth, err := isUserAdmin(r)
  if err == nil && adminTruth {
    return true, nil
  }

  userid, err := GetCurrentUser(r)
  if err != nil {
    return false, err
  }

  var roles sql.NullString
  err = SQLDB.QueryRow("select group_concat(roleid separator ',') from qf_user_roles where userid = ?", userid).Scan(&roles)
  if err != nil {
    return false, err
  }
  if ! roles.Valid {
    return false, nil
  }
  rids := strings.Split(roles.String, ",")

  for _, rid := range rids {
    var count int
    err = SQLDB.QueryRow("select count(*) from qf_permissions where object = ? and roleid = ?", object, rid).Scan(&count)
    if err != nil {
      return false, err
    }
    if count == 0 {
      continue
    }
    var permissions string
    err = SQLDB.QueryRow("select permissions from qf_permissions where object = ? and roleid = ?", object, rid).Scan(&permissions)
    if err != nil {
      return false, err
    }
    if optionSearch(permissions, permission) {
      return true, nil
    }
  }

  return false, nil
}


func getBaseTemplate() string {
  if BaseTemplate != "" {
    return BaseTemplate
  } else {
    badBasePath := filepath.Join(getProjectPath(), "templates/bad-base.html")
    return badBasePath
  }
}


type DocData struct {
  Label string
  Name string
  Type string
  Required bool
  Unique bool
  OtherOptions []string
}


func GetDocData(dsid int) []DocData{
  var label, name, type_, options, otherOptions string

  dds := make([]DocData, 0)
  rows, err := SQLDB.Query("select label, name, type, options, other_options from qf_fields where dsid = ? order by id asc", dsid)
  if err != nil {
    panic(err)
  }
  defer rows.Close()
  for rows.Next() {
    err := rows.Scan(&label, &name, &type_, &options, &otherOptions)
    if err != nil {
      panic(err)
    }
    var required, unique bool
    if optionSearch(options, "required") {
      required = true
    }
    if optionSearch(options, "unique") {
      unique = true
    }
    dd := DocData{label, name, type_, required, unique, strings.Split(otherOptions, "\n")}
    dds = append(dds, dd)
  }
  err = rows.Err()
  if err != nil {
    panic(err)
  }

  return dds
}


func getApprovalTable(documentStructure, role string) string {
  return fmt.Sprintf("qf%s %s Approvals", documentStructure, role)
}


func GetRoles() ([]string, error) {
  strSlice := make([]string, 0)
  var str string
  rows, err := SQLDB.Query("select role from qf_roles order by role asc")
  if err != nil {
    return strSlice, err
  }
  defer rows.Close()
  for rows.Next() {
    err := rows.Scan(&str)
    if err != nil {
      return strSlice, err
    }
    strSlice = append(strSlice, str)
  }
  if err = rows.Err(); err != nil {
    return strSlice, err
  }
  return strSlice, nil
}


func GetCurrentUserRoles(r *http.Request) ([]string, error) {
  userRoles := make([]string, 0)

  adminTruth, err := isUserAdmin(r)
  if err == nil && adminTruth {
    userRoles = append(userRoles, "Administrator")
  }

  userid, err := GetCurrentUser(r)
  if err != nil {
    return userRoles, err
  }

  var roles sql.NullString
  err = SQLDB.QueryRow("select group_concat(roleid separator ',') from qf_user_roles where userid = ?", userid).Scan(&roles)
  if err != nil {
    return userRoles, err
  }
  if ! roles.Valid {
    return userRoles, nil
  }
  rids := strings.Split(roles.String, ",")

  for _, rid := range rids {
    var roleName string
    ridInt, _ := strconv.Atoi(rid)
    err = SQLDB.QueryRow("select role from qf_roles where id = ?", ridInt).Scan(&roleName)
    if err != nil {
      return userRoles, err
    }
    userRoles = append(userRoles, roleName)
  }
  return userRoles, nil
}


func getApprovers(documentStructure string) ([]string, error) {
  approversList := make([]string, 0)

  var approvers sql.NullString
  err := SQLDB.QueryRow("select approval_steps from qf_document_structures where name = ?", documentStructure).Scan(&approvers)
  if err != nil {
    return approversList, err
  }

  if ! approvers.Valid {
    return approversList, nil
  }

  return strings.Split(approvers.String, ","), nil
}


type ColAndData struct {
  ColName string
  Data string
}


type Row struct {
  Id uint64
  ColAndDatas []ColAndData
  RowUpdatePerm bool
  RowDeletePerm bool
}


func errorPage(w http.ResponseWriter, r *http.Request, msg string, err error) {
  type Context struct {
    Message string
    ExactError string
  }
  var exactError string
  if err != nil {
    exactError = err.Error()
  } else {
    exactError = ""
  }
  ctx := Context{msg, exactError}
  fullTemplatePath := filepath.Join(getProjectPath(), "templates/error-page.html")
  tmpl := template.Must(template.ParseFiles(getBaseTemplate(), fullTemplatePath))
  tmpl.Execute(w, ctx)
}


func getEC(documentStructure string) (ExtraCode, bool) {
  var dsid int
  err := SQLDB.QueryRow("select id from qf_document_structures where name = ?", documentStructure).Scan(&dsid)
  if err != nil {
    return ExtraCode{}, false
  }

  for _, ec := range ExtraCodeList {
    if dsid == ec.DSNo {
      return ec, true
    }
  }
  return ExtraCode{}, false
}


func getColumnNames(ds string) ([]string, error){
  colNames := make([]string, 0)

  var dsid int
  err := SQLDB.QueryRow("select id from qf_document_structures where name = ?", ds).Scan(&dsid)
  if err != nil {
    return colNames, err
  }

  var colName string
  rows, err := SQLDB.Query("select name from qf_fields where dsid = ? and type != \"Section Break\" order by id asc limit 3", dsid)
  if err != nil {
    return colNames, err
  }
  defer rows.Close()
  for rows.Next() {
    err := rows.Scan(&colName)
    if err != nil {
      return colNames, err
    }
    colNames = append(colNames, colName)
  }
  if err = rows.Err(); err != nil {
    return colNames, err
  }
  colNames = append(colNames, "created", "created_by")
  return colNames, nil
}
