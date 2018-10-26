package main

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	"github.com/streadway/amqp"
	"log"
	"net/http"
	"net/url"
	"time"
)

type ResultRequestResult struct {
}

type ResultRequest struct {
	ID     int                 `json:"id"`
	Result ResultRequestResult `json:"result"`
}

type RmoState struct {
	AvailableForCalls bool `json:"available_for_calls"`
}

type SocketAuthRequestParams struct {
	Name          string    `json:"name"`
	UIVersion     string    `json:"ui_version"`
	UaVersion     string    `json:"ua_version"`
	UaBuildNumber string    `json:"ua_build_number"`
	CommitNumber  string    `json:"commit_number"`
	Away          bool      `json:"away"`
	AppInstanceID string    `json:"app_instance_id"`
	RmoState      RmoState  `json:"rmo_state"`
	Features      [3]string `json:"features"`
	AccessToken   string    `json:"access_token"`
}

type SocketAuthRequest struct {
	ID      int                     `json:"id"`
	Method  string                  `json:"method"`
	Params  SocketAuthRequestParams `json:"params"`
	Jsonrpc string                  `json:"jsonrpc"`
}

type SocketRegisterRequestParams struct {
	Callback string      `json:"callback"`
	Batch    string      `json:"batch"`
	Sid      interface{} `json:"sid"`
}

type SocketRegisterRequest struct {
	ID      int                         `json:"id"`
	Method  string                      `json:"method"`
	Params  SocketRegisterRequestParams `json:"params"`
	Jsonrpc string                      `json:"jsonrpc"`
}

type CannedPhrasesParams struct {
	Name       string      `json:"name"`
	GetPhrases int         `json:"get_phrases"`
	Version    interface{} `json:"version"`
}

type CannedPhrases struct {
	ID      int                 `json:"id"`
	Method  string              `json:"method"`
	Params  CannedPhrasesParams `json:"params"`
	Jsonrpc string              `json:"jsonrpc"`
}

type Status struct {
	IsOnline bool `json:"isOnline"`
}

type Manager struct {
	Id                   string `json:"id"`
	SuccessLoginResponse *SuccessLoginResponse
	connection           *websocket.Conn
	requests             int
}

type ManagerStatus struct {
	Manager *Manager `json:"manager"`
	Status  Status   `json:"status"`
}

type ServerMessage struct {
	ID     int    `json:"id"`
	Method string `json:"method"`
	Params struct {
		Name string `json:"name"`
	} `json:"params"`
	Jsonrpc string `json:"jsonrpc"`
}

type ServerMessageLogout struct {
	ID     int    `json:"id"`
	Method string `json:"method"`
	Params struct {
		Name string `json:"name"`
	} `json:"params"`
	Jsonrpc string `json:"jsonrpc"`
}

func (manager *Manager) subscribe() {
	time.Sleep(time.Second * 1)

	socketRegisterRequestParams := SocketRegisterRequestParams{"handle", "batch", nil}
	manager.requests = manager.requests + 1
	socketRegisterRequest := SocketRegisterRequest{manager.requests, "subscribe", socketRegisterRequestParams, "2.0"}

	b, err := json.Marshal(socketRegisterRequest)

	err = manager.connection.WriteJSON(socketRegisterRequest)
	fmt.Println("\nSend Body:", string(b), "\n")

	if err != nil {
		logger.WithFields(logrus.Fields{
			"manager": manager.Id,
			"error":   err,
		}).Error("Can`t write subscribe request to socket:")
	}
}

func (manager *Manager) auth() {
	go func() {
		time.Sleep(time.Second * 2)
		var features [3]string
		features[0] = "inbox"
		features[1] = "multidevices"
		features[2] = "support_admin_login"

		rmoState := RmoState{false}
		socketAuthRequestParams := SocketAuthRequestParams{
			"login",
			"3.1.2",
			"3.1.2",
			"3.1.2",
			"web - 1.2.5 61a1133 Linux x86_64",
			false,
			"1488c95-9e6f-51cc-bb-65fbf84e9b19",
			rmoState,
			features,
			manager.SuccessLoginResponse.AccessToken,
		}

		manager.requests = manager.requests + 1
		socketAuthRequest := SocketAuthRequest{manager.requests, "cometan", socketAuthRequestParams, "2.0"}

		b, err := json.Marshal(socketAuthRequest)

		err = manager.connection.WriteJSON(socketAuthRequest)
		fmt.Println("\nSend Body:", string(b), "\n")

		if err != nil {
			logger.WithFields(logrus.Fields{
				"manager": manager.Id,
				"error":   err,
			}).Error("Can`t write auth request to socket:")
		}

	}()
}

func (manager *Manager) getCannedPhrases() {
	time.Sleep(time.Second * 10)

	cannedPhrasesParams := CannedPhrasesParams{"canned_phrases", 1, nil}

	manager.requests = manager.requests + 1
	cannedPhrases := CannedPhrases{manager.requests, "cometan", cannedPhrasesParams, "2.0"}

	b, err := json.Marshal(cannedPhrases)

	err = manager.connection.WriteJSON(cannedPhrases)
	fmt.Println("\nSend Body:", string(b), "\n")

	if err != nil {
		logger.WithFields(logrus.Fields{
			"manager": manager.Id,
			"error":   err,
		}).Error("Can`t write auth request to socket:")
	}
}

func (manager *Manager) connectToSocket() {
	socketUrl := url.URL{Scheme: "wss", Host: manager.SuccessLoginResponse.EndpointList.Chatserver, Path: "/cometan"}

	header := http.Header{}
	header.Add("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/69.0.3497.100 Safari/537.36")

	chatSocketConnection, _, err := websocket.DefaultDialer.Dial(socketUrl.String(), header)

	if err != nil {
		log.Fatal("Can`t connect to chat socket:", err)
	}

	manager.connection = chatSocketConnection
}

func (manager *Manager) reader(server *Server) {
	quit := make(chan bool)
	for {
		select {
		case <-quit:
			return
		default:
			_, message, err := manager.connection.ReadMessage()

			if err != nil {
				logger.WithFields(logrus.Fields{
					"error": err,
				}).Fatal("Socket reader failed:")
			}

			if string(message) != "." {
				logger.WithFields(logrus.Fields{
					"message": string(message),
				}).Info("New message from server:")

				serverMessage := ServerMessage{}

				err = json.Unmarshal(message, &serverMessage)

				if err != nil {
					logger.WithFields(logrus.Fields{
						"error": err,
					}).Error("Can`t decode response from socket:")
				}

				if serverMessage.Method == "handle" {
					if serverMessage.Params.Name == "login_another_dev" {

						server.offline <- manager

						logger.WithFields(logrus.Fields{
							"message": serverMessage,
						}).Error("Login from another dev:")

						quit <- true

					} else if serverMessage.Params.Name == "login_ok" {
						//@todo not nice now
					} else {
						err = AMQPChannel.Publish(
							"",
							"chat_to_erp_handle_messages",
							false,
							false,
							amqp.Publishing{
								DeliveryMode: amqp.Transient,
								ContentType:  "application/json",
								Body:         message,
								Timestamp:    time.Now(),
							})

						if err != nil {
							logger.WithFields(logrus.Fields{
								"error": err,
							}).Error("Failed to declare a queue:")
						}
					}
				}

				manager.requests = manager.requests + 1
				resultRequest := ResultRequest{serverMessage.ID, ResultRequestResult{}}

				err = manager.connection.WriteJSON(resultRequest)

				if err != nil {
					logger.WithFields(logrus.Fields{
						"error": err,
					}).Error("Can`t write result request to socket:")
				}

				logger.WithFields(logrus.Fields{
					"body": resultRequest,
				}).Info("Send Body:")
			} else {
				logger.WithField("manager", manager.Id).Info("Recv pong:")
			}
		}
	}
	quit <- true
}

func (manager *Manager) ticker() {
	ticker := time.NewTicker(time.Second * 10)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case t := <-ticker.C:
			err := manager.connection.WriteMessage(websocket.TextMessage, []byte("."))
			logger.WithField("manager", manager.Id).Info("Send ping:")

			if err != nil {
				log.Println("write:", err, t)

				return
			}
		case <-interrupt:
			log.Println("interrupt")

			err := manager.connection.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				log.Println("write close:", err)

				return
			}
			select {
			case <-done:
			case <-time.After(time.Second):
			}

			return
		}
	}
}
