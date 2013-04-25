package main

import (
	"bufio"
	"flag"
	"fmt"
	"github.com/mattn/go-lingr"
	"log"
	"net"
	"runtime"
	"strings"
	"time"
)

var addr = flag.String("addr", ":26667", "address:port")
var apikey = flag.String("apikey", "", "lingr apikey")
var rooms = flag.String("rooms", "", "lingr rooms")
var debug = flag.Bool("debug", false, "debug stream")

//var backlog = flag.Int("backlog", 0, "backlog count")

func prefix(user string) string {
	return fmt.Sprintf("%s!%s@lingr.com", user, user)
}

func updateChannels(client *lingr.Client, conn net.Conn, user string) {
	client.ShowRoom(strings.Join(client.RoomIds, ","))
	client.Subscribe(strings.Join(client.RoomIds, ","))
	log.Printf("connected to Lingr\n")

	fmt.Fprintf(conn, ":lingr %03d %s %s\n", 1, user, ":Welcome to Lingr!")
	fmt.Fprintf(conn, ":lingr %03d %s %s\n", 376, user, ":End of MOTD")

	var room *lingr.Room
	for _, id := range client.RoomIds {
		room = nil
		for _, r := range client.Rooms {
			if r.Id == id {
				room = &r
				break
			}
		}
		if room != nil {
			fmt.Fprintf(conn, ":%s JOIN #%s\n", prefix(user), id)
			fmt.Fprintf(conn, ":lingr %03d %s #%s :%s\n", 332, user, room.Id, room.Name)
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
		}
		/*
			if backlog {
				for _, message := range room.BackLog {
					lines := strings.Split(message.Text, "\n")
					for _, line := range lines {
						fmt.Fprintf(conn, ":%s %s #%s :%s\n",
							prefix(message.SpeakerId),
							"NOTICE",
							room.Id,
							strings.TrimSpace(line))
					}
				}
				room.BackLog = []Message {}
			}
		*/
	}
}

func ClientConn(conn net.Conn) {
	user := ""
	password := ""
	var client *lingr.Client

	defer conn.Close()

	r := bufio.NewReader(conn)

	done := make(chan bool)
	defer func() {
		done <- true
	}()

	for {
		line, _, err := r.ReadLine()
		if err != nil {
			return
		}
		tokens := strings.SplitN(string(line), " ", 3)
		cmd := strings.ToUpper(tokens[0])
		args := tokens[1:]
		if cmd != "PASS" {
			log.Printf("%v\n", tokens)
		}
		switch cmd {
		case "NICK":
			user = args[0]
		case "PASS":
			password = args[0]
		case "USER":
			log.Printf("connecting to Lingr: %s\n", user)
			client = lingr.NewClient(user, password, *apikey)
			client.Debug = *debug
			client.OnMessage = func(room lingr.Room, message lingr.Message) {
				if message.Mine {
					return
				}
				cmd := "PRIVMSG"
				if message.Type == "bot" {
					cmd = "NOTICE"
				}

				lines := strings.Split(message.Text, "\n")
				log.Printf("%v\n", lines)
				for _, line := range lines {
					fmt.Fprintf(conn, ":%s %s #%s :%s\n",
						prefix(message.SpeakerId),
						cmd,
						room.Id,
						strings.TrimSpace(line))
				}
			}
			client.OnPresence = func(room lingr.Room, presence lingr.Presence) {
				isJoin := true
				if presence.Status == "offline" {
					isJoin = false
				}
				for _, member := range room.Roster.Members {
					if member.Username == presence.Username {
						if isJoin {
							fmt.Fprintf(conn, ":%s %s #%s\n",
								prefix(presence.Username),
								"JOIN",
								room.Id)
							if member.IsOwner {
								fmt.Fprintf(conn, ":%s %s #%s +o %s\n",
									prefix(presence.Username),
									"MODE",
									room.Id,
									presence.Username)
							}
						} else {
							fmt.Fprintf(conn, ":%s %s #%s\n",
								prefix(presence.Username),
								"PART",
								room.Id)
						}
					}
				}
			}
			client.OnMembership = func(room lingr.Room, membership lingr.Membership) {
				for _, member := range room.Roster.Members {
					if member.Username == membership.Username {
						member.IsOwner = membership.IsOwner
						mode := "-o"
						if member.IsOwner {
							mode = "+o"
						}
						fmt.Fprintf(conn, ":%s %s #%s %s %s\n",
							prefix(membership.Username),
							"MODE",
							room.Id,
							mode,
							membership.Username)
					}
				}
			}
			for {
				if client.CreateSession() == false {
					time.Sleep(1 * time.Second)
					continue
				}
				if rooms != nil && len(*rooms) > 0 {
					client.RoomIds = strings.Split(*rooms, ",")
				} else {
					client.GetRooms()
				}
				if client.RoomIds == nil {
					time.Sleep(1 * time.Second)
					continue
				}
				break
			}
			updateChannels(client, conn, user)
			go func() {
				for {
					select {
					case <-done:
						return
					default:
					}
					log.Printf("observing")
					if client.Observe() != nil || len(client.RoomIds) == 0 {
						time.Sleep(1 * time.Second)
					}
					runtime.GC()
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
			log.Printf("saying #%s %s\n", room, text)
			client.Say(room, text)
		case "PING":
			fmt.Fprintf(conn, ":%s PONG #%s\n", prefix(user), args[0])
		case "JOIN":
			rooms := strings.Split(args[0], ",")
			requireUpdate := false
			for _, room := range rooms {
				for len(room) > 0 && room[0] == '#' {
					room = room[1:]
				}
				found := -1
				for i := range client.RoomIds {
					if client.RoomIds[i] == room {
						found = i
						break
					}
				}
				if len(room) > 0 && found == -1 {
					client.RoomIds = append(client.RoomIds, room)
					requireUpdate = true
				}
			}
			if requireUpdate {
				log.Printf("subscribing %s\n", args[0])
				updateChannels(client, conn, user)
			}
		case "PART":
			rooms := strings.Split(args[0], ",")
			requireUpdate := false
			for _, room := range rooms {
				for len(room) > 0 && room[0] == '#' {
					room = room[1:]
				}
				found := -1
				for i := range client.RoomIds {
					if client.RoomIds[i] == room {
						found = i
						break
					}
				}
				if found != -1 {
					client.RoomIds = append(client.RoomIds[:found], client.RoomIds[found+1:]...)
					requireUpdate = true
				}
			}
			if requireUpdate {
				log.Printf("unsubscribing %s\n", args[0])
				updateChannels(client, conn, user)
			}
		case "QUIT":
			fmt.Fprintf(conn, "ERROR :Closing Link: %s (\"Client quit\")\n", prefix(user))
			return
		}
	}
}

func main() {
	flag.Parse()

	l, err := net.Listen("tcp", *addr)
	if err != nil {
		panic(err.Error())
	}
	for {
		c, err := l.Accept()
		if err != nil {
			panic(err.Error())
		}
		go ClientConn(c)
	}

}
