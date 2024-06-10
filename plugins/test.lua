local milla = require("milla")

local function sleep(n)
    local t0 = os.clock()
    while os.clock() - t0 <= n do end
end

local function test_printer()
    local config = milla.toml_config.new()
    print(config:IrcServer())
    config:IrcServer("irc.libera.chat")
    print(config:IrcServer())

    while true do
        milla.send_message(config:IrcServer(), "#warroom")
        sleep(5)
    end
end

local function test_query()
    local query =
        "select log from liberanet_milla_us_market_news order by log desc;"
    local result = milla.query_db(query)

    print(result)

    for _, v in ipairs(result) do print(v) end
end

-- test_printer()
test_query()
