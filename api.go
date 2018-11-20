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

type UploadImageEndpoint struct {
	URL        string `json:"url"`
	Date       string `json:"date"`
	Policy     string `json:"policy"`
	Credential string `json:"credential"`
	Algorithm  string `json:"algorithm"`
	Signature  string `json:"signature"`
	Key        string `json:"key"`
	Ok         bool   `json:"ok"`
}

func getUploadImageEndpoint(manager *Manager) (*UploadImageEndpoint, error) {
	var err error
	uploadImageEndpoint := &UploadImageEndpoint{}
	endpointApiUrl := fmt.Sprintf("https://api.jivosite.com/api/1.0/sites/%d/rmo/media/transfer/access/gain?extension=%s&allow_content_type=%d", 839750, "jpg", 1)

	data := url.Values{}

	req, err := http.NewRequest("GET", endpointApiUrl, strings.NewReader(data.Encode()))

	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", manager.SuccessLoginResponse.AccessToken)
	req.Header.Set("Host", "api.jivosite.com")
	req.Header.Set("Origin", "https://app.jivosite.com")
	req.Header.Set("Referer", "https://app.jivosite.com")

	client := &http.Client{}
	resp, err := client.Do(req)

	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	responseBody, _ := ioutil.ReadAll(resp.Body)

	err = json.Unmarshal(responseBody, &uploadImageEndpoint)

	if err != nil {
		return nil, err
	}

	if uploadImageEndpoint.Ok == false {
		err := errors.New("uploadImageEndpoint request failed")
		return nil, err
	}

	return uploadImageEndpoint, nil
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
