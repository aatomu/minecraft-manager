package main

import (
	"bufio"
	"context"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

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
	jvmArgs     []string       = os.Args[1:]
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
	// Broadcast streams
	broadcaster = NewBroadcaster()
	go broadcaster.Run()

	// Graceful Shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Start http Server
	http.Handle("/state", middleware(http.HandlerFunc(ServerState)))
	http.Handle("/up", middleware(http.HandlerFunc(ServerUp)))
	http.Handle("/down", middleware(http.HandlerFunc(ServerDown)))
	http.Handle("/exec", middleware(http.HandlerFunc(ServerExec)))
	http.Handle("/tail", middleware(websocket.Handler(ServerTail)))

	server := &http.Server{Addr: "0.0.0.0:80"}
	go func() {
		err := server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP server failed to start", "error", err)
		}
	}()

	// Wait signal
	<-ctx.Done()
	logger.Info("Shutting down gracefully, sending signal to JVM...")

	// Cleanup jvm
	if jvm != nil {
		// Try normal termination(SIGINT)
		jvm.Signal(os.Interrupt)

		// Waiting...
		select {
		case <-time.After(3 * time.Second):
			if jvm != nil {
				logger.Warn("JVM did not terminate gracefully, sending SIGKILL.")
				jvm.Signal(os.Kill)
			}
		}
	}

	// Stop http server
	timeoutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(timeoutCtx); err != nil {
		logger.Error("HTTP server forced to shutdown", "error", err)
	}

	logger.Info("Manager program finished.")
}

func middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. IPアドレスの取得
		// 標準的なRemoteAddrを使用。プロキシ経由の場合は X-Forwarded-For などのヘッダも確認すべき。
		ip := r.RemoteAddr

		// 2. HTTPメソッドの取得
		method := r.Method

		// 3. リクエストURIの取得
		uri := r.RequestURI

		// 💡 slog で情報をログに記録
		logger.Info("HTTP request received",
			slog.String("ip", ip),
			slog.String("method", method),
			slog.String("uri", uri))

		// 次のハンドラ（オリジナルの関数）を呼び出す
		next.ServeHTTP(w, r)
	})
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

	cmd := exec.Command("java", jvmArgs...)

	var err error
	jvmIn, err = cmd.StdinPipe()
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}

	jo, _ := cmd.StdoutPipe()
	je, _ := cmd.StderrPipe()

	err = cmd.Start()
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}

	jvm = cmd.Process
	logger.Info("JVM process started successfully", "pid", jvm.Pid, "args", jvmArgs)
	w.WriteHeader(http.StatusOK)

	go func(p *os.Process) {
		state, err := p.Wait()
		if err != nil {
			logger.Error("JVM process waiting error", "error", err)
		} else {
			logger.Info("JVM process has finished", "state", state.String())
		}

		if jvmIn != nil {
			jvmIn.Close()
		}
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

	return
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

	return
}
