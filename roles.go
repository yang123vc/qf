package qf

import (
  "github.com/gorilla/mux"
  "net/http"
  "fmt"
  "path/filepath"
  "html/template"
  "sort"
  "database/sql"
)


func getRoles(w http.ResponseWriter) []string {
  strSlice := make([]string, 0)
  var str string
  rows, err := SQLDB.Query("select role from qf_roles order by role asc")
  if err != nil {
    fmt.Fprintf(w, "Error occured while trying to collect roles. Exact Error: " + err.Error())
    return strSlice
  }
  defer rows.Close()
  for rows.Next() {
    err := rows.Scan(&str)
    if err != nil {
      fmt.Fprintf(w, "Error occured while trying to collect role. Exact Error: " + err.Error())
      return strSlice
    }
    strSlice = append(strSlice, str)
  }
  if err = rows.Err(); err != nil {
    fmt.Fprintf(w, "An error occured: " + err.Error())
    return strSlice
  }
  return strSlice
}


func RolesView(w http.ResponseWriter, r *http.Request) {
  roles := getRoles(w)

  if r.Method == http.MethodGet {
    type Context struct {
      Roles []string
      NumberOfRoles int
    }
    ctx := Context{roles, len(roles)}
    tmpl := template.Must(template.ParseFiles(filepath.Join(getProjectPath(), "templates/roles-view.html")))
    tmpl.Execute(w, ctx)

  } else if r.Method == http.MethodPost {

    role := r.FormValue("role")
    if len(roles) == 0 {
      sort.Strings(roles)
      i := sort.SearchStrings(roles, role)
      if i != len(roles) {
        fmt.Fprintf(w, "The role \"%s\" already exists.", role)
        return
      }
    }

    _, err := SQLDB.Exec("insert into qf_roles(role) values(?)", role)
    if err != nil {
      fmt.Fprintf(w, "Error creating role. Exact Error: " + err.Error())
      return
    }

    fmt.Fprintf(w, "Successfully create role.")
  }
}


func DeleteRole(w http.ResponseWriter, r *http.Request) {
  vars := mux.Vars(r)
  role := vars["role"]

  _, err := SQLDB.Exec("delete from qf_roles where role=?", role)
  if err != nil {
    fmt.Fprintf(w, "Error occured while deleting role \"%s\". Exact Error: " + err.Error())
    return
  }

  http.Redirect(w, r, "/roles-view/", 307)
}


func UsersToRolesList(w http.ResponseWriter, r *http.Request) {

  type UserData struct {
    UserId uint64
    Firstname string
    Surname string
    Roles []string
  }

  sqlStmt := fmt.Sprintf("select users.id, `%s`.firstname, `%s`.surname, qf_roles.role ", UsersTable, UsersTable)
  sqlStmt += fmt.Sprintf("from (qf_user_roles right join `%s` on qf_user_roles.userid = `%s`.id)", UsersTable, UsersTable)
  sqlStmt += fmt.Sprintf("left join qf_roles on qf_user_roles.roleid = qf_roles.id ")
  sqlStmt += "order by users.id asc limit 100"

  uds := make([]UserData, 0)
  var userid uint64
  var firstname, surname string
  var role sql.NullString
  userRoleMap := make(map[uint64][]string)
  rows, err := SQLDB.Query(sqlStmt)
  if err != nil {
    fmt.Fprintf(w, "An error occured while reading user and role data. Exact error: " + err.Error())
    return
  }
  defer rows.Close()
  for rows.Next() {
    err := rows.Scan(&userid, &firstname, &surname, &role)
    if err != nil {
      fmt.Fprintf(w, "An error occured while reading a row. Exact error: " + err.Error())
      return
    }

    list, ok := userRoleMap[userid]
    if ! ok {
      if role.Valid {
        userRoleMap[userid] = []string{role.String}
      } else {
        userRoleMap[userid] = []string{}
      }
    } else {
      if role.Valid {
        list = append(list, role.String)
        userRoleMap[userid] = list
      }
    }
    uds = append(uds, UserData{userid, firstname, surname, []string{}})
  }

  if err = rows.Err(); err != nil {
    fmt.Fprintf(w, "A post reading user and role data error occured. Exact error: " + err.Error())
    return
  }

  // remove duplicates
  udsNoDuplicates := make([]UserData, 0)
  hasDuplicates := make(map[uint64]bool)
  for i := 0; i < len(uds); i++ {
    ud := uds[i]
    _, ok := hasDuplicates[ud.UserId]
    if ! ok {
      udsNoDuplicates = append(udsNoDuplicates, ud)
      hasDuplicates[ud.UserId] = true
    }
  }

  for i := 0; i < len(udsNoDuplicates); i++ {
    ud := &udsNoDuplicates[i]
    ud.Roles = userRoleMap[ud.UserId]
  }

  type Context struct {
    UserDatas []UserData
  }
  ctx := Context{udsNoDuplicates}
  tmpl := template.Must(template.ParseFiles(filepath.Join(getProjectPath(), "templates/users-to-roles-list.html")))
  tmpl.Execute(w, ctx)
}
