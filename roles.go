package qf

import (
  "github.com/gorilla/mux"
  "net/http"
  "fmt"
  "path/filepath"
  "html/template"
  "sort"
  "database/sql"
  "strconv"
  "strings"
)


func getRoles(w http.ResponseWriter) ([]string, bool) {
  strSlice := make([]string, 0)
  var str string
  rows, err := SQLDB.Query("select role from qf_roles order by role asc")
  if err != nil {
    fmt.Fprintf(w, "Error occured while trying to collect roles. Exact Error: " + err.Error())
    return strSlice, false
  }
  defer rows.Close()
  for rows.Next() {
    err := rows.Scan(&str)
    if err != nil {
      fmt.Fprintf(w, "Error occured while trying to collect role. Exact Error: " + err.Error())
      return strSlice, false
    }
    strSlice = append(strSlice, str)
  }
  if err = rows.Err(); err != nil {
    fmt.Fprintf(w, "An error occured: " + err.Error())
    return strSlice, false
  }
  return strSlice, true
}


func RolesView(w http.ResponseWriter, r *http.Request) {
  roles, ok := getRoles(w)
  if ! ok {
    return
  }

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

  roleid, err := getRoleId(role)
  if err != nil {
    fmt.Fprintf(w, "Error occured while getting role id. Exact Error: " + err.Error())
    return
  }

  _, err = SQLDB.Exec("delete from qf_permissions where roleid = ?", roleid)
  if err != nil {
    fmt.Fprintf(w, "Error occured while deleting role permissions. Exact Error: " + err.Error())
    return
  }
  _, err = SQLDB.Exec("delete from qf_user_roles where roleid = ?", roleid)
  if err != nil {
    fmt.Fprintf(w, "Error occured while deleting user and this role data. Exact Error: " + err.Error())
    return
  }

  _, err = SQLDB.Exec("delete from qf_roles where role=?", role)
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


func EditUserRoles(w http.ResponseWriter, r *http.Request) {
  vars := mux.Vars(r)
  userid := vars["userid"]
  useridUint64, err := strconv.ParseUint(userid, 10, 64)

  var count int
  sqlStmt := fmt.Sprintf("select count(*) from `%s` where id = %s", UsersTable, userid)
  err = SQLDB.QueryRow(sqlStmt).Scan(&count)
  if err != nil {
    fmt.Fprintf(w, "An error occured when verifiying whether the user id exists. Exact error: " + err.Error())
    return
  }
  if count == 0 {
    fmt.Fprintf(w, "The userid does not exist.")
    return
  }

  userRoles := make([]string, 0)
  var role string
  rows, err := SQLDB.Query(`select qf_roles.role from qf_roles inner join qf_user_roles on qf_roles.id = qf_user_roles.roleid
    where qf_user_roles.userid = ?`, useridUint64)
  if err != nil {
    fmt.Fprintf(w, "Error reading roles. Exact Error: " + err.Error())
    return
  }
  defer rows.Close()
  for rows.Next() {
    err := rows.Scan(&role)
    if err != nil {
      fmt.Fprintf(w, "Error reading single row. Exact Error: " + err.Error())
      return
    }
    userRoles = append(userRoles, role)
  }
  if err = rows.Err(); err != nil {
    fmt.Fprintf(w, "Error after reading roles. Exact Error: " + err.Error())
    return
  }

  if r.Method == http.MethodGet {
    roles, ok := getRoles(w)
    if ! ok {
      return
    }

    var firstname, surname string
    sqlStmt = fmt.Sprintf("select firstname, surname from `%s` where id = %d", UsersTable, useridUint64)
    err = SQLDB.QueryRow(sqlStmt).Scan(&firstname, &surname)
    if err != nil {
      fmt.Fprintf(w, "Error reading user details. Exact Error: " + err.Error())
      return
    }

    type Context struct {
      UserId string
      UserRoles []string
      RolesStr string
      FullName string
    }

    ctx := Context{userid, userRoles, strings.Join(roles, ","), firstname + " " + surname}
    tmpl := template.Must(template.ParseFiles(filepath.Join(getProjectPath(), "templates/edit-user-roles.html")))
    tmpl.Execute(w, ctx)

  } else if r.Method == http.MethodPost {

    newRoles := strings.Split(r.FormValue("roles"), "\n")
    stmt, err := SQLDB.Prepare("insert into qf_user_roles(userid, roleid) values(?, ?)")
    if err != nil {
      fmt.Fprintf(w, "An error occured while trying to make prepared statemnt. Exact Error: " + err.Error())
      return
    }
    for _, str := range newRoles {
      t := strings.TrimSpace(str)
      if t == "" {
        continue
      }
      roleid, err := getRoleId(t)
      if err != nil {
        fmt.Fprintf(w, "An error occured while trying to get roleid. Exact Error: " + err.Error())
        return
      }
      _, err = stmt.Exec(useridUint64, roleid)
      if err != nil {
        fmt.Fprintf(w, "An error occured while writing a user role. Exact Error: " + err.Error())
        return
      }
    }

    http.Redirect(w, r, "/users-to-roles-list/", 307)
  }

}


func RemoveRoleFromUser(w http.ResponseWriter, r *http.Request) {
  vars := mux.Vars(r)
  userid := vars["userid"]
  role := vars["role"]
  useridUint64, err := strconv.ParseUint(userid, 10, 64)
  if err != nil {
    fmt.Fprintf(w, "The userid is not a uint64. Exact Error: " + err.Error())
    return
  }

  var count int
  sqlStmt := fmt.Sprintf("select count(*) from `%s` where id = %s", UsersTable, userid)
  err = SQLDB.QueryRow(sqlStmt).Scan(&count)
  if err != nil {
    fmt.Fprintf(w, "An error occured when verifiying whether the user id exists. Exact error: " + err.Error())
    return
  }
  if count == 0 {
    fmt.Fprintf(w, "The userid does not exist.")
    return
  }

  roleid, err := getRoleId(role)
  if err != nil {
    fmt.Fprintf(w, "Error occured while getting role id. Exact Error: " + err.Error())
    return
  }

  _, err = SQLDB.Exec("delete from qf_user_roles where userid = ? and roleid = ?", useridUint64, roleid)
  if err != nil {
    fmt.Fprintf(w, "Error occured while deleting role. Exact Error: " + err.Error())
    return
  }

  redirectURL := fmt.Sprintf("/edit-user-roles/%s/", userid)
  http.Redirect(w, r, redirectURL, 307)
}


func DeleteRolePermissions(w http.ResponseWriter, r *http.Request) {
  vars := mux.Vars(r)
  role := vars["role"]
  documentSchema := vars["document-schema"]

  if ! docExists(documentSchema, w) {
    fmt.Fprintf(w, "The document schema %s does not exists.", documentSchema)
    return
  }

  roleid, err := getRoleId(role)
  if err != nil {
    fmt.Fprintf(w, "Error occured while getting role id. Exact Error: " + err.Error())
    return
  }

  _, err = SQLDB.Exec("delete from qf_permissions where roleid = ? and object = ?", roleid, documentSchema)
  if err != nil {
    fmt.Fprintf(w, "Error occured while trying to delete permission of roles on this document. Exact Error: " + err.Error())
    return
  }

  redirectURL := fmt.Sprintf("/edit-document-schema-permissions/%s/", documentSchema)
  http.Redirect(w, r, redirectURL, 307)
}
