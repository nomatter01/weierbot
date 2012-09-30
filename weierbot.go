package main

import (
	"fmt"
	irc "github.com/fluffle/goirc/client"
	"html/template"
	"io/ioutil"
	"log"
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
	timezone   = time.UTC
	wolframURL = "http://www.wolframalpha.com/input/?i=%s"
    mathbinPreviewURL = "http://mathbin.net/preview.cgi?body=%s"
    mathbinURL = "http://mathbin.net/%s"
    reMathbin = regexp.MustCompile(`equation_previews/[\d_]+\.png`)
)

func init() {
	tz, err := time.LoadLocation("Europe/Vienna")
	if err != nil {
		log.Fatal(err)
	}
	timezone = tz
}

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
<pre>{{.Chatlog}}</pre>
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
		conn.Privmsg("NickServ", "identify "+password)
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
        return "error"
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

	switch {
	case strings.HasPrefix(msg.Message, "!wolfram "):
		bot.send(target, fmt.Sprintf(wolframURL, url.QueryEscape(msg.Message[9:])))
	case msg.Message == "!log":
		bot.send(target, "http://weierbot.tux21b.org/")
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
	chatlog = reURL.ReplaceAllFunc(chatlog, func(m []byte) []byte {
		return []byte(fmt.Sprintf("<a href=\"%s\">%s</a>", m, m))
	})
	chatlog = reTime.ReplaceAllFunc(chatlog, func(m []byte) []byte {
		return []byte(fmt.Sprintf("<span style=\"color: #888;\">%s</span>", m))
	})

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

func main() {
	bot := NewWeierBot("irc.euirc.net", "weierbot2", "secret", []string{"#tm-test"})

	go func() {
		if err := http.ListenAndServe(":8005", bot); err != nil {
			log.Fatal(err)
		}
	}()

	bot.ServeIRC()
}
