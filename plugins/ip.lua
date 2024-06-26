local milla = require("milla")
local os = require("os")
local json = require("json")

os.setenv("ALL_PROXY", "socks5://172.17.0.1:9057")

local http = require("http")

local function get_ip(arg)
    local ip = arg
    local response, err = http.request("GET",
                                       "https://getip-api.com/json?" .. ip)
    local json_response, err = json.decode(response.body)

    local result = ""
    for key, value in ipairs(json_response) do
        result = result .. key .. ": " .. value .. "\n"
    end

    return result
end

milla.register_cmd("ip", "get_ip")
