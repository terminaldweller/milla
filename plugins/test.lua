local milla = require("milla")

local function sleep(n)
    local t0 = os.clock()
    while os.clock() - t0 <= n do end
end

local function printer()

    local config = milla.toml_config.new()
    print(config:IrcServer())
    config:IrcServer("irc.libera.chat")
    print(config:IrcServer())

    while true do
        milla.send_message(config:IrcServer(), "#warroom")
        sleep(5)
    end
end

printer()
