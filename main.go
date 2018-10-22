package main

import (
	"database/sql"
	"fmt"
	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
	"github.com/streadway/amqp"
	"os"
	"os/signal"
)

var AMQPConnection *amqp.Connection
var AMQPChannel *amqp.Channel
var logger = logrus.New()
var interrupt = make(chan *Manager)
var done chan struct{}
var MySQL *sql.DB

func failOnError(err error, msg string) {
	if err != nil {
		logger.WithFields(logrus.Fields{
			"error": err,
		}).Fatal(msg)
		panic(fmt.Sprintf("%s: %s", msg, err))
	}
}

func init() {
	logger.WithFields(logrus.Fields{}).Info("Server init:")
	err := godotenv.Load()
	if err != nil {
		panic(fmt.Sprintf("%s: %s", "Error loading .env file", err))
	}

	logger.SetLevel(logrus.DebugLevel)
	logger.SetOutput(os.Stdout)
	logger.SetFormatter(&logrus.TextFormatter{})

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
}

func main() {
	logger.WithFields(logrus.Fields{}).Info("Server start:")
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	done = make(chan struct{})

	server := server()

	go server.start()
	go server.commandQuery()
	server.managerQuery()

	defer MySQL.Close()
	defer AMQPConnection.Close()
	defer AMQPChannel.Close()
}
