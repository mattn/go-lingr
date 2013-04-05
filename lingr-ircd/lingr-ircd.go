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

var addr = flag.String("addr", ":6667", "address:port")
var apikey = flag.String("apikey", "", "lingr apikey")
var rooms = flag.String("rooms", "", "lingr rooms")

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
		}
		/*
		for _, arg := range args {
			arg = strings.ToUpper(arg)
			if arg == "BACKLOG" {

			}
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
		done <-true
	}()

	for {
		line, _, e := r.ReadLine()
		if e != nil {
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
			//client.Debug = true
			client.CreateSession()
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

			if rooms != nil && len(*rooms) > 0 {
				client.RoomIds = strings.Split(*rooms, ",")
			} else {
				client.GetRooms()
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
					if client.Observe() != nil {
						time.Sleep(1e9)
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
				}
			}
			log.Printf("subscribing %s\n", args[0])
			updateChannels(client, conn, user)
		case "PART":
			rooms := strings.Split(args[0], ",")
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
				}
			}
			log.Printf("unsubscribing %s\n", args[0])
			updateChannels(client, conn, user)
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
