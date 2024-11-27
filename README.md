# minecraft-manager

Docker/Discord を利用した Minecraft Manager

注意:

- 指示がなければ、`manager-container`がカレントディレクトリです
- 変数`${****}`はそのまま実行可能ですが `<****>`は随時 補足に合わせて書き換えてください

## 1.server の設定

### 1-1. Server 起動用 Java の入った Image 作成

```bash
./script/download/java.sh <Tag>
```

`<Tag>`のところは[eclipse-temurin Tags | Docker Hub](https://hub.docker.com/_/eclipse-temurin/tags)より探してください \
`docker images`に正しい Tag の`mc_java`があれば成功です \
今回は`21-jdk-jammy`(minecraft ver1.21.3 用)を使用して話を進めていきます

> [!NOTE]
>
> ```bash
> ./script/download_server.sh <Version> <API Token>
>
> # assets/*に変更があったとき
> ./script/download_server.sh rebuild
> ```
>
> `<Version>`のところは minecraft version を書いてください \
> レート制限に引っかかる場合、`<API Token>`のところに GitHub API Token を書いてください \
> `download/<Version>/`にダウンロードされたバイナリを、サーバーディレクトリにコピーしてください。

> [!WARNING]
>
> 上記の機能はサポートされていません

### 1-2. サーバーの設定

```env
Java=21-jdk-jammy
Jar=server.jar
JVM="-Xms2G -Xmx2G"
# subOps="--forceUpgrade --eraseCache" server_boot.shでバージョンアップ時
Dir=/home/User/servers/example
schemaDir=/home/User/servers/schematics
structDir=/home/User/servers/structures
```

上記のようにすべて埋めた`<Server>.env`を`config/`に入れてください \
(`example.env`を Copy&Modify がおすすめ)

各項目の説明:
|Key|Value|
| :-:|:-:|
|Java|Image 作成のときに使用した TAG|
|Jar|server.jar の名前を記入|
|JVM|java の引数|
|subOps|.jar の引数|
|Dir|サーバーディレクトリ|
|schemaDir|共有する schema ファイルのディレクトリ|
|structDir|共有する structure ファイルのディレクトリ|

> [!NOTE]
>
> `subOps=...`は必要な際にコメントアウトを外してください

### 1-3. サーバー の起動

```bash
./script/boot_server.sh <Server>
```

`<Server>`にはサーバー名(config ディレクトリ内の`<Server>.env`)を入れると起動します \
`docker ps`にあれば、ログは`docker logs -f <Server>_mc`で確認できるはずです \
なければ、`latest.log`を確認します

### 1-4. ターミナル/Docker から MC 鯖に接続

> [!IMPORTANT]
>
> アタッチ(接続)は`docker attach -it <Server>_mc` \
> デアタッチ(切断)は`ctrl+P ctrl+Q` \
> 接続した際過去のログは出ないので注意してください。

## 2. discordBot の準備

### 2-1. servers.json の設定

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
  }
}
```

各種設定項目:

- SSH(no Required)
  - User: SSH User Name
  - Port: SSH Port Number
- Scripts(no Required)
  - Boot: Server Boot Script Path(Full Path)
  - Backup: Server Backup Script Path(Full Path)
- Discord(Required)
  - Token: Discord Bot Token
  - AdminRole: Role Can Execute Server Control Commands
  - WebhookURL: Transfer Server <=> Discord Message
- Rcon(Required)
  - Port: Rcon Connect Port
  - Pass: Rcon Login Password

### 2-2. discord-boot.sh の設定

```bash
readonly DOCKER_SOCK="/var/run/docker.sock"
readonly SSH_IDENTITY="${HOME}/.ssh/minecraft-manager"
readonly CONFIG_DIR="${PWD%/*}/config"
```

`DOCKER_SOCK`: docker.d の.sock の場所に \
`SSH_IDENTITY`: minecraft-manager で使用される SSH-key \
`CONFIG_DIR`: `discord-boot.sh`で使用する`servers.json`へのフルパス

> [!TIP]
>
> 基本はデフォルトで問題ありません。

### 2-3. path.env の設定

※ `path_example.env`を`path.env`にリネームして使用してください

```bash
# log file
<ServerName>="<LogDir>"
example="/home/minecraft/servers/example/logs/"
```

`serverDir`,`backupDir`はファイル構造に合わせて更新してください \
またサーバーごとに行を増やして`<ServerName>`に合う`<LogDir>`を記入してください

### 2-4. Bot の起動

```bash
./script/discord-boot.sh <Server>
```

`<Server>`にはサーバー名(config ディレクトリ内の`<Server>.env`)を入れると起動します \
ログは`docker logs -f <Server>_bot`で確認できるはずです \
DiscordBot には以下の権限が必要です

- Oauth2
  - [x] bot
    - General Permissions
      - [x] Manage Webhooks
      - [x] Read Messages/View Channels
    - Text Permissions
      _ [x] Send Messages
      _ [x] Embed Links
      _ [x] Attach Files
      _ [x] Read Message History
      [x] applications.commands

招待リンク `https://discord.com/api/oauth2/authorize?client_id=<Your Bot Client ID>&permissions=536988672&scope=bot%20applications.commands`
以上で Bot 編は終わりです。

> [!CAUTION]
>
> **設定の変更について**
> docker.file の変更や build を挟むような変更があった際は \
> 各自 自分で調べて 対応する DockerImage を削除し 再度`./script/***.sh`を実行してください
