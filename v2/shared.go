package v2

import (
	"github.com/gorilla/mux"
	"net/http"
	"runtime"
	"os"
	"html/template"
	"google.golang.org/api/iterator"
)


func serveJS(w http.ResponseWriter, r *http.Request) {
  vars := mux.Vars(r)
  lib := vars["library"]

  if lib == "jquery" {
    http.ServeFile(w, r, "qffiles/jquery-3.3.1.min.js")
  } else if lib == "autosize" {
    http.ServeFile(w, r, "qffiles/autosize.min.js")
  }
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


func getBaseTemplate() string {
  if BaseTemplate != "" {
    return BaseTemplate
  } else {
    return "qffiles/bad-base.html"
  }
}



func errorPage(w http.ResponseWriter, msg string) {
  _, fn, line, _ := runtime.Caller(1)
  type Context struct {
    Message string
    SourceFn string
    SourceLine int
    QF_DEVELOPER bool
  }

  var ctx Context
  if os.Getenv("QF_DEVELOPER") == "true" {
    ctx = Context{msg, fn, line, true}
  } else {
    ctx = Context{msg, fn, line, false}
  }
  tmpl := template.Must(template.ParseFiles(getBaseTemplate(), "qffiles/error-page.html"))
  tmpl.Execute(w, ctx)
}


func GetRoles() (map[string]string, error) {
	iter := Gclient.Collection("qf_roles").Documents(Gctx)
	roles := make(map[string]string)
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}

		name, err := doc.DataAt("name")
		if err != nil {
			return nil, err
		}

		roles[name.(string)] = doc.Ref.ID
	}
	return roles, nil
}

