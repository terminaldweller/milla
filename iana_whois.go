package main

import (
	"log"
	"net"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/net/html"
	"golang.org/x/net/proxy"
)

func IANAWhoisGet(query string, appConfig *TomlConfig) string {
	var httpClient http.Client

	var dialer proxy.Dialer

	if appConfig.GeneralProxy != "" {
		proxyURL, err := url.Parse(appConfig.GeneralProxy)
		if err != nil {
			log.Fatal(err.Error())

			return ""
		}

		dialer, err = proxy.FromURL(proxyURL, &net.Dialer{Timeout: time.Duration(appConfig.RequestTimeout) * time.Second})
		if err != nil {
			log.Fatal(err.Error())

			return ""
		}

		httpClient = http.Client{
			Transport: &http.Transport{
				Dial: dialer.Dial,
			},
		}
	}

	resp, err := httpClient.Get("https://www.iana.org/whois?q=" + query)
	if err != nil {
		log.Println(err)

		return ""
	}

	defer resp.Body.Close()

	doc, err := html.Parse(resp.Body)
	if err != nil {
		log.Println(err)

		return ""
	}

	var getContent func(*html.Node) string

	getContent = func(n *html.Node) string {
		if n.Type == html.ElementNode && n.Data == "pre" {
			var content string
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.TextNode {
					content += c.Data
				}
			}
			return content
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			result := getContent(c)
			if result != "" {
				return result
			}
		}
		return ""
	}

	preContent := getContent(doc)
	log.Println(preContent)

	return preContent
}
