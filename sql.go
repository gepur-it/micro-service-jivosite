package main

import (
	_ "github.com/go-sql-driver/mysql"
)

type Credentials struct {
	Login    *string
	Password *string
}

func getCredentials(managerId string) (*string, *string, error) {
	var err error
	var c Credentials
	err = MySQL.QueryRow("SELECT login, password FROM chat_jivosite_manager WHERE manager_id = ?", managerId).Scan(&c.Login, &c.Password)

	if err != nil {
		return nil, nil, err
	}

	return c.Login, c.Password, nil
}

func setStatus(managerId string, status bool) error {
	var err error

	stmt, err := MySQL.Prepare("UPDATE chat_jivosite_manager set is_online=? where manager_id=?")

	if err != nil {
		return err
	}

	stmt.Exec(status, managerId)

	return nil
}

func selOfflineAll() error {
	var err error

	stmt, err := MySQL.Prepare("UPDATE chat_jivosite_manager set is_online=?")

	if err != nil {
		return err
	}

	stmt.Exec(false)

	return nil
}
