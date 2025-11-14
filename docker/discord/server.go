package main

import (
	"bufio"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"golang.org/x/net/websocket"
)

func tailLog() {
	var id, token string
	var err error

	for {
		for {
			id, token, err = getToken()
			if err == nil {
				break
			}

			slog.Warn("Failed to get new token",
				slog.String("thread", ThreadMinecraft),
				slog.String("retry_in", "5s"),
				slog.Any("error", err),
			)

			time.Sleep(5 * time.Second)
		}

		wsURL := strings.Replace(ManagerUrl, "http", "ws", 1) + "/tail"
		config, err := websocket.NewConfig(wsURL, "http://localhost")
		if err != nil {
			slog.Error("Failed to create websocket config",
				slog.String("thread", ThreadMinecraft),
				slog.Any("error", err),
			)
			return
		}
		config.Header.Set("Authorization", fmt.Sprintf("%s:%s", id, token))

		for {
			slog.Info("Connecting to log stream",
				slog.String("thread", ThreadMinecraft),
			)

			ws, err := websocket.DialConfig(config)
			if err != nil {
				slog.Warn("Failed to connect to websocket",
					slog.String("thread", ThreadMinecraft),
					slog.String("retry_in", "5s"),
					slog.Any("error", err),
				)
				time.Sleep(5 * time.Second)
				continue
			}

			slog.Info("Connected to log stream.",
				slog.String("thread", ThreadMinecraft),
			)

			reader := bufio.NewReader(ws)
			for {
				line, err := reader.ReadString('\n')
				if err != nil {
					if err != io.EOF {
						slog.Error("Log stream/websocket failed to read",
							slog.String("thread", ThreadMinecraft),
							slog.Any("error", err),
						)
					}
					break // Reconnect on any error
				}
				logAnalyze(line)
			}

			ws.Close()
			slog.Info("Disconnected from log stream.",
				slog.String("thread", ThreadMinecraft),
			)

			time.Sleep(1 * time.Second) // Shorter delay for reconnection
		}
	}
}

func logAnalyze(line string) {
	slog.Info(strings.TrimSuffix(line, "\n"),
		slog.String("thread", ThreadMinecraft),
	)

	if len(line) > 2000 {
		// Embed Text Max is 2000 char
		return
	}

	for _, logConfig := range Log {
		for _, regexpString := range logConfig.Regexp {
			reg := regexp.MustCompile(regexpString)
			if !reg.MatchString(line) {
				continue
			}

			switch logConfig.Action {
			case "bypass":
				match := Submatch(reg, line, 1, "") // $0: Message
				SendWebhook(discordgo.WebhookParams{
					Content: match[0],
				})

			case "player":
				match := Submatch(reg, line, 2, "") // $0:MCID(unsafe) $1:Message
				mcid := Submatch(regexp.MustCompile(`([\w_]{3,16})`), match[0], 1, "Steve")
				SendWebhook(discordgo.WebhookParams{
					Embeds: GetWebhookEmbed(mcid[0], fmt.Sprintf("%s %s", mcid[0], match[1])),
				})

			case "message":
				match := Submatch(reg, line, 2, "") // $0:MCID(unsafe) $1:Message
				mcid := Submatch(regexp.MustCompile(`([\w_]{3,16})`), match[0], 1, "Steve")
				SendWebhook(discordgo.WebhookParams{
					Username:  mcid[0],
					AvatarURL: "https://minotar.net/helm/" + mcid[0] + "/600",
					Content:   match[1],
				})
			}

			if logConfig.Command != "" {
				sendServerStatus(logConfig.Command)
			}
		}
	}
}

// return [$0:firstSubmatch, $1:secondSubmatch, ...]
func Submatch(reg *regexp.Regexp, t string, req int, dummy string) []string {
	match := reg.FindStringSubmatch(t)

	if match == nil {
		match = make([]string, req+1)
		for i := 1; i <= req; i++ {
			match[i] = dummy
		}
	} else {
		for len(match) <= req {
			match = append(match, dummy)
		}
	}

	return match[1:]
}

func sendServerStatus(command string) {
	switch command {
	case "server_starting":
		SendWebhook(discordgo.WebhookParams{
			Embeds: []*discordgo.MessageEmbed{
				{
					Color: ColorWarning,
					Title: "Minecraft server starting",
				},
			},
		})

	case "server_started":
		SendWebhook(discordgo.WebhookParams{
			Embeds: []*discordgo.MessageEmbed{
				{
					Color: ColorSuccess,
					Title: "Minecraft server started",
				},
			},
		})

	case "server_stopping":
		SendWebhook(discordgo.WebhookParams{
			Embeds: []*discordgo.MessageEmbed{
				{
					Color: ColorWarning,
					Title: "Minecraft server stopping",
				},
			},
		})
		for IsServerBooted() {
			time.Sleep(5 * time.Second)
		}
		// When server has down
		SendWebhook(discordgo.WebhookParams{
			Embeds: []*discordgo.MessageEmbed{
				{
					Color: ColorSuccess,
					Title: "Minecraft server stopped",
				},
			},
		})

	case "server_save":
		SendWebhook(discordgo.WebhookParams{
			Embeds: []*discordgo.MessageEmbed{
				{
					Color: ColorSuccess,
					Title: "Minecraft server saved",
				},
			},
		})
	}
}

func serverStart() {
	err := APIPost(ManagerUrl + "/up")
	if err != nil {
		SendWebhook(discordgo.WebhookParams{
			Embeds: []*discordgo.MessageEmbed{
				{
					Color:       ColorError,
					Title:       "Server up execution failed",
					Description: "Failed fetch server",
				},
			},
		})
		slog.Error("Failed to start server",
			slog.String("thread", ThreadCommand),
			slog.Any("error", err),
		)
	}
}

func serverStop() {
	err := APIPost(ManagerUrl + "/exec?input=" + url.QueryEscape("say Server shutdown has been called, will stop in 10 seconds."))
	if err != nil {
		SendWebhook(discordgo.WebhookParams{
			Embeds: []*discordgo.MessageEmbed{
				{
					Color:       ColorError,
					Title:       "Server stop annonce execution failed",
					Description: "Failed fetch server",
				},
			},
		})
		slog.Error("Failed to announce server stop",
			slog.String("thread", ThreadCommand),
			slog.Any("error", err),
		)
		return
	}

	time.Sleep(10 * time.Second)

	err = APIPost(ManagerUrl + "/exec?input=" + url.QueryEscape("stop"))
	if err != nil {
		SendWebhook(discordgo.WebhookParams{
			Embeds: []*discordgo.MessageEmbed{
				{
					Color:       ColorError,
					Title:       "Server stop execution failed",
					Description: "Failed fetch server",
				},
			},
		})
		slog.Error("Failed to stop server",
			slog.String("thread", ThreadCommand),
			slog.Any("error", err),
		)
		return
	}
}

func serverKill() {
	APIPost(ManagerUrl + "/exec?input=" + url.QueryEscape("say Server shutdown has been called, will stop in 10 seconds."))
	time.Sleep(10 * time.Second)

	err := APIPost(ManagerUrl + "/down?force")
	if err != nil {
		SendWebhook(discordgo.WebhookParams{
			Embeds: []*discordgo.MessageEmbed{
				{
					Color:       ColorError,
					Title:       "Server kill execution failed",
					Description: "Failed fetch server",
				},
			},
		})
		slog.Error("Failed to kill server",
			slog.String("thread", ThreadCommand),
			slog.Any("error", err),
		)
		return
	}
}

func serverBackup() {
	APIPost(ManagerUrl + "/exec?input=" + url.QueryEscape("save-off"))
	APIPost(ManagerUrl + "/exec?input=" + url.QueryEscape("save-all flush"))
	time.Sleep(30 * time.Second)

	err := APIPost(ManagerUrl + "/backup")
	if err != nil {
		SendWebhook(discordgo.WebhookParams{
			Embeds: []*discordgo.MessageEmbed{
				{
					Color:       ColorError,
					Title:       "Server backup execution failed",
					Description: "Please check log",
				},
			},
		})
		slog.Error("Failed to backup server",
			slog.String("thread", ThreadCommand),
			slog.Any("error", err),
		)
	}
	APIPost(ManagerUrl + "/exec?input=" + url.QueryEscape("save-on"))
}

func serverRestore(timestamp string) {
	err := APIPost(ManagerUrl + "/restore?t=" + url.QueryEscape(timestamp))
	if err != nil {
		SendWebhook(discordgo.WebhookParams{
			Embeds: []*discordgo.MessageEmbed{
				{
					Color:       ColorError,
					Title:       "Server restore execution failed",
					Description: "Please check log",
				},
			},
		})
		slog.Error("Failed to restore server",
			slog.String("thread", ThreadCommand),
			slog.String("timestamp", timestamp),
			slog.Any("error", err),
		)
	}
}

// IsServerBooted checks if the server is running.
func IsServerBooted() (isBooted bool) {
	id, token, err := getToken()
	if err != nil {
		slog.Error("Failed to get new token",
			slog.String("thread", ThreadCommand),
			slog.Any("error", err),
		)
		return false
	}

	req, _ := http.NewRequest(http.MethodGet, ManagerUrl+"/state", nil)
	req.Header.Set("Authorization", fmt.Sprintf("%s:%s", id, token))

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		slog.Error("GET /state failed",
			slog.String("thread", ThreadCommand),
			slog.Any("error", err),
		)
		return false
	}

	return res.StatusCode == http.StatusOK
}

// sendCmd sends a command to the server.
func sendCmd(command string) {
	if !IsServerBooted() {
		return
	}

	escapeString := url.QueryEscape(command)
	slog.Info("send command",
		slog.String("thread", ThreadDiscord),
		slog.String("command", command),
	)

	err := APIPost(ManagerUrl + "/exec?input=" + escapeString)
	if err != nil {
		slog.Error("Failed to send command to server",
			slog.String("thread", ThreadDiscord),
			slog.String("command", command),
			slog.Any("error", err),
		)
	}
}
