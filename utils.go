package main

import "github.com/lrstanley/girc"

func IrcJoin(irc *girc.Client, channel []string) {
	if len(channel) > 1 && channel[1] != "" {
		irc.Cmd.JoinKey(channel[0], channel[1])
	} else {
		irc.Cmd.Join(channel[0])
	}
}
