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

  if r.Method == http.MethodGet {
    var rolesConcatenated string
    var strFromDB sql.NullString
    err = SQLDB.QueryRow(`select group_concat(qf_roles.role separator "\n")
    from qf_roles inner join qf_user_roles on qf_roles.id = qf_user_roles.roleid
    where qf_user_roles.userid = ?`, useridUint64).Scan(&strFromDB)
    if err != nil {
      fmt.Fprintf(w, "Error reading roles. Exact Error: " + err.Error())
      return
    }
    if strFromDB.Valid {
      rolesConcatenated = strFromDB.String
    }

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
      RolesConcatenated string
      RolesStr string
      FullName string
    }

    ctx := Context{userid, rolesConcatenated, strings.Join(roles, ","), firstname + " " + surname}
    tmpl := template.Must(template.ParseFiles(filepath.Join(getProjectPath(), "templates/edit-user-roles.html")))
    tmpl.Execute(w, ctx)
  }


}
