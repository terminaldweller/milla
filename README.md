# milla

Milla is an IRC bot that:

- sends things over to an LLM when you ask it questions and prints the answer with optional syntax-highlighting.Currently supported providers: Ollama, Openai, Gemini <br/>
- Milla can run more than one instance of itself
- Each instance can connect to a different ircd, and will get the full set of configs, e.g. different proxies, different postgres instance, ...
- You can define custom commands in the form of SQL queries to the database with the SQL query result being passed to the bot along with the given prompt and an optional limit so you don't go bankrupt(unless you are running ollama locally like the smart cookie that you are).<br/>
- lua plugin system to extend the bot's functionality.<br/>

![milla](./milla.png)

milla accepts one cli arg which tells it where to look for the config file:<br/>

```$ milla -help
Usage of ./milla:
  -config string
          path to the config file (default "./config.toml")
```

The bot will respond to chat prompts if they begin with `botnick:`.<br/>
The bot will see a chat prompt as a command if the message begins with `botnick: /`.<br/>

## Config

An example is provided under `config-example.toml`. Please note that all the config options are specific to one instance which is defined by `ircd.nameofyourinstance`.<br/>

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
- `terminal16m` for truecolor terminals
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

List of admins for the bot. Only admins can use commands.

```
admins = ["admin1", "admin2"]
```

#### ircChannels

List of channels for the bot to join when it connects to the server.

```
ircChannels = ["#channel1", "#channel2"]
```

Please note that the bot does not have to join a channel to be usable. One can simply query the bot directly as well.<br/>

#### databaseUser

Name of the database user.

#### databasePassword

Password for the database user.

#### databaseAddress

Address of the database.

#### databaseName

Name of the database.

#### scrapeChannels

List of channels that the bot will scrape into a database table. You can later on use these databases for the custom commands.<br/>

#### ircProxy

Determines which proxy to use to connect to the IRC network:

```
ircProxy = "socks5://127.0.0.1:9050"
```

#### llmProxy

Determines which proxy to use to connect to the LLM endpoint:

```
llmProxy = "socks5://127.0.0.1:9050"
```

#### ircdName

Name of the milla instance, must be unique across all instances.

#### adminOnly

Milla will only answer if the nick is in the admin list.

#### webIRCGateway

webirc gateway to use.

#### webIRCHostname

webirc hostname to use.

#### webIRCPassword

webirc password to use.

#### webIRCAddress

webirc address to use.

#### rssFile

The file that contains the rss feeeds.

#### channel

The channel to send the rss feeds to.

### plugins

A list of plugins to load:`plugins = ["./plugins/rss.lua", "./plugins/test.lua"]`

## Custom Commands

Custom commands let you define a command that does a SQL query to the database and performs the given task. Here's an example:

```toml
[ircd.devinet_terra.customCommands.digest]
sql = "select log from liberanet_milla_us_market_news order by log desc;"
limit = 300
context = ["you are a sentiment-analysis bot"]
prompt= "i have provided to you news headlines in the form of previous conversations between you and me using the user role. please provide the digest of the news for me."
[ircd.devinet_terra.customCommands.summarize]
sql= "select log from liberanet_milla_us_market_news order by log desc;"
limit= 300
context = ["you are a sentiment-analysis bot"]
prompt= "i have provided to you news headlines in the form of previous conversations between you and me using the user role. please summarize the provided news for me. provide some details."
[ircd.devinet_terra.customCommands.canada]
sql= "select log from liberanet_milla_us_market_news order by log desc;"
limit= 300
context = ["you are a canadian news anchor", "you only care about news that is relevant to canada"]
prompt= "i have provided to you news headlines in the form of previous conversations between you and me using the user role. please summarize the provided news for me. provide some details."
```

In the above example digest and summarize will be the names of the commands: `milla: /cmd summarize`.<br/>
Currently you should only ask for the log column in the query. Asking for the other column will result in the query not succeeding.<br/>
The `limit` parameter limits the number of SQL queries that are used to generate the response. Whether you hit the token limit of the provider you use and the cost is something you should be aware of.<br/>
A `limit` value of 0 disables the limit on the amount of rows that are passed to milla.<br/>
NOTE: since each milla instance can have its own database, all instances might not necessarily have access to all the data milla is gathering. If you use the same database for all the instances, all instances will have access to all the gathered data.<br/>

## Watchlist

Watchlists allow you to specify a list of channels to watch. The watched values are given in a list of files, each line of the file specifying a value to watch for. Finally a value is given for the alertchannel where the bot will mirror the message that triggered a match.<br/>

```toml
[ircd.devinet_terra.watchlist.security]
watchList = ["#securityfeeds"]
watchFiles = ["/watchfiles/voidbox.list"]
alertChannel = "#milla_alerts"
eventTypes = ["PRIVMSG"]
fgColor = 0
bgColor = 28
```

## RSS

The rss file is self-explanatory. Here's an example:

```json
{
  "feeds": [
    {
      "name": "one",
      "url": "http://feeds.feedburner.com/crunchyroll/rss",
      "proxy": "socks5://172.17.0.1:9007",
      "userAgent": "Mozilla/5.0 (X11; U; Linux i686; pl; rv:1.8.1.1) Gecko/20061204 Firefox/2.0.0.1 (Ubuntu-edgy)",
      "timeout": 10
    },
    {
      "name": "two",
      "url": "http://feeds.feedburner.com/crunchyroll/rss/anime",
      "proxy": "socks5://172.17.0.1:9007",
      "userAgent": "Mozilla/5.0 (X11; U; Linux i686; pl; rv:1.8.1.1) Gecko/20061204 Firefox/2.0.0.1 (Ubuntu-edgy)",
      "timeout": 10
    }
  ],
  "period": 3600
}
```

### Example Config File

```toml
[ircd.devinet]
ircServer = "irc.myawesomeircnet.com"
ircPort = 6697
ircNick = "milla"
enableSasl = true
ircSaslUser = "milla"
ircSaslPass = "xxxxx"
ircChannels = ["##chan1", "##chan2"]
temp = 0.2
requestTimeout = 10
millaReconnectDelay = 60
model = "gpt-3.5-turbo"
chromaStyle = "rose-pine-moon"
chromaFormatter = "terminal256"
provider = "chatgpt"
apikey = "xxxx"
memoryLimit = 20
admins = ["noone_has_this_nick"]
debug = true
out = true
databaseAddress = "postgres:5432"
databasePassword = "changeme"
databaseUser = "devi"
databaseName = "milla"
scrapeChannels = ["#soulhack", "#warroom", "#securityfeeds"]
ircProxy = "socks5://127.0.0.1:9050"
llmProxy = "http://127.0.0.1:8180"
skipTLSVerify = false
useTLS = true
adminOnly = false
plugins = ["/plugins/ip.lua", "/plugins/urban.lua"]
[ircd.devinet.watchlist.security]
watchList = ["#securityfeeds"]
watchFiles = ["/watchfiles/voidbox.list"]
alertChannel = "#milla_alerts"
eventTypes = ["PRIVMSG"]
fgColor = 0
bgColor = 28
[ircd.devinet.rss.manga]
rssFile = "/rssfeeds/manga.json"
channel = "#manga"
[ircd.devinet.rss.anime]
rssFile = "/rssfeeds/anime.json"
channel = "#anime"

[ircd.liberanet]
ircServer = "irc.libera.chat"
ircNick = "milla"
model = "gpt-3.5-turbo"
ircPort = 6697
chromaStyle = "rose-pine-moon"
chromaFormatter = "terminal16m"
provider = "gemini"
apikey = "xxxx"
temp = 0.5
requestTimeout = 10
millaReconnectDelay = 60
keepAlive = 20
memoryLimit = 20
pingDelay = 20
pingTimeout = 600
skipTLSVerify = false
useTLS = true
disableSTSFallback = true
allowFlood = false
admins = ["noone_has_this_nick"]
ircChannels = ["##milla1", "##milla2"]
debug = true
out = true
ircProxy = "socks5://127.0.0.1:9051"
llmProxy = "http://127.0.0.1:8181"
adminOnly = true
[ircd.liberanet.customCommands.digest]
sql = "select log from liberanet_milla_us_market_news order by log desc;"
limit = 300
context = ["you are a sentiment-analysis bot"]
prompt= "i have provided to you news headlines in the form of previous conversations between you and me using the user role. please provide the digest of the news for me."
[ircd.liberanet.customCommands.summarize]
sql= "select log from liberanet_milla_us_market_news order by log desc;"
limit= 300
context = ["you are a sentiment-analysis bot"]
prompt= "i have provided to you news headlines in the form of previous conversations between you and me using the user role. please summarize the provided news for me. provide some details."
```

## Commands

#### help

Prints the help message.

#### get

Get the value of a config option. Use the same name as the config file but capitalized: `/get chromaFormatter`

#### getall

Get the value of all config options.

#### set

Set a config option on the fly. Use the same name as the config file but capitalized: `/set chromaFormatter noop`

#### memstats

Returns memory stats for milla.

#### join

Joins a channel: `/join #channel`

#### leave

Leaves a channel: `/leave #channel`

#### load

Load a plugin: `/load /plugins/rss.lua`

#### unload

Unload a plugin: `/unload /plugins/rss.lua`

#### remind

Pings the user after the given amount in seconds: `/remind 1200`

#### roll

Rolls a number between 1 and 6 if no arguments are given. With one argument it rolls a number between 1 and the given number. With two arguments it rolls a number between the two numbers: `/rool 10000 66666`

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

The env vars `UID` and `GID` need to be defined or they can replaces by your host user's uid and gid.<br/>

As a convenience, there is a [distroless](https://github.com/GoogleContainerTools/distroless) dockerfile, `Dockerfile_distroless` also provided.<br/>
A vendored build of milla is available by first running `go mod vendor` and then using the provided dockerfile, `Dockerfile_distroless_vendored`.<br/>

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

### Plugins and Scripting

milla can be extended with plugins. The plugins are written in lua and are loaded at runtime. The plugins are loaded after an IRC connection has been made.<br/>
milla uses [gopher-lua](https://github.com/yuin/gopher-lua) which implements a lua 5.1 VM in Go.<br/>
This means that lua libraries that are implemented in C will not be available to gopher-lua, only pure lua libraries will be available..<br/>
There are a few libraries written in go specifically for gopher-lua that are available to milla. Below there is a list of the current ones.<br/>

An example plugin is provided under `plugins/rss.lua`.<br/>

```yaml
period: 3600
channel: "#rssfeed"
rssfeeds:
  - name: "one"
    url: "https://www.youtube.com/feeds/videos.xml?channel_id=UCaiL2GDNpLYH6Wokkk1VNcg"
  - name: "two"
    url: "https://www.youtube.com/feeds/videos.xml?channel_id=UCd26IHBHcbtxD7pUdnIgiCw"
  - name: "three"
    url: "https://www.youtube.com/feeds/videos.xml?channel_id=UCS4FAVeYW_IaZqAbqhlvxlA"
```

```lua
local milla = require("milla")
local yaml = require("yaml")
local http = require("http")
local xmlpath = require("xmlpath")

local function read_file(file)
    local f = assert(io.open(file, "rb"))
    local content = f:read("*all")
    f:close()
    return content
end

local function sleep(n) os.execute("sleep " .. tonumber(n)) end

local function get_config()
    local yaml_config = read_file("./plugins/rss.yaml")
    local config = yaml.parse(yaml_config)
    return config
end

local function get_rss_feed(config)
    local titles = {}
    local author_names = {}
    local uris = {}
    local rss_feed_list = {}

    for _, v in pairs(config.rssfeeds) do
        local response, err = http.request("GET", v.url)
        if err ~= nil then
            milla.send_message(err, "")
            goto continue
        end
        local node, err = xmlpath.loadxml(response.body)
        if err ~= nil then
            milla.send_message(err, "")
            goto continue
        end

        local path, err = xmlpath.compile("//entry/title")
        if err ~= nil then
            milla.send_message(err, "")
            goto continue
        end
        local iterator = path:iter(node)
        for _, match in ipairs(iterator) do
            table.insert(titles, match:string())
        end

        path, err = xmlpath.compile("//entry/author/name")
        -- local path, err = xmlpath.compile("//entry/title")
        if err ~= nil then
            milla.send_message(err, "")
            goto continue
        end
        iterator = path:iter(node)
        for _, match in ipairs(iterator) do
            table.insert(author_names, match:string())
        end

        path, err = xmlpath.compile("//entry/author/uri")
        -- local path, err = xmlpath.compile("//entry/title")
        if err ~= nil then
            milla.send_message(err, "")
            goto continue
        end
        iterator = path:iter(node)
        for _, match in ipairs(iterator) do
            table.insert(uris, match:string())
        end
        ::continue::
    end

    for i = 1, #titles do
        table.insert(rss_feed_list,
                     author_names[i] .. ": " .. titles[i] .. " -- " .. uris[i])
    end

    return rss_feed_list
end

local function rss_feed()
    local config = get_config()
    while true do
        for _, v in pairs(get_rss_feed(config)) do
            milla.send_message(v, config.channel)
            sleep(config.period)
        end
    end
end

rss_feed()
```

The example rss plugin, accepts a yaml file as input, reeds the provided rss feeds once, extracts the title, author name and link to the resource, sends the feed over to the `#rssfeed` irc channel and exits.<br/>
Also please note that this is just an example script. If you want milla to handle some rss feeds for you, you can use the builtin rss functionality.<br/>

### Milla's Lua Module

Here's a list of the available function in milla's lua module:

```lua
milla.send_message(msg, target)
```

```lua
milla.join_channel(channel)
```

```lua
milla.part_channel(channel)
```

```lua
milla.send_ollama_request(prompt)
```

```lua
milla.send_gemini_request(prompt)
```

```lua
milla.send_chatgpt_request(prompt)
```

```lua
milla.query_db(query)
```

```lua
milla.register_cmd(script_path, cmd_name, function_name)
```

```lua
milla.url_encode(str)
```

Using `register_cmd` we can register a command that will be available to run like the built-in and customs commands.<br/>
Here's an example of how to use it:<br/>

```lua
local milla = require("milla")
local os = require("os")
local json = require("json")

-- setting the proxy value before loading the http module
-- this way, only this script will be using this proxy
os.setenv("ALL_PROXY", "socks5://172.17.0.1:9057")

local http = require("http")

-- this function should be global
-- one string arg that holds all args
-- should only return one string value
function milla_get_ip(arg)
    local ip = arg
    local response, err = http.request("GET", "http://ip-api.com/json/" .. ip)
    if err ~= nil then print(err) end

    local json_response, err = json.decode(response.body)
    if err ~= nil then print(err) end
    for k, v in pairs(json_response) do print(k, v) end

    local result = ""
    for key, value in pairs(json_response) do
        result = result .. key .. ": " .. value .. " -- "
    end

    return result
end

milla.register_cmd("/plugins/ip.lua", "ip", "milla_get_ip")
```

This will allow us to do:<br/>

```txt
terra: /ip 1.1.1.1
```

And get this in response:<br/>

```txt
isp: Cloudflare, Inc -- query: 1.1.1.1 -- status: success -- regionName: Queensland -- lat: -27.4766 -- timezone: Australia/Brisbane -- region: QLD -- lon: 153.0166 -- country: Australia -- countryCode: AU -- city: South Brisbane --ip: 4101 -- org: APNIC and Cloudflare DNS Resolver project -- as: AS13335 Cloudflare, Inc. --
```

### NOTES

- Each lua plugin gets its own lua state and will run in a goroutine.<br/>
- Lua plugins will not go through a proxy if they are not instructed to do so. If you are using the provided http module, you can set the proxy value before loading the http module as provided in the examples under `plugins`. The module will read and set the following environment variables in the order given:

  - `ALL_PROXY`
  - `HTTPS_PROXY`
  - `HTTP_PROXY`
  - `https_proxy`
  - `http_proxy`

  `http` and `socks5` proxies are supported. unfortunately, the `socks5h` proxy is not supported.<br/>

```sh
ALL_PROXY=socks5://172.17.0.1:9050
```

More of milla's functionality will be available through milla's lua module over time.<br/>

The following libraries are loaded by milla by default:

- [gluaxmlpath](https://github.com/ailncode/gluaxmlpath)
- [gluahttp](https://github.com/cjoudrey/gluahttp)
- [gluayaml](https://github.com/kohkimakimoto/gluayaml)
- [gluasocket](https://gitlab.com/megalithic-llc/gluasocket)
- [gluare](https://github.com/yuin/gluare)
- [gopher-json](https://github.com/layeh/gopher-json)

## FAQ

- I end up with color escape sequences getting printed at the end of a line/begging of the next line. What gives?
  This is happening because you have reached the message limit on irc which 512 for the event. This practically leaves around 390-400 character left for the message itself. Certain ircds allow for bigger sizes and certain clients might do. But most ircds don't send `linelen` to the clients. In a closed-loop situation where you control everything, as in, the ircd and all the clients(i.e. A private irc network), you can try to increase the `linelen` for the ircd and the client. Please note that the client in this case is girc. You irc client can have its own set of limits too. The 512 limit is hardcoded in girc. You can vendor the build or use the vendored dockerfile, change the hard limit and run milla with an increased limit. Needless to say, you can try to use a `chromaFormatter` that produces less characters which is basically not using truecolor or `terminal16m`.

## Resources

- [OpenRSS](https://openrss.org/)
- [Google Alerts](https://www.google.com/alerts)

## Thanks

Milla would not exist without the following projects:

- [girc](https://github.com/lrstanley/girc)
- [gopher-lua](https://github.com/yuin/gopher-lua)
- [chroma](https://github.com/alecthomas/chroma)
- [pgx](https://github.com/jackc/pgx)
- [ollama](https://github.com/ollama/ollama)
- [toml](https://github.com/BurntSushi/toml)
- [gofeed](https://github.com/mmcdole/gofeed)

## Similar Projects

- [soulshack](https://github.com/pkdindustries/soulshack)
