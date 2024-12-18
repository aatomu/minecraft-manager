package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/aatomu/aatomlib/disgord"
	"github.com/aatomu/aatomlib/utils"
	"github.com/bwmarrin/discordgo"
)

type LogConfig struct {
	Regexp  []string `json:"regexp"`
	Action  string   `json:"action"`
	Command string   `json:"command,omitempty"`
}

const (
	ColorError   = 0xFF2929
	ColorWarning = 0xFAB12F
	ColorSuccess = 0x6EC207
)

var (
	ServerDir                = TrimDoubleQuote(os.Getenv("server_dir"))
	BackupDir                = TrimDoubleQuote(os.Getenv("backup_dir"))
	SshUser                  = TrimDoubleQuote(os.Getenv("ssh_user"))
	SshPort                  = TrimDoubleQuote(os.Getenv("ssh_port"))
	ScriptBoot               = TrimDoubleQuote(os.Getenv("script_boot"))
	ScriptBackup             = TrimDoubleQuote(os.Getenv("script_backup"))
	ScriptBackupRsyncArg     = TrimDoubleQuote(os.Getenv("script_backup_rsync_arg"))
	ScriptBackupRsyncCommand = TrimDoubleQuote(os.Getenv("script_backup_rsync_command"))
	ScriptRestore            = TrimDoubleQuote(os.Getenv("script_restore"))
	DiscordBotToken          = TrimDoubleQuote(os.Getenv("discord_bot_token"))
	DiscordAdminRole         = TrimDoubleQuote(os.Getenv("discord_admin_role"))
	DiscordWebhookUrl        = TrimDoubleQuote(os.Getenv("discord_webhook_url"))
	RconPort                 = TrimDoubleQuote(os.Getenv("rcon_port"))
	RconPassword             = TrimDoubleQuote(os.Getenv("rcon_password"))

	Log        []LogConfig
	ChannelID  = ""
	ServerName = flag.String("name", "", "Monitoring Server Name")                 //! Required
	LogDir     = flag.String("log-dir", "/logs", "Minecraft latest.log Directory") //* Not Required
	ConfigDir  = flag.String("config-dir", "/config", "Config Directory")          //* Not Required
)

func main() {
	// Flag check
	flag.Parse()
	if *ServerName == "" {
		panic("Required \"-name\" flag")
	}

	b, _ := os.ReadFile(filepath.Join(*ConfigDir, "logs.json"))
	json.Unmarshal(b, &Log)
	if len(Log) == 0 {
		panic("log transfer config not found")
	}

	PrintLog(ManagerStandard, "=============== [Minecraft] ===============")
	PrintLog(ManagerStandard, fmt.Sprintf("Target Server       : %s", *ServerName))
	PrintLog(ManagerStandard, fmt.Sprintf("Server Directory    : %s", ServerDir))
	PrintLog(ManagerStandard, fmt.Sprintf("Backup Directory    : %s", BackupDir))
	PrintLog(ManagerStandard, "=============== [Discord] ===============")
	PrintLog(ManagerStandard, fmt.Sprintf("Discord Bot Token   : %s", DiscordBotToken))
	PrintLog(ManagerStandard, fmt.Sprintf("Discord Webhook URL : %s", DiscordWebhookUrl))
	PrintLog(ManagerStandard, "=============== [Server Control] ===============")
	PrintLog(ManagerStandard, fmt.Sprintf("SSH Login           : %s@localhost:%s", SshUser, SshPort))
	PrintLog(ManagerStandard, fmt.Sprintf("Rcon Login Port     : %s", RconPort))
	PrintLog(ManagerStandard, fmt.Sprintf("Rcon Login Password : %s", RconPassword))
	fmt.Print(strings.Repeat("\n", 5))

	// 呼び出し
	go tailLog()
	//--------------Bot本体--------------
	if DiscordBotToken != "" {
		//bot起動準備
		discord, _ := discordgo.New("Bot " + DiscordBotToken)

		//eventトリガー設定
		discord.AddHandler(onReady)
		discord.AddHandler(onMessageCreate)
		discord.AddHandler(onInteractionCreate)

		//起動
		err := discord.Open()
		if err != nil {
			PrintLog(ManagerError, "Discord bot authentication failed/Connect to Discord failed")
			panic(err)
		}

		defer func() {
			//Bot停止
			discord.Close()

			//サーバー停止
			if IsServerBooted() {
				//一応連絡
				sendCmd("say Sorry, Bot has stopped.")
			}
		}()
	}

	//停止対策
	<-utils.BreakSignal()
}

// Botの起動時に呼び出し
func onReady(discord *discordgo.Session, r *discordgo.Ready) {
	//起動メッセージ
	PrintLog(ManagerStandard, fmt.Sprintf("`%s` manager discord bot is ready.\n", *ServerName))

	URL, _ := url.Parse(DiscordWebhookUrl)
	webhook, err := discord.Webhook(strings.Split(URL.Path, "/")[3])
	if err != nil {
		PrintLog(ManagerError, "Webhook parse error/Webhook permission denied?")
		panic(err)
	}
	ChannelID = webhook.ChannelID

	// コマンド生成
	disgord.InteractionCommandCreate(discord, webhook.GuildID, []*discordgo.ApplicationCommand{
		{
			Type:                     discordgo.ChatApplicationCommand,
			Name:                     "start",
			Description:              "サーバーを起動します",
			DefaultMemberPermissions: Pinter(discordgo.PermissionViewChannel),
		},
		{
			Type:                     discordgo.ChatApplicationCommand,
			Name:                     "stop",
			Description:              "サーバーを停止します",
			DefaultMemberPermissions: Pinter(discordgo.PermissionViewChannel),
		},
		{
			Type:                     discordgo.ChatApplicationCommand,
			Name:                     "backup",
			Description:              "サーバーのバックアップをします",
			DefaultMemberPermissions: Pinter(discordgo.PermissionViewChannel),
		},
		{
			Type:                     discordgo.ChatApplicationCommand,
			Name:                     "restore",
			Description:              "サーバーのデータ復元をします",
			DefaultMemberPermissions: Pinter(discordgo.PermissionViewChannel),
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "timestamp",
					Description: "適応するタイムスタンプ",
					Required:    true,
				},
			},
		},
	})
}

// Botがメッセージ受信時に呼び出し
func onMessageCreate(discord *discordgo.Session, m *discordgo.MessageCreate) {
	if ChannelID != m.ChannelID {
		return
	}

	//bot,WebHook return
	if m.Author.Bot {
		return
	}

	//サーバーに転送
	if IsServerBooted() {
		text := m.Content
		// メッセージ確認
		if text == "" {
			return
		}

		if strings.HasPrefix(text, "\\") { // "\"から始まる
			ok, _ := disgord.HaveRole(discord, m.GuildID, m.Author.ID, DiscordAdminRole)
			if ok { // 権限を持ってる
				text = strings.Replace(text, "\\", "", 1)
				sendCmd(text)
				return
			}
		}

		//改行削除
		if strings.Contains(text, "\n") {
			text = regexpReplace(text, "  以下略..", "\n.*")
		}

		//文字をunicode化
		unicode := ""
		for _, word := range strings.Split(text, "") {
			unicode = unicode + fmt.Sprintf("\\u%04x", []rune(word)[0])
		}
		command := fmt.Sprintf(`execute if entity @a run tellraw @a {"text":"(%s) %s"}`, m.Author.Username, unicode)
		//送信
		sendCmd(command)
	}
}

// InteractionCreate
func onInteractionCreate(discord *discordgo.Session, iData *discordgo.InteractionCreate) {
	// 表示&処理しやすく
	i := disgord.InteractionParse(discord, iData.Interaction)

	// slashじゃない場合return
	if i.InteractionType != discordgo.InteractionApplicationCommand {
		return
	}

	// 権限確認
	ok, err := disgord.HaveRole(discord, iData.GuildID, i.User.ID, DiscordAdminRole)
	if !ok || err != nil {
		return
	}

	// チャンネル確認
	if i.ChannelID != ChannelID {
		return
	}

	// 返答用
	res := disgord.NewInteractionResponse(discord, iData.Interaction)

	// 処理
	switch i.Command.Name {
	case "start":
		if IsServerBooted() {
			res.Reply(&discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{
					{
						Color: ColorError,
						Title: "Server has running",
					},
				},
				Flags: discordgo.MessageFlagsEphemeral,
			})
			return
		}

		res.Reply(&discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{
				{
					Color:       ColorSuccess,
					Title:       "Server boot command called",
					Description: "Please wait...",
				},
			},
		})
		go serverStart()

	case "stop":
		if !IsServerBooted() {
			res.Reply(&discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{
					{
						Color: ColorError,
						Title: "Server has not running",
					},
				},
				Flags: discordgo.MessageFlagsEphemeral,
			})
			return
		}

		res.Reply(&discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{
				{
					Color:       ColorSuccess,
					Title:       "Server shutdown command called",
					Description: "Please wait...",
				},
			},
		})
		go serverStop()

	case "backup":
		res.Reply(&discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{
				{
					Color:       ColorSuccess,
					Title:       "Server backup command called",
					Description: "Please wait...",
				},
			},
		})
		go serverBackup()

	case "restore":
		timestamp := i.CommandOptions["timestamp"].StringValue()

		res.Reply(&discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{
				{
					Color:       ColorSuccess,
					Title:       "Server backup command called",
					Description: "Please wait...",
				},
			},
		})
		go serverRestore(timestamp)
	}
}

func SendWebhook(m discordgo.WebhookParams) {
	b, _ := json.Marshal(m)
	req, _ := http.NewRequest(http.MethodPost, DiscordWebhookUrl, bytes.NewBuffer(b))
	req.Header.Set("Content-Type", "application/json")
	client := new(http.Client)
	// Request送信
	client.Do(req)
}

func GetWebhookEmbed(MCID, text string) []*discordgo.MessageEmbed {
	return []*discordgo.MessageEmbed{
		{
			Author: &discordgo.MessageEmbedAuthor{
				IconURL: "https://minotar.net/helm/" + MCID + "/600",
				Name:    text,
			},
			Color: 0x99aab5,
		},
	}
}

func regexpReplace(src, rep, reg string) string {
	return regexp.MustCompile(reg).ReplaceAllString(src, rep)
}

func Pinter(n int64) *int64 {
	return &n
}

func TrimDoubleQuote(s string) string {
	s = strings.TrimPrefix(s, "\"")
	s = strings.TrimSuffix(s, "\"")
	return s
}
