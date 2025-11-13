package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"

	"github.com/aatomu/aatomlib/disgord"
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
	Password       = getEnv("PASSWORD", "minecraft-server-manager")
	Token          = getEnv("BOT_TOKEN", "")
	AdminRoleId    = getEnv("ADMIN_ROLE_ID", "")
	ReadChannelId  = getEnv("READ_CHANNEL_ID", "")
	SendWebhookUrl = getEnv("SEND_WEBHOOK_URL", "")

	ManagerUrl = getEnv("MANAGER_URL", "http://server:80")
	ConfigPath = getEnv("CONFIG_PATH", "/mnt/logs.json")

	Log []LogConfig
)

const (
	ThreadCommand   string = "Command"
	ThreadDiscord   string = "Discord"
	ThreadMinecraft string = "Minecraft"
)

func init() {
	slog.SetDefault(slog.New(&LogHandler{
		threadPad: 9,
		level:     slog.LevelDebug,
		out:       os.Stdout,
	}))
}

func main() {
	b, _ := os.ReadFile(filepath.Join(ConfigPath))
	json.Unmarshal(b, &Log)
	if len(Log) == 0 {
		panic("log transfer config not found")
	}

	slog.Info("Loaded Environment",
		slog.String("thread", ThreadDiscord),
		slog.String("BOT_TOKEN", Token),
		slog.String("ADMIN_ROLE_ID", AdminRoleId),
		slog.String("READ_CHANNEL_ID", ReadChannelId),
		slog.String("SEND_WEBHOOK_URL", SendWebhookUrl),
	)
	fmt.Print(strings.Repeat("\n", 3))

	// 呼び出し
	go tailLog()
	//--------------Bot本体--------------
	if Token != "" {
		//bot起動準備
		discord, _ := discordgo.New("Bot " + Token)

		//eventトリガー設定
		discord.AddHandler(onReady)
		discord.AddHandler(onMessageCreate)
		discord.AddHandler(onInteractionCreate)

		//起動
		err := discord.Open()
		if err != nil {
			slog.Error("Discord bot boot failed",
				slog.String("thread", ThreadDiscord),
				slog.Any("err", err),
			)
			os.Exit(1)
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
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc
}

// Botの起動時に呼び出し
func onReady(discord *discordgo.Session, r *discordgo.Ready) {
	//起動メッセージ
	slog.Info("Discord bot on ready",
		slog.String("thread", ThreadDiscord),
	)

	channel, err := discord.Channel(ReadChannelId)
	if err != nil {
		slog.Warn("Read channel luukup failed",
			slog.Any("thread", ThreadDiscord),
			slog.Any("error", err),
		)
		return
	}

	// コマンド生成
	disgord.InteractionCommandCreate(discord, channel.GuildID, []*discordgo.ApplicationCommand{
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
			Name:                     "kill",
			Description:              "サーバーを強制停止します",
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
	if m.ChannelID != ReadChannelId {
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
			ok, _ := disgord.HaveRole(discord, m.GuildID, m.Author.ID, AdminRoleId)
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

	// チャンネル確認
	if i.ChannelID != ReadChannelId {
		return
	}

	slog.Info("User interaction called.",
		slog.String("thread", ThreadDiscord),
		slog.String("name", i.User.String()),
		slog.String("mention", fmt.Sprintf("<@%s>", i.User.ID)),
		slog.String("command", i.Command.Name),
	)

	// 権限確認
	ok, err := disgord.HaveRole(discord, iData.GuildID, i.User.ID, AdminRoleId)
	if !ok || err != nil {
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

	case "kill":
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
					Title:       "Server kill command called",
					Description: "Please wait...",
				},
			},
		})
		go serverKill()

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
	req, _ := http.NewRequest(http.MethodPost, SendWebhookUrl, bytes.NewBuffer(b))
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

func getToken() (id, token string, err error) {
	resp, err := http.Get(ManagerUrl + "/new_token")
	if err != nil {
		err = fmt.Errorf("GET /new_token failed: %w", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("GET /new_token status fail: %s", resp.Status)
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		err = fmt.Errorf("GET /new_token failed read body: %w", err)
		return
	}

	parts := strings.Split(strings.TrimSpace(string(body)), ",")
	if len(parts) != 2 {
		err = errors.New("GET /new_token invalid response body")
		return
	}
	id = parts[0]
	keyHex := parts[1]

	key, _ := hex.DecodeString(keyHex)
	mac := hmac.New(sha512.New, key)
	mac.Write([]byte(id + Password))
	token = hex.EncodeToString(mac.Sum(nil))
	return
}

func APIPost(entorypoint string) error {
	id, token, err := getToken()
	if err != nil {
		return fmt.Errorf("getToken() err:%v", err)
	}

	req, _ := http.NewRequest(http.MethodPost, entorypoint, nil)
	req.Header.Set("Authorization", fmt.Sprintf("%s:%s", id, token))

	client := &http.Client{}
	_, err = client.Do(req)
	if err != nil {
		return fmt.Errorf("POST \"%s\" err:%v", entorypoint, err)
	}

	return nil
}
