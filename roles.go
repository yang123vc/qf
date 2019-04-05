package qf

import (
  "github.com/gorilla/mux"
  "net/http"
  "fmt"
  "html/template"
  "database/sql"
  "strconv"
  "html"
)


func rolesView(w http.ResponseWriter, r *http.Request) {
  truthValue, err := isUserAdmin(r)
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  if ! truthValue {
    errorPage(w, "You are not an admin here. You don't have permissions to view this page.")
    return
  }

  roles, err := GetRoles()
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  type Context struct {
    Roles []string
    NumberOfRoles int
  }
  ctx := Context{roles, len(roles)}
  tmpl := template.Must(template.ParseFiles(getBaseTemplate(), "qffiles/roles-view.html"))
  tmpl.Execute(w, ctx)
}


func newRole(w http.ResponseWriter, r *http.Request) {
  roles, err := GetRoles()
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  if r.Method == http.MethodPost {

    role := html.EscapeString(r.FormValue("role"))
    if len(roles) != 0 {
      for _, rl := range roles {
        if role == rl {
          errorPage(w, fmt.Sprintf("The role \"%s\" already exists.", role))
          return
        }
      }
    }

    _, err := SQLDB.Exec("insert into qf_roles(role) values(?)", role)
    if err != nil {
      errorPage(w, err.Error())
      return
    }

    http.Redirect(w, r, "/roles-view/", 307)
  }
}


func deleteRole(w http.ResponseWriter, r *http.Request) {
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
  role := vars["role"]

  roleid, err := getRoleId(role)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  _, err = SQLDB.Exec("delete from qf_permissions where roleid = ?", roleid)
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  _, err = SQLDB.Exec("delete from qf_user_roles where roleid = ?", roleid)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  _, err = SQLDB.Exec("delete from qf_roles where role=?", role)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  http.Redirect(w, r, "/roles-view/", 307)
}


func usersToRolesList(w http.ResponseWriter, r *http.Request) {
  truthValue, err := isUserAdmin(r)
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  if ! truthValue {
    errorPage(w, "You are not an admin here. You don't have permissions to view this page.")
    return
  }

  type UserData struct {
    UserId uint64
    Firstname string
    Surname string
    Roles []string
  }

  sqlStmt := fmt.Sprintf("select users.id, `%s`.firstname, `%s`.surname, qf_roles.role ", UsersTable, UsersTable)
  sqlStmt += fmt.Sprintf("from (qf_user_roles right join `%s` on qf_user_roles.userid = `%s`.id)", UsersTable, UsersTable)
  sqlStmt += fmt.Sprintf("left join qf_roles on qf_user_roles.roleid = qf_roles.id ")
  sqlStmt += "order by users.id asc"

  uds := make([]UserData, 0)
  var userid uint64
  var firstname, surname string
  var role sql.NullString
  userRoleMap := make(map[uint64][]string)
  rows, err := SQLDB.Query(sqlStmt)
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  defer rows.Close()
  for rows.Next() {
    err := rows.Scan(&userid, &firstname, &surname, &role)
    if err != nil {
      errorPage(w, err.Error())
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
    errorPage(w, err.Error())
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
  tmpl := template.Must(template.ParseFiles(getBaseTemplate(), "qffiles/users-to-roles-list.html"))
  tmpl.Execute(w, ctx)
}


func editUserRoles(w http.ResponseWriter, r *http.Request) {
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
  userid := vars["userid"]
  useridUint64, err := strconv.ParseUint(userid, 10, 64)

  var count int
  sqlStmt := fmt.Sprintf("select count(*) from `%s` where id = %s", UsersTable, userid)
  err = SQLDB.QueryRow(sqlStmt).Scan(&count)
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  if count == 0 {
    errorPage(w, "The userid does not exist.")
    return
  }

  userRoles := make([]string, 0)
  var role string
  rows, err := SQLDB.Query(`select qf_roles.role from qf_roles inner join qf_user_roles on qf_roles.id = qf_user_roles.roleid
    where qf_user_roles.userid = ?`, useridUint64)
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  defer rows.Close()
  for rows.Next() {
    err := rows.Scan(&role)
    if err != nil {
      errorPage(w, err.Error())
      return
    }
    userRoles = append(userRoles, role)
  }
  if err = rows.Err(); err != nil {
    errorPage(w, err.Error())
    return
  }

  if r.Method == http.MethodGet {
    roles, err := GetRoles()
    if err != nil {
      errorPage(w, err.Error())
      return
    }

    var firstname, surname string
    sqlStmt = fmt.Sprintf("select firstname, surname from `%s` where id = %d", UsersTable, useridUint64)
    err = SQLDB.QueryRow(sqlStmt).Scan(&firstname, &surname)
    if err != nil {
      errorPage(w, err.Error())
      return
    }

    type Context struct {
      UserId string
      UserRoles []string
      Roles []string
      FullName string
    }

    ctx := Context{userid, userRoles, roles, firstname + " " + surname}
    tmpl := template.Must(template.ParseFiles(getBaseTemplate(), "qffiles/edit-user-roles.html"))
    tmpl.Execute(w, ctx)

  } else if r.Method == http.MethodPost {

    r.ParseForm()
    newRoles := r.PostForm["roles"]
    stmt, err := SQLDB.Prepare("insert into qf_user_roles(userid, roleid) values(?, ?)")
    if err != nil {
      errorPage(w, err.Error())
      return
    }
    for _, str := range newRoles {
      roleid, err := getRoleId(str)
      if err != nil {
        errorPage(w, err.Error())
        return
      }
      _, err = stmt.Exec(useridUint64, roleid)
      if err != nil {
        errorPage(w, err.Error())
        return
      }
    }

    http.Redirect(w, r, "/users-to-roles-list/", 307)
  }

}


func removeRoleFromUser(w http.ResponseWriter, r *http.Request) {
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
  userid := vars["userid"]
  role := vars["role"]
  useridUint64, err := strconv.ParseUint(userid, 10, 64)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  var count int
  sqlStmt := fmt.Sprintf("select count(*) from `%s` where id = %s", UsersTable, userid)
  err = SQLDB.QueryRow(sqlStmt).Scan(&count)
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  if count == 0 {
    errorPage(w, "The userid does not exist.")
    return
  }

  roleid, err := getRoleId(role)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  _, err = SQLDB.Exec("delete from qf_user_roles where userid = ? and roleid = ?", useridUint64, roleid)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  redirectURL := fmt.Sprintf("/edit-user-roles/%s/", userid)
  http.Redirect(w, r, redirectURL, 307)
}


func deleteRolePermissions(w http.ResponseWriter, r *http.Request) {
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
  role := vars["role"]
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

  roleid, err := getRoleId(role)
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  dsid, err := getDocumentStructureID(ds)
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  _, err = SQLDB.Exec("delete from qf_permissions where roleid = ? and dsid = ?", roleid, dsid)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  redirectURL := fmt.Sprintf("/edit-document-structure-permissions/%s/", ds)
  http.Redirect(w, r, redirectURL, 307)
}


func userDetails(w http.ResponseWriter, r * http.Request) {
  _, err := GetCurrentUser(r)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  vars := mux.Vars(r)
  useridToView := vars["userid"]

  var count int
  sqlStmt := fmt.Sprintf("select count(*) from `%s` where id = %s", UsersTable, useridToView)
  err = SQLDB.QueryRow(sqlStmt).Scan(&count)
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  if count == 0 {
    errorPage(w, "The userid does not exist.")
    return
  }

  var firstname, surname string
  sqlStmt = fmt.Sprintf("select firstname, surname from `%s` where id = %s", UsersTable, useridToView)
  err = SQLDB.QueryRow(sqlStmt).Scan(&firstname, &surname)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  userRoles := make([]string, 0)
  var role string
  rows, err := SQLDB.Query(`select qf_roles.role from qf_roles inner join qf_user_roles on qf_roles.id = qf_user_roles.roleid
    where qf_user_roles.userid = ?`, useridToView)
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  defer rows.Close()
  for rows.Next() {
    err := rows.Scan(&role)
    if err != nil {
      errorPage(w, err.Error())
      return
    }
    userRoles = append(userRoles, role)
  }
  if err = rows.Err(); err != nil {
    errorPage(w, err.Error())
    return
  }

  useridUint64, err := strconv.ParseUint(useridToView, 10, 64)
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  for _, id := range Admins {
    if useridUint64 == id {
      userRoles = append(userRoles, "Administrator")
      break
    }
  }

  type Context struct {
    UserId string
    UserRoles []string
    FullName string
  }

  ctx := Context{useridToView, userRoles, firstname + " " + surname}
  tmpl := template.Must(template.ParseFiles(getBaseTemplate(), "qffiles/user-details.html"))
  tmpl.Execute(w, ctx)
}
