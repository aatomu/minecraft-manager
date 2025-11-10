package main

import (
	"context"
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
	// Log
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
	// JVM
	jvm     *os.Process    = nil
	jvmArgs []string       = os.Args[1:]
	jvmIn   io.WriteCloser = nil
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

			logger.Info("HTTP request received",
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

		logger.Info("HTTP request received",
			slog.String("ip", ip),
			slog.String("method", method),
			slog.String("uri", uri))

		next.ServeHTTP(w, r)
		return
	})
}
