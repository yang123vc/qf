package main

import (
  "net/http"
  "github.com/bankole7782/qf"
  "database/sql"
  _ "github.com/go-sql-driver/mysql"
  "os"
  "fmt"
  "github.com/gorilla/mux"
  "strconv"
)

func main() {
  user := os.Getenv("QF_MYSQL_USER")
  pass := os.Getenv("QF_MYSQL_PASSWORD")
  database := os.Getenv("QF_MYSQL_DATABASE")

  mysqlConnectStr := fmt.Sprintf("%s:%s@tcp(127.0.0.1:3306)/%s", user, pass, database)
  db, err := sql.Open("mysql", mysqlConnectStr)
  if err != nil {
    panic(err)
  }
  defer db.Close()
  if err = db.Ping(); err != nil {
    panic(err)
  }

  // QF setup. Very important
  qf.SQLDB = db
  qf.SiteDB = database

  // qf.UsersTable must have a primary key id with type bigint unsigned
  // it must also have fields firstname and lastname for easy recognition.
  // it must also have field email for communications.
  qf.UsersTable = "users"
  qf.Admins = []uint64{1, 2}
  qf.Inspectors = []uint64{5,}

  // This test makes use of environment variables to get the current user. Real life application
  // could save a random string to the browser cookies. And this random string point to a userid
  // in the database.
  // The function accepts http.Request as argument which can be used to get the cookies.
  qf.GetCurrentUser = func(r *http.Request) (uint64, error) {
    userid := os.Getenv("USERID")
    if userid == "" {
      return 0, nil
    }
    useridUint64, err := strconv.ParseUint(userid, 10, 64)
    if err != nil {
      return 0, err
    }
    return useridUint64, nil
  }

  // qf.ExtraCodeMap[1] = qf.ExtraCode{CanCreateFn: testCreateFn}

  qf.QFBucketName = os.Getenv("QF_GCLOUD_BUCKET")

  // qf.BaseTemplate = "basetemplate.html"
  r := mux.NewRouter()
  qf.AddQFHandlers(r)

  http.ListenAndServe(":3001", r)
}
