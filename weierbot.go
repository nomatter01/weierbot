// Copyright (c) 2012 by The Weierbot Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	irc "github.com/fluffle/goirc/client"
	"html/template"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

var (
	timezone          = time.UTC
	wolframURL        = "http://www.wolframalpha.com/input/?i=%s"
	mathbinPreviewURL = "http://mathbin.net/preview.cgi?body=%s"
	mathbinURL        = "http://mathbin.net/%s"
	reMathbin         = regexp.MustCompile(`equation_previews/[\d_]+\.png`)
)

func init() {
	tz, err := time.LoadLocation("Europe/Vienna")
	if err != nil {
		log.Fatal(err)
	}
	timezone = tz
}

var commands map[string]func(*WeierBot, string, Message)

var tmpl = template.Must(template.New("weierbot").Parse(`<!doctype html>
<html>
<head>
<title>{{.Title}} - Weierbot</title>
<style>a { color: #111; }</style>
</head>
<body>
<div style="float:right;">
{{if .Prev}}<a style="color:#FF1A00;" href="/{{.Prev}}">Previous</a>{{end}}
{{if .Next}}<a style="color:#FF1A00;" href="/{{.Next}}">Next</a>{{end}}
</div>
<h1 style="color:#111; font-size: 1.2em; margin:0;">Weierbot</h1>
<h2 style="margin: .1em 0 .5em; font-size: 1.8em;">
<a style="color:#FF1A00; text-decoration: none;" href="/{{.Title}}">{{.Title}}</a>
</h2>
<pre style="white-space:pre-wrap;">{{.Chatlog}}</pre>
<p class="margin-top: 1em; color:#888;">Proudly powered by
<a href="https://github.com/nomatter01/weierbot">weierbot</a></p>
</body>
</html>`))

var reURL = regexp.MustCompile(`https?://[-A-Za-z0-9+&@#/%?=~_|!:,.;]*[-A-Za-z0-9+&@#/%=~_|]`)
var reTime = regexp.MustCompile(`(?m)^\[\S+\]`)

type WeierBot struct {
	server     string
	client     *irc.Conn
	log        chan Message
	disconnect chan bool
}

func NewWeierBot(server, nick, password string, channels []string) *WeierBot {
	w := &WeierBot{
		server:     server,
		client:     irc.SimpleClient(nick),
		log:        make(chan Message, 512),
		disconnect: make(chan bool),
	}
	w.client.Me.Name = nick
	w.client.Me.Ident = nick

	w.client.AddHandler("connected", func(conn *irc.Conn, line *irc.Line) {
		if password != "" {
			conn.Privmsg("NickServ", "identify "+password)
		}
		for _, ch := range channels {
			log.Printf("Joining %s\n", ch)
			conn.Join(ch)
		}
	})
	w.client.AddHandler("disconnected", func(conn *irc.Conn, line *irc.Line) {
		w.disconnect <- true
	})
	w.client.AddHandler("join", func(conn *irc.Conn, line *irc.Line) {
		if len(line.Args) >= 1 && strings.HasPrefix(line.Args[0], "#") {
			conn.Privmsg(line.Nick, fmt.Sprintf("Hallo %s, willkommen auf %s. "+
				"Dieser Channel wird unter http://weierbot.tux21b.org/ mitgespeichert.",
				line.Nick, line.Args[0]))
		}
	})
	w.client.AddHandler("privmsg", func(conn *irc.Conn, line *irc.Line) {
		if len(line.Args) < 2 {
			return
		}
		w.handleMessage(line.Args[0], Message{
			Nick:    line.Nick,
			Time:    line.Time,
			Message: line.Args[1],
		})
	})

	go w.writeLog()

	return w
}

func createMathbinImage(latexcode string) string {
	resp, err := http.Get(fmt.Sprintf(mathbinPreviewURL, url.QueryEscape("[EQ]"+latexcode+"[/EQ]")))
	if err != nil {
		return err.Error()
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)

	result := reMathbin.FindString(string(body))

	return fmt.Sprintf(mathbinURL, result)
}

func (bot *WeierBot) handleMessage(target string, msg Message) {
	if strings.HasPrefix(target, "#") {
		bot.log <- msg
	} else {
		target = msg.Nick
	}

	command, ok := commands[msg.Message]
	if ok {
		command(bot, target, msg)
	}

	switch {
	case strings.HasPrefix(msg.Message, "!wolfram "):
		bot.send(target, fmt.Sprintf(wolframURL, url.QueryEscape(msg.Message[9:])))
	case msg.Message == "!log":
		bot.send(msg.Nick, "http://weierbot.tux21b.org/")
	case strings.HasPrefix(msg.Message, "!mathbin "):
		imgurl := createMathbinImage(msg.Message[9:])
		bot.send(target, fmt.Sprintf(imgurl))
	}
}

func (bot *WeierBot) send(target, message string) {
	bot.client.Privmsg(target, message)
	if strings.HasPrefix(target, "#") {
		bot.log <- Message{Nick: bot.client.Me.Nick, Time: time.Now(),
			Message: message}
	}
}

func (bot *WeierBot) writeLog() {
	var (
		file     *os.File
		format   = "2006-01-02.log"
		filename string
	)
	defer func() {
		if file != nil {
			file.Close()
		}
	}()
	for msg := range bot.log {
		msg.Time = msg.Time.In(timezone)
		current := msg.Time.Format(format)
		if file == nil || current != filename {
			if file != nil {
				file.Close()
			}
			f, err := os.OpenFile(current,
				os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0666)
			if err != nil {
				log.Println(err)
			} else {
				file, filename = f, current
			}
		}
		fmt.Fprintf(file, "%s\n", msg)
	}
}

func (bot *WeierBot) ServeIRC() {
	tries := 0
	for {
		tries++
		if err := bot.client.Connect(bot.server); err != nil {
			log.Println(err)
			if tries >= 5 {
				log.Fatal("maximum number of tries exceeded")
			}
			<-time.After(5 * time.Second)
			continue
		}
		tries = 0
		log.Printf("Connected to %s\n", bot.server)
		<-bot.disconnect
	}
}

func (bot *WeierBot) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	files, err := filepath.Glob("*.log")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sort.Strings(files)

	if r.URL.Path == "/" && len(files) > 0 {
		http.Redirect(w, r, "/"+filepath.Base(files[len(files)-1]),
			http.StatusTemporaryRedirect)
	}
	index := sort.SearchStrings(files, r.URL.Path[1:])
	if index < 0 || index >= len(files) {
		http.NotFound(w, r)
		return
	}

	chatlog, err := ioutil.ReadFile(files[index])
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	chatlog = []byte(template.HTMLEscapeString(string(chatlog)))
	chatlog = reURL.ReplaceAll(chatlog, []byte(`<a href="$0">$0</a>`))
	chatlog = reTime.ReplaceAll(chatlog, []byte(`<span style="color: #888;">$0</span>`))

	data := struct {
		Title      string
		Prev, Next string
		Chatlog    template.HTML
	}{
		Title:   filepath.Base(files[index]),
		Chatlog: template.HTML(chatlog),
	}
	if index > 0 {
		data.Prev = files[index-1]
	}
	if index < len(files)-1 {
		data.Next = files[index+1]
	}
	if err := tmpl.Execute(w, data); err != nil {
		log.Println(err.Error())
	}
}

type Message struct {
	Time    time.Time
	Nick    string
	Message string
}

func (m Message) String() string {
	return fmt.Sprintf("[%s] %s: %s", m.Time.Format(time.Kitchen), m.Nick, m.Message)
}

var (
	flagServer   = flag.String("server", "irc.euirc.net", "address of the irc server")
	flagNick     = flag.String("nick", "weierbot", "irc nickname of the bot")
	flagPassword = flag.String("password", "", "NickServ password")
	flagChannels = flag.String("channels", "#tm", "irc channels (comma separated)")
	flagListen   = flag.String("http", ":8005", "http listen address")
)

var (
	start = []string{
		"Just biject it to a",
		"Just view the problem as a",
	}
	first = []string{
		"abelian",
		"associative",
		"computable",
		"Lebesgue-measurable",
		"semi-decidable",
		"simple",
		"combinatorial",
		"structure-preserving",
		"diagonalizable",
		"notsingular",
		"orientable",
		"twice-differentiable",
		"thrice-differentiable",
		"countable",
		"prime",
		"complete",
	}
	second = []struct {
		singular  string
		plural    string
		something bool
	}{
		{"multiset", "multisets", true},
		{"integer", "integers", false},
		{"metric space", "metric spaces", true},
		{"group", "groups", true},
		{"monoid", "monoids", true},
		{"semigroup", "semigroups", true},
		{"bijection", "bijections", false},
		{"4-form", "4-forms", false},
		{"triangulation", "triangulations", false},
	}

	suffix = map[bool]string{
		true:  "n",
		false: "",
	}
)

func addn(ind int) bool {
	return first[ind][0] == 'a' ||
		first[ind][0] == 'e' ||
		first[ind][0] == 'i' ||
		first[ind][0] == 'o' ||
		first[ind][0] == 'u'
}

func randomStart() string {
	var ind = rand.Intn(len(start))
	return start[ind]
}

func randomFirst() string {
	var ind = rand.Intn(len(first))
	return first[ind]
}

func randomSecond(plural bool) string {
	var ind = rand.Intn(len(start))
	if plural {
		return second[ind].plural
	} else {
		for !second[ind].something {
			ind = rand.Intn(len(second))
		}
		return second[ind].singular
	}
	return ""
}

func buildProof() string {
	firstInd := rand.Intn(len(first))

	str := suffix[addn(firstInd)]

	text := fmt.Sprintf("The proof is trivial! %s%s %s %s whose elements are %s %s.", randomStart(), str, first[firstInd], randomSecond(false),
		randomFirst(), randomSecond(true))
	return text
}

func main() {
	rand.Seed(time.Now().UnixNano())
	flag.Parse()

	commands = make(map[string]func(*WeierBot, string, Message))
	commands["!coin"] = func(bot *WeierBot, target string, msg Message) {
		if rand.Intn(2) == 0 {
			bot.send(target, "head")
		} else {
			bot.send(target, "tail")
		}
	}
	commands["!proof"] = func(bot *WeierBot, target string, msg Message) {
		bot.send(target, buildProof())
	}

	bot := NewWeierBot(*flagServer, *flagNick, *flagPassword,
		strings.Split(*flagChannels, ","))

	go func() {
		if err := http.ListenAndServe(*flagListen, bot); err != nil {
			log.Fatal(err)
		}
	}()

	bot.ServeIRC()
}
