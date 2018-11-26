package main

import (
	"database/sql"
	"fmt"
	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
	"github.com/streadway/amqp"
	"github.com/zbindenren/logrus_mail"
	"os"
	"os/signal"
	"strconv"
)

var AMQPConnection *amqp.Connection
var AMQPChannel *amqp.Channel
var logger = logrus.New()
var interrupt = make(chan *Manager)
var MySQL *sql.DB

func failOnError(err error, msg string) {
	if err != nil {
		logger.WithFields(logrus.Fields{
			"error": err,
		}).Fatal(msg)
		panic(fmt.Sprintf("%s: %s", msg, err))
	}
}

func getenvInt(key string) (int, error) {
	s := os.Getenv(key)
	v, err := strconv.Atoi(s)

	if err != nil {
		return 0, err
	}

	return v, nil
}

func init() {
	err := godotenv.Load()

	if err != nil {
		panic(fmt.Sprintf("%s: %s", "Error loading .env file", err))
	}

	port, err := getenvInt("LOGTOEMAIL_SMTP_PORT")

	if err != nil {
		panic(fmt.Sprintf("%s: %s", "Error read smtp port from env", err))
	}

	hook, err := logrus_mail.NewMailAuthHook(
		os.Getenv("LOGTOEMAIL_APP_NAME"),
		os.Getenv("LOGTOEMAIL_SMTP_HOST"),
		port,
		os.Getenv("LOGTOEMAIL_SMTP_FROM"),
		os.Getenv("LOGTOEMAIL_SMTP_TO"),
		os.Getenv("LOGTOEMAIL_SMTP_USERNAME"),
		os.Getenv("LOGTOEMAIL_SMTP_PASSWORD"),
	)

	logger.SetLevel(logrus.DebugLevel)
	logger.SetOutput(os.Stdout)
	logger.SetFormatter(&logrus.TextFormatter{})

	logger.Hooks.Add(hook)

	if err != nil {
		panic(fmt.Sprintf("%s: %s", "Error add hook to send logs to email", err))
	}

	cs := fmt.Sprintf("amqp://%s:%s@%s:%s/%s",
		os.Getenv("RABBITMQ_ERP_LOGIN"),
		os.Getenv("RABBITMQ_ERP_PASS"),
		os.Getenv("RABBITMQ_ERP_HOST"),
		os.Getenv("RABBITMQ_ERP_PORT"),
		os.Getenv("RABBITMQ_ERP_VHOST"))

	connection, err := amqp.Dial(cs)
	failOnError(err, "Failed to connect to RabbitMQ")
	AMQPConnection = connection

	channel, err := AMQPConnection.Channel()

	failOnError(err, "Failed to open a channel")
	AMQPChannel = channel

	failOnError(err, "Failed to declare a queue")

	db, err := sql.Open("mysql", fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s",
		os.Getenv("MYSQL_DATABASE_USER"),
		os.Getenv("MYSQL_DATABASE_PASSWORD"),
		os.Getenv("MYSQL_DATABASE_HOST"),
		os.Getenv("MYSQL_DATABASE_PORT"),
		os.Getenv("MYSQL_DATABASE_DB"),
	))

	MySQL = db

	failOnError(err, "Failed to connect MySQL")

	logger.WithFields(logrus.Fields{}).Info("Server init:")
}

func main() {
	logger.WithFields(logrus.Fields{}).Info("Server starting:")

	err := selOfflineAll()

	if err != nil {
		logger.WithFields(logrus.Fields{
			"err": err,
		}).Error("Managers can`t set offline status:")

		return
	}

	logger.Info("All manager set to offline:")

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	server := server()

	go server.start()
	go server.commandQuery()
	server.managerQuery()

	defer MySQL.Close()
	defer AMQPConnection.Close()
	defer AMQPChannel.Close()
	logger.WithFields(logrus.Fields{}).Info("Server stopped:")
}
