package main

import (
	"bufio"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"golang.org/x/net/websocket"
)

const (
	ClientChannelBufferSize    = 20
	BroadcastChannelBufferSize = 50
)

var (
	logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: true,
		Level:     slog.LevelDebug,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				a.Value = slog.StringValue(a.Value.Time().Format("2006-01-02T15:04:05.000MST"))
			}
			return a
		},
	}))

	jvm         *os.Process    = nil
	jvmIn       io.WriteCloser = nil
	broadcaster *Broadcaster
)

type Broadcaster struct {
	register    chan chan string
	unregister  chan chan string
	subscribers map[chan string]bool
	broadcast   chan string
}

func NewBroadcaster() *Broadcaster {
	return &Broadcaster{
		register:    make(chan chan string),
		unregister:  make(chan chan string),
		subscribers: make(map[chan string]bool),
		broadcast:   make(chan string, BroadcastChannelBufferSize),
	}
}

func (b *Broadcaster) Run() {
	for {
		select {
		case client := <-b.register:
			b.subscribers[client] = true
		case client := <-b.unregister:
			if _, ok := b.subscribers[client]; ok {
				delete(b.subscribers, client)
				close(client)
			}
		case message := <-b.broadcast:
			for client := range b.subscribers {
				select {
				case client <- message:
				default:
					close(client)
					delete(b.subscribers, client)
				}
			}
		}
	}
}

func main() {
	broadcaster = NewBroadcaster()
	go broadcaster.Run()

	http.HandleFunc("/state", ServerState)
	http.HandleFunc("/up", ServerUp)
	http.HandleFunc("/down", ServerDown)
	http.HandleFunc("/exec", ServerExec)
	http.Handle("/tail", websocket.Handler(ServerTail))

	err := http.ListenAndServe("0.0.0.0:80", nil)
	if err != nil {
		panic(err)
	}
}

func ServerState(w http.ResponseWriter, r *http.Request) {
	if jvm == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
	return
}

func ServerUp(w http.ResponseWriter, r *http.Request) {
	if jvm != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("JVM is already working,"))
		return
	}

	requestCmd := r.URL.Query().Get("cmd")
	cmd := exec.Command("java", strings.Split(requestCmd, " ")...)

	jvmIn, _ = cmd.StdinPipe()
	jo, _ := cmd.StdoutPipe()
	je, _ := cmd.StderrPipe()

	err := cmd.Start()
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}

	jvm = cmd.Process
	w.WriteHeader(http.StatusOK)

	go func(p *os.Process) {
		state, err := p.Wait()
		if err != nil {
			logger.Error("JVM process waiting error", "error", err)
		} else {
			logger.Info("JVM process has finished", "state", state.String())
		}

		jvmIn.Close()
		jvm = nil
	}(jvm)

	go func() {
		scanner := bufio.NewScanner(io.MultiReader(jo, je))
		for scanner.Scan() {
			line := scanner.Text()
			broadcaster.broadcast <- line
			logger.Debug(line)
		}

		if err := scanner.Err(); err != nil && err != io.EOF {
			logger.Error("JVM output read error", "error", err)
		}
	}()
}

func ServerExec(w http.ResponseWriter, r *http.Request) {
	if jvm == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	input := r.URL.Query().Get("input")
	_, err := jvmIn.Write([]byte(input + "\n"))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}
	w.WriteHeader(http.StatusOK)
	return
}

func ServerDown(w http.ResponseWriter, r *http.Request) {
	if jvm == nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	force := r.URL.Query().Has("force")

	if !force {
		jvm.Signal(os.Interrupt)
	} else {
		jvm.Signal(os.Kill)
	}
	w.WriteHeader(http.StatusAccepted)
	return
}

func ServerTail(ws *websocket.Conn) {
	clientChan := make(chan string, ClientChannelBufferSize)

	broadcaster.register <- clientChan

	defer func() {
		broadcaster.unregister <- clientChan
		ws.Close()
		logger.Info("Websocket client has closed.")
	}()

	for message := range clientChan {
		_, err := ws.Write([]byte(message))
		if err != nil {
			logger.Warn("Websocket write error(by client)", "error", err)
			return
		}
	}
}
