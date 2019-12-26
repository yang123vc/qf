package qf

import (
  "net/http"
  "html/template"
  "database/sql"
  "fmt"
  "strings"
  "github.com/gorilla/mux"
  "strconv"
)


func newDSGroup(w http.ResponseWriter, r *http.Request) {
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
      DSGroups string
    }
    var dsgl sql.NullString
    err = SQLDB.QueryRow("select group_concat(group_name separator ',,,') from qf_dsgroups").Scan(&dsgl)
    if err != nil {
      errorPage(w, err.Error())
      return
    }

    tmpl := template.Must(template.ParseFiles(getBaseTemplate(), "qffiles/new-dsgroup.html"))
    tmpl.Execute(w, Context{dsgl.String})
  } else {
    dsGroupName := r.FormValue("group-name")
    _, err = SQLDB.Exec("insert into qf_dsgroups (group_name) values (?)", dsGroupName)
    if err != nil {
      errorPage(w, err.Error())
      return
    }

    redirectURL := fmt.Sprintf("/manage-dsgroup/%s/", dsGroupName)
    http.Redirect(w, r, redirectURL, 307)
  }
}


func manageDSGroup(w http.ResponseWriter, r *http.Request) {
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
  dsg := vars["document-structure-group"]

  var count int
  err = SQLDB.QueryRow("select count(*) from qf_dsgroups where group_name = ?", dsg).Scan(&count)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  if count == 0 {
    errorPage(w, fmt.Sprintf("The dsgroup '%s' does not exists.", dsg))
    return
  }

  var dsGroupId int
  err = SQLDB.QueryRow("select id from qf_dsgroups where group_name = ?", dsg).Scan(&dsGroupId)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  type LinkAllDesc struct {
    Id int
    DText string
    Link string
  }

  lads := make([]LinkAllDesc, 0)
  var id int
  var dtext, link string

  rows, err := SQLDB.Query("select id, display_text, link from qf_dsgroups_links where dsgroupid = ?", dsGroupId)
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  defer rows.Close()
  for rows.Next() {
    err := rows.Scan(&id, &dtext, &link)
    if err != nil {
      errorPage(w, err.Error())
      return
    }
    lads = append(lads, LinkAllDesc{id, dtext, link})
  }
  err = rows.Err()
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  type Context struct {
    DSGroup string
    DSList []string
    IsChecked func(string) bool
    ExtraLinks []LinkAllDesc
  }

  var tmp sql.NullString
  qStmt := `select group_concat(fullname separator ',,,') from qf_dsgroups_ds inner join qf_document_structures on
  qf_dsgroups_ds.dsid = qf_document_structures.id
  `
  err = SQLDB.QueryRow(qStmt).Scan(&tmp)
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  alreadySelected := strings.Split(tmp.String, ",,,")

  isChecked := func(ds string) bool {
    for _, ds2 := range alreadySelected {
      if ds == ds2 {
        return true
      }
    }
    return false
  }

  dsList, err := GetDocumentStructureList()
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  tmpl := template.Must(template.ParseFiles(getBaseTemplate(), "qffiles/manage-dsgroup.html"))
  tmpl.Execute(w, Context{dsg, dsList, isChecked, lads})
}


func updateDSGroupOnDS(w http.ResponseWriter, r *http.Request) {
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
  dsg := vars["document-structure-group"]

  var count int
  err = SQLDB.QueryRow("select count(*) from qf_dsgroups where group_name = ?", dsg).Scan(&count)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  if count == 0 {
    errorPage(w, fmt.Sprintf("The dsgroup '%s' does not exists.", dsg))
    return
  }

  r.FormValue("email")
  selectedDSs := r.PostForm["selected-dss"]

  var dsGroupId int
  err = SQLDB.QueryRow("select id from qf_dsgroups where group_name = ?", dsg).Scan(&dsGroupId)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  _, err = SQLDB.Exec("delete from qf_dsgroups_ds where dsgroupid = ?", dsGroupId)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  for _, ds := range selectedDSs {
    dsid, err := getDocumentStructureID(ds)
    if err != nil {
      errorPage(w, err.Error())
      return
    }

    _, err = SQLDB.Exec("insert into qf_dsgroups_ds (dsgroupid, dsid) values (?, ?)", dsGroupId, dsid)
    if err != nil {
      errorPage(w, err.Error())
      return
    }
  }

  redirectURL := fmt.Sprintf("/manage-dsgroup/%s/", dsg)
  http.Redirect(w, r, redirectURL, 307)
}


func updateDSGroupOnLinks(w http.ResponseWriter, r *http.Request) {
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
  dsg := vars["document-structure-group"]

  var count int
  err = SQLDB.QueryRow("select count(*) from qf_dsgroups where group_name = ?", dsg).Scan(&count)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  if count == 0 {
    errorPage(w, fmt.Sprintf("The dsgroup '%s' does not exists.", dsg))
    return
  }

  r.FormValue("email")

  type LinkDesc struct {
    DText string
    Link string
  }

  lds := make([]LinkDesc, 0)
  for i := 1; i < 100; i++ {
    iStr := strconv.Itoa(i)
    if r.FormValue("dtext-" + iStr) == "" {
      break
    } else {
      ld := LinkDesc{r.FormValue("dtext-" + iStr), r.FormValue("link-" + iStr)}
      lds = append(lds, ld)
    }
  }

  var dsGroupId int
  err = SQLDB.QueryRow("select id from qf_dsgroups where group_name = ?", dsg).Scan(&dsGroupId)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  for _, ld := range lds {
    _, err = SQLDB.Exec("insert into qf_dsgroups_links (dsgroupid, display_text, link) values (?, ?, ?)",
      dsGroupId, ld.DText, ld.Link)
    if err != nil {
      errorPage(w, err.Error())
      return
    }
  }

  redirectURL := fmt.Sprintf("/manage-dsgroup/%s/", dsg)
  http.Redirect(w, r, redirectURL, 307)
}


func deleteExtraLink(w http.ResponseWriter, r *http.Request) {
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
  dsg := vars["document-structure-group"]
  linkId := vars["id"]

  var count int
  err = SQLDB.QueryRow("select count(*) from qf_dsgroups where group_name = ?", dsg).Scan(&count)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  if count == 0 {
    errorPage(w, fmt.Sprintf("The dsgroup '%s' does not exists.", dsg))
    return
  }

  _, err = SQLDB.Exec("delete from qf_dsgroups_links where id = ?", linkId)
  if err != nil {
    errorPage(w, err.Error())
    return
  }

  redirectURL := fmt.Sprintf("/manage-dsgroup/%s/", dsg)
  http.Redirect(w, r, redirectURL, 307)
}
