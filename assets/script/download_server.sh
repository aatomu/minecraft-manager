cd /mcData

echo "curl -fsSL https://launchermeta.mojang.com/mc/game/version_manifest_v2.json"
VersionURL=$(curl -fsSL https://launchermeta.mojang.com/mc/game/version_manifest_v2.json | jq ".versions[] | select(.id == \"${Version}\") | .url" | sed -e 's/"//g')
echo "curl -fsSL ${VersionURL}"
ServerJar=$(curl -fsSL ${VersionURL} | jq .downloads.server.url | sed -e 's/"//g')
echo "##################################################"
echo "############### --- Vanilla DownLoad URL ---"
echo "############### Version    : ${Version}"
echo "############### VersionURL : ${VersionURL}"
echo "############### ServerJar  : ${ServerJar}"
echo "##################################################"
echo "curl -fsSL -o \"./server.jar\" ${ServerJar}"
curl -fsSL -o "./server_${Version}.jar" ${ServerJar}

# fabricサーバー.jarをダウンロード
echo "curl -fsSL https://meta.fabricmc.net/v2/versions/loader/${Version}/"
Loader=$(curl -fsSL https://meta.fabricmc.net/v2/versions/loader/${Version}/ | jq ".[] | select (.loader.stable == true) | .loader.version" | sed -e 's/"//g')
echo "curl -fsSL https://maven.fabricmc.net/net/fabricmc/fabric-installer/maven-metadata.xml"
Installer=$(curl -fsSL https://maven.fabricmc.net/net/fabricmc/fabric-installer/maven-metadata.xml | grep release | sed -E 's/.*<release>|<\/release>//g')
FabricJar="https://meta.fabricmc.net/v2/versions/loader/${Version}/${Loader}/${Installer}/server/jar"
echo "##################################################"
echo "############### --- Fabric DownLoad URL ---"
echo "############### Version   : ${Version}"
echo "############### Loader    : ${Loader}"
echo "############### Installer : ${Installer}"
echo "############### FabricJar : ${FabricJar}"
echo "##################################################"
echo "curl -fsSL -o \"./fabric-server.jar\" ${FabricJar}"
curl -fsSL -o "./fabric-server_${Version}.jar" ${FabricJar}

# 結果を表示
chmod 755 /mcData/*
chown -R ${UserID}:${GroupID} /mcData/*
ls -lah /mcData
