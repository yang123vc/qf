package qf

import (
  "os/user"
  "path/filepath"
  "strings"
  "net/http"
  "fmt"
  "database/sql"
)


func getProjectPath() string {
  userStruct, err := user.Current()
  if err != nil {
    panic(err)
  }
  projectPath := filepath.Join(userStruct.HomeDir, "go/src/github.com/bankole7782/qf")
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


func doesCurrentUserHavePerm(r *http.Request, object, permission string) (bool, error) {
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
