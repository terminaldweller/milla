package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/lrstanley/girc"
	"github.com/pelletier/go-toml/v2"
)

const (
	reconnectTime = 30
)

type handlerWrapper struct {
	irc    *girc.Client
	config TomlConfig
}

type TomlConfig struct {
	IrcServer      string
	IrcPort        int
	IrcNick        string
	IrcSaslUser    string
	IrcSaslPass    string
	IrcChannel     string
	OllamaEndpoint string
	OllamaTemp     float64
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

	if saslUser != "" && saslPass != "" {
		irc.Config.SASL = &girc.SASLPlain{
			User: appConfig.IrcSaslUser,
			Pass: appConfig.IrcSaslPass,
		}
	}

	irc.Handlers.AddBg(girc.CONNECTED, func(c *girc.Client, e girc.Event) {
		c.Cmd.Join(appConfig.IrcChannel)
	})

	irc.Handlers.AddBg(girc.PRIVMSG, func(client *girc.Client, event girc.Event) {
		if strings.HasPrefix(event.Last(), appConfig.IrcNick+": ") {
			prompt := strings.TrimPrefix(event.Last(), appConfig.IrcNick+": ")
			log.Println(prompt)

			ollamaRequest := OllamaRequest{
				Model:  "llama2-uncensored",
				Prompt: prompt,
				Stream: false,
				Format: "json",
				Options: OllamaRequestOptions{
					Temperature: appConfig.OllamaTemp,
				},
			}

			jsonPayload, err := json.Marshal(ollamaRequest)
			if err != nil {
				client.Cmd.ReplyTo(event, girc.Fmt(fmt.Sprintf("error: %s", err.Error())))

				return
			}

			request, err := http.NewRequest("POST", appConfig.OllamaEndpoint, bytes.NewBuffer(jsonPayload))
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

			client.Cmd.ReplyTo(event, girc.Fmt(ollamaResponse.Response))
		}
	})

	ircChan <- irc

	for {
		if err := irc.Connect(); err != nil {
			log.Println(err)
			log.Println("reconnecting in 30 seconds")
			time.Sleep(reconnectTime * time.Second)
		} else {
			return
		}
	}
}

func main() {
	var appConfig TomlConfig

	data, err := os.ReadFile("/opt/milla/config.toml")
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
