package main

import "github.com/lrstanley/girc"

func IrcJoin(irc *girc.Client, channel []string) {
	if len(channel) >= 0 && channel[0] == "" {
		irc.Cmd.JoinKey(channel[0], channel[1])
	} else {
		irc.Cmd.Join(channel[0])
	}
}
