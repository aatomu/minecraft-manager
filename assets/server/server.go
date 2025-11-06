package main

import (
	"bufio"
	"io"
	"net/http"
	"os"
	"os/exec"

	"golang.org/x/net/websocket"
)

func serverState(w http.ResponseWriter, r *http.Request) {
	if jvm == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
	return
}

func serverUp(w http.ResponseWriter, r *http.Request) {
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

func serverExec(w http.ResponseWriter, r *http.Request) {
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
