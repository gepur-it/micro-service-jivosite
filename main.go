package main

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"time"
)

type RmoState struct {
	AvailableForCalls bool `json:"available_for_calls"`
}

type SocketAuthRequestParams struct {
	Name          string `json:"name"`
	UIVersion     string `json:"ui_version"`
	UaVersion     string `json:"ua_version"`
	UaBuildNumber string `json:"ua_build_number"`
	CommitNumber  string `json:"commit_number"`
	Away          bool   `json:"away"`
	AppInstanceID string `json:"app_instance_id"`
	RmoState      RmoState `json:"rmo_state"`
	Features      [3]string `json:"features"`
	AccessToken   string   `json:"access_token"`
}

type SocketAuthRequest struct {
	ID     int    `json:"id"`
	Method string `json:"method"`
	Params SocketAuthRequestParams `json:"params"`
	Jsonrpc string `json:"jsonrpc"`
}

type SocketRegisterRequestParams struct {
	Callback string      `json:"callback"`
	Batch    string      `json:"batch"`
	Sid      interface{} `json:"sid"`
}

type SocketRegisterRequest struct {
	ID     int    `json:"id"`
	Method string `json:"method"`
	Params SocketRegisterRequestParams `json:"params"`
	Jsonrpc string `json:"jsonrpc"`
}

type SuccessLoginResponse struct {
	EndpointList struct {
		Chatserver string `json:"chatserver"`
	} `json:"endpoint_list"`
	AccessToken string `json:"access_token"`
	Ok          bool   `json:"ok"`
}

func main() {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	apiUrl := "https://api.jivosite.com/api/1.0/auth/agent/access"
	fmt.Println("URL:>", apiUrl)

	data := url.Values{}
	data.Set("login", "qwe2@qwe.qwe")
	data.Add("password", "nTj4*6F#9")

	req, err := http.NewRequest("POST", apiUrl, strings.NewReader(data.Encode()))

	req.Header.Set("Host", "api.jivosite.com")
	req.Header.Set("Origin", "https://app.jivosite.com")
	req.Header.Set("Referer", "https://app.jivosite.com")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")

	client := &http.Client{}
	resp, err := client.Do(req)

	if err != nil {
		panic(err)
	}

	defer resp.Body.Close()

	fmt.Println("response Status:", resp.Status)
	fmt.Println("response Headers:", resp.Header)

	responseBody, _ := ioutil.ReadAll(resp.Body)

	successLoginResponse := SuccessLoginResponse{}

	err = json.Unmarshal(responseBody, &successLoginResponse)

	if err != nil {
		log.Fatal("Can`t decode login response")
	}

	if successLoginResponse.Ok == false {
		log.Fatal("Login data incorrect")
	}

	fmt.Println("response Body:", string(responseBody))

	socketUrl := url.URL{Scheme: "wss", Host: successLoginResponse.EndpointList.Chatserver, Path: "/cometan?ua_version=3.1.2&os=Linux%20x86_64&ui_version=1.2.5"}

	chatSocketConnection, _, err := websocket.DefaultDialer.Dial(socketUrl.String(), nil)

	if err != nil {
		log.Fatal("Can`t connect to chat socket:", err)
	}

	defer chatSocketConnection.Close()

	done := make(chan struct{})

	go func() {
		defer close(done)
		for {
			_, message, err := chatSocketConnection.ReadMessage()

			if err != nil {
				log.Println("read:", err)
				return
			}

			log.Printf("recv: %s", message)
		}
	}()

	//****************************************

	socketRegisterRequestParams := SocketRegisterRequestParams{"handle", "batch", nil}
	socketRegisterRequest := SocketRegisterRequest{1, "subscribe", socketRegisterRequestParams, "2.0"}
	b, err := json.Marshal(socketRegisterRequest)
	fmt.Println("Send Body:", string(b), "\n")


	err = chatSocketConnection.WriteMessage(websocket.TextMessage, b)

	if err != nil {
		log.Fatal("Can`t write subscribe request to socket:", err)
	}

	//****************************************

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
		"5c315e08-44ce-f959-58ef-6a0c98f15862",
		rmoState,
		features,
		successLoginResponse.AccessToken,
	}

	socketAuthRequest := SocketAuthRequest{2, "cometan", socketAuthRequestParams, "2.0"}
	b, err = json.Marshal(socketAuthRequest)
	fmt.Println("Send Body:", string(b), "\n")

	err = chatSocketConnection.WriteJSON(socketAuthRequest)

	if err != nil {
		log.Fatal("Can`t write auth request to socket:", err)
	}

	ticker := time.NewTicker(time.Second*10)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case t := <-ticker.C:
			err := chatSocketConnection.WriteMessage(websocket.TextMessage, []byte("."))
			if err != nil {
				log.Println("write:", err, t)
				return
			}
		case <-interrupt:
			log.Println("interrupt")

			// Cleanly close the connection by sending a close message and then
			// waiting (with timeout) for the server to close the connection.
			err := chatSocketConnection.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
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