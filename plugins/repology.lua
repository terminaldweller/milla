local milla = require("milla")
local os = require("os")
local json = require("json")

-- https://repology.org/api
-- /repology void_x86_64
function repology(arg)
    -- os.setenv("http_proxy", "http://172.17.0.1:8120")

    local http = require("http")

    local url = "https://repology.org/api/v1/repository/" .. arg .. "/problems"

    local response = http.request("GET", url)

    io.write(response.body)

    local json_response, err = json.decode(response.body)
    io.write(json_response)
    if err ~= nil then print(err) end

    for _, item in pairs(json_response) do
        for k, v in ipairs(item) do print(k, v) end
    end

    local result = ""
    for key, value in pairs(json_response) do
        result = result .. key .. ": " .. value .. " -- "
    end

    return result
end

milla.register_cmd("/plugins/repology.lua", "repology", "repology")
