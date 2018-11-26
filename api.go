package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
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

func uploadImageToEndpoint(agentImageCommand AgentImageCommand, uploadImageEndpoint *UploadImageEndpoint, data []byte) (*string, error) {
	var err error

	bodyBuf := &bytes.Buffer{}
	bodyWriter := multipart.NewWriter(bodyBuf)

	acl, err := bodyWriter.CreateFormField("acl")
	acl.Write([]byte("public-read"))

	ct, err := bodyWriter.CreateFormField("Content-Type")
	ct.Write([]byte(agentImageCommand.Params.Image.Type))

	key, err := bodyWriter.CreateFormField("key")
	key.Write([]byte(uploadImageEndpoint.Key))

	cd, err := bodyWriter.CreateFormField("Content-disposition")
	cd.Write([]byte(fmt.Sprintf("attachment; filename*=UTF-8''%s", agentImageCommand.Params.Image.Name)))

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

	writer, err := bodyWriter.CreateFormFile("file", agentImageCommand.Params.Image.Name)

	if err != nil {
		return nil, err
	}

	imageData := bytes.NewReader(data)

	_, err = io.Copy(writer, imageData)

	if err != nil {
		return nil, err
	}

	bodyWriter.Close()

	req, err := http.NewRequest("POST", uploadImageEndpoint.URL, bodyBuf)

	if err != nil {
		return nil, err
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
		return nil, err
	}

	resp.Body.Close()

	var location = resp.Header.Get("Location")

	return &location, nil
}

func getUploadImageEndpoint(manager *Manager, ext string) (*UploadImageEndpoint, error) {
	var err error

	endpointApiUrl := fmt.Sprintf("https://api.jivosite.com/api/1.0/sites/%d/rmo/media/transfer/access/gain?extension=%s&allow_content_type=%d", 839750, ext, 1)
	fmt.Println(endpointApiUrl)

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

	uploadImageEndpoint := &UploadImageEndpoint{}
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

	successLoginResponse := SuccessLoginResponse{}
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

func refreshApiKey(manager *Manager) (*SuccessLoginResponse, error) {
	var err error

	refreshApiUrl := "https://api.jivosite.com/api/1.0/auth/access/refresh"
	fmt.Println("URL:>", refreshApiUrl)

	data := url.Values{}
	data.Set("token", manager.SuccessLoginResponse.AccessToken)

	req, err := http.NewRequest("POST", refreshApiUrl, strings.NewReader(data.Encode()))

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

	successLoginResponse := SuccessLoginResponse{}

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
