package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
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

	// Loop until a token is successfully obtained
	for {
		id, token, err = getToken()
		if err == nil {
			break
		}
		PrintLog(ManagerError, fmt.Sprintf("Failed to get token: %s. Retrying in 5 seconds...", err.Error()))
		time.Sleep(5 * time.Second)
	}

	wsURL := strings.Replace(ManagerUrl, "http", "ws", 1) + "/tail"
	config, err := websocket.NewConfig(wsURL, "http://localhost")
	if err != nil {
		// This is a configuration error, likely not recoverable by retrying.
		PrintLog(ManagerError, fmt.Sprintf("Failed to create websocket config: %s", err.Error()))
		return
	}
	config.Header.Set("Authorization", fmt.Sprintf("%s:%s", id, token))

	for {
		PrintLog(ManagerStandard, "Connecting to log stream...")
		ws, err := websocket.DialConfig(config)
		if err != nil {
			PrintLog(ManagerError, fmt.Sprintf("Failed to connect to websocket: %s. Retrying in 5 seconds...", err.Error()))
			time.Sleep(5 * time.Second)
			continue // Retry connection
		}

		PrintLog(ManagerStandard, "Connected to log stream.")

		reader := bufio.NewReader(ws)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if err != io.EOF {
					PrintLog(ManagerError, fmt.Sprintf("Websocket read error: %s", err.Error()))
				}
				break // Reconnect on any error
			}
			logAnalyze(line)
		}

		ws.Close()
		PrintLog(ManagerStandard, "Disconnected from log stream. Reconnecting...")
		time.Sleep(1 * time.Second) // Shorter delay for reconnection
	}
}

func logAnalyze(line string) {
	PrintLog(MinecraftStandard, line)

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
		PrintLog(CommandError, fmt.Sprintf("serverStart() failed\n%s", err.Error()))
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
		PrintLog(CommandError, fmt.Sprintf("serverStop() failed\n%s", err.Error()))
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
		PrintLog(CommandError, fmt.Sprintf("serverStop() failed\n%s", err.Error()))
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
		PrintLog(CommandError, fmt.Sprintf("serverStop() failed\n%s", err.Error()))
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
		PrintLog(CommandError, fmt.Sprintf("backup API failed\n%s", err.Error()))
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
		PrintLog(CommandError, fmt.Sprintf("restore API failed\n%s", err.Error()))
	}
}

// 鯖確認
func IsServerBooted() (isBooted bool) {
	id, token, err := getToken()
	if err != nil {
		PrintLog(CommandError, fmt.Sprintf("getToken() err:%s", err.Error()))
		return false
	}

	req, _ := http.NewRequest(http.MethodGet, ManagerUrl+"/state", nil)
	req.Header.Set("Authorization", fmt.Sprintf("%s:%s", id, token))

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		PrintLog(CommandError, fmt.Sprintf("GET /state err:%s", err.Error()))
		return false
	}

	return res.StatusCode == http.StatusOK
}

// 鯖にコマンド送信
func sendCmd(command string) {
	if !IsServerBooted() {
		return
	}

	escapeString := url.QueryEscape(command)
	PrintLog(MinecraftInput, command)

	err := APIPost(ManagerUrl + "/exec?input=" + escapeString)
	if err != nil {
		PrintLog(ManagerError, err.Error())
	}
}

type OutputType int

const (
	// Source: minecraft-manager
	ManagerStandard OutputType = iota
	ManagerError
	// Source: user input/discord interaction
	CommandStandard
	CommandError
	// Source: minecraft latest.log/Rcon
	MinecraftInput
	MinecraftStandard
	MinecraftError
)

func PrintLog(t OutputType, m string) {
	switch t {
	case ManagerStandard:
		log.Printf("[Manager/OUTPUT]  : %s\n", m)
	case ManagerError:
		log.Printf("[Manager/ERROR]   : %s\n", m)
	case CommandStandard:
		log.Printf("[Command/OUTPUT]  : %s\n", m)
	case CommandError:
		log.Printf("[Command/ERROR]   : %s\n", m)
	case MinecraftInput:
		log.Printf("[Minecraft/INPUT] : %s\n", m)
	case MinecraftStandard:
		log.Printf("[Minecraft/OUTPUT]: %s\n", m)
	case MinecraftError:
		log.Printf("[Minecraft/ERROR] : %s\n", m)
	}
}
