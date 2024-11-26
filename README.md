# minecraft-manager
Docker/Discordを利用した Minecraft Manager

注意: 
* このドキュメントは、Ubuntu 22.04.2LTS(Codename:jammy)のインストールを終えた環境を想定しています。
* 基本 **su, sudoやrootは使用しません**、ぶっ壊れます。  
ターミナル表示が`#`ならroot、`$`なら標準ユーザある可能性が高いです。
* 指示がなければ、`manager-container`がカレントディレクトリです
* 変数`${****}`はそのまま実行可能ですが `<****>`は随時 補足に合わせて書き換えてください

## ホストOS側の設定

### Docker のインストール
```bash
sudo curl -sL get.docker.com | bash # 公式からscriptをDL&実行
sudo usermod -aG docker ${USER} # Dockerを使う際にroot権限が必要ないように
newgrp docker # groupを即時反映させるために
```

## serverの設定

### Server起動用Java の入ったImage作成
```bash
./script/download/java.sh <Tag>
```
`<Tag>`のところは[eclipse-temurin Tags | Docker Hub](https://hub.docker.com/_/eclipse-temurin/tags)より探してください  
`docker images`に正しいTagの`mc_java`があれば成功です
今回は`21-jdk-jammy`(minecraft ver1.21.3用)を使用して話を進めていきます

### server.jar等 のダウンロード
```bash
./script/download_server.sh <Version> <API Token>

# assets/*に変更があったとき
./script/download_server.sh rebuild
```
`<Version>`のところは MCversionを書いてください  
レート制限に引っかかる場合、`<API Token>`のところにGitHub API Tokenを書いてください  
今回は`1.18.2`を使用して話を進めていきます  
`download/<Version>/`にダウンロードされたバイナリを、サーバーディレクトリにコピーしてください。  
`server_1.18.2.jar`はVanillaならenv, Fabricなら`fabric-server-launcher.properties`で指定されているように`server/server.jar`、`fabric-server_1.18.2.jar`はenvに合わせて`fabric.jar`にするといいでしょう

### サーバー の設定
```env
Java=21-jdk-jammy
Jar=server.jar
JVM="-Xms2G -Xmx2G"
# subOps="--forceUpgrade --eraseCache" boot_server.shでバージョンアップ時
Dir=/home/User/servers/example
schemaDir=/home/User/servers/schematics
structDir=/home/User/servers/structures
```
上記のようにすべて埋めた`サーバー名.env`を`config/`に入れてください  
(`example.env`をCopy&Modifyがおすすめ)

各項目の説明:
|Key|Value|
| :-:|:-:|
|Java|Image作成のときに使用したTAG|
|Jar|server.jarの名前を記入|
|JVM|javaの引数|
|subOps|.jarの引数|
|Dir|サーバーディレクトリ|
|schemaDir|共有するschemaファイルのディレクトリ|
|structDir|共有するstructureファイルのディレクトリ|

### サーバー の起動
```bash
./script/boot_server.sh <Server>
```
`<Server>`にはサーバー名(configディレクトリ内の`<Server>.env`)を入れると起動します  
`docker ps`にあれば、ログは`docker logs -f <Server>_mc`で確認できるはずです  
なければ、`latest.log`を確認します
確認出来たら 任意の方法(MC内からのコマンド,Docker attach)でMCを落としてください

### ターミナル/DockerからMC鯖に接続
アタッチ(接続)は`docker attach -it <Server>_mc`
デアタッチ(切断)は`ctrl+P ctrl+Q`
です 接続した際過去のログは出ないので注意してください。

## 設定の変更について
docker.fileの変更やbuildを挟むような変更があった際は  
各自 自分で調べて 対応するDockerImageを削除し 再度`./script/***.sh`を実行してください


# [WIP] minecraft-manager

## Works
* MC chat <=> Discord chat
* Start,Backup,Restore,Stop command run by Discord

## 1.discordBotの準備
注意:  
* 指示がなければ、`minecraft-manager`がカレントディレクトリです
* 変数`${???}`はそのまま実行可能ですが `<???>`は随時 補足に合わせて書き換えてください

### 1-1.servers.json の設定
※ `servers_example.json`を`servers.json`にリネームして使用してください
```json
{
  "<Server1>": {
    "SSH": {
      "User": "",
      "Port": ""
    },
    "Scripts": {
      "Boot": "",
      "Backup": ""
    },
    "Discord": {
      "Token": "",
      "AdminRole": "",
      "WebhookURL": ""
    },
    "Rcon": {
      "Port": "",
      "Pass": ""
    }
  },
}
```

各種設定項目:  
* SSH(no Required)
  * User: SSH User Name
  * Port: SSH Port Number
* Scripts(no Required)
  * Boot: Server Boot Script Path(Full Path)
  * Backup: Server Backup Script Path(Full Path)
* Discord(Required)
  * Token: Discord Bot Token
  * AdminRole: Role Can Execute Server Control Commands
  * WebhookURL: Transfer Server <=> Discord Message
* Rcon(Required)
  * Port: Rcon Connect Port
  * Pass: Rcon Login Password

### 1-2.boot_bot.shの設定
```bash
readonly DOCKER_SOCK="/var/run/docker.sock"
readonly SSH_INDENTITY="${HOME}/.ssh/minecraft-manager"
readonly CONFIG_DIR="${PWD%/*}/config" # 変数展開ｨ
```

`DOCKER_SOCK`: docker.dの.sockの場所に
`SSH_IDENTITY`: minecraft-managerで使用されるSSH-key
`CONFIG_DIR`: `boot_bot.sh`で使用する`servers.json`へのフルパス

### 1-3.path.envの設定
※ `path_example.env`を`path.env`にリネームして使用してください

```bash
# log file
<ServerName>="<LogDir>"
example="/home/minecraft/servers/example/logs/"
```
`serverDir`,`backupDir`はファイル構造に合わせて更新してください  
またサーバーごとに行を増やして`<ServerName>`に合う`<LogDir>`を記入してください  

### Bot の起動
```bash
./script/boot_bot.sh <Server>
```
`<Server>`にはサーバー名(configディレクトリ内の`<Server>.env`)を入れると起動します  
ログは`docker logs -f <Server>_bot`で確認できるはずです  
DiscordBotには以下の権限が必要です
* Outh2
  * [x] bot
    * General Permissions
      * [x] Manage Webhooks
      * [x] Read Messages/View Channels
    * Text Permissions
      * [x] Send Messages
      * [x] Embed Links
      * [x] Attach Files
      * [x] Read Message Hisory
  [x] applications.commands

招待リンク `https://discord.com/api/oauth2/authorize?client_id=<Your Bot Client ID>&permissions=536988672&scope=bot%20applications.commands` 
以上でBot編は終わりです。

## 設定の変更について
docker.fileの変更やbuildを挟むような変更があった際は  
各自 自分で調べて 対応するDockerImageを削除し 再度`./script/***.sh`を実行してください