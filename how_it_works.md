# How it Works.

## Configuration Data

The project makes use of global variables to communicate between a project using this framework and
the framework itself.


## Interactions with the Database

This project does not make use of any orm, or wrapper around the mysql database. It uses
SQL directly. This decision leads to a lot of flexibility and gives the ability to write
some queries that would not be supported in an orm.

For some lists they are comma seperated and saved as a varchar(255) in the database eg Tables.

Boolean fields are saved as char(1) with truth values saved as 't' and false values
saved as 'f'.


## Viewing Documents and Listing Documents

It makes use of the theory that every piece of data can be stored in a string.


## UI

This project makes use of jquery for things like adding repetitions of some fields. This
project is impossible without the use of javascript which jquery makes a lot easier.


## Document Structures.

On creation of a document structure, its data is first saved to a table `qf_document_structures`
and `qf_fields` before creating a table for the document structure. This data is used to create forms.
Forms such as new document form and the update document form.


## Editing Tables

For comfort sake and for the fact that the primary keys of tables are not used in the UI,
every editing of tables is programmed to be a delete of old information and insertion of
new information.

So for every edit action, the primary keys of the child tables would advance.


## Files

Any uploaded file is not stored directly on database. Instead it is stored on Google Cloud Storage (GCS).

For conflicts sake eg. Five or more people uploading documents all with the name certificate.jpg, the names
are replaced with an long random string. The long random string reduces the case of conflicts.

What is stored about files to the database is the path of the file. This part consists of the database name
of the table (this enables renaming and categorization) followed by a forward slash and ends
with the generated random name for the document.
