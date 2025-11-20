package main

import (
	"bufio"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"golang.org/x/net/websocket"
)

func serverState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if jvm == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
	return
}

func serverUp(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if jvm != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("JVM is already working,"))
		return
	}

	command := append([]string{"-c"}, serverArguments...)
	cmd := exec.Command("/bin/sh", command...)

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
	slog.Info("JVM process started successfully",
		slog.String("thread", ThreadExecution),
		slog.Int("pid", jvm.Pid),
		slog.String("jvm_args", fmt.Sprintf("[%s]", strings.Join(serverArguments, " "))),
	)

	w.WriteHeader(http.StatusOK)

	go func(p *os.Process) {
		state, err := p.Wait()
		if err != nil {
			slog.Info("JVM process terminated abnormally",
				slog.String("thread", ThreadExecution),
				slog.Any("error", err),
			)
		}
		slog.Info("JVM process has finished",
			slog.String("thread", ThreadExecution),
			slog.String("state", state.String()),
		)

		if jvmIn != nil {
			jvmIn.Close()
		}
		jvm = nil
	}(jvm)

	go func() {
		scanner := bufio.NewScanner(io.MultiReader(jo, je))
		for scanner.Scan() {
			line := scanner.Text()
			broadcaster.broadcast <- line + "\n"
			slog.Info(line,
				slog.String("thread", ThreadJVM),
			)
		}

		if err := scanner.Err(); err != nil && err != io.EOF {
			slog.Error("JVM output read lines failed",
				slog.String("thread", ThreadExecution),
				slog.Any("error", err),
			)
		}
	}()

	return
}

func serverExec(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

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

func serverDown(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

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

func serverTail(ws *websocket.Conn) {
	clientChan := make(chan string, clientChannelBufferSize)

	broadcaster.register <- clientChan

	defer func() {
		broadcaster.unregister <- clientChan
		ws.Close()
	}()

	for message := range clientChan {
		_, err := ws.Write([]byte(message))
		if err != nil {
			slog.Warn("Websocket write error",
				slog.String("thread", ThreadManager),
				slog.Any("error", err),
			)
			return
		}
	}

	return
}
