package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/aatomu/aatomlib/rcon"
	"github.com/bwmarrin/discordgo"
	"github.com/fsnotify/fsnotify"
)

func tailLog() {
	watcher, _ := fsnotify.NewWatcher()
	watcher.Add(*LogDir)
	defer watcher.Close()
	var logFilePath = filepath.Join(*LogDir, "latest.log")
	var firstRead = true

	// Tailing Log File
	for {
		func() {
			// Open File
			f, err := os.Open(logFilePath)
			if err != nil {
				return
			}
			if firstRead {
				// Jump to EOF
				f.Seek(0, 2)
				firstRead = false
			}
			defer f.Close()

			// Check File Events
			changeFile := make(chan bool)
			isWroteFile := make(chan bool)
			go func() {
				for {
					event := <-watcher.Events
					switch {
					case event.Name == logFilePath && event.Op == fsnotify.Rename:
						changeFile <- true
						return
					case event.Op == fsnotify.Write:
						isWroteFile <- true
					}
				}
			}()

			// Read File
			reader := bufio.NewReader(f)
			for {
				line, err := reader.ReadString('\n')
				if err != nil {
					if err == io.EOF { // End Of File
						select {
						case <-changeFile:
							return
						case <-isWroteFile:
							continue
						}
					}
					return
				}
				logAnalyze(strings.Trim(line, "\n"))
			}
		}()
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
				match := reg.FindStringSubmatch(line) // $2:Message
				SendWebhook(discordgo.WebhookParams{
					Content: match[1],
				})

			case "player":
				match := reg.FindStringSubmatch(line) // $1:MCID(unsafe) $2:Message
				mcid := regexp.MustCompile(`([\w_]{3,16})`).FindStringSubmatch(match[1])
				SendWebhook(discordgo.WebhookParams{
					Embeds: GetWebhookEmbed(mcid[1], fmt.Sprintf("%s %s", mcid[1], match[2])),
				})

			case "message":
				match := reg.FindStringSubmatch(line) // $1:MCID(unsafe) $2:Message
				mcid := regexp.MustCompile(`([\w_]{3,16})`).FindStringSubmatch(match[1])
				SendWebhook(discordgo.WebhookParams{
					Username:  mcid[1],
					AvatarURL: "https://minotar.net/helm/" + mcid[1] + "/600",
					Content:   match[2],
				})
			}

			if logConfig.Command != "" {
				sendServerStatus(logConfig.Command)
			}
		}
	}
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

	case "server_stopped":
		for try := 0; try < 10; try++ {
			// Server status check
			if IsServerBooted() {
				time.Sleep(time.Duration(try) * 100 * time.Millisecond)
				continue
			}

			// When server has down
			SendWebhook(discordgo.WebhookParams{
				Embeds: []*discordgo.MessageEmbed{
					{
						Color: ColorError,
						Title: "Minecraft server stopped",
					},
				},
			})
		}
	}
}
func serverStart() {
	b, err := sshCommand(fmt.Sprintf("%s %s", ScriptBoot, *ServerName)).CombinedOutput()
	if err != nil {
		SendWebhook(discordgo.WebhookParams{
			Embeds: []*discordgo.MessageEmbed{
				{
					Color:       ColorError,
					Title:       "Server boot script execution failed",
					Description: "Please check log",
				},
			},
		})
		PrintLog(CommandError, fmt.Sprintf("code:%s\n%s", err.Error(), string(b)))
	}
}

func serverStop() {
	// MC停止
	sendCmd("say Server shutdown has been called, will stop in 10 seconds.")
	time.Sleep(10 * time.Second)
	sendCmd("stop")
}

func serverBackup() {
	sendCmd("save-off")
	sendCmd("save-all flush")
	b, err := sshCommand(fmt.Sprintf("%s \"%s\" \"%s\" \"%s\" \"%s\" \"%s\"", ScriptBackup, ServerDir, BackupDir, ScriptBackupRsyncArg, ScriptBackupRsyncCommand, DiscordWebhookUrl)).CombinedOutput()
	if err != nil {
		SendWebhook(discordgo.WebhookParams{
			Embeds: []*discordgo.MessageEmbed{
				{
					Color:       ColorError,
					Title:       "Server backup script execution failed",
					Description: "Please check log",
				},
			},
		})
		PrintLog(CommandError, fmt.Sprintf("code:%s\n%s", err.Error(), string(b)))
	}
	sendCmd("save-on")
}

func serverRestore(timestamp string) {
	b, err := sshCommand(fmt.Sprintf("%s \"%s\" \"%s\" \"%s\" \"%s\"", ScriptRestore, ServerDir, BackupDir, timestamp, DiscordWebhookUrl)).CombinedOutput()
	if err != nil {
		SendWebhook(discordgo.WebhookParams{
			Embeds: []*discordgo.MessageEmbed{
				{
					Color:       ColorError,
					Title:       "Server restore script execution failed",
					Description: "Please check log",
				},
			},
		})
		PrintLog(CommandError, fmt.Sprintf("code:%s\n%s", err.Error(), string(b)))
	}
}

// 鯖確認
func IsServerBooted() (isBooted bool) {
	out, err := getCommand(fmt.Sprintf("ssh -o LogLevel=quiet -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -p %s -i /identity %s@localhost %s", SshPort, SshUser, fmt.Sprintf("docker ps -a -q --filter name=^%s_mc", *ServerName))).CombinedOutput()
	if err != nil {
		return true
	}
	return len(out) != 0
}

// 鯖にコマンド送信
func sendCmd(command string) {
	if !IsServerBooted() {
		return
	}
	rcon, err := rcon.Login(fmt.Sprintf("localhost:%s", RconPort), RconPassword)
	if err != nil {
		PrintLog(MinecraftError, err.Error())
		return
	}
	defer rcon.Close()

	PrintLog(MinecraftInput, command)
	_, err = rcon.SendCommand(command)
	if err != nil {
		PrintLog(MinecraftError, err.Error())
		return
	}
}

func getCommand(cmd string) (command *exec.Cmd) {
	split := strings.Split(cmd, " ")
	return exec.Command(split[0], split[1:]...)
}

func sshCommand(cmd string) (command *exec.Cmd) {
	cmd = fmt.Sprintf("ssh -v -o LogLevel=quiet -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -p %s -i /identity %s@localhost %s", SshPort, SshUser, cmd)
	return getCommand(cmd)
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
