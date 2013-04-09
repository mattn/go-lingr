package lingr

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
)

type Client struct {
	c *http.Client

	nickname string
	endpoint string
	user     string
	password string
	apiKey   string
	session  string
	publicId string
	counter  int
	messageIds []string

	RoomIds   []string
	Rooms     []Room
	Debug     bool
	OnJoin    func(Room, Presence)
	OnLeave   func(Room, Presence)
	OnMessage func(Room, Message)
}

type request map[string]string
type response map[string]interface{}

type Bot struct {
	Id      string `json:"id"`
	Name    string `json:"name"`
	IconUrl string `json:"icon_url"`
	Status  string `json:"status"`
}

type Member struct {
	Username string `json:"username"`
	Name     string `json:"name"`
	IconUrl  string `json:"icon_url"`
	IsOwner  bool   `json:"is_owner"`
	IsOnline bool   `json:"is_online"`
}

type Roster struct {
	Members []Member `json:"members"`
	Bots    []Bot    `json:"bots"`
}

type Room struct {
	Id      string      `json:"id"`
	Name    string      `json:"name"`
	Blurb   interface{} `json:"blurb"`
	BackLog []Message   `json:"messages"`
	Roster  Roster      `json:"roster"`
}

type Message struct {
	Id              string      `json:"id"`
	Room            string      `json:"room"`
	PublicSessionId string      `json:"public_session_id"`
	IconUrl         string      `json:"icon_url"`
	Type            string      `json:"type"`
	SpeakerId       string      `json:"speaker_id"`
	Nickname        string      `json:"nickname"`
	Text            string      `json:"text"`
	Timestamp       string      `json:"timestamp"`
	Mine            bool        `json:"mine"`
}

type Presence struct {
	Room            string `json:"room"`
	PublicSessionId string `json:"public_session_id"`
	IconUrl         string `json:"icon_url"`
	Username        string `json:"username"`
	Nickname        string `json:"nickname"`
	Timestamp       string `json:"timestamp"`
	Status          string `json:"status"`
	Text            string `json:"text"`
}

type Event struct {
	Id       int       `json:"event_id"`
	Message  *Message  `json:"message"`
	Presence *Presence `json:"presence"`
}

type resRoomIds struct {
	Status  string   `json:"status"`
	RoomIds []string `json:"rooms"`
}

type resRooms struct {
	Status string `json:"status"`
	Rooms  []Room `json:"rooms"`
}

type resSession struct {
	Status   string `json:"status"`
	Session  string `json:"session"`
	Nickname string `json:"nickanem"`
	PublicId string `json:"public_id"`
}

type resSubscribe struct {
	Status  string `json:"status"`
	Counter int    `json:"counter"`
}

type resUnsubscribe struct {
	Status string `json:"status"`
}

type resSay struct {
	Status string `json:"status"`
}

type resObserve struct {
	Status  string  `json:"status"`
	Events  []Event `json:"events"`
	Counter int     `json:"counter"`
}

type resArchives struct {
	Status  string     `json:"status"`
	Messages []Message `json:"messages"`
}

func NewClient(user, password, apiKey string) *Client {
	c := new(Client)
	c.endpoint = "http://lingr.com/api/"
	c.user = user
	c.password = password
	c.apiKey = apiKey
	c.c = http.DefaultClient
	return c
}

func (c *Client) get(path string, params request, res interface{}) error {
	values := make(url.Values)
	for k, v := range params {
		values[k] = []string{v}
	}
	r, e := c.c.Get(c.endpoint + path + "?" + values.Encode())
	if e != nil {
		return e
	}
	defer r.Body.Close()

	var reader io.Reader
	if c.Debug {
		reader = io.TeeReader(r.Body, os.Stdout)
	} else {
		reader = r.Body
	}

	e = json.NewDecoder(reader).Decode(&res)
	if e != nil {
		return e
	}
	return nil
}

func (c *Client) post(path string, params request, res interface{}) error {
	values := make(url.Values)
	for k, v := range params {
		values[k] = []string{v}
	}
	r, e := c.c.Post(c.endpoint+path, "application/x-www-form-urlencoded", strings.NewReader(values.Encode()))
	if e != nil {
		return e
	}
	defer r.Body.Close()

	var reader io.Reader
	if c.Debug {
		reader = io.TeeReader(r.Body, os.Stdout)
	} else {
		reader = r.Body
	}

	e = json.NewDecoder(reader).Decode(&res)
	if e != nil {
		return e
	}
	return nil
}

func (c *Client) CreateSession() bool {
	var res resSession
	e := c.post("session/create", request{
		"user":     c.user,
		"password": c.password,
		"api_key":  c.apiKey}, &res)
	if e == nil && res.Status == "ok" {
		c.publicId = res.PublicId
		c.nickname = res.Nickname
		c.session = res.Session
		return true
	} else if e != nil {
		println(e.Error())
	}
	return false
}

func (c *Client) GetRooms() []string {
	var res resRoomIds
	e := c.get("user/get_rooms", request{"session": c.session}, &res)
	if e == nil && res.Status == "ok" {
		c.RoomIds = res.RoomIds
		return res.RoomIds
	} else if e != nil {
		println(e.Error())
	}
	return nil
}

func (c *Client) ShowRoom(room_id string) bool {
	var res resRooms
	e := c.get("room/show", request{"session": c.session, "room": room_id}, &res)
	if e == nil && res.Status == "ok" {
		c.Rooms = res.Rooms
		return true
	} else if e != nil {
		println(e.Error())
	}
	return false
}

func (c *Client) Subscribe(room_id string) bool {
	var res resSubscribe
	e := c.get("room/subscribe", request{"session": c.session, "room": room_id, "reset": "true"}, &res)
	if e == nil && res.Status == "ok" {
		c.counter = res.Counter
		return true
	} else if e != nil {
		println(e.Error())
	}
	return false
}

func (c *Client) Unsubscribe(room_id string) bool {
	var res resUnsubscribe
	e := c.get("room/unsubscribe", request{"session": c.session, "room": room_id}, &res)
	if e == nil && res.Status == "ok" {
		return true
	} else if e != nil {
		println(e.Error())
	}
	return false
}

func (c *Client) Say(room_id string, text string) bool {
	var res resSay
	e := c.get("room/say", request{"session": c.session, "room": room_id, "nickname": c.nickname, "text": text}, &res)
	if e == nil && res.Status == "ok" {
		return true
	} else if e != nil {
		println(e.Error())
	}
	return false
}

func (c *Client) Observe() error {
	var res resObserve
	messageIds := []string{}

	e := c.get("event/observe", request{"session": c.session, "counter": fmt.Sprintf("%d", c.counter)}, &res)
	if e != nil {
		println(e.Error())
		return e
	}
	if res.Status == "ok" {
		if res.Counter != 0 {
			c.counter = res.Counter
		}
		for _, event := range res.Events {
			for _, r := range c.Rooms {
				if event.Message != nil && c.OnMessage != nil {
					if event.Message.PublicSessionId == c.publicId {
						event.Message.Mine = true
					}

					messageIds = append(messageIds, event.Message.Id)
					found := false
					for _, id := range c.messageIds {
						if id == event.Message.Id {
							found = true
						}
					}

					if !found {
						if r.Id == event.Message.Room {
							c.OnMessage(r, *event.Message)
						}
					}
				}
				if event.Presence != nil {
					if r.Id == event.Presence.Room {
						if event.Presence.Status == "online" && c.OnJoin != nil {
							c.OnJoin(r, *event.Presence)
						}
						if event.Presence.Status == "offline" && c.OnLeave != nil {
							c.OnLeave(r, *event.Presence)
						}
					}
				}
			}
		}
		if len(messageIds) > 0 {
			c.messageIds = messageIds
		}
	}
	return nil
}
