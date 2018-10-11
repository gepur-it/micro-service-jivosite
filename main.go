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

type ResultRequestResult struct {
}

type JivoResponse struct {
	ID int `json:"id"`
}

type ResultRequest struct {
	ID     int                 `json:"id"`
	Result ResultRequestResult `json:"result"`
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

type SuccessLoginResponse struct {
	EndpointList struct {
		Chatserver string `json:"chatserver"`
	} `json:"endpoint_list"`
	AccessToken string `json:"access_token"`
	Ok          bool   `json:"ok"`
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

func main() {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	loginApiUrl := "https://api.jivosite.com/api/1.0/auth/agent/access"
	fmt.Println("URL:>", loginApiUrl)

	data := url.Values{}

	data.Set("login", "zogxray@gmail.com")
	data.Add("password", "~%_(c[,y;@NB/6wW")
	//data.Set("login", "qwe2@qwe.qwe")
	//data.Add("password", "nTj4*6F#9")

	req, err := http.NewRequest("POST", loginApiUrl, strings.NewReader(data.Encode()))

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

	refreshApiUrl := "https://api.jivosite.com/api/1.0/auth/access/refresh"
	fmt.Println("URL:>", refreshApiUrl)

	data = url.Values{}
	data.Set("token", successLoginResponse.AccessToken)

	req, err = http.NewRequest("POST", refreshApiUrl, strings.NewReader(data.Encode()))

	req.Header.Set("Host", "api.jivosite.com")
	req.Header.Set("Origin", "https://app.jivosite.com")
	req.Header.Set("Referer", "https://app.jivosite.com")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")

	client = &http.Client{}
	resp, err = client.Do(req)

	if err != nil {
		panic(err)
	}

	defer resp.Body.Close()

	fmt.Println("response Status:", resp.Status)
	fmt.Println("response Headers:", resp.Header)

	responseBody, _ = ioutil.ReadAll(resp.Body)

	successLoginResponse = SuccessLoginResponse{}

	err = json.Unmarshal(responseBody, &successLoginResponse)

	if err != nil {
		log.Fatal("Can`t decode login response")
	}

	if successLoginResponse.Ok == false {
		log.Fatal("Login data incorrect")
	}

	fmt.Println("response Body:", string(responseBody))

	socketUrl := url.URL{Scheme: "wss", Host: successLoginResponse.EndpointList.Chatserver, Path: "/cometan"}

	header := http.Header{}
	header.Add("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/69.0.3497.100 Safari/537.36")

	chatSocketConnection, _, err := websocket.DefaultDialer.Dial(socketUrl.String(), header)

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

			if string(message) != "." {
				jivoResponse := JivoResponse{}

				err = json.Unmarshal(message, &jivoResponse)

				if err != nil {
					log.Fatal("Can`t decode response from socket:", err)
				}

				resultRequest := ResultRequest{jivoResponse.ID, ResultRequestResult{}}

				err = chatSocketConnection.WriteJSON(resultRequest)

				if err != nil {
					log.Fatal("Can`t write auth request to socket:", err)
				}

				b, _ := json.Marshal(resultRequest)

				fmt.Println("\nSend Body:", string(b), "\n")
			}
		}
	}()

	//****************************************

	go func() {
		time.Sleep(time.Second * 1)

		socketRegisterRequestParams := SocketRegisterRequestParams{"handle", "batch", nil}
		socketRegisterRequest := SocketRegisterRequest{1, "subscribe", socketRegisterRequestParams, "2.0"}

		b, err := json.Marshal(socketRegisterRequest)

		err = chatSocketConnection.WriteJSON(socketRegisterRequest)
		fmt.Println("\nSend Body:", string(b), "\n")

		if err != nil {
			log.Fatal("Can`t write subscribe request to socket:", err)
		}
	}()

	//****************************************

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
			successLoginResponse.AccessToken,
		}

		socketAuthRequest := SocketAuthRequest{2, "cometan", socketAuthRequestParams, "2.0"}

		b, err := json.Marshal(socketAuthRequest)

		err = chatSocketConnection.WriteJSON(socketAuthRequest)
		fmt.Println("\nSend Body:", string(b), "\n")

		if err != nil {
			log.Fatal("Can`t write auth request to socket:", err)
		}
	}()

	//****************************************

	go func() {
		time.Sleep(time.Second * 10)

		cannedPhrasesParams := CannedPhrasesParams{"canned_phrases", 1, nil}

		cannedPhrases := CannedPhrases{3, "cometan", cannedPhrasesParams, "2.0"}

		b, err := json.Marshal(cannedPhrases)

		err = chatSocketConnection.WriteJSON(cannedPhrases)
		fmt.Println("\nSend Body:", string(b), "\n")

		if err != nil {
			log.Fatal("Can`t write auth request to socket:", err)
		}
	}()

	//****************************************

	ticker := time.NewTicker(time.Second * 10)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case t := <-ticker.C:
			err := chatSocketConnection.WriteMessage(websocket.TextMessage, []byte("."))
			log.Println("ping: .")
			if err != nil {
				log.Println("write:", err, t)

				return
			}
		case <-interrupt:
			log.Println("interrupt")

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
