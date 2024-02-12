package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/alecthomas/chroma/v2/quick"
	"github.com/google/generative-ai-go/genai"
	"github.com/lrstanley/girc"
	"github.com/pelletier/go-toml/v2"
	"google.golang.org/api/option"
)

type TomlConfig struct {
	IrcServer           string
	IrcPort             int
	IrcNick             string
	IrcSaslUser         string
	IrcSaslPass         string
	IrcChannel          string
	OllamaEndpoint      string
	Temp                float64
	OllamaSystem        string
	RequestTimeout      int
	MillaReconnectDelay int
	EnableSasl          bool
	Model               string
	ChromaStyle         string
	ChromaFormatter     string
	Provider            string
	Apikey              string
	TopP                float32
	TopK                int32
}

type OllamaResponse struct {
	Response string `json:"response"`
}

type OllamaRequestOptions struct {
	Temperature float64 `json:"temperature"`
}

type OllamaRequest struct {
	Model   string               `json:"model"`
	System  string               `json:"system"`
	Prompt  string               `json:"prompt"`
	Stream  bool                 `json:"stream"`
	Format  string               `json:"format"`
	Options OllamaRequestOptions `json:"options"`
}

func printResponse(resp *genai.GenerateContentResponse) string {
	result := ""

	for _, cand := range resp.Candidates {
		if cand.Content != nil {
			for _, part := range cand.Content.Parts {
				result += fmt.Sprintln(part)
				log.Println(part)
			}
		}
	}

	return result
}

func runIRC(appConfig TomlConfig, ircChan chan *girc.Client) {
	irc := girc.New(girc.Config{
		Server:    appConfig.IrcServer,
		Port:      appConfig.IrcPort,
		Nick:      appConfig.IrcNick,
		User:      appConfig.IrcNick,
		Name:      appConfig.IrcNick,
		SSL:       true,
		TLSConfig: &tls.Config{InsecureSkipVerify: true},
	})

	saslUser := appConfig.IrcSaslUser
	saslPass := appConfig.IrcSaslPass

	if appConfig.EnableSasl && saslUser != "" && saslPass != "" {
		irc.Config.SASL = &girc.SASLPlain{
			User: appConfig.IrcSaslUser,
			Pass: appConfig.IrcSaslPass,
		}
	}

	irc.Handlers.AddBg(girc.CONNECTED, func(c *girc.Client, e girc.Event) {
		channels := strings.Split(appConfig.IrcChannel, " ")
		for _, channel := range channels {
			c.Cmd.Join(channel)
		}
	})

	if appConfig.Provider == "ollama" {
		irc.Handlers.AddBg(girc.PRIVMSG, func(client *girc.Client, event girc.Event) {
			if strings.HasPrefix(event.Last(), appConfig.IrcNick+": ") {
				prompt := strings.TrimPrefix(event.Last(), appConfig.IrcNick+": ")
				log.Println(prompt)

				ollamaRequest := OllamaRequest{
					Model:  appConfig.Model,
					System: appConfig.OllamaSystem,
					Prompt: prompt,
					Stream: false,
					Format: "json",
					Options: OllamaRequestOptions{
						Temperature: appConfig.Temp,
					},
				}

				jsonPayload, err := json.Marshal(ollamaRequest)
				if err != nil {
					client.Cmd.ReplyTo(event, girc.Fmt(fmt.Sprintf("error: %s", err.Error())))

					return
				}

				ctx, cancel := context.WithTimeout(context.Background(), time.Duration(appConfig.RequestTimeout)*time.Second)
				defer cancel()

				request, err := http.NewRequest(http.MethodPost, appConfig.OllamaEndpoint, bytes.NewBuffer(jsonPayload))
				request = request.WithContext(ctx)
				if err != nil {
					client.Cmd.ReplyTo(event, girc.Fmt(fmt.Sprintf("error: %s", err.Error())))

					return
				}

				request.Header.Set("Content-Type", "application/json")

				httpClient := http.Client{}

				response, err := httpClient.Do(request)
				if err != nil {
					client.Cmd.ReplyTo(event, girc.Fmt(fmt.Sprintf("error: %s", err.Error())))

					return
				}
				defer response.Body.Close()

				var ollamaResponse OllamaResponse
				err = json.NewDecoder(response.Body).Decode(&ollamaResponse)
				if err != nil {
					client.Cmd.ReplyTo(event, girc.Fmt(fmt.Sprintf("error: %s", err.Error())))

					return
				}

				var writer bytes.Buffer
				err = quick.Highlight(&writer,
					ollamaResponse.Response,
					"markdown",
					appConfig.ChromaFormatter,
					appConfig.ChromaStyle)
				if err != nil {
					client.Cmd.ReplyTo(event, girc.Fmt(fmt.Sprintf("error: %s", err.Error())))

					return
				}

				client.Cmd.ReplyTo(event, girc.Fmt("\033[0m"+writer.String()))
			}
		})
	} else if appConfig.Provider == "gemini" {
		irc.Handlers.AddBg(girc.PRIVMSG, func(client *girc.Client, event girc.Event) {
			if strings.HasPrefix(event.Last(), appConfig.IrcNick+": ") {
				prompt := strings.TrimPrefix(event.Last(), appConfig.IrcNick+": ")
				log.Println(prompt)

				ctx, cancel := context.WithTimeout(context.Background(), time.Duration(appConfig.RequestTimeout)*time.Second)
				defer cancel()

				transport := http.Transport{
					Proxy: http.ProxyFromEnvironment,
				}
				httpClient := http.Client{
					Transport: &transport,
					Timeout:   time.Duration(appConfig.RequestTimeout) * time.Second,
				}

				clientGemini, err := genai.NewClient(ctx, option.WithAPIKey(appConfig.Apikey), option.WithHTTPClient(&httpClient))
				if err != nil {
					client.Cmd.ReplyTo(event, girc.Fmt(fmt.Sprintf("error: %s", err.Error())))

					return
				}
				defer clientGemini.Close()

				model := clientGemini.GenerativeModel(appConfig.Model)
				model.SetTemperature(float32(appConfig.Temp))
				model.SetTopK(appConfig.TopK)
				model.SetTopP(appConfig.TopP)
				resp, err := model.GenerateContent(ctx, genai.Text(prompt))
				if err != nil {
					client.Cmd.ReplyTo(event, girc.Fmt(fmt.Sprintf("error: %s", err.Error())))

					return
				}

				var writer bytes.Buffer
				err = quick.Highlight(
					&writer,
					printResponse(resp),
					"markdown",
					appConfig.ChromaFormatter,
					appConfig.ChromaStyle)
				if err != nil {
					client.Cmd.ReplyTo(event, girc.Fmt(fmt.Sprintf("error: %s", err.Error())))

					return
				}

				fmt.Println(writer.String())
				client.Cmd.ReplyTo(event, girc.Fmt("\033[0m"+writer.String()))
			}
		})
	}

	ircChan <- irc

	for {
		if err := irc.Connect(); err != nil {
			log.Println(err)
			log.Println("reconnecting in 30 seconds")
			time.Sleep(time.Duration(appConfig.MillaReconnectDelay) * time.Second)
		} else {
			return
		}
	}
}

func main() {
	var appConfig TomlConfig

	configPath := flag.String("config", "./config-gemini.toml", "path to the config file")

	flag.Parse()

	data, err := os.ReadFile(*configPath)
	if err != nil {
		log.Fatal(err)
	}

	err = toml.Unmarshal(data, &appConfig)
	if err != nil {
		log.Fatal(err)
	}

	log.Println(appConfig)

	ircChan := make(chan *girc.Client, 1)

	runIRC(appConfig, ircChan)
}
