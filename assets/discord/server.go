package main

import (
	"bufio"
	"fmt"
	"io"
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

func LogReader() {
	var text chan string = make(chan string)
	go tailLog(text)

	for {
		line := <-text
		PrintLog(MinecraftStandard, line)

		// discord転送
		go func() {
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

					switch logConfig.Command {
					case "bypass":
						match := reg.FindStringSubmatch(line) // $2:Message
						SendWebhook(discordgo.WebhookParams{
							Content: match[1],
						})
						return
					case "player":
						match := reg.FindStringSubmatch(line) // $1:MCID(unsafe) $2:Message
						mcid := regexp.MustCompile(`([\w_]{3,16})`).FindStringSubmatch(match[1])
						SendWebhook(discordgo.WebhookParams{
							Embeds: GetWebhookEmbed(mcid[1], fmt.Sprintf("%s %s", mcid[1], match[2])),
						})
						return
					case "message":
						match := reg.FindStringSubmatch(line) // $1:MCID(unsafe) $2:Message
						mcid := regexp.MustCompile(`([\w_]{3,16})`).FindStringSubmatch(match[1])
						SendWebhook(discordgo.WebhookParams{
							Username:  mcid[1],
							AvatarURL: "https://minotar.net/helm/" + mcid[1] + "/600",
							Content:   match[2],
						})
						return
					}
				}
			}
		}()
	}
}

func tailLog(text chan<- string) {
	// New "latest.log" Checker
	watcher, _ := fsnotify.NewWatcher()
	watcher.Add(*LogDir)
	// "latest.log" File Path
	var logFile = filepath.Join(*LogDir, "latest.log")

	// Tailing Log File
	for {
		func() {
			// Open File
			f, err := os.Open(logFile)
			if err != nil {
				return
			}
			defer f.Close()

			// Check File Events
			keepFile := true
			isWrite := make(chan bool)
			go func() {
				for {
					event := <-watcher.Events
					switch {
					case event.Name == logFile && event.Op == fsnotify.Create:
						keepFile = false
					case event.Op == fsnotify.Write:
						isWrite <- true
					}
				}
			}()

			// Read File
			firstBlocking := true
			reader := bufio.NewReader(f)
			for keepFile {
				line, err := reader.ReadString('\n')
				if err != nil {
					if err == io.EOF { // End Of File
						firstBlocking = false
						<-isWrite
						continue
					}
					return
				}
				if !firstBlocking {
					text <- strings.Trim(line, "\n")
				}
			}
		}()
		time.Sleep(100 * time.Millisecond)
	}
}

func serverStart() {
	b, err := sshCommand(fmt.Sprintf("%s %s", ScriptBoot, *ServerName)).CombinedOutput()
	if err != nil {
		SendWebhook(discordgo.WebhookParams{
			Embeds: []*discordgo.MessageEmbed{
				{
					Color:       CommandError,
					Title:       "Server boot script execution failed",
					Description: "Please check log",
				},
			},
		})
		PrintLog(OutputError, fmt.Sprintf("code:%s\n%s", err.Error(), string(b)))
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
					Color:       CommandError,
					Title:       "Server backup script execution failed",
					Description: "Please check log",
				},
			},
		})
		PrintLog(OutputError, fmt.Sprintf("code:%s\n%s", err.Error(), string(b)))
	}
	sendCmd("save-on")
}

func serverRestore(timestamp string) {
	b, err := sshCommand(fmt.Sprintf("%s \"%s\" \"%s\" \"%s\" \"%s\"", ScriptRestore, ServerDir, BackupDir, timestamp, DiscordWebhookUrl)).CombinedOutput()
	if err != nil {
		SendWebhook(discordgo.WebhookParams{
			Embeds: []*discordgo.MessageEmbed{
				{
					Color:       CommandError,
					Title:       "Server restore script execution failed",
					Description: "Please check log",
				},
			},
		})
		PrintLog(OutputError, fmt.Sprintf("code:%s\n%s", err.Error(), string(b)))
	}
}

// 鯖確認
func IsServerBooted() (isBooted bool) {
	out, err := sshCommand(fmt.Sprintf("docker ps -a -q --filter name=^%s_mc", *ServerName)).CombinedOutput()
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
	cmd = fmt.Sprintf("ssh -v -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -p %s -i /identity %s@localhost %s", SshPort, SshUser, cmd)
	return getCommand(cmd)
}

type OutputType int

const (
	OutputStandard OutputType = iota
	OutputError
	MinecraftInput
	MinecraftStandard
	MinecraftError
)

func PrintLog(t OutputType, m string) {
	switch t {
	case OutputStandard:
		fmt.Printf("%s)   %s\n", *ServerName, m)
	case OutputError:
		fmt.Printf("%s))) %s\n", *ServerName, m)
	case MinecraftInput:
		fmt.Printf("%s<   %s\n", *ServerName, m)
	case MinecraftStandard:
		fmt.Printf("%s>   %s\n", *ServerName, m)
	case MinecraftError:
		fmt.Printf("%s>>> %s\n", *ServerName, m)
	}
}
