package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"github.com/sirupsen/logrus"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime/multipart"
	"net/http"
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
	MimeType string `json:"mime_type"`
	Type     string `json:"type"`
	File     string `json:"file"`
	FileName string `json:"file_name"`
	FileURL  string `json:"file_url"`
	FileSize int    `json:"file_size"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	Thumb    string `json:"thumb"`
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

					uploadImageEndpoint, err := getUploadImageEndpoint(manager)

					if err != nil {
						logger.WithFields(logrus.Fields{
							"manager": whatCommand.ManagerId,
							"command": whatCommand.Params.Name,
							"err":     err,
						}).Error("Server can`t get uploadImageEndpoint:")

						return
					}

					logger.WithFields(logrus.Fields{
						"data": uploadImageEndpoint,
					}).Info("Server get uploadImageEndpoint:")

					index := strings.Index(commandToSend.Params.Image.Src, ",")
					data, err := base64.StdEncoding.DecodeString(commandToSend.Params.Image.Src[index+1:])

					bodyBuf := &bytes.Buffer{}
					bodyWriter := multipart.NewWriter(bodyBuf)

					acl, err := bodyWriter.CreateFormField("acl")
					acl.Write([]byte("public-read"))

					ct, err := bodyWriter.CreateFormField("Content-Type")
					ct.Write([]byte(commandToSend.Params.Image.Type))

					key, err := bodyWriter.CreateFormField("key")
					key.Write([]byte(uploadImageEndpoint.Key))

					cd, err := bodyWriter.CreateFormField("Content-disposition")
					cd.Write([]byte("attachment; filename*=UTF-8''27272_3.jpg"))

					xad, err := bodyWriter.CreateFormField("X-Amz-Date")
					xad.Write([]byte(uploadImageEndpoint.Date))

					policy, err := bodyWriter.CreateFormField("Policy")
					policy.Write([]byte(uploadImageEndpoint.Policy))

					xac, err := bodyWriter.CreateFormField("X-Amz-Credential")
					xac.Write([]byte(uploadImageEndpoint.Credential))

					xaa, err := bodyWriter.CreateFormField("X-Amz-Algorithm")
					xaa.Write([]byte(uploadImageEndpoint.Algorithm))

					xas, err := bodyWriter.CreateFormField("X-Amz-Signature")
					xas.Write([]byte(uploadImageEndpoint.Signature))

					writer, err := bodyWriter.CreateFormFile("file", commandToSend.Params.Image.Name)

					if err != nil {
						logger.WithFields(logrus.Fields{
							"manager": whatCommand.ManagerId,
							"command": whatCommand.Params.Name,
							"err":     err,
						}).Error("Can`t create form file:")

						return
					}

					imageData := bytes.NewReader(data)

					_, err = io.Copy(writer, imageData)

					if err != nil {
						logger.WithFields(logrus.Fields{
							"manager": whatCommand.ManagerId,
							"command": whatCommand.Params.Name,
							"err":     err,
						}).Error("Can`t copy form file:")

						return
					}

					bodyWriter.Close()

					println(bodyBuf)

					req, err := http.NewRequest("POST", uploadImageEndpoint.URL, bodyBuf)

					if err != nil {
						logger.WithFields(logrus.Fields{
							"manager": whatCommand.ManagerId,
							"command": whatCommand.Params.Name,
							"err":     err,
						}).Error("Can`t create send message request:")

						return
					}

					req.Header.Set("Accept", "*/*")
					req.Header.Set("Accept-Encoding", "gzip, deflate, br")
					req.Header.Set("Accept-Language", "en-US,en;q=0.9")
					req.Header.Set("Connection", "keep-alive")
					req.Header.Set("Host", "files.jivosite.com")
					req.Header.Set("Origin", "https://app.jivosite.com")
					req.Header.Set("Referer", "https://app.jivosite.com")
					req.Header.Set("Content-Type", bodyWriter.FormDataContentType())
					req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/69.0.3497.100 Safari/537.36")

					resp, err := http.DefaultClient.Do(req)

					if err != nil {
						logger.WithFields(logrus.Fields{
							"manager": whatCommand.ManagerId,
							"command": whatCommand.Params.Name,
							"err":     err,
						}).Error("Can`t send message request:")

						return
					}

					resp.Body.Close()

					logger.WithFields(logrus.Fields{
						"manager":    whatCommand.ManagerId,
						"command":    whatCommand.Params.Name,
						"StatusCode": resp.StatusCode,
						"Status":     resp.Status,
						"Location":   resp.Header.Get("Location"),
					}).Info("Upload image to S3:")

					image, _, err := image.Decode(bytes.NewReader(data))

					if err != nil {
						logger.WithFields(logrus.Fields{
							"manager": whatCommand.ManagerId,
							"command": whatCommand.Params.Name,
							"err":     err,
						}).Error("Can`t decode image:")

						return
					}

					manager.requests = manager.requests + 1

					agentImageRequestParamsMedia := AgentImageRequestParamsMedia{
						MimeType: commandToSend.Params.Image.Type,
						Type:     "photo",
						File:     resp.Header.Get("Location"),
						FileName: commandToSend.Params.Image.Name,
						FileURL:  resp.Header.Get("Location"),
						FileSize: len(data),
						Width:    image.Bounds().Dx(),
						Height:   image.Bounds().Dy(),
						Thumb:    resp.Header.Get("Location"),
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

					manager.connection.WriteJSON(agentImageRequest)

					logger.WithFields(logrus.Fields{
						"manager": whatCommand.ManagerId,
						"command": whatCommand.Params.Name,
					}).Info("Server send image message to socket:")

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
