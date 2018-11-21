package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"github.com/sirupsen/logrus"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"path/filepath"
	"strings"
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

type AgentImageRequestParamsMedia struct {
	MimeType string  `json:"mime_type"`
	Type     string  `json:"type"`
	File     *string `json:"file"`
	FileName string  `json:"file_name"`
	FileURL  *string `json:"file_url"`
	FileSize int     `json:"file_size"`
	Width    int     `json:"width"`
	Height   int     `json:"height"`
	Thumb    *string `json:"thumb"`
}

type AgentImageRequestParams struct {
	Name      string                       `json:"name"`
	Message   string                       `json:"message"`
	ChatID    int                          `json:"chat_id"`
	ClientID  int                          `json:"client_id"`
	IsQuick   int                          `json:"is_quick"`
	PrivateID string                       `json:"private_id"`
	Media     AgentImageRequestParamsMedia `json:"media"`
}

type AgentImageRequest struct {
	ID      int                     `json:"id"`
	Method  string                  `json:"method"`
	Params  AgentImageRequestParams `json:"params"`
	Jsonrpc string                  `json:"jsonrpc"`
}

type AgentImageCommand struct {
	ManagerID string `json:"managerId"`
	Params    struct {
		Name      string `json:"name"`
		Message   string `json:"message"`
		ChatID    int    `json:"chat_id"`
		ClientID  int    `json:"client_id"`
		IsQuick   int    `json:"is_quick"`
		PrivateID string `json:"private_id"`
		Image     struct {
			Name string `json:"name"`
			Src  string `json:"src"`
			Type string `json:"type"`
		} `json:"image"`
	} `json:"params"`
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
		false,
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

			d.Ack(false)
		}
	}()

	<-forever
}

func (server *Server) commandQuery() {
	logger.WithFields(logrus.Fields{}).Info("Server start command query:")

	msgs, err := AMQPChannel.Consume(
		"erp_chat_manager_command",
		"",
		false,
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
			d.Ack(false)
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

				manager.SuccessLoginResponse = response

				response, err = refreshApiKey(manager)

				if err != nil {
					logger.WithFields(logrus.Fields{
						"manager": manager.Id,
						"err":     err,
					}).Error("Manager can`t refresh token:")

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

			logger.WithFields(logrus.Fields{
				"manager": whatCommand.ManagerId,
				"command": whatCommand.Params.Name,
			}).Info("Server start work with command:")

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

				if whatCommand.Params.Name == "agent_image" {
					commandToSend := AgentImageCommand{}
					err := json.Unmarshal(command, &commandToSend)

					extension := strings.TrimLeft(filepath.Ext(commandToSend.Params.Image.Name), ".")

					response, err := refreshApiKey(manager)

					if err != nil {
						logger.WithFields(logrus.Fields{
							"manager": manager.Id,
							"err":     err,
						}).Error("Manager can`t refresh token:")

						return
					}

					manager.SuccessLoginResponse = response

					uploadImageEndpoint, err := getUploadImageEndpoint(manager, extension)

					if err != nil {
						logger.WithFields(logrus.Fields{
							"manager": whatCommand.ManagerId,
							"command": whatCommand.Params.Name,
							"err":     err,
						}).Error("Server can`t get uploadImageEndpoint:")
					}

					if uploadImageEndpoint != nil {
						logger.WithFields(logrus.Fields{
							"data": uploadImageEndpoint,
						}).Info("Server get uploadImageEndpoint:")

						index := strings.Index(commandToSend.Params.Image.Src, ",")
						data, err := base64.StdEncoding.DecodeString(commandToSend.Params.Image.Src[index+1:])

						if err != nil {
							logger.WithFields(logrus.Fields{
								"manager": whatCommand.ManagerId,
								"command": whatCommand.Params.Name,
								"err":     err,
							}).Error("Can`t decode base64 file:")
						}

						location, err := uploadImageToEndpoint(commandToSend, uploadImageEndpoint, data)

						if err != nil {
							logger.WithFields(logrus.Fields{
								"manager": whatCommand.ManagerId,
								"command": whatCommand.Params.Name,
								"err":     err,
							}).Error("Can`t get file location:")
						}

						logger.WithFields(logrus.Fields{
							"location": location,
						}).Info("Upload file to S3:")

						manager.requests = manager.requests + 1

						var fileType = "document"

						if commandToSend.Params.Image.Type == "image/jpeg" || commandToSend.Params.Image.Type == "image/png" {
							fileType = "photo"
						}

						agentImageRequestParamsMedia := AgentImageRequestParamsMedia{
							MimeType: commandToSend.Params.Image.Type,
							Type:     fileType,
							File:     location,
							FileName: commandToSend.Params.Image.Name,
							FileURL:  location,
							FileSize: len(data),
						}

						if commandToSend.Params.Image.Type == "image/jpeg" || commandToSend.Params.Image.Type == "image/png" {
							img, _, err := image.Decode(bytes.NewReader(data))

							if err != nil {
								logger.WithFields(logrus.Fields{
									"manager": whatCommand.ManagerId,
									"command": whatCommand.Params.Name,
									"err":     err,
								}).Error("Can`t decode image:")
							}

							agentImageRequestParamsMedia.Width = img.Bounds().Dx()
							agentImageRequestParamsMedia.Height = img.Bounds().Dy()
							agentImageRequestParamsMedia.Thumb = location
						} else {
							agentImageRequestParamsMedia.Thumb = nil
						}

						agentImageRequestParams := AgentImageRequestParams{
							Name:      "agent_message",
							Message:   commandToSend.Params.Message,
							ChatID:    commandToSend.Params.ChatID,
							ClientID:  commandToSend.Params.ClientID,
							IsQuick:   commandToSend.Params.IsQuick,
							PrivateID: commandToSend.Params.PrivateID,
							Media:     agentImageRequestParamsMedia,
						}

						agentImageRequest := AgentImageRequest{
							ID:      manager.requests,
							Params:  agentImageRequestParams,
							Method:  "cometan",
							Jsonrpc: "2.0",
						}

						message, err := json.Marshal(agentImageRequest)

						err = publishToErp(message)

						if err != nil {
							logger.WithFields(logrus.Fields{
								"error":   err,
								"message": string(message),
							}).Error("Failed to publish:")
						}

						manager.connection.WriteJSON(agentImageRequest)

						logger.WithFields(logrus.Fields{
							"manager": whatCommand.ManagerId,
							"command": whatCommand.Params.Name,
						}).Info("Server send image message to socket:")
					}
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
