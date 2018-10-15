package main

import (
	"encoding/json"
	"github.com/sirupsen/logrus"
)

type Command struct {
	Manager Manager                `json:"manager"`
	Data    map[string]interface{} `json:"data"`
}

type Server struct {
	managers map[string]*Manager
	online   chan *Manager
	offline  chan *Manager
	command  chan *Command
}

func server() *Server {
	return &Server{
		online:   make(chan *Manager),
		offline:  make(chan *Manager),
		managers: make(map[string]*Manager),
		command:  make(chan *Command),
	}
}

func (server *Server) managerQuery() {
	logger.WithFields(logrus.Fields{}).Info("Server start manager query:")

	query, err := AMQPChannel.QueueDeclare(
		"erp_chat_manager_status",
		true,
		false,
		false,
		false,
		nil,
	)

	failOnError(err, "Failed to declare a queue")

	msgs, err := AMQPChannel.Consume(
		query.Name,
		"",
		true,
		false,
		false,
		false,
		nil,
	)
	failOnError(err, "Failed to register a consumer")

	forever := make(chan bool)

	go func() {
		for d := range msgs {
			managerStatus := &ManagerStatus{}

			err := json.Unmarshal(d.Body, &managerStatus)

			if err != nil {
				logger.WithFields(logrus.Fields{
					"error": err,
				}).Error("Can`t decode manager query callBack:")
			}

			manager := managerStatus.Manager

			if managerStatus.Status.IsOnline == true {
				server.online <- manager
			} else {
				server.offline <- manager
			}
		}
	}()

	<-forever
}

func (server *Server) commandQuery() {
	logger.WithFields(logrus.Fields{}).Info("Server start command query:")

	query, err := AMQPChannel.QueueDeclare(
		"erp_chat_manager_command",
		true,
		false,
		false,
		false,
		nil,
	)

	failOnError(err, "Failed to declare a queue")

	msgs, err := AMQPChannel.Consume(
		query.Name,
		"",
		true,
		false,
		false,
		false,
		nil,
	)
	failOnError(err, "Failed to register a consumer")

	forever := make(chan bool)

	go func() {
		for d := range msgs {
			command := &Command{}

			err := json.Unmarshal(d.Body, &command)

			if err != nil {
				logger.WithFields(logrus.Fields{
					"error": err,
				}).Error("Can`t decode  command callBack:")
			}

			server.command <- command
		}
	}()

	<-forever
}

func (server *Server) start() {
	for {
		select {
		case manager := <-server.online:
			if _, ok := server.managers[manager.Id]; ok {

				logger.WithFields(logrus.Fields{
					"manager": manager.Id,
				}).Warn("Manager already online:")

			} else {
				response, err := getApiKey(manager.Login, manager.Pass)

				if err != nil {
					logger.WithFields(logrus.Fields{
						"manager": manager.Id,
						"err":     err,
					}).Error("Manager can`t register:")

					return
				}

				manager.SuccessLoginResponse = response
				server.managers[manager.Id] = manager

				manager.connectToSocket()
				manager.subscribe()
				manager.auth()

				go manager.ticker()
				go manager.reader()

				logger.WithFields(logrus.Fields{
					"manager": manager.Id,
				}).Info("Manager is online:")
			}

		case manager := <-server.offline:

			if _, ok := server.managers[manager.Id]; ok {

				server.managers[manager.Id].connection.Close()
				delete(server.managers, manager.Id)

				logger.WithFields(logrus.Fields{
					"manager": manager.Id,
				}).Info("Manager is offline:")

			} else {
				logger.WithFields(logrus.Fields{
					"manager": manager.Id,
				}).Warn("Manager already offline:")
			}

		case command := <-server.command:
			if _, ok := server.managers[command.Manager.Id]; ok {

				logger.WithFields(logrus.Fields{
					"manager": command.Manager.Id,
					"command": command,
				}).Info("Server receive command:")

			} else {
				logger.WithFields(logrus.Fields{
					"manager": command.Manager.Id,
					"command": command,
				}).Warn("Server receive command from offline manager:")
			}
		}
	}
}
