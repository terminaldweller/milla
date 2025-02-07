package main

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/alecthomas/chroma/v2/quick"
	"github.com/lrstanley/girc"
	"golang.org/x/net/proxy"
)

func DoORRequest(
	appConfig *TomlConfig,
	memory *[]MemoryElement,
	prompt string,
) (string, error) {
	var jsonPayload []byte

	var err error

	memoryElement := MemoryElement{
		Role:    "user",
		Content: prompt,
	}

	if len(*memory) > appConfig.MemoryLimit {
		*memory = []MemoryElement{}

		for _, context := range appConfig.Context {
			*memory = append(*memory, MemoryElement{
				Role:    "assistant",
				Content: context,
			})
		}
	}

	*memory = append(*memory, memoryElement)

	ollamaRequest := OllamaChatRequest{
		Model:    appConfig.Model,
		System:   appConfig.SystemPrompt,
		Messages: *memory,
	}

	jsonPayload, err = json.Marshal(ollamaRequest)
	if err != nil {

		return "", err
	}

	log.Printf("json payload: %s", string(jsonPayload))

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(appConfig.RequestTimeout)*time.Second)
	defer cancel()

	request, err := http.NewRequest(http.MethodPost, appConfig.Endpoint, bytes.NewBuffer(jsonPayload))
	if err != nil {

		return "", err
	}

	request = request.WithContext(ctx)
	request.Header.Set("content-type", "application/json")
	request.Header.Set("Authorization", "Bearer "+appConfig.Apikey)

	var httpClient http.Client

	var dialer proxy.Dialer

	if appConfig.LLMProxy != "" {
		proxyURL, err := url.Parse(appConfig.IRCProxy)
		if err != nil {
			cancel()

			log.Fatal(err.Error())
		}

		dialer, err = proxy.FromURL(proxyURL, &net.Dialer{Timeout: time.Duration(appConfig.RequestTimeout) * time.Second})
		if err != nil {
			cancel()

			log.Fatal(err.Error())
		}

		httpClient = http.Client{
			Transport: &http.Transport{
				Dial: dialer.Dial,
			},
		}
	}
	response, err := httpClient.Do(request)

	if err != nil {
		return "", err
	}

	defer response.Body.Close()

	log.Println("response body:", response.Body)

	var orresponse ORResponse

	err = json.NewDecoder(response.Body).Decode(&orresponse)
	if err != nil {
		return "", err
	}

	var result string

	for _, choice := range orresponse.Choices {
		result += choice.Message.Content + "\n"
	}

	return result, nil
}

func ORRequestProcessor(
	appConfig *TomlConfig,
	client *girc.Client,
	event girc.Event,
	memory *[]MemoryElement,
	prompt string,
) string {
	response, err := DoORRequest(appConfig, memory, prompt)
	if err != nil {
		client.Cmd.ReplyTo(event, "error: "+err.Error())

		return ""
	}

	assistantElement := MemoryElement{
		Role:    "assistant",
		Content: response,
	}

	*memory = append(*memory, assistantElement)

	log.Println(response)

	var writer bytes.Buffer

	err = quick.Highlight(&writer,
		response,
		"markdown",
		appConfig.ChromaFormatter,
		appConfig.ChromaStyle)
	if err != nil {
		client.Cmd.ReplyTo(event, "error: "+err.Error())

		return ""
	}

	return writer.String()
}

func ORHandler(
	irc *girc.Client,
	appConfig *TomlConfig,
	memory *[]MemoryElement) {
	irc.Handlers.AddBg(girc.PRIVMSG, func(client *girc.Client, event girc.Event) {
		if !strings.HasPrefix(event.Last(), appConfig.IrcNick+": ") {
			return
		}

		if appConfig.AdminOnly {
			byAdmin := false

			for _, admin := range appConfig.Admins {
				if event.Source.Name == admin {
					byAdmin = true
				}
			}

			if !byAdmin {
				return
			}
		}

		prompt := strings.TrimPrefix(event.Last(), appConfig.IrcNick+": ")
		log.Println(prompt)

		if string(prompt[0]) == "/" {
			runCommand(client, event, appConfig)

			return
		}

		result := ORRequestProcessor(appConfig, client, event, memory, prompt)
		if result != "" {
			SendToIRC(client, event, result, appConfig.ChromaFormatter)
		}
	})

}
