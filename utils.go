package main

import (
	"fmt"
	"strings"

	"github.com/lrstanley/girc"
)

func IrcJoin(irc *girc.Client, channel []string) {
	if len(channel) > 1 && channel[1] != "" {
		irc.Cmd.JoinKey(channel[0], channel[1])
	} else {
		irc.Cmd.Join(channel[0])
	}
}

func chunker(inputString string, chromaFormatter string) []string {
	chunks := strings.Split(inputString, "\n")

	switch chromaFormatter {
	case "terminal":
		fallthrough
	case "terminal8":
		fallthrough
	case "terminal16":
		fallthrough
	case "terminal256":
		for count, chunk := range chunks {
			lastColorCode, err := extractLast256ColorEscapeCode(chunk)
			if err != nil {
				continue
			}

			if count <= len(chunks)-2 {
				chunks[count+1] = fmt.Sprintf("\033[38;5;%sm", lastColorCode) + chunks[count+1]
			}
		}
	case "terminal16m":
		fallthrough
	default:
	}

	return chunks
}

func SendToIRC(
	client *girc.Client,
	event girc.Event,
	message string,
	chromaFormatter string,
) {
	chunks := chunker(message, chromaFormatter)

	for _, chunk := range chunks {
		if len(strings.TrimSpace(chunk)) == 0 {
			continue
		}

		client.Cmd.Reply(event, chunk)
	}
}
