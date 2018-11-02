package main

import (
	"encoding/json"
	"github.com/sirupsen/logrus"
)

type WhatCommand struct {
	ManagerId string `json:"managerId"`
	Params    struct {
		Name string `json:"name"`
	} `json:"params"`
}

type AcceptCommand struct {
	ID     int    `json:"id"`
	Method string `json:"method"`
	Params struct {
		Name     string `json:"name"`
		ChatID   int    `json:"chat_id"`
		ClientID int    `json:"client_id"`
	} `json:"params"`
	Jsonrpc string `json:"jsonrpc"`
}

type AgentMessageCommand struct {
	ID     int    `json:"id"`
	Method string `json:"method"`
	Params struct {
		Name      string `json:"name"`
		Message   string `json:"message"`
		ChatID    int    `json:"chat_id"`
		ClientID  int    `json:"client_id"`
		IsQuick   int    `json:"is_quick"`
		PrivateID string `json:"private_id"`
	} `json:"params"`
	Jsonrpc string `json:"jsonrpc"`
}

type Server struct {
	managers map[string]*Manager
	online   chan *Manager
	offline  chan *Manager
	command  chan []byte
}

func server() *Server {
	return &Server{
		online:   make(chan *Manager),
		offline:  make(chan *Manager),
		managers: make(map[string]*Manager),
		command:  make(chan []byte),
	}
}

func (server *Server) managerQuery() {
	logger.WithFields(logrus.Fields{}).Info("Server start manager query:")

	msgs, err := AMQPChannel.Consume(
		"erp_chat_manager_status",
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

			managerStatus.Manager.requests = 0
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

	msgs, err := AMQPChannel.Consume(
		"erp_chat_manager_command",
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
			server.command <- d.Body
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
				login, password, err := getCredentials(manager.Id)

				if err != nil {
					logger.WithFields(logrus.Fields{
						"manager": manager.Id,
						"err":     err,
					}).Error("Manager can`t get credentials from MySQL:")

					return
				}

				response, err := getApiKey(login, password)

				if err != nil {
					logger.WithFields(logrus.Fields{
						"manager": manager.Id,
						"err":     err,
					}).Error("Manager can`t register:")

					return
				}

				err = setStatus(manager.Id, true)

				if err != nil {
					logger.WithFields(logrus.Fields{
						"manager": manager.Id,
						"err":     err,
					}).Error("Manager can`t update status:")

					return
				}

				manager.SuccessLoginResponse = response
				server.managers[manager.Id] = manager

				manager.quit = make(chan struct{})
				manager.connectToSocket()
				manager.subscribe()
				manager.auth()

				go manager.ticker()
				go manager.reader(server)

				logger.WithFields(logrus.Fields{
					"manager": manager.Id,
				}).Info("Manager is online:")
			}

		case manager := <-server.offline:

			if _, ok := server.managers[manager.Id]; ok {

				err := setStatus(manager.Id, false)

				if err != nil {
					logger.WithFields(logrus.Fields{
						"manager": manager.Id,
						"err":     err,
					}).Error("Manager can`t update status:")

					return
				}

				logger.WithFields(logrus.Fields{
					"manager": manager.Id,
				}).Info("Manager quit:")

				close(server.managers[manager.Id].quit)
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
			whatCommand := WhatCommand{}

			err := json.Unmarshal(command, &whatCommand)

			if err != nil {
				logger.WithFields(logrus.Fields{
					"manager": whatCommand.ManagerId,
					"command": whatCommand.Params.Name,
					"err":     err,
				}).Error("Server can`t decode command:")
			}

			if _, ok := server.managers[whatCommand.ManagerId]; ok {
				manager := server.managers[whatCommand.ManagerId]
				if whatCommand.Params.Name == "accept" {
					commandToSend := AcceptCommand{}

					err := json.Unmarshal(command, &commandToSend)

					if err != nil {
						logger.WithFields(logrus.Fields{
							"manager": whatCommand.ManagerId,
							"command": whatCommand.Params.Name,
							"err":     err,
						}).Error("Server can`t decode command:")
					}

					manager.requests = manager.requests + 1
					commandToSend.ID = manager.requests
					commandToSend.Method = "cometan"
					commandToSend.Jsonrpc = "2.0"
					manager.connection.WriteJSON(commandToSend)

					logger.WithFields(logrus.Fields{
						"manager": whatCommand.ManagerId,
						"command": whatCommand.Params.Name,
					}).Info("Server send command to socket:")
				}

				if whatCommand.Params.Name == "agent_message" {
					commandToSend := AgentMessageCommand{}
					err := json.Unmarshal(command, &commandToSend)

					if err != nil {
						logger.WithFields(logrus.Fields{
							"manager": whatCommand.ManagerId,
							"command": whatCommand.Params.Name,
							"err":     err,
						}).Error("Server can`t decode command:")
					}

					manager.requests = manager.requests + 1
					commandToSend.ID = manager.requests
					commandToSend.Method = "cometan"
					commandToSend.Jsonrpc = "2.0"
					manager.connection.WriteJSON(commandToSend)

					logger.WithFields(logrus.Fields{
						"manager": whatCommand.ManagerId,
						"command": whatCommand.Params.Name,
					}).Info("Server send command to socket:")
				}

			} else {
				logger.WithFields(logrus.Fields{
					"manager": whatCommand.ManagerId,
					"command": whatCommand.Params.Name,
				}).Warn("Server receive command from offline manager:")
			}
		}
	}
}
