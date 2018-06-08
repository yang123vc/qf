# qf

__qf : quick forms__

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

After copying the paths in the sample, go to `/qf-setup/` to create some tables that
the project would need.



## Theming Your Project

The sample project has no design. To add yours copy the template `templates/bad-base.html` to your project.
Edit it and then point your version to `qf.BaseTemplate` before registering any handlers.



## Adding Extra Code to Your Project

Extra code does things like document validation, after save actions like sending emails, updating read only values.

Steps:

1. Go to `/view-document-structure/{document-structure}/` where document-structure is changed to
  the name of a document structure that you created.

2. You would see some instructions on the name of the command to create.

3. The command is expected going to receive some arguments.

  1.  For validation `qfec3 v {jsonstring}` where jsonString is a json representation of the input.
  If you print from this command, it would be returned as an error to the page.

  2. For after new save `qfec3 n {id}` where id is the primary key of the newly created document.

  3. For after update `qfec3 u {id}` where id is the primary key of the updated document.

3. Create the command and add it to path.



## License

Released with the MIT License
