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
  "html"
  "errors"
  "math/rand"
  "time"
  "runtime"
)


func getProjectPath() string {
  gp := os.Getenv("GOPATH")
  if gp == "" {
    userStruct, err := user.Current()
    if err != nil && gp == "" {
      panic(err)
    }
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
  rows, err := SQLDB.Query("select fullname from qf_document_structures")
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


func DoesCurrentUserHavePerm(r *http.Request, documentStructure, permission string) (bool, error) {
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

  dsid, err := getDocumentStructureID(documentStructure)
  if err != nil {
    return false, err
  }
  for _, rid := range rids {
    var count int
    err = SQLDB.QueryRow("select count(*) from qf_permissions where dsid = ? and roleid = ?", dsid, rid).Scan(&count)
    if err != nil {
      return false, err
    }
    if count == 0 {
      continue
    }
    var permissions string
    err = SQLDB.QueryRow("select permissions from qf_permissions where dsid = ? and roleid = ?", dsid, rid).Scan(&permissions)
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


func GetDocData(documentStructure string) ([]DocData, error) {
  dds := make([]DocData, 0)

  var dsid int
  err := SQLDB.QueryRow("select id from qf_document_structures where fullname = ?", documentStructure).Scan(&dsid)
  if err != nil {
    return dds, err
  }
  var label, name, type_, options, otherOptions string

  rows, err := SQLDB.Query("select label, name, type, options, other_options from qf_fields where dsid = ? order by id asc", dsid)
  if err != nil {
    return dds, err
  }
  defer rows.Close()
  for rows.Next() {
    err := rows.Scan(&label, &name, &type_, &options, &otherOptions)
    if err != nil {
      return dds, err
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
    return dds, err
  }

  return dds, nil
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
    strSlice = append(strSlice, html.UnescapeString(str))
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
  err := SQLDB.QueryRow("select approval_steps from qf_document_structures where fullname = ?", documentStructure).Scan(&approvers)
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


func errorPage(w http.ResponseWriter, msg string) {
  _, fn, line, _ := runtime.Caller(1)
  type Context struct {
    Message string
    SourceFn string
    SourceLine int
  }

  ctx := Context{msg, fn, line}
  fullTemplatePath := filepath.Join(getProjectPath(), "templates/error-page.html")
  tmpl := template.Must(template.ParseFiles(getBaseTemplate(), fullTemplatePath))
  tmpl.Execute(w, ctx)
}


func getEC(documentStructure string) (ExtraCode, bool) {
  var dsid int
  err := SQLDB.QueryRow("select id from qf_document_structures where fullname = ?", documentStructure).Scan(&dsid)
  if err != nil {
    return ExtraCode{}, false
  }

  ec, ok := ExtraCodeMap[dsid]
  if ok {
    return ec, true
  }
  return ExtraCode{}, false
}


func getColumnNames(ds string) (map[string]string, error){
  returnMap := make(map[string]string)
  var dsid int
  err := SQLDB.QueryRow("select id from qf_document_structures where fullname = ?", ds).Scan(&dsid)
  if err != nil {
    return nil, err
  }

  var colName string
  var label string
  rows, err := SQLDB.Query(`select name, label from qf_fields where dsid = ? and  type != "Table"
    and type != "Section Break" and type != "File" and type != "Image" order by id asc limit 3`, dsid)
  if err != nil {
    return nil, err
  }
  defer rows.Close()
  for rows.Next() {
    err := rows.Scan(&colName, &label)
    if err != nil {
      return nil, err
    }
    returnMap[colName] = label
  }
  if err = rows.Err(); err != nil {
    return nil, err
  }
  returnMap["created"] = "Date of Creation"
  returnMap["created_by"] = "Created By"

  return returnMap, nil
}


func getMentionedUserColumn(ds string) (string, error) {
  var col string

  err := SQLDB.QueryRow("select name from qf_fields where other_options = 'MentionedUser'").Scan(&col)
  if err != nil {
    return col, err
  }
  return col, nil
}


func newTableName(documentStructure string) (string, error) {
  for {
    newName := "qftbl_" + untestedRandomString(3)
    var count int
    err := SQLDB.QueryRow("select count(*) from qf_document_structures where tbl_name = ?", newName).Scan(&count)
    if err != nil {
      return "", err
    }
    if count == 0 {
      return newName, nil
    }
  }
}


func tableName(documentStructure string) (string, error) {
  var name sql.NullString
  err := SQLDB.QueryRow("select tbl_name from qf_document_structures where fullname = ?", documentStructure).Scan(&name)
  if err != nil {
    return "", err
  }

  if ! name.Valid {
    return "", errors.New("document structure does not exists.")
  } else {
    return name.String, nil
  }
}


func newApprovalTableName(documentStructure, role string) (string, error) {
  for {
    newName := "qfatbl_" + untestedRandomString(4)

    var count int
    err := SQLDB.QueryRow(`select count(*) as count from qf_approvals_tables where tbl_name = ?`,
      newName).Scan(&count)
    if err != nil {
      return "", err
    }

    if count == 0 {
      return newName, nil
    }
  }
}


func getApprovalTable(documentStructure, role string) (string, error) {
  var name sql.NullString
  err := SQLDB.QueryRow("select tbl_name from qf_approvals_tables where document_structure = ? and role = ?",
    documentStructure, role).Scan(&name)
  if err != nil {
    return "", err
  }

  if ! name.Valid {
    return "", errors.New("document structure or role does not exists.")
  } else {
    return name.String, nil
  }
}


func isApproved(documentStructure string, docid uint64) (bool, error) {
  approvers, err := getApprovers(documentStructure)
  if err != nil {
    return false, err
  }

  approved := true
  for _, approver := range approvers {
    atn, err := getApprovalTable(documentStructure, approver)
    if err != nil {
      return false, err
    }

    sqlStmt := fmt.Sprintf("select count(*) from `%s` where docid = ? and status = 'Approved'", atn)
    var count int
    err = SQLDB.QueryRow(sqlStmt, docid).Scan(&count)
    if err != nil {
      return false, err
    }
    if count == 0 {
      return false, nil
    }
  }

  return approved, nil
}


func untestedRandomString(length int) string {
  var seededRand *rand.Rand = rand.New(rand.NewSource(time.Now().UnixNano()))
  const charset = "abcdefghijklmnopqrstuvwxyz1234567890"

  b := make([]byte, length)
  for i := range b {
    b[i] = charset[seededRand.Intn(len(charset))]
  }
  return string(b)
}


func getDocumentStructureID(documentStructure string) (int, error) {
  var dsid int
  err := SQLDB.QueryRow("select id from qf_document_structures where fullname = ?", documentStructure).Scan(&dsid)
  if err != nil {
    return dsid, err
  }
  return dsid, nil
}


func documentStructureHasForm(documentStructure string) (bool, error) {
  dsid, err := getDocumentStructureID(documentStructure)
  if err != nil {
    return false, err
  }

  var count int
  err = SQLDB.QueryRow("select count(*) from qf_fields where dsid = ? and (type = 'File' or type = 'Image')", dsid).Scan(&count)
  if err != nil {
    return false, err
  }
  ret := false
  if count > 0 {
    ret = true
  }
  return ret, nil
}



func getRolePermissions(documentStructure string) ([]RolePermissions, error) {
  rps := make([]RolePermissions, 0)

  dsid, err := getDocumentStructureID(documentStructure)
  if err != nil {
    return rps, err
  }

  var role, permissions string
  rows, err := SQLDB.Query(`select qf_roles.role, qf_permissions.permissions
    from qf_roles inner join qf_permissions on qf_roles.id = qf_permissions.roleid
    where qf_permissions.dsid = ?`, dsid)
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
