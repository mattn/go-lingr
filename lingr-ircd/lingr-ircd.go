package main

import (
	"bufio"
	"flag"
	"fmt"
	"github.com/mattn/go-lingr"
	"log"
	"net"
	"strings"
)

var addr = flag.String("addr", ":6667", "address:port")
var apikey = flag.String("apikey", "", "lingr apikey")

func prefix(user string) string {
	return fmt.Sprintf("%s!%s@lingr.com", user, user)
}

func ClientConn(conn net.Conn) {
	user := ""
	password := ""
	var client *lingr.Client
	var roomIds []string

	defer conn.Close()

	r := bufio.NewReader(conn)
	for {
		line, _, e := r.ReadLine()
		if e != nil {
			return
		}
		tokens := strings.SplitN(string(line), " ", 3)
		cmd := strings.ToUpper(tokens[0])
		args := tokens[1:]
		switch cmd {
		case "NICK":
			user = args[0]
		case "PASS":
			password = args[0]
		case "USER":
			log.Printf("connecting to Lingr: %s\n", user)
			client = lingr.NewClient(user, password, *apikey)
			//client.Debug = true
			client.CreateSession()
			roomIds = client.GetRooms()
			client.ShowRoom(strings.Join(roomIds, ","))
			client.Subscribe(strings.Join(roomIds, ","))
			log.Printf("connected to Lingr\n")

			fmt.Fprintf(conn, ":lingr %03d %s %s\n", 1, user, ":Welcome to Lingr!")
			fmt.Fprintf(conn, ":lingr %03d %s %s\n", 376, user, ":End of MOTD")

			var room lingr.Room
			for _, id := range roomIds {
				for _, r := range client.Rooms {
					if r.Id == id {
						room = r
						break
					}
				}
				fmt.Fprintf(conn, ":%s JOIN #%s\n", prefix(user), id)
				fmt.Fprintf(conn, ":lingr %03d #%s :%s\n", 332, user, room.Name)
				var names []string
				for _, member := range room.Roster.Members {
					if member.IsOwner {
						names = append(names, "@"+member.Username)
					} else {
						names = append(names, member.Username)
					}
				}
				fmt.Fprintf(conn, ":lingr %03d %s = #%s :%s\n", 353, user, id, strings.Join(names, " "))
				fmt.Fprintf(conn, ":lingr %03d %s #%s :End of NAMES list.\n", 366, user, id)
				/*
					for _, arg := range args {
						arg = strings.ToUpper(arg)
						if arg == "BACKLOG" {

						}
					}
				*/
			}

			client.OnMessage = func(room lingr.Room, message lingr.Message) {
				if message.Mine {
					return
				}
				cmd := "PRIVMSG"
				if message.Type == "bot" {
					cmd = "NOTICE"
				}

				lines := strings.Split(message.Text, "\n")
				for _, line := range lines {
					fmt.Fprintf(conn, ":%s %s #%s :%s\n",
						prefix(message.SpeakerId),
						cmd,
						room.Id,
						strings.TrimSpace(line))
				}
			}
			go func() {
				for {
					client.Observe()
				}
			}()
		case "WHOIS":
			var member lingr.Member
			var joined []string
			for _, r := range client.Rooms {
				for _, m := range r.Roster.Members {
					if m.Username == args[0] {
						member = m
						joined = append(joined, r.Id)
					}
				}
			}

			if len(joined) > 0 {
				fmt.Fprintf(conn, ":lingr %03d %s %s lingr.com * :%s\n", 311, args[0], args[0], member.Name)
				fmt.Fprintf(conn, ":lingr %03d %s :%s\n", 319, args[0], strings.Join(joined, " "))
				fmt.Fprintf(conn, ":lingr %03d %s lingr.com :San Francisco, US\n", 312, args[0])
				fmt.Fprintf(conn, ":lingr %03d %s lingr.com :End of WHOIS list.\n", 318, args[0])
			}
		case "PRIVMSG", "NOTICE":
			room := args[0]
			for len(room) > 0 && room[0] == '#' {
				room = room[1:]
			}
			text := args[1]
			for len(text) > 0 && text[0] == ':' {
				text = text[1:]
			}
			client.Say(room, text)
		case "QUIT":
			fmt.Fprintf(conn, "ERROR :Closing Link: %s (\"Client quit\")\n", prefix(user))
			return
		}
	}
}

func main() {
	flag.Parse()

	l, e := net.Listen("tcp", *addr)
	if e != nil {
		panic(e.Error())
	}
	for {
		c, e := l.Accept()
		if e != nil {
			panic(e.Error())
		}
		go ClientConn(c)
	}

}
