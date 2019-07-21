package v2

import (
  "github.com/gorilla/mux"
  "net/http"
  "fmt"
  "html/template"
  "strconv"
  // "html"
  "strings"
  "math"
  "github.com/adam-hanna/arrayOperations"
  "cloud.google.com/go/firestore"
  "google.golang.org/api/iterator"
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

  roleNames := make([]string, 0)
  for k, _ := range roles {
    roleNames = append(roleNames, k)
  }

  type Context struct {
    Roles map[string]string
    NumberOfRoles int
    RolesStr string
  }
  ctx := Context{roles, len(roleNames), strings.Join(roleNames, ",,,")}
  tmpl := template.Must(template.ParseFiles(getBaseTemplate(), "qffiles/roles-view.html"))
  tmpl.Execute(w, ctx)
}


func newRole(w http.ResponseWriter, r *http.Request) {
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

  if r.Method == http.MethodPost {
    newRoles := strings.Split(strings.TrimSpace(r.FormValue("roles")), "\n")

    toInsert := arrayOperations.DistinctString(newRoles)
    for _, r := range toInsert {
      r = strings.TrimSpace(r)
      if len(r) == 0 {
        continue
      }

      found := false
      for _, rl := range roles {
        if r == rl {
          found = true
          break
        }
      }

      if ! found {
        _, _, err := Gclient.Collection("qf_roles").Add(Gctx, map[string]interface{}{"name": r })
        if err != nil {
          errorPage(w, err.Error())
          return
        }
      }
    }

    http.Redirect(w, r, "/roles-view/", 307)
  }
}


func renameRole(w http.ResponseWriter, r *http.Request) {
  truthValue, err := isUserAdmin(r)
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  if ! truthValue {
    errorPage(w, "You are not an admin here. You don't have permissions to view this page.")
    return
  }

  if r.Method == http.MethodPost {
    _, err := Gclient.Collection("qf_roles").Doc(r.FormValue("role-to-rename")).Set(Gctx, map[string]interface{}{
      "name": r.FormValue("new-name"),
    }, firestore.MergeAll)
    if err != nil {
      errorPage(w, err.Error())
      return
    }

    http.Redirect(w, r, "/roles-view/", 307)
  }
}


// func deleteRole(w http.ResponseWriter, r *http.Request) {
//   truthValue, err := isUserAdmin(r)
//   if err != nil {
//     errorPage(w, err.Error())
//     return
//   }
//   if ! truthValue {
//     errorPage(w, "You are not an admin here. You don't have permissions to view this page.")
//     return
//   }

//   vars := mux.Vars(r)
//   role := vars["role"]

//   roleid, err := getRoleId(role)
//   if err != nil {
//     errorPage(w, err.Error())
//     return
//   }

//   _, err = SQLDB.Exec("delete from qf_permissions where roleid = ?", roleid)
//   if err != nil {
//     errorPage(w, err.Error())
//     return
//   }
//   _, err = SQLDB.Exec("delete from qf_user_roles where roleid = ?", roleid)
//   if err != nil {
//     errorPage(w, err.Error())
//     return
//   }

//   _, err = SQLDB.Exec("delete from qf_roles where role=?", role)
//   if err != nil {
//     errorPage(w, err.Error())
//     return
//   }

//   http.Redirect(w, r, "/roles-view/", 307)
// }


func usersList(w http.ResponseWriter, r *http.Request) {
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
  page := vars["page"]
  var pageI int
  if page != "" {
    pageI, err = strconv.Atoi(page)
    if err != nil {
      errorPage(w, err.Error())
      return
    }
  } else {
    pageI = 1
  }

  type UserInfo struct {
    ID string
    Firstname string
    Surname string
    Email string
  }

  ufs := make([]UserInfo, 0)

  allDocs, err := Gclient.Collection(UsersCollection).Documents(Gctx).GetAll()
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  totalItems := len(allDocs)
  totalPages := math.Ceil( float64(totalItems) / float64(50) )

  startPoint := (pageI - 1) * 50
  docs := Gclient.Collection(UsersCollection).OrderBy("firstname", firestore.Asc).StartAt(startPoint).Limit(50).Documents(Gctx)
  for {
    doc, err := docs.Next()
    if err == iterator.Done {
      break
    }
    if err != nil {
      errorPage(w, err.Error())
      return
    }

    data := doc.Data()
    uf := UserInfo{doc.Ref.ID, data["firstname"].(string), data["surname"].(string), data["email"].(string)}
    ufs = append(ufs, uf)
  }

  type Context struct {
    UserInfos []UserInfo
    Pages []uint64
  }
  pages := make([]uint64, 0)
  for i := uint64(0); i < uint64(totalPages); i++ {
    pages = append(pages, i+1)
  }

  ctx := Context{ufs, pages}
  tmpl := template.Must(template.ParseFiles(getBaseTemplate(), "qffiles/users-list.html"))
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

  dsnap, err := Gclient.Collection(UsersCollection).Doc(userid).Get(Gctx)
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  if ! dsnap.Exists() {
    errorPage(w, "The userid does not exist.")
    return
  }


  userRoles := make(map[string]string)
  iter := Gclient.Collection("qf_user_roles").Where("userid", "==", userid).Documents(Gctx)
  for {
    doc, err := iter.Next()
    if err == iterator.Done {
      break
    }
    if err != nil {
      errorPage(w, err.Error())
      return
    }

    roleid, err := doc.DataAt("roleid")
    if err != nil {
      errorPage(w, err.Error())
      return
    }

    rdsnap, err := Gclient.Collection("qf_roles").Doc(roleid.(string)).Get(Gctx)
    if err != nil {
      errorPage(w, err.Error())
      return
    }
    rData := rdsnap.Data()
    userRoles[rData["name"].(string)] = roleid.(string)
  }


  if r.Method == http.MethodGet {
    roles, err := GetRoles()
    if err != nil {
      errorPage(w, err.Error())
      return
    }

    type Context struct {
      UserId string
      UserRoles map[string]string
      Roles map[string]string
      FullName string
    }

    data := dsnap.Data()
    ctx := Context{userid, userRoles, roles, data["firstname"].(string) + " " + data["surname"].(string)}
    tmpl := template.Must(template.ParseFiles(getBaseTemplate(), "qffiles/edit-user-roles.html"))
    tmpl.Execute(w, ctx)

  } else if r.Method == http.MethodPost {

    r.ParseForm()
    newRoles := r.PostForm["roles"]
    for _, str := range newRoles {
      _, _, err := Gclient.Collection("qf_user_roles").Add(Gctx, map[string]interface{}{"roleid": str, "userid": userid })
      if err != nil {
        errorPage(w, err.Error())
        return
      }
    }

    http.Redirect(w, r, "/users-list/", 307)
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
  roleid := vars["roleid"]

  dsnap, err := Gclient.Collection(UsersCollection).Doc(userid).Get(Gctx)
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  if ! dsnap.Exists() {
    errorPage(w, "The userid does not exist.")
    return
  }

  iter := Gclient.Collection("qf_user_roles").Where("userid", "==", userid).Where("roleid", "==", roleid).Documents(Gctx)
  for {
    doc, err := iter.Next()
    if err == iterator.Done {
      break
    }
    if err != nil {
      errorPage(w, err.Error())
      return
    }

    _, err = doc.Ref.Delete(Gctx)
    if err != nil {
      errorPage(w, err.Error())
      return
    }
  }

  redirectURL := fmt.Sprintf("/edit-user-roles/%s/", userid)
  http.Redirect(w, r, redirectURL, 307)
}


// func deleteRolePermissions(w http.ResponseWriter, r *http.Request) {
//   truthValue, err := isUserAdmin(r)
//   if err != nil {
//     errorPage(w, err.Error())
//     return
//   }
//   if ! truthValue {
//     errorPage(w, "You are not an admin here. You don't have permissions to view this page.")
//     return
//   }

//   vars := mux.Vars(r)
//   role := vars["role"]
//   ds := vars["document-structure"]

//   detv, err := docExists(ds)
//   if err != nil {
//     errorPage(w, err.Error())
//     return
//   }
//   if detv == false {
//     errorPage(w, fmt.Sprintf("The document structure %s does not exists.", ds))
//     return
//   }

//   roleid, err := getRoleId(role)
//   if err != nil {
//     errorPage(w, err.Error())
//     return
//   }
//   dsid, err := getDocumentStructureID(ds)
//   if err != nil {
//     errorPage(w, err.Error())
//     return
//   }
//   _, err = SQLDB.Exec("delete from qf_permissions where roleid = ? and dsid = ?", roleid, dsid)
//   if err != nil {
//     errorPage(w, err.Error())
//     return
//   }

//   redirectURL := fmt.Sprintf("/edit-document-structure-permissions/%s/", ds)
//   http.Redirect(w, r, redirectURL, 307)
// }


// func userDetails(w http.ResponseWriter, r * http.Request) {
//   _, err := GetCurrentUser(r)
//   if err != nil {
//     errorPage(w, err.Error())
//     return
//   }

//   vars := mux.Vars(r)
//   useridToView := vars["userid"]

//   var count int
//   sqlStmt := fmt.Sprintf("select count(*) from `%s` where id = %s", UsersTable, useridToView)
//   err = SQLDB.QueryRow(sqlStmt).Scan(&count)
//   if err != nil {
//     errorPage(w, err.Error())
//     return
//   }
//   if count == 0 {
//     errorPage(w, "The userid does not exist.")
//     return
//   }

//   var firstname, surname string
//   sqlStmt = fmt.Sprintf("select firstname, surname from `%s` where id = %s", UsersTable, useridToView)
//   err = SQLDB.QueryRow(sqlStmt).Scan(&firstname, &surname)
//   if err != nil {
//     errorPage(w, err.Error())
//     return
//   }

//   userRoles := make([]string, 0)
//   var role string
//   rows, err := SQLDB.Query(`select qf_roles.role from qf_roles inner join qf_user_roles on qf_roles.id = qf_user_roles.roleid
//     where qf_user_roles.userid = ?`, useridToView)
//   if err != nil {
//     errorPage(w, err.Error())
//     return
//   }
//   defer rows.Close()
//   for rows.Next() {
//     err := rows.Scan(&role)
//     if err != nil {
//       errorPage(w, err.Error())
//       return
//     }
//     userRoles = append(userRoles, role)
//   }
//   if err = rows.Err(); err != nil {
//     errorPage(w, err.Error())
//     return
//   }

//   useridUint64, err := strconv.ParseUint(useridToView, 10, 64)
//   if err != nil {
//     errorPage(w, err.Error())
//     return
//   }
//   for _, id := range Admins {
//     if useridUint64 == id {
//       userRoles = append(userRoles, "Administrator")
//       break
//     }
//   }

//   type Context struct {
//     UserId string
//     UserRoles []string
//     FullName string
//   }

//   ctx := Context{useridToView, userRoles, firstname + " " + surname}
//   tmpl := template.Must(template.ParseFiles(getBaseTemplate(), "qffiles/user-details.html"))
//   tmpl.Execute(w, ctx)
// }
