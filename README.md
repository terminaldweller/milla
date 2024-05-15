# milla

Milla is an IRC bot that sends things over to an LLM when you ask it questions and prints the answer with optional syntax-highlighting.<br/>
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

The SASL password for SASL plain authentication. Can also be passed as and environment variable.

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

**_NOTE_**: please note that the terminal formatters will increase the size of the IRC event. Depending on the IRC server, this may or may not be a problem.

#### provider

Which LLM provider to use. The supported options are:

- [ollama](https://github.com/ollama/ollama)
- chatgpt
- gemini

#### apikey

The apikey to use for the LLM provider. Can also be passed as and environment variable.

#### ollamaSystem

The system message to use for ollama.

#### clientCertPath

The path to the client certificate to use for client cert authentication.

#### serverPass

The password to use for the IRC server the bot is trying to connect to if the server has a password. Can also be passed as and environment variable.

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

Disables the "fallback" to a non-TLS connection if the strict transport policy expires and the first attempt to reconnect back to the TLS version fails.

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

### databaseUser

Name of the database user. Can also be passed an an environment variable.

### databasePassword

Password for the database user. Can also be passed an an environment variable.

### databaseAddress

Address of the database. Can also be passed as and environment variable.

### databaseName

Name of the database. Can also be passed as and environment variable.

### ircProxy

Determines which proxy to use to connect to the irc network:

```
ircProxy = "socks5://127.0.0.1:9050"
```

### llmProxy

Determines which proxy to use to connect to the LLM endpoint:

```
llmProxy = "socks5://127.0.0.1:9050"
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

#### memstats

Returns memory stats for milla.

## Environment Variables

- MILLA_SASL_PASSWORD
- MILLA_SERVER_PASSWORD
- MILLA_APIKEY
- MILLA_DB_USER
- MILLA_DB_PASSWORD
- MILLA_DB_ADDRESS
- MILLA_DB_NAME

## Proxy Support

milla will read and use the `ALL_PROXY` environment variable.
It is rather a non-standard way of using the env var but you define your socks5 proxies like so:

```
ALL_PROXY=127.0.0.1:9050
```

**_NOTE_**: the proxy is used for making calls to the LLMs, not for connecting to the IRC server.

## Deploy

### Docker

Images are automatically pushed to dockerhub. So you can get it from [there](https://hub.docker.com/r/terminaldweller/milla).
An example docker compose file is provided in the repo under `docker-compose.yaml`.
milla can be used with [gvisor](https://gvisor.dev/)'s docker runtime, `runsc`.

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
    logging:
      driver: "json-file"
      options:
        max-size: "100m"
    networks:
      - millanet
      user: ${UID}:${GID}
    restart: unless-stopped
    command: ["--config", "/opt/milla/config.toml"]
    volumes:
      - ./config-gpt.toml:/opt/milla/config.toml
      - /etc/localtime:/etc/localtime:ro
      - /etc/resolv.conf:/etc/resolv.conf:ro
    cap_drop:
      - ALL
    runtime: runsc
networks:
  millanet:
    driver: bridge
```

### Public Message Storage

milla can be configured to store all incoming public messages for future use in a postgres database. An example docker compose file is provided under `docker-compose-postgres.yaml`.<br/>

```yaml
services:
  terra:
    image: milla_distroless_vendored
    build:
      context: .
      dockerfile: ./Dockerfile_distroless_vendored
    deploy:
      resources:
        limits:
          memory: 128M
    logging:
      driver: "json-file"
      options:
        max-size: "100m"
    networks:
      - terranet
    user: 1000:1000
    restart: unless-stopped
    entrypoint: ["/usr/bin/milla"]
    command: ["--config", "/config.toml"]
    volumes:
      - ./config-gpt.toml:/config.toml
      - /etc/localtime:/etc/localtime:ro
    cap_drop:
      - ALL
    environment:
      - HTTPS_PROXY=http://172.17.0.1:8120
      - https_proxy=http://172.17.0.1:8120
      - HTTP_PROXY=http://172.17.0.1:8120
      - http_proxy=http://172.17.0.1:8120
  postgres:
    image: postgres:16-alpine3.19
    deploy:
      resources:
        limits:
          memory: 4096M
    logging:
      driver: "json-file"
      options:
        max-size: "200m"
    restart: unless-stopped
    ports:
      - "127.0.0.1:5455:5432/tcp"
    volumes:
      - terra_postgres_vault:/var/lib/postgresql/data
      - ./scripts/:/docker-entrypoint-initdb.d/:ro
    environment:
      - POSTGRES_PASSWORD_FILE=/run/secrets/pg_pass_secret
      - POSTGRES_USER_FILE=/run/secrets/pg_user_secret
      - POSTGRES_INITDB_ARGS_FILE=/run/secrets/pg_initdb_args_secret
      - POSTGRES_DB_FILE=/run/secrets/pg_db_secret
    networks:
      - terranet
      - dbnet
    secrets:
      - pg_pass_secret
      - pg_user_secret
      - pg_initdb_args_secret
      - pg_db_secret
    runtime: runsc
  pgadmin:
    image: dpage/pgadmin4:8.6
    deploy:
      resources:
        limits:
          memory: 1024M
    logging:
      driver: "json-file"
      options:
        max-size: "100m"
    environment:
      - PGADMIN_LISTEN_PORT=${PGADMIN_LISTEN_PORT:-5050}
      - PGADMIN_DEFAULT_EMAIL=${PGADMIN_DEFAULT_EMAIL:-devi@terminaldweller.com}
      - PGADMIN_DEFAULT_PASSWORD_FILE=/run/secrets/pgadmin_pass
      - PGADMIN_DISABLE_POSTFIX=${PGADMIN_DISABLE_POSTFIX:-YES}
    ports:
      - "127.0.0.1:5050:5050/tcp"
    restart: unless-stopped
    volumes:
      - terra_pgadmin_vault:/var/lib/pgadmin
    networks:
      - dbnet
    secrets:
      - pgadmin_pass
networks:
  terranet:
    driver: bridge
  dbnet:
volumes:
  terra_postgres_vault:
  terra_pgadmin_vault:
secrets:
  pg_pass_secret:
    file: ./pg/pg_pass_secret
  pg_user_secret:
    file: ./pg/pg_user_secret
  pg_initdb_args_secret:
    file: ./pg/pg_initdb_args_secret
  pg_db_secret:
    file: ./pg/pg_db_secret
  pgadmin_pass:
    file: ./pgadmin/pgadmin_pass
```

The env vars `UID`and `GID`need to be defined or they can replaces by your host user's uid and gid.<br/>

As a convenience, there is a a [distroless](https://github.com/GoogleContainerTools/distroless) dockerfile, `Dockerfile_distroless` also provided.<br/>
A vendored build of milla is available by first running `go mod vendor` and then using the provided Dockerfile, `Dockerfile_distroless_vendored`.<br/>

### Build

For a regular build:

```sh
go mod download
go build
```

For a vendored build:

```sh
go mod vendor
go build
```

## Thanks

- [girc](https://github.com/lrstanley/girc)
- [chroma](https://github.com/alecthomas/chroma)
- [pgx](https://github.com/jackc/pgx)
- [ollama](https://github.com/ollama/ollama)

## Similar Projects

- [soulshack](https://github.com/pkdindustries/soulshack)
