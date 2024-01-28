# milla

Milla is an IRC bot that sends things over to ollama when you ask her questions, prints the answer with synta-hilighting.<br/>
![milla](./milla.png)

### Config

config:

```toml
ircServer = "myaswesomeircserver.com"
ircPort = 6697
ircNick = "milla"
ircSaslUser = "milla"
ircSaslPass = "myaswesomepassword"
ircChannel = "#myaswesomechannel"
ollamaEndpoint = "http://172.17.0.1:11434/api/generate"
ollamaTemp = 0.2
ollamaSystem = ""
requestTimeout = 10
millaReconnectDelay = 60
enableSasl = true
model = "llama2-uncensored"
chromaStyle = "rose-pine-moon"
chromaFormatter = "terminal256"
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
