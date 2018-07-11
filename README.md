# qf

**qf : quick forms**

The method in use here is to mix it with complicated forms. This provides the
benefits of one installation ( reducing server maintenance works) and also using
one authentication system to log on to the system (comfort).


## Projects Used

* Golang
* MySQL
* Ubuntu


## Setup

Get the framework through the following command
`go get github.com/bankole7782/qf`

There is a sample application which details how to complete the setup. Take a look at it [here](https://github.com/bankole7782/qf_example)

Make sure you look at `main.go` in the sample app, copy and edit it to your own preferences.

Go to `/qf-setup/` to create some tables that the project would need.

The links used in the project can be found in the function `AddQFHandlers` in the utils.go of this project.


## Theming Your Project

The sample project has no design. To add yours copy the template `templates/bad-base.html` to your project.
Edit it and then point your version to `qf.BaseTemplate` before registering any handlers.



## Adding Extra Code to Your Project

Extra code does things like document validation, after save actions like sending emails, updating read only values.

Steps:

- Go to `/view-document-structure/{document-structure}/` where document-structure is changed to
  the name of a document structure that you created.

- You would see the number of the document structure.

- qf.ExtraCode has the following definitions:
  ```go
  type ExtraCode struct {
    DSNo int
    ValidationFn func(jsonData string) string
    AfterCreateFn func(id uint64)
    AfterUpdateFn func(id uint64)
    AfterDeleteFn func(jsonData string)
  }
  ```

- Create a type qf.ExtraCode and add it to the qf.ExtraCodeList in your main function. Example is :

  ```go
  validateProfile := func(jsonData string) string{
    fmt.Println(jsonData)
    return "not valid."
  }

  qf.ExtraCodeList = make([]qf.ExtraCode, 0)
  qf.ExtraCodeList = append(qf.ExtraCodeList, qf.ExtraCode{DSNo: 1, ValidationFn: validateProfile})
  ```
 - For ValidationFn whenever it returns a string it would be taken as an error and printed to the user.
  If it doesn't then there is no error.

 - Other functions under ExtraCode do not print to screen.



## Setup of Approval Framework Mails

Mail support before and after approvals was not fully done so as to give room for any designs you might
want to use in your mail.

To send mails notifying the approver of mails, set an ExtraCode for the document structure. Make sure you
set the AfterCreateFn and the AfterDeleteFn.

To send mails after approvals set the `qf.ApprovalFrameworkMailsFn`. It's definition is
`func(docid uint64, role, status, message string)`



## Listing of Document Structure Links in your Web App

There is a list of document structures which are accessible to the admininstrator only. It lists
all the document structures on your installation. To list document structures to the users
you would need to write a custom page. This is because you might have a category page which mixes
complicated forms, and forms created with this project.

You would need to call `qf.DoesCurrentUserHavePerm` to check if the current user have read permission
to the document structure before listing it. This would ensure a clean interface with the user
seeing only what he uses.

`qf.DoesCurrentUserHavePerm` has the following definition:
`func(r *http.Request, documentStructure string, permission string) (bool, error)`

The link to display to the user is of the form `doc/{documentStructure}/list/`


## License

Released with the MIT License
