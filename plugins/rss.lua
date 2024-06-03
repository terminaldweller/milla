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

local function get_rss_feed()
    local yaml_config = read_file("./plugins/rss.yaml")
    local config = yaml.parse(yaml_config)
    local titles = {}
    local author_names = {}
    local uris = {}
    local rss_feed_list = {}

    for _, v in pairs(config.rssfeeds) do
        local response, err = http.request("GET", v.url)
        local node, err = xmlpath.loadxml(response.body)

        local path, err = xmlpath.compile("//entry/title")
        local iterator = path:iter(node)
        for _, match in ipairs(iterator) do
            table.insert(titles, match:string())
        end

        path, err = xmlpath.compile("//entry/author/name")
        iterator = path:iter(node)
        for _, match in ipairs(iterator) do
            table.insert(author_names, match:string())
        end

        path, err = xmlpath.compile("//entry/author/uri")
        iterator = path:iter(node)
        for _, match in ipairs(iterator) do
            table.insert(uris, match:string())
        end
    end

    for i = 1, #titles do
        table.insert(rss_feed_list,
                     author_names[i] .. ": " .. titles[i] .. " -- " .. uris[i])
    end

    return rss_feed_list
end

local function rss_feed()
    local rss_feeds = get_rss_feed()
    for _, v in pairs(rss_feeds) do milla.send_message(v, "#rssfeed") end
end

rss_feed()
