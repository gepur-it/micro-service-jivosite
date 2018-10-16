package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
)

type SuccessLoginResponse struct {
	EndpointList struct {
		Chatserver string `json:"chatserver"`
	} `json:"endpoint_list"`
	AccessToken string `json:"access_token"`
	Ok          bool   `json:"ok"`
}

func getApiKey(login *string, pass *string) (*SuccessLoginResponse, error) {
	var err error

	successLoginResponse := SuccessLoginResponse{}

	loginApiUrl := "https://api.jivosite.com/api/1.0/auth/agent/access"
	fmt.Println("URL:>", loginApiUrl)

	data := url.Values{}

	data.Set("login", *login)
	data.Add("password", *pass)

	req, err := http.NewRequest("POST", loginApiUrl, strings.NewReader(data.Encode()))

	if err != nil {
		return nil, err
	}

	req.Header.Set("Host", "api.jivosite.com")
	req.Header.Set("Origin", "https://app.jivosite.com")
	req.Header.Set("Referer", "https://app.jivosite.com")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")

	client := &http.Client{}
	resp, err := client.Do(req)

	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	responseBody, _ := ioutil.ReadAll(resp.Body)

	err = json.Unmarshal(responseBody, &successLoginResponse)

	if err != nil {
		return nil, err
	}

	if successLoginResponse.Ok == false {
		err := errors.New("login request failed")
		return nil, err
	}

	fmt.Println("response Body:", string(responseBody))

	refreshApiUrl := "https://api.jivosite.com/api/1.0/auth/access/refresh"
	fmt.Println("URL:>", refreshApiUrl)

	data = url.Values{}
	data.Set("token", successLoginResponse.AccessToken)

	req, err = http.NewRequest("POST", refreshApiUrl, strings.NewReader(data.Encode()))

	if err != nil {
		return nil, err
	}

	req.Header.Set("Host", "api.jivosite.com")
	req.Header.Set("Origin", "https://app.jivosite.com")
	req.Header.Set("Referer", "https://app.jivosite.com")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")

	client = &http.Client{}
	resp, err = client.Do(req)

	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	responseBody, _ = ioutil.ReadAll(resp.Body)

	successLoginResponse = SuccessLoginResponse{}

	err = json.Unmarshal(responseBody, &successLoginResponse)

	if err != nil {
		return nil, err
	}

	if successLoginResponse.Ok == false {
		err := errors.New("login request failed")
		return nil, err
	}

	return &successLoginResponse, err
}
