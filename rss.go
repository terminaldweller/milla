package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"slices"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lrstanley/girc"
	"github.com/mmcdole/gofeed"
	"golang.org/x/net/proxy"
)

func GetFeed(feed FeedConfig,
	feedChanel chan<- *gofeed.Feed,
	client *girc.Client,
	pool *pgxpool.Pool,
	channel, groupName string,
) {
	rowName := groupName + "__" + feed.Name + "__"

	parsedFeed, err := feed.FeedParser.ParseURL(feed.URL)
	if err != nil {
		log.Print(err)
	} else {
		query := fmt.Sprintf("select newest_unix_time from rss where name = '%s'", rowName)
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(10)*time.Second)
		defer cancel()

		newestFromDB := int64(0)

		err := pool.QueryRow(ctx, query).Scan(&newestFromDB)
		if err != nil {
			pool.Exec(ctx, fmt.Sprintf("insert into rss (name, newest_unix_time) values ('%s',0)", rowName))
		}

		log.Print("Newset from DB: ", newestFromDB)

		sortFunc := func(a, b *gofeed.Item) int {
			if a.PublishedParsed.Before(*b.PublishedParsed) {
				return -1
			} else if a.PublishedParsed.After(*b.PublishedParsed) {
				return 1
			}

			return 0
		}

		slices.SortFunc(parsedFeed.Items, sortFunc)

		for _, item := range parsedFeed.Items {
			if item.PublishedParsed.Unix() > newestFromDB {
				client.Cmd.Message(channel, parsedFeed.Title+": "+item.Title)
			}
		}

		log.Print(parsedFeed.Items[0].PublishedParsed.Unix())
		log.Print(parsedFeed.Items[len(parsedFeed.Items)-1].PublishedParsed.Unix())

		query = fmt.Sprintf("update rss set newest_unix_time = %d where name = '%s'", parsedFeed.Items[len(parsedFeed.Items)-1].PublishedParsed.Unix(), rowName)

		ctx2, cancel := context.WithTimeout(context.Background(), time.Duration(10)*time.Second)
		defer cancel()

		_, err = pool.Exec(ctx2, query)
		if err != nil {
			log.Print(err)
		}
	}

	feedChanel <- parsedFeed
}

func feedDispatcher(
	config RSSConfig,
	client *girc.Client,
	pool *pgxpool.Pool,
	channel, groupName string,
) {
	feedChanel := make(chan *gofeed.Feed)

	for i := range len(config.Feeds) {
		config.Feeds[i].FeedParser = gofeed.NewParser()

		config.Feeds[i].FeedParser.UserAgent = config.Feeds[i].UserAgent

		if config.Feeds[i].Proxy != "" {
			proxyURL, err := url.Parse(config.Feeds[i].Proxy)
			if err != nil {
				log.Print(err)
				continue
			}

			dialer, err := proxy.FromURL(proxyURL, &net.Dialer{Timeout: time.Duration(config.Feeds[i].Timeout) * time.Second})
			if err != nil {
				log.Print(err)

				continue
			}

			httpClient := http.Client{
				Transport: &http.Transport{
					Dial: dialer.Dial,
				},
			}

			config.Feeds[i].FeedParser.Client = &httpClient
		}
	}

	for _, feed := range config.Feeds {
		go GetFeed(feed, feedChanel, client, pool, channel, groupName)
	}

	// <-feedChanel
}

func ParseRSSConfig(rssConfFilePath string) *RSSConfig {
	file, err := os.Open(rssConfFilePath)
	if err != nil {
		log.Print(err)

		return nil
	}

	var config *RSSConfig

	decoder := json.NewDecoder(file)

	err = decoder.Decode(&config)
	if err != nil {
		log.Print(err)

		return nil
	}

	return config
}

func runRSS(appConfig *TomlConfig, client *girc.Client) {
	for {
		query := fmt.Sprintf(
			`create table if not exists rss (
			id serial primary key,
			name text not null unique,
			newest_unix_time bigint not null
		)`)

		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(appConfig.RequestTimeout)*time.Second)
		defer cancel()

		_, err := appConfig.pool.Exec(ctx, query)
		if err != nil {
			log.Print(err)
			time.Sleep(time.Duration(appConfig.MillaReconnectDelay) * time.Second)
		} else {
			for groupName, rss := range appConfig.Rss {
				log.Print("RSS: joining ", rss.Channel)
				client.Cmd.Join(rss.Channel)
				rssConfig := ParseRSSConfig(rss.RssFile)
				if rssConfig == nil {
					log.Print("Could not parse RSS config file " + rss.RssFile + ". Exiting.")
				} else {
					for {
						feedDispatcher(*rssConfig, client, appConfig.pool, rss.Channel, groupName)
						time.Sleep(time.Duration(rssConfig.Period) * time.Second)
					}
				}
			}
		}
	}
}
