package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/net/websocket"
)

var (
	// JVM
	jvm             *os.Process    = nil
	serverArguments []string       = os.Args[1:]
	jvmIn           io.WriteCloser = nil
	// Broadcast
	clientChannelBufferSize    = getEnv("CLIENT_BUFFER_SIZE", 20)
	broadcastChannelBufferSize = getEnv("BROADCAST_BUFFER_SIZE", 50)
	broadcaster                *Broadcaster
	// Rsync
	backupSource      = getEnv("SOURCE", "/mnt/resource/")
	backupDestination = getEnv("DESTINATION", "/mnt/backup/")
	keepGenerations   = getEnv("KEEP_GENERATIONS", 10)
	// API auth
	password = getEnv("PASSWORD", "minecraft-server-manager")
	session  = SessionStore{
		mu: sync.Mutex{},
		s:  map[string][]byte{},
	}
)

type SessionStore struct {
	mu sync.Mutex
	s  map[string][]byte
}

type Broadcaster struct {
	register    chan chan string
	unregister  chan chan string
	subscribers map[chan string]bool
	broadcast   chan string
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

const (
	ThreadJVM       string = "JVM"
	ThreadManager   string = "Manager"
	ThreadExecution string = "Execution"
)

func init() {
	slog.SetDefault(slog.New(&LogHandler{
		threadPad: 9,
		level:     slog.LevelDebug,
		out:       os.Stdout,
	}))
}

func main() {
	// Broadcast streams
	broadcaster = NewBroadcaster()
	go broadcaster.Run()

	// Graceful Shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Start http Server
	http.Handle("/new_token", middleware(http.HandlerFunc(newToken), false)) // GET
	http.Handle("/state", middleware(http.HandlerFunc(serverState), true))   // GET
	http.Handle("/up", middleware(http.HandlerFunc(serverUp), true))         // POST
	http.Handle("/exec", middleware(http.HandlerFunc(serverExec), true))     // POST
	http.Handle("/down", middleware(http.HandlerFunc(serverDown), true))     // POST
	http.Handle("/tail", middleware(websocket.Handler(serverTail), true))    // UPGRADE
	http.Handle("/backup", middleware(http.HandlerFunc(backup), true))       // POST
	http.Handle("/restore", middleware(http.HandlerFunc(restore), true))     // POST

	server := &http.Server{Addr: "0.0.0.0:80"}
	go func() {
		err := server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			slog.Error("HTTP server has failed to listen port",
				slog.String("thread", ThreadManager),
				slog.Any("error", err),
			)
		}
	}()

	// Wait signal
	<-ctx.Done()

	// Cleanup jvm
	if jvm != nil {
		slog.Info("Shutting down gracefully, sending signal to JVM...",
			slog.String("thread", ThreadManager),
		)
		// Try normal termination(SIGINT)
		jvm.Signal(os.Interrupt)

		// Waiting...
		select {
		case <-time.After(3 * time.Second):
			if jvm != nil {
				slog.Error("JVM did not terminate gracefully, sending SIGKILL.",
					slog.String("thread", ThreadManager),
					slog.Any("error", errors.New("JVM signal response has timedout")),
				)

				jvm.Signal(os.Kill)
			}
		}
	}

	// Stop http server
	timeoutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	slog.Info("HTTP server send shutdown request",
		slog.String("thread", ThreadManager),
	)
	if err := server.Shutdown(timeoutCtx); err != nil {
		slog.Error("HTTP server failed shutdown.",
			slog.String("thread", ThreadManager),
			slog.Any("error", err),
		)
	}

	slog.Info("Manger processes is exited",
		slog.String("thread", ThreadManager),
	)
}

func getEnv[T float64 | int | bool | string](key string, defaultVal T) T {
	valueStr := os.Getenv(key)

	if valueStr == "" {
		return defaultVal
	}

	switch any(defaultVal).(type) {
	case string:
		return any(valueStr).(T)

	case bool:
		if v, err := strconv.ParseBool(valueStr); err == nil {
			return any(v).(T)
		}

	case int:
		if v, err := strconv.ParseInt(valueStr, 10, 64); err == nil {
			return any(v).(T)
		}

	case float64:
		if v, err := strconv.ParseFloat(valueStr, 64); err == nil {
			return any(v).(T)
		}
	}

	return defaultVal
}

func NewBroadcaster() *Broadcaster {
	return &Broadcaster{
		register:    make(chan chan string),
		unregister:  make(chan chan string),
		subscribers: make(map[chan string]bool),
		broadcast:   make(chan string, broadcastChannelBufferSize),
	}
}

func middleware(next http.Handler, authRequest bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := r.RemoteAddr
		method := r.Method
		uri := r.RequestURI

		if authRequest {
			auth := r.Header.Get("Authorization")
			split := strings.Split(auth, ":")

			authorize := false
			available := false
			if len(split) == 2 {
				available, authorize = verify(split[0], split[1])
			}

			slog.Info("HTTP request received",
				slog.String("thread", ThreadManager),
				slog.String("ip", ip),
				slog.String("method", method),
				slog.String("uri", uri),
				slog.Bool("Available", available),
				slog.Bool("Authorize", authorize))

			if authorize {
				next.ServeHTTP(w, r)
				return
			}

			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		slog.Info("HTTP request received",
			slog.String("thread", ThreadManager),
			slog.String("ip", ip),
			slog.String("method", method),
			slog.String("uri", uri))

		next.ServeHTTP(w, r)
		return
	})
}
