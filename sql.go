package main

import (
	_ "github.com/go-sql-driver/mysql"
	"time"
)

type Credentials struct {
	Login    *string
	Password *string
}

func getCredentials(id string) (*string, *string, error) {
	var err error
	var c Credentials
	err = MySQL.QueryRow("SELECT login, password FROM chat_jivosite_manager WHERE id = ?", id).Scan(&c.Login, &c.Password)

	if err != nil {
		return nil, nil, err
	}

	return c.Login, c.Password, nil
}

func setStatus(id string, status bool) error {
	var err error

	stmt, err := MySQL.Prepare("UPDATE chat_jivosite_manager set is_online=? where id=?")

	if err != nil {
		return err
	}

	stmt.Exec(status, id)
	stmt.Close()

	return nil
}

func setLastOnline(managerId string) error {
	var err error

	var updatedAt = time.Now()

	stmt, err := MySQL.Prepare("UPDATE chat_jivosite_manager set online_at=? where id=?")

	if err != nil {
		return err
	}

	stmt.Exec(updatedAt, managerId)
	stmt.Close()

	return nil
}

func selOfflineAll() error {
	var err error

	stmt, err := MySQL.Prepare("UPDATE chat_jivosite_manager set is_online=?")

	if err != nil {
		return err
	}

	stmt.Exec(false)
	stmt.Close()

	return nil
}
