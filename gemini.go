package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/alecthomas/chroma/v2/quick"
	"github.com/lrstanley/girc"
	"google.golang.org/genai"
)

func (t *ProxyRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()

	if t.ProxyURL != "" {
		proxyURL, err := url.Parse(t.ProxyURL)
		if err != nil {
			return nil, err
		}

		transport.Proxy = http.ProxyURL(proxyURL)
	}

	newReq := req.Clone(req.Context())
	vals := newReq.URL.Query()
	vals.Set("key", t.APIKey)
	newReq.URL.RawQuery = vals.Encode()

	resp, err := transport.RoundTrip(newReq)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func DoGeminiRequest(
	appConfig *TomlConfig,
	geminiMemory *[]*genai.Content,
	prompt, systemPrompt string,
) (string, error) {
	httpProxyClient := &http.Client{Transport: &ProxyRoundTripper{
		APIKey:   appConfig.Apikey,
		ProxyURL: appConfig.LLMProxy,
	}}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(appConfig.RequestTimeout)*time.Second)
	defer cancel()

	clientGemini, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:     appConfig.Apikey,
		HTTPClient: httpProxyClient,
	})
	if err != nil {
		return "", fmt.Errorf("Could not create a genai client.", err)
	}

	*geminiMemory = append(*geminiMemory, genai.NewContentFromText(prompt, "user"))

	temperature := float32(appConfig.Temperature)
	topk := float32(appConfig.TopK)

	result, err := clientGemini.Models.GenerateContent(ctx, appConfig.Model, *geminiMemory, &genai.GenerateContentConfig{
		Temperature:       &temperature,
		SystemInstruction: genai.NewContentFromText(systemPrompt, "system"),
		TopK:              &topk,
		TopP:              &appConfig.TopP,
		// SafetySettings: []*genai.SafetySetting{
		// 	{
		// 		Category:  genai.HarmCategoryDangerousContent,
		// 		Threshold: genai.HarmBlockThresholdBlockNone,
		// 	},
		// 	{
		// 		Category:  genai.HarmCategoryHarassment,
		// 		Threshold: genai.HarmBlockThresholdBlockNone,
		// 	},
		// 	{
		// 		Category:  genai.HarmCategoryHateSpeech,
		// 		Threshold: genai.HarmBlockThresholdBlockNone,
		// 	},
		// 	{
		// 		Category:  genai.HarmCategorySexuallyExplicit,
		// 		Threshold: genai.HarmBlockThresholdBlockNone,
		// 	},
		// 	{
		// 		Category:  genai.HarmCategoryCivicIntegrity,
		// 		Threshold: genai.HarmBlockThresholdBlockNone,
		// 	},
		// 	{
		// 		Category:  genai.HarmCategoryUnspecified,
		// 		Threshold: genai.HarmBlockThresholdBlockNone,
		// 	},
		// },
	})
	if err != nil {
		return "", fmt.Errorf("Gemini: Could not generate content", err)
	}

	return result.Text(), nil
}

func GeminiRequestProcessor(
	appConfig *TomlConfig,
	client *girc.Client,
	event girc.Event,
	geminiMemory *[]*genai.Content,
	prompt, systemPrompt string,
) string {
	geminiResponse, err := DoGeminiRequest(appConfig, geminiMemory, prompt, systemPrompt)
	if err != nil {
		client.Cmd.ReplyTo(event, "error: "+err.Error())

		return ""
	}

	log.Println(geminiResponse)

	if len(*geminiMemory) > appConfig.MemoryLimit {
		*geminiMemory = []*genai.Content{}

		for _, context := range appConfig.Context {
			*geminiMemory = append(*geminiMemory, genai.NewContentFromText(context, "model"))
		}
	}

	*geminiMemory = append(*geminiMemory, genai.NewContentFromText(prompt, "user"))
	*geminiMemory = append(*geminiMemory, genai.NewContentFromText(geminiResponse, "model"))

	var writer bytes.Buffer

	err = quick.Highlight(
		&writer,
		geminiResponse,
		"markdown",
		appConfig.ChromaFormatter,
		appConfig.ChromaStyle)
	if err != nil {
		client.Cmd.ReplyTo(event, "error: "+err.Error())

		return ""
	}

	return writer.String()
}

func GeminiHandler(
	irc *girc.Client,
	appConfig *TomlConfig,
	geminiMemory *[]*genai.Content,
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

		result := GeminiRequestProcessor(appConfig, client, event, geminiMemory, prompt, appConfig.SystemPrompt)

		if result != "" {
			SendToIRC(client, event, result, appConfig.ChromaFormatter)
		}
	})
}
