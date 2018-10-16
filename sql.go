package main

import (
	"database/sql"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"os"
)

type Credentials struct {
	Login    *string
	Password *string
}

func getCredentials(managerId string) (*string, *string, error) {
	var err error

	db, err := sql.Open("mysql", fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s",
		os.Getenv("MYSQL_DATABASE_USER"),
		os.Getenv("MYSQL_DATABASE_PASSWORD"),
		os.Getenv("MYSQL_DATABASE_HOST"),
		os.Getenv("MYSQL_DATABASE_PORT"),
		os.Getenv("MYSQL_DATABASE_DB"),
	))

	defer db.Close()

	if err != nil {
		return nil, nil, err
	}

	var c Credentials
	err = db.QueryRow("SELECT login, password FROM chat_jivosite_manager WHERE manager_id = ?", managerId).Scan(&c.Login, &c.Password)

	if err != nil {
		return nil, nil, err
	}

	return c.Login, c.Password, nil
}

func setStatus(managerId string, status bool) error {
	var err error

	db, err := sql.Open("mysql", fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s",
		os.Getenv("MYSQL_DATABASE_USER"),
		os.Getenv("MYSQL_DATABASE_PASSWORD"),
		os.Getenv("MYSQL_DATABASE_HOST"),
		os.Getenv("MYSQL_DATABASE_PORT"),
		os.Getenv("MYSQL_DATABASE_DB"),
	))

	defer db.Close()

	if err != nil {
		return err
	}

	stmt, err := db.Prepare("UPDATE chat_jivosite_manager set is_online=? where manager_id=?")

	if err != nil {
		return err
	}

	stmt.Exec(status, managerId)

	return nil
}
