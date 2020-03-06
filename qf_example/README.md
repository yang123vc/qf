# qf_example
An example use of qf.


## How to Run
Set the following environment variables before running this project:

1.	`QF_MYSQL_USER`
2.	`QF_MYSQL_PASSWORD`
3.	`QF_MYSQL_DATABASE`
4.	`QF_GCLOUD_BUCKET`

Copy the `qffiles` from the [qf](https://github.com/bankole7782/qf) to the root folder of your
copy of this repository. Take note to copy the `qffiles` from the same version which you are using.
Especially if you using not the latest version of the [qf framework](https://github.com/bankole7782/qf).

This test project is designed to be minimal. So it uses environment variable to get the current
user while the qf project provides facilities to get the current user using cookies.

To run this project do it this way `USERID=3 go run main.go` with the current user being 3 for example.
