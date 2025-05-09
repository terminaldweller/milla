package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

const (
	UserAgentsURL = "https://useragents:443/api/v1/agent"
)

func UserAgentsGet(uaActionName, query string, appConfig *TomlConfig) string {
	var userAgentRequest UserAgentRequest

	var userAgentResponse UserAgentResponse

	_, ok := appConfig.UserAgentActions[uaActionName]
	if !ok {
		log.Println("UserAgentAction not found:", uaActionName)

		return fmt.Sprintf("useragents: %s: action not found", uaActionName)
	}

	userAgentRequest.Agent_Name = appConfig.UserAgentActions[uaActionName].Agent_Name
	userAgentRequest.Instructions = appConfig.UserAgentActions[uaActionName].Instructions
	userAgentRequest.Query = appConfig.UserAgentActions[uaActionName].Query

	if query != "" {
		userAgentRequest.Query = query
	}

	log.Println("UserAgentRequest:", appConfig.UserAgentActions[uaActionName])
	log.Println(userAgentRequest)

	jsonData, err := json.Marshal(userAgentRequest)
	if err != nil {
		log.Println(err)

		return fmt.Sprintf("useragents: %s: could not marshall json request", userAgentRequest.Agent_Name)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(appConfig.RequestTimeout)*time.Second)
	defer cancel()

	req, err := http.NewRequest(
		http.MethodPost,
		UserAgentsURL,
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		log.Println(err)

		return fmt.Sprintf("useragents: %s: could not create request", userAgentRequest.Agent_Name)
	}

	req = req.WithContext(ctx)

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	response, err := client.Do(req)
	if err != nil {
		log.Println(err)

		return fmt.Sprintf("useragents: %s: could not send request", userAgentRequest.Agent_Name)
	}

	defer response.Body.Close()

	if err != nil {
		log.Println(err)

		return fmt.Sprintf("useragents: %s: could not read response", userAgentRequest.Agent_Name)
	}

	err = json.NewDecoder(response.Body).Decode(&userAgentResponse)
	if err != nil {
		log.Println(err)

		return fmt.Sprintf("useragents: %s: could not unmarshall json response", userAgentRequest.Agent_Name)
	}

	return userAgentResponse.Response
}
