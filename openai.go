package main

import (
	"bytes"
	"context"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/alecthomas/chroma/v2/quick"
	"github.com/lrstanley/girc"
	openai "github.com/sashabaranov/go-openai"
	"golang.org/x/net/proxy"
)

func DoChatGPTRequest(
	appConfig *TomlConfig,
	gptMemory *[]openai.ChatCompletionMessage,
	prompt, systemPrompt string,
) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(appConfig.RequestTimeout)*time.Second)
	defer cancel()

	var httpClient http.Client

	if appConfig.LLMProxy != "" {
		proxyURL, err := url.Parse(appConfig.IRCProxy)
		if err != nil {
			cancel()

			return "", err
		}

		dialer, err := proxy.FromURL(proxyURL, &net.Dialer{Timeout: time.Duration(appConfig.RequestTimeout) * time.Second})
		if err != nil {
			cancel()

			return "", err
		}

		httpClient = http.Client{
			Transport: &http.Transport{
				Dial: dialer.Dial,
			},
		}
	}

	config := openai.DefaultConfig(appConfig.Apikey)
	config.HTTPClient = &httpClient

	if appConfig.Endpoint != "" {
		config.BaseURL = appConfig.Endpoint
		log.Print(config.BaseURL)
	}

	gptClient := openai.NewClientWithConfig(config)

	*gptMemory = append(*gptMemory, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleSystem,
		Content: systemPrompt,
	})

	*gptMemory = append(*gptMemory, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: prompt,
	})

	resp, err := gptClient.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:    appConfig.Model,
		Messages: *gptMemory,
	})
	if err != nil {
		return "", err
	}

	return resp.Choices[0].Message.Content, nil
}

func ChatGPTRequestProcessor(
	appConfig *TomlConfig,
	client *girc.Client,
	event girc.Event,
	gptMemory *[]openai.ChatCompletionMessage,
	prompt, systemPrompt string,
) string {
	resp, err := DoChatGPTRequest(appConfig, gptMemory, prompt, systemPrompt)
	if err != nil {
		client.Cmd.ReplyTo(event, "error: "+err.Error())

		return ""
	}

	*gptMemory = append(*gptMemory, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleAssistant,
		Content: resp,
	})

	if len(*gptMemory) > appConfig.MemoryLimit {
		*gptMemory = []openai.ChatCompletionMessage{}

		for _, context := range appConfig.Context {
			*gptMemory = append(*gptMemory, openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleAssistant,
				Content: context,
			})
		}
	}

	var writer bytes.Buffer

	err = quick.Highlight(
		&writer,
		resp,
		"markdown",
		appConfig.ChromaFormatter,
		appConfig.ChromaStyle)
	if err != nil {
		client.Cmd.ReplyTo(event, "error: "+err.Error())

		return ""
	}

	return writer.String()
}

func ChatGPTHandler(
	irc *girc.Client,
	appConfig *TomlConfig,
	gptMemory *[]openai.ChatCompletionMessage,
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

		result := ChatGPTRequestProcessor(appConfig, client, event, gptMemory, prompt, appConfig.SystemPrompt)
		if result != "" {
			SendToIRC(client, event, result, appConfig.ChromaFormatter)
		}
	})
}
