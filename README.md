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

- You would see some instructions on the name of the command to create. e.g. qfec3.

- The command is expected to receive arguments in the following format:

  - For validation `qfec3 v {jsonstring}` where jsonString is a json representation of the input.
    If you print from this command, it would be returned as an error to the page.

  - For after new save `qfec3 n {id}` where id is the primary key of the newly created document.

  - For after update `qfec3 u {id}` where id is the primary key of the updated document.

- Create the command and add it to path.



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
