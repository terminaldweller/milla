# milla

Milla is an IRC bot that sends things over to an LLM when you ask it questions, prints the answer with syntax-hilighting.<br/>
Currently Supported Models:

- Ollama
- Openai
- Gemini

![milla](./milla.png)

### Config

config:

```toml
ircServer = "irc.terminaldweller.com"
ircPort = 6697
ircNick = "mybot"
ircSaslUser = "mybot"
ircSaslPass = "mypass"
ircChannels = ["#mychannel1", "#mychannel2"]
ollamaEndpoint = ""
temp = 0.2
ollamaSystem = ""
requestTimeout = 10
millaReconnectDelay = 60
enableSasl = true
model = "llama2-uncensored"
chromaStyle = "rose-pine-moon"
chromaFormatter = "terminal256"
provider = "ollama" # ollama, chatgpt, gemini 
apikey = "key"
topP = 0.9
topK = 20
```

### Deploy

You can use the provided compose file:<br/>

```yaml
version: "3.9"
services:
  milla:
    image: milla
    build:
      context: .
    deploy:
      resources:
        limits:
          memory: 64M
    logging:
      driver: "json-file"
      options:
        max-size: "100m"
    networks:
      - millanet
    restart: unless-stopped
    command: ["--config", "/opt/milla/config.toml"]
    volumes:
      - ./config.toml:/opt/milla/config.toml
    cap_drop:
      - ALL
    dns:
      - 9.9.9.9
    environment:
      - SERVER_DEPLOYMENT_TYPE=deployment
    entrypoint: ["/milla/milla"]
networks:
  millanet:
```
