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
    local response, err = http.request("GET", "http://ip-api.com/json?" .. ip)
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

-- script_path, command_name, function_name
milla.register_cmd("/plugins/ip.lua", "ip", "milla_get_ip")
