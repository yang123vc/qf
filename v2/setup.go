package v2

import (
  "net/http"
  "github.com/gorilla/mux"
  "net/url"
  "html/template"
  "cloud.google.com/go/firestore"
  "context"
  "fmt"
)

var Gctx context.Context
var Gclient *firestore.Client

var UsersCollection string
var Admins []string
var GetCurrentUser func(r *http.Request) (string, error)
var BaseTemplate string

type ExtraCode struct {
  ValidationFn func(postForm url.Values) string
  AfterCreateFn func(id string)
  AfterUpdateFn func(id string)
  BeforeDeleteFn func(id string)
}

var ExtraCodeMap = make(map[int]ExtraCode)

var ApprovalFrameworkMailsFn func(docid uint64, role, status, message string)

var QFBucketName string
var KeyFilePath string
var GoogleAccessID string


func setupChecks(w http.ResponseWriter, r *http.Request) {

  if UsersCollection == "" {
    errorPage(w, "You have not set the \"qf.UsersCollection\". Create your users table and set this variable to it.")
    return
  }

  if Admins == nil {
    errorPage(w, "You have not set the \"qf.Admins\". Please set this to a list of ids (in uint64) of the Admins of this site.")
    return
  }

  if GetCurrentUser == nil {
    errorPage(w, "You must set the \"qf.GetCurrentUser\". Please set this variable to a function with signature func(r *http.Request) (uint64, err.Error()).")
    return
  }

  if QFBucketName == "" {
    errorPage(w, "You must set the \"qf.QFBucketName\". Create a bucket on google cloud and set it to this variable.")
    return
  }

  fmt.Fprintf(w, "ok")
}


func AddQFHandlers(r *mux.Router) {

  // Please don't change the paths.

  // Please call this link first to do your setup.
  r.HandleFunc("/setup-checks/", setupChecks)
  r.HandleFunc("/qf-page/", qfPage)
  r.HandleFunc("/serve-js/{library}/", serveJS)

  // // document structure links
  // r.HandleFunc("/new-document-structure/", newDocumentStructure)
  // r.HandleFunc("/list-document-structures/", listDocumentStructures)
  // r.HandleFunc("/delete-document-structure/{document-structure}/", deleteDocumentStructure)
  // r.HandleFunc("/view-document-structure/{document-structure}/", viewDocumentStructure)
  // r.HandleFunc("/edit-document-structure-permissions/{document-structure}/", editDocumentStructurePermissions)
  // r.HandleFunc("/edit-document-structure/{document-structure}/", editDocumentStructure)
  // r.HandleFunc("/update-document-structure-name/{document-structure}/", updateDocumentStructureName)
  // r.HandleFunc("/update-help-text/{document-structure}/", updateHelpText)
  // r.HandleFunc("/update-field-labels/{document-structure}/", updateFieldLabels)
  // r.HandleFunc("/delete-fields/{document-structure}/", deleteFields)
  // r.HandleFunc("/change-fields-order/{document-structure}/", changeFieldsOrder)
  // r.HandleFunc("/add-fields/{document-structure}/", addFields)


  // roles links
  r.HandleFunc("/roles-view/", rolesView)
  r.HandleFunc("/new-roles/", newRole)
  r.HandleFunc("/rename-role/", renameRole)
  // r.HandleFunc("/delete-role/{role}/", deleteRole)
  r.HandleFunc("/users-list/", usersList)
  r.HandleFunc("/users-list/{page:[0-9]+}/", usersList)
  r.HandleFunc("/edit-user-roles/{userid}/", editUserRoles)
  r.HandleFunc("/remove-role-from-user/{userid}/{roleid}/", removeRoleFromUser)
  // r.HandleFunc("/delete-role-permissions/{document-structure}/{role}/", deleteRolePermissions)
  // r.HandleFunc("/user-details/{userid}/", userDetails)

  // // document links
  // r.HandleFunc("/create/{document-structure}/", createDocument)
  // r.HandleFunc("/update/{document-structure}/{id:[0-9]+}/", updateDocument)
  // r.HandleFunc("/edit-log/{document-structure}/{id:[0-9]+}/", editLog)
  // r.HandleFunc("/list/{document-structure}/", listDocuments)
  // r.HandleFunc("/list/{document-structure}/{page:[0-9]+}/", listDocuments)
  // r.HandleFunc("/delete/{document-structure}/{id:[0-9]+}/", deleteDocument)
  // r.HandleFunc("/search/{document-structure}/", searchDocuments)
  // r.HandleFunc("/search-results/{document-structure}/", searchResults)
  // r.HandleFunc("/search-results/{document-structure}/{page:[0-9]+}/", searchResults)
  // r.HandleFunc("/delete-search-results/{document-structure}/", deleteSearchResults)
  // r.HandleFunc("/date-lists/{document-structure}/", dateLists)
  // r.HandleFunc("/date-lists/{document-structure}/{page:[0-9]+}/", dateLists)
  // r.HandleFunc("/date-list/{document-structure}/{date}/", dateList)
  // r.HandleFunc("/date-list/{document-structure}/{date}/{page:[0-9]+}/", dateList)
  // r.HandleFunc("/approved-list/{document-structure}/", approvedList)
  // r.HandleFunc("/approved-list/{document-structure}/{page:[0-9]+}/", approvedList)
  // r.HandleFunc("/unapproved-list/{document-structure}/", unapprovedList)
  // r.HandleFunc("/unapproved-list/{document-structure}/{page:[0-9]+}/", unapprovedList)
  // r.HandleFunc("/delete-file/{document-structure}/{id:[0-9]+}/{name}/", deleteFile)

  // // My List links
  // r.HandleFunc("/mylist-setup/{document-structure}/", myListSetup)
  // r.HandleFunc("/remove-list-config/{document-structure}/{field}/{data}/", removeOneMylistConfig)
  // r.HandleFunc("/mylist/{document-structure}/", myList)
  // r.HandleFunc("/mylist/{document-structure}/{page:[0-9]+}/", myList)

  // // Approvals
  // r.HandleFunc("/add-approvals-to-document-structure/{document-structure}/", addApprovals)
  // r.HandleFunc("/remove-approvals-from-document-structure/{document-structure}/", removeApprovals)
  // r.HandleFunc("/approvals/{document-structure}/{id:[0-9]+}/", viewOrUpdateApprovals)

  // // Buttons
  // r.HandleFunc("/create-button/", createButton)
  // r.HandleFunc("/list-buttons/", listButtons)
  // r.HandleFunc("/delete-button/{id}/", deleteButton)

}


func qfPage(w http.ResponseWriter, r *http.Request) {
  truthValue, err := isUserAdmin(r)
  if err != nil {
    errorPage(w, err.Error())
    return
  }
  if ! truthValue {
    errorPage(w, "You are not an admin here. You don't have permissions to view this page.")
    return
  }

  tmpl := template.Must(template.ParseFiles(getBaseTemplate(), "qffiles/qf-page.html"))
  tmpl.Execute(w, nil)
}
