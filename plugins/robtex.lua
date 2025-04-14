local milla = require("milla")
local os = require("os")

-- https://www.robtex.com/api/
function robtex(cli_args)
    local args = {}
    for i in string.gmatch(cli_args, "%S+") do table.insert(args, i) end

    os.setenv("http_proxy", "http://172.17.0.1:8120")

    local http = require("http")

    local url_ipquery = "https://freeapi.robtex.com/ipquery/"
    local url_asquery = "https://freeapi.robtex.com/asquery/"
    local url_pdns_forward = "https://freeapi.robtex.com/pdns/forward/"
    local url_rpns_reverse = "https://freeapi.robtex.com/pdns/reverse"

    local url

    if args[1] == "ipquery" then
        url = url_ipquery .. args[2]
    elseif args[1] == "asquery" then
        url = url_asquery .. args[2]
    elseif args[1] == "pdns_forward" then
        url = url_pdns_forward .. args[2]
    elseif args[1] == "pdns_reverse" then
        url = url_rpns_reverse .. args[2]
    else
        return "Invalid command"
    end

    local response = http.request("GET", url)

    io.write(response.body)

    return response.body
end

milla.register_cmd("/plugins/robtex.lua", "robtex", "robtex")
