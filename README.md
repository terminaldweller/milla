# milla

Milla is an IRC bot that sends things over to an LLM when you ask it questions and prints the answer with optional syntax-hilighting.<br/>
Currently Supported:

- Ollama
- Openai
- Gemini

![milla](./milla.png)

milla accepts one cli arg which tells it where to look for the config file:<br/>

```$ milla -help
Usage of ./milla:
  -config string
          path to the config file (default "./config.toml")
```

## Config

An exhaustive example is in `config-example.toml`.

#### ircServer

The address for the IRC server to connect to.

#### ircNick

The nick the bot should use.

#### enableSasl

Whether to use SASL for authentication.

#### ircSaslUser

The SASL username.

#### ircSaslPass

The SASL password for SASL plain authentication.

#### ollamaEndpoint

The address for the Ollama chat endpoint.

#### model

The name of the model to use.

#### chromaStyle

The style to use for syntax highlighting done by [chroma](https://github.com/alecthomas/chroma). This is basically what's called a "theme".

#### chromaFormatter

The formatter to use. This tells chroma how to generate the color in the output. The supported options are:

- `noop` for no syntax highlighting
- `terminal` for 8-color terminals
- `terminal8` for 8-color terminals
- `terminal16` for 16-color terminals
- `terminal256` for 256-color terminals
- `terminal16m` for treucolor terminals
- `html` for HTML output

#### provider

Which LLM provider to use. The supported options are:

- [ollama](https://github.com/ollama/ollama)
- chatgpt
- gemini

#### apikey

The apikey to use for the LLM provider.

#### ollamaSystem

The system message to use for ollama.

#### clientCertPath

The path to the client certificate to use for SASL external authentication.

#### serverPass

The password to use for the IRC server the bot is trying to connect to if the server has a password.

#### bind

Which address to bind to for the IRC server.

#### temp

The temperature to config the model with.

#### requestTimeout

The timeout for requests made to the LLM provider.

#### millaReconnectDelay

How much to wait before reconnecting to the IRC server.

#### ircPort

Which port to connect to for the IRC server.

#### keepAlive

#### memoryLimit

How many conversations to keep in memory for a model.

#### pingDelay

Ping delay for the IRC server.

#### pingTimeout

Ping timeout for the IRC server.

#### topP

#### topK

#### skipTLSVerify

Skip verifying the IRC server's TLS certificate. This only makes sense if you are trying to connect to an IRC server with a self-signed certificate.

#### useTLS

Whether to use TLS to connect to the IRC server. This option is provided to support usage on overlay networks such as Tor, i2p and [yggdrassil](https://github.com/yggdrasil-network/yggdrasil-go).

#### disableSTSFallback

#### allowFlood

Disable [girc](https://github.com/lrstanley/girc)'s built-in flood protection.

#### debug

Whether to enable debug logging. The logs are written to stdout.

#### out

Whether to write raw messages to stdout.

#### admins

List of adimns for the bot. Only admins can use commands.

```
admins = ["admin1", "admin2"]
```

#### ircChannels

List of channels for the bot to join when it connects to the server.

```
ircChannels = ["#channel1", "#channel2"]
```

## Commands

#### help

Prints the help message.

#### get

Get the value of a config option. Use the same name as the config file but capitalized.

#### getall

Get the value of all config options.

#### set

Set a config option on the fly. Use the same name as the config file but capitalized.

## Deploy

An example docker compose file is provided in the repo under `docker-compose.yaml`.

```yaml
services:
  milla:
    image: milla
    build:
      context: .
    deploy:
      resources:
        limits:
          memory: 64M
    user: ${UID}:${GID}
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
      - /etc/ssl/certs:/etc/ssl/certs:ro
    cap_drop:
      - ALL
    environment:
      - SERVER_DEPLOYMENT_TYPE=deployment
networks:
  millanet:
```

## Thanks

- [girc](https://github.com/lrstanley/girc)
- [chroma](https://github.com/alecthomas/chroma)
- [ollama](https://github.com/ollama/ollama)

## Similar Projects

- [soulshack](https://github.com/pkdindustries/soulshack)
