package qf

import (
  "os/user"
  "path/filepath"
  "strings"
  "sort"
  "net/http"
  "fmt"
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
    sort.Strings(options)
    i := sort.SearchStrings(options, option)
    if i != len(options) {
      return true
      } else {
        return false
      }
  }
}


func tableName(name string) string {
  return fmt.Sprintf("qf%s", name)
}


func docExists(documentName string, w http.ResponseWriter) bool {
  docNames := getDocNames(w)
  sort.Strings(docNames)
  i := sort.SearchStrings(docNames, documentName)
  if i != len(docNames) {
    return true
  } else {
    return false
  }
}


func getDocNames(w http.ResponseWriter) []string {
  tempSlice := make([]string, 0)
  var str string
  rows, err := SQLDB.Query("select doc_name from qf_forms")
  if err != nil {
    fmt.Fprintf(w, "An error occured: " + err.Error())
    panic(err)
  }
  defer rows.Close()
  for rows.Next() {
    err := rows.Scan(&str)
    if err != nil {
      panic(err)
    }
    tempSlice = append(tempSlice, str)
  }
  err = rows.Err()
  if err != nil {
    fmt.Fprintf(w, "An error occured: " + err.Error())
    panic(err)
  }
  return tempSlice
}
