# How it Works.


## Interactions with the Database

This project does not make use of any orm, or wrapper around the mysql database. It uses
SQL directly. This decision leads to a lot of flexibility and gives the ability to write
some queries that would not be supported in an orm.

For some lists they are comma seperated and saved as a varchar(255) in the database.

Boolean fields are saved as char(1) with truth values saved as 't' and false values
saved as 'f'.


## Viewing Documents and Listing Documents

The SQL statements backing viewing documents and listing documents is a bit different. This
is because we can't tell the fields before hand and create variables for the fields.

It works by getting the column names from `qf_fields` and then looping through the columns and getting
the data for each column. It also makes use of the theory that every data can be saved as string.


## UI

This project makes use of jquery for things like adding repetitions of some fields. This
project is impossible without the use of javascript which jquery makes a lot easier.


## Document Structures.

On creation of a document structure, its data is first saved to a table `qf_document_structures`
and `qf_fields` before creating a table for the document structure. This data is used to create forms.
Forms such as new document form and the update document form.


## Configuration Data

The project makes use of global variables to communicate between a project using this framework and
the framework itself.
