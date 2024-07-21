local milla = require("milla")
local os = require("os")
local json = require("json")

os.setenv("ALL_PROXY", "socks5://172.17.0.1:9057")

local http = require("http")

function milla_urban(arg)
    local user_agent =
        "Mozilla/5.0 (X11; U; Linux x86_64; pl-PL; rv:1.9.2.10) Gecko/20100922 Ubuntu/10.10 (maverick) Firefox/3.6.10"
    local term = arg
    local url = "http://api.urbandictionary.com/v0/define?term=" .. term
    local response, err = http.request("GET", url, {
        timeout = "10s",
        headers = {
            ["User-Agent"] = user_agent,
            ["Accept"] = "application/json",
            ["Host"] = "api.urbandictionary.com",
            ["Connection"] = "keep-alive",
            ["Cache-Control"] = "no-cache",
            ["DNT"] = 1,
            ["sec-ch-ua-platform"] = "Windows",
            ["Pragma"] = "no-cache"
        }
    })
    if err ~= nil then print(err) end
    print(response.body)

    local json_response, err = json.decode(response.body)
    if err ~= nil then print(err) end

    local result = ""
    for k, v in ipairs(json_response["list"]) do
        for kk, vv in pairs(v) do print(kk, vv) end
        if k == 1 then result = v["definition"] end
    end

    return result
end

milla.register_cmd("/plugins/urban.lua", "urban", "milla_urban")
