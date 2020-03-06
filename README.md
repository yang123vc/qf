# qf : quick forms

![alt text](https://github.com/bankole7782/qf/raw/master/qf-logo.png "QF logo")

A website builder for forms. Could be used to write ERPs, CRMs and other record web apps.


## Project Design

1.  The method in use here is to mix it with complicated forms. This provides the
    benefits of one installation ( reducing server maintenance works) and also using
    one authentication system to log on to the system (comfort).

2.  Document Structures (Forms) are not provided with the installation so as to create only what
    one needs. Also it is impossible to get good forms for every use case for there are differences between
    similar organizations.

3.  There is a list of document structures which are accessible to the administrator only.
    To list document structures to the users you would need to write a custom page.
    Reasons for this design are:

    * It makes the list of page very configurable. One could achieve dropdowns, menus on the top, menus on
    the side. With these menus pointing to document structure links.

    * Document Structures pages can be grouped with non-qf pages.


## Projects Used

* Golang
* MySQL


## Setup

### Users Table Setup

Note that user creation and updating is not part of this project. You as the administrator is to provide this. This is to
ensure that you can use any form of authentication you want eg. social auth (Facebook, google, twitter), passwords,
fingerprint, keys etc.

Create a users table with the following properties:
* users table must have a primary key id with type bigint unsigned
* it must also have fields `firstname` and `lastname` for easy recognition.
* it must also have field `email` for communications.
Then set the table name to `qf.UsersTable`

You must also provide a function that would get the currently logged in users. The function is given the request object
to get the cookies for its purpose. Set the `qf.GetCurrentUser` to this function. The function has the following
declaration `func(r *http.Request) (uint64, error)`.

The `qf.GetCurrentUser` should return 0 for public.


### Begin

Get the framework through the following command `go get -u github.com/bankole7782/qf`

There is a sample application which details how to complete the setup. Take a look at it [here](https://github.com/bankole7782/qf/tree/master/qf_example)

Copy the folder `qffiles` from the main repo into the same path as your `main.go`.

Make sure you look at `main.go` in the sample app, copy and edit it to your own preferences.

Go to `/qf-setup/` to create some tables that the project would need.

Then go to `/qf-page/` to start using this project.


### Files Setup

Read [this](https://cloud.google.com/docs/authentication/production) for how to setup a service account
to use in communicating with google cloud storage.

You would need to create a bucket on google cloud storage for all your files in a qf installation. Then
set the name of the bucket to `qf.QFBucketName`.


### Setting up Role Permissions and Approvals for a Document Structure.

Go to `/roles-view/` to create some roles for the project.

Go to `/users-to-roles-list/` to update the users' roles.

To set up permissions or approvals framework for a document structure, go to `/view-document-structure/{document-structure-full-name}/`
you would see the links to do so in this page.


### Listing of Document Structure Links in your Web App

You would need to call `qf.DoesCurrentUserHavePerm` to check if the current user have read permission
to the document structure before listing it. This would ensure a clean interface with the user
seeing only what he uses.

`qf.DoesCurrentUserHavePerm` has the following definition:
`func(r *http.Request, documentStructure string, permission string) (bool, error)`

The permissions to test for is `read`.

The link to display to the user is of the form `/list/{documentStructure}/`. Replace {documentStructure} with
the name of the document structure.


### Creating Inspectors

Inspectors are users who have read access to all the documents in an installation.

To add a user as inspector add his/her id to `qf.Inspectors`


### Theming Your Project

The sample project has no design. To make it beautiful make a template from this template :`qffiles/bad-base.html`
. Save it to your project and then point your version to `qf.BaseTemplate`.

Also if you want to add dynamic contents to any `qf` page, please use JavaScript.
First check the address of the page `window.location` before adding it.


### Adding Extra Code to Your Project

Extra code does things like document validation, after save actions like sending emails, updating read only values.

Steps:

- Go to `/view-document-structure/{document-structure}/` where document-structure is changed to
  the name of a document structure that you created.

- You would see the ID of the document structure.

- qf.ExtraCode has the following definitions:
  ```go
  type ExtraCode struct {
    ValidationFn func(postForm url.Values) string
    AfterCreateFn func(id uint64)
    AfterUpdateFn func(id uint64)
    BeforeDeleteFn func(id uint64)
    CanCreateFn func() string
  }
  ```
  For ValidationFn take a look at [url.Values description](https://golang.org/pkg/net/url/#Values)

- Create a type qf.ExtraCode and add it to the qf.ExtraCodeMap in your main function with
the ID of the document structure as the key. Example is :

  ```go
  validateProfile := func(postForm url.Values) string{
    if postForm.Get("email") == "john@dd.com" {
      return "not valid."
    }
    return ""
  }

  qf.ExtraCodeMap[1] = qf.ExtraCode{ValidationFn: validateProfile}
  ```
- For ValidationFn and CanCreateFn whenever it returns a string it would be taken as an error and printed to the user.
If it doesn't then there is no error.

- Other functions under ExtraCode do not print to screen.

- For AfterCreateFn, AfterUpdateFn and BeforeDeleteFn you would need to do an SQL query to get the document data.


### Setup of Approval Framework Mails

Mail support before and after approvals was not fully done so as to give room for any email platform
and any designs you might want to use in your mail.

To send mails notifying the approver of mails, set an ExtraCode for the document structure. Make sure you
set the AfterCreateFn and the AfterDeleteFn.

To send mails after approvals set the `qf.ApprovalFrameworkMailsFn`. It's definition is
`func(docid uint64, role, status, message string)`


## Upgrading

1.  Upgrade the code
2.  Copy the new `qffiles` to your project. Please delete the old one first.
3.  Run `/qf-upgrade/` to upgrade the database tables.


## FAQs

### Groups of Programmers building on QF. How to merge.

Use a shared database.

You could set up one on a cloud platform.

It's better that having a merger program of databases because the primary keys of the document structure
would need to change. This would invalidate the fields data and would need the document structure IDS to be changed
in the ExtraCodes.


### How do I send mail after saving a document in QF.

Use ExtraCode.

There is no inbuilt mail function so as to give a lot of choices in terms of email provider
and the design of the email itself.


### When is X Database Support Coming

I don't intend to support more than one database so has to make the work cheaper.


### When is X Cloud Support Comming

I don't intend to support more than one cloud.


## License

Released with the MIT License
