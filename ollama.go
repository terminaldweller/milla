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

func DoOllamaRequest(
	appConfig *TomlConfig,
	ollamaMemory *[]MemoryElement,
	prompt, systemPrompt string,
) (string, error) {
	var jsonPayload []byte

	var err error

	memoryElement := MemoryElement{
		Role:    "user",
		Content: prompt,
	}

	if len(*ollamaMemory) > appConfig.MemoryLimit {
		*ollamaMemory = []MemoryElement{}

		for _, context := range appConfig.Context {
			*ollamaMemory = append(*ollamaMemory, MemoryElement{
				Role:    "assistant",
				Content: context,
			})
		}
	}

	*ollamaMemory = append(*ollamaMemory, memoryElement)

	ollamaRequest := OllamaChatRequest{
		Model:     appConfig.Model,
		KeepAlive: time.Duration(appConfig.KeepAlive),
		Stream:    false,
		Think:     appConfig.OllamaThink,
		Messages:  *ollamaMemory,
		System:    systemPrompt,
		Options: OllamaRequestOptions{
			Mirostat:      appConfig.OllamaMirostat,
			MirostatEta:   appConfig.OllamaMirostatEta,
			MirostatTau:   appConfig.OllamaMirostatTau,
			NumCtx:        appConfig.OllamaNumCtx,
			RepeatLastN:   appConfig.OllamaRepeatLastN,
			RepeatPenalty: appConfig.OllamaRepeatPenalty,
			Temperature:   appConfig.Temperature,
			Seed:          appConfig.OllamaSeed,
			NumPredict:    appConfig.OllamaNumPredict,
			TopK:          appConfig.TopK,
			TopP:          appConfig.TopP,
			MinP:          appConfig.OllamaMinP,
		},
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
	request.Header.Set("Content-Type", "application/json")

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

	var ollamaChatResponse OllamaChatMessagesResponse

	err = json.NewDecoder(response.Body).Decode(&ollamaChatResponse)
	if err != nil {
		return "", err
	}

	log.Println("ollama chat response: ", ollamaChatResponse)

	return ollamaChatResponse.Messages.Content, nil
}

func OllamaRequestProcessor(
	appConfig *TomlConfig,
	client *girc.Client,
	event girc.Event,
	ollamaMemory *[]MemoryElement,
	prompt, systemPrompt string,
) string {
	response, err := DoOllamaRequest(appConfig, ollamaMemory, prompt, systemPrompt)
	if err != nil {
		client.Cmd.ReplyTo(event, "error: "+err.Error())

		return ""
	}

	assistantElement := MemoryElement{
		Role:    "assistant",
		Content: response,
	}

	*ollamaMemory = append(*ollamaMemory, assistantElement)

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

func OllamaHandler(
	irc *girc.Client,
	appConfig *TomlConfig,
	ollamaMemory *[]MemoryElement,
) {
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

		result := OllamaRequestProcessor(appConfig, client, event, ollamaMemory, prompt, appConfig.SystemPrompt)
		if result != "" {
			SendToIRC(client, event, result, appConfig.ChromaFormatter)
		}
	})
}
