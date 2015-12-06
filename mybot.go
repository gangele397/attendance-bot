package main

import (
	// "encoding/csv"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"sync/atomic"

	"database/sql"
	_ "github.com/go-sql-driver/mysql"
	"golang.org/x/net/websocket"
)

func main() {
	file, _ := os.Open("conf.json")
	decoder := json.NewDecoder(file)
	configuration := Configuration{}
	err := decoder.Decode(&configuration)
	if err != nil {
		fmt.Println("error:", err)
	}

	// start a websocket-based Real Time API session
	ws, id := slackConnect(configuration.Slack_api_key)
	fmt.Println("mybot ready, ^C exits")

	for {
		// read each incoming message
		m, err := getMessage(ws)
		if err != nil {
			log.Fatal(err)
		}

		// see if we're mentioned
		if m.Type == "message" && strings.HasPrefix(m.Text, "<@"+id+">") {
			// if so try to parse if
			parts := strings.Fields(m.Text)
			if len(parts) == 3 && parts[1] == "attendance" {
				go func(m Message) {
					m.Text = getAttendance(parts[2])
					postMessage(ws, m)
				}(m)
			} else if len(parts) == 2 && parts[1] == "attendance" {
				go func(m Message) {
					m.Text = attendance()
					postMessage(ws, m)
				}(m)
			} else {
				// huh?
				m.Text = fmt.Sprintf("sorry, that does not compute\n")
				postMessage(ws, m)
			}
			// Interruping cow
			// } else if m.Type == "message" {
			// 	go func(m Message) {
			// 		m.Text = "MOOO"
			// 		postMessage(ws, m)
			// 	}(m)
		}
	}
}

type responseRtmStart struct {
	Ok    bool         `json:"ok"`
	Error string       `json:"error"`
	Url   string       `json:"url"`
	Self  responseSelf `json:"self"`
}

type responseSelf struct {
	Id string `json:"id"`
}

type Configuration struct {
	Db_user       string
	Db_pass       string
	Db_name       string
	Db_table      string
	Db_host       string
	Db_port       string
	Slack_api_key string
}

// slackStart does a rtm.start, and returns a websocket URL and user ID. The
// websocket URL can be used to initiate an RTM session.
func slackStart(token string) (wsurl, id string, err error) {
	url := fmt.Sprintf("https://slack.com/api/rtm.start?token=%s", token)
	resp, err := http.Get(url)
	if err != nil {
		return
	}
	if resp.StatusCode != 200 {
		err = fmt.Errorf("API request failed with code %d", resp.StatusCode)
		return
	}
	body, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return
	}
	var respObj responseRtmStart
	err = json.Unmarshal(body, &respObj)
	if err != nil {
		return
	}

	if !respObj.Ok {
		err = fmt.Errorf("Slack error: %s", respObj.Error)
		return
	}

	wsurl = respObj.Url
	id = respObj.Self.Id
	return
}

// These are the messages read off and written into the websocket. Since this
// struct serves as both read and write, we include the "Id" field which is
// required only for writing.

type Message struct {
	Id      uint64 `json:"id"`
	Type    string `json:"type"`
	Channel string `json:"channel"`
	Text    string `json:"text"`
}

func getMessage(ws *websocket.Conn) (m Message, err error) {
	err = websocket.JSON.Receive(ws, &m)
	return
}

var counter uint64

func postMessage(ws *websocket.Conn, m Message) error {
	m.Id = atomic.AddUint64(&counter, 1)
	return websocket.JSON.Send(ws, m)
}

// Starts a websocket-based Real Time API session and return the websocket
// and the ID of the (bot-)user whom the token belongs to.
func slackConnect(token string) (*websocket.Conn, string) {
	wsurl, id, err := slackStart(token)
	if err != nil {
		log.Fatal(err)
	}

	ws, err := websocket.Dial(wsurl, "", "https://api.slack.com/")
	if err != nil {
		log.Fatal(err)
	}

	return ws, id
}

func getAttendance(name string) string {
	var (
		attending int
		att       string
		i         int
	)
	file, _ := os.Open("conf.json")
	decoder := json.NewDecoder(file)
	configuration := Configuration{}
	err := decoder.Decode(&configuration)
	if err != nil {
		fmt.Println("error:", err)
	}
	db, err := sql.Open("mysql", configuration.Db_user+":"+configuration.Db_pass+"@tcp("+configuration.Db_host+":"+configuration.Db_port+")/"+configuration.Db_name)
	if err != nil {
		log.Fatal(err)
	}

	stmt, err := db.Prepare("select * from attendance where username = ?")

	if err != nil {
		log.Fatal(err)
	}

	rows, err := stmt.Query(name)

	if err != nil {
		log.Fatal(err)
	}

	for rows.Next() {
		i++
		err = rows.Scan(&name, &attending)
		if err != nil {
			log.Fatal(err)
		}
	}

	defer db.Close()

	if attending == 1 {
		att = "is attending"
	} else if attending == 0 && i > 0 {
		att = "is not attending"
	} else {
		att = "has not responded"
	}
	return fmt.Sprintf("%s %s", name, att)
}

func attendance() string {
	var (
		name string
		i    int
	)
	file, _ := os.Open("conf.json")
	decoder := json.NewDecoder(file)
	configuration := Configuration{}
	err := decoder.Decode(&configuration)
	if err != nil {
		fmt.Println("error:", err)
	}
	db, err := sql.Open("mysql", configuration.Db_user+":"+configuration.Db_pass+"@tcp("+configuration.Db_host+":"+configuration.Db_port+")/"+configuration.Db_name)
	if err != nil {
		log.Fatal(err)
	}

	stmt, err := db.Prepare("select username from attendance where attending = ?")

	if err != nil {
		log.Fatal(err)
	}

	rows, err := stmt.Query(1)

	if err != nil {
		log.Fatal(err)
	}

	attending := []string{}
	for rows.Next() {
		i++
		err = rows.Scan(&name)
		if err != nil {
			log.Fatal(err)
		}
		attending = append(attending, name)
	}

	defer db.Close()

	return fmt.Sprintf("%s", strings.Join(attending, "\n"))
}

func setAttendance(name string, attendance int) {
	// TODO
}
