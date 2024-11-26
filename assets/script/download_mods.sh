cd /mods
if [ "${Token}" != "" ]
 then
  Token="-H \"Authorization:token ${Token}\""
fi

Range=$(echo ${Version} | sed -E 's/\.[0-9]+$/.x/')
echo "curl -fsSL ${Token} https://api.github.com/repos/"
# https://docs.github.com/ja/rest/releases/releases?apiVersion=2022-11-28#list-releases
Carpet=$(curl -fsSL ${Token} https://api.github.com/repos/gnembon/fabric-carpet/releases?per_page=100 | jq ".[].assets[] | select(.name | contains(\"${Version}\") or contains(\"${Range}\")) | select(.name | contains(\"dev\") or contains(\"source\") | not) | .browser_download_url" | head -n 1 | sed -e 's/"//g')
# CarpetExtra=$(curl -fsSL ${Token} https://api.github.com/repos/gnembon/carpet-extra/releases?per_page=100 | jq ".[].assets[] | select(.name | contains(\"${Version}\") or contains(\"${Range}\")) | select(.name | contains(\"dev\") or contains(\"source\") | not) | .browser_download_url" | head -n 1 | sed -e 's/"//g')
CarpetAddition=$(curl -fsSL ${Token} https://api.github.com/repos/TISUnion/Carpet-TIS-Addition/releases?per_page=100 | jq ".[].assets[] | select(.name | contains(\"${Version}\") or contains(\"${Range}\")) | select(.name | contains(\"dev\") or contains(\"source\") | not) | .browser_download_url" | head -n 1 | sed -e 's/"//g')
Lithium=$(curl -fsSL ${Token} https://api.github.com/repos/CaffeineMC/lithium-fabric/releases?per_page=100 | jq ".[].assets[] | select(.name | contains(\"${Version}\") or contains(\"${Range}\")) | select(.name | contains(\"dev\") or contains(\"source\") or contains(\"api\") | not) | .browser_download_url" | head -n 1 | sed -e 's/"//g')
Phosphor=$(curl -fsSL ${Token} https://api.github.com/repos/CaffeineMC/phosphor-fabric/releases?per_page=100 | jq ".[].assets[] | select(.name | contains(\"${Version}\") or contains(\"${Range}\")) | select(.name | contains(\"dev\") or contains(\"source\") | not) | .browser_download_url" | head -n 1 | sed -e 's/"//g')
Syncmatica=$(curl -fsSL ${Token} https://api.github.com/repos/End-Tech/syncmatica/releases?per_page=100 | jq ".[].assets[] | select(.name | contains(\"${Version}\") or contains(\"${Range}\")) | select(.name | contains(\"dev\") or contains(\"source\") | not) | .browser_download_url" | head -n 1 | sed -e 's/"//g')
# litematicasp=$(curl -fsSL ${Token} https://api.github.com/repos/Fallen-Breath/litematica-server-paster/releases?per_page=100 | jq ".[].assets[] | select(.name | contains(\"${Version}\") or contains(\"${Range}\")) | select(.name | contains(\"dev\") or contains(\"source\") | not) | .browser_download_url" | head -n 1 | sed -e 's/"//g')
Servux=$(curl -fsSL ${Token} https://api.github.com/repos/sakura-ryoko/servux/releases?per_page=100 | jq ".[].assets[] | select(.name | contains(\"${Version}\") or contains(\"${Range}\")) | select(.name | contains(\"dev\") or contains(\"source\") | not) | .browser_download_url" | head -n 1 | sed -e 's/"//g')
# https://github.com/sakura-ryoko/syncmatica/releases
#WorldEdit=$(curl -fsSL 'https://builds.enginehub.org/job/worldedit/last-successful?branch=version/7.2.x' | grep -oiE 'https://ci.enginehub.org/repository/download/[/:0-9a-z]+/worldedit-fabric-mc([0-9\.\]+)-[0-9\.\]+-SNAPSHOT-dist.jar\?branch=version/7.2.x\&amp;guest=1')
echo "##################################################"
echo "############### --- Fabric Mods DownLoad URL ---"
echo "############### Carpet                  : ${Carpet}"
# echo "############### CarpetExtra             : ${CarpetExtra}"
echo "############### CarpetAddition          : ${CarpetAddition}"
echo "############### Lithium                 : ${Lithium}"
echo "############### Phosphor                : ${Phosphor}"
echo "############### Syncmatica              : ${Syncmatica}"
# echo "############### litematica-server-paster: ${litematicasp}"
echo "############### Servux                  :${Servux}"
#echo "############### WorldEdit      : ${WorldEdit}"
echo "##################################################"
echo "curl -fsSL -o \"???.jar\" https://github.com/???/???/releases/download/???"
curl -fsSL -O ${Carpet}
# curl -fsSL -O ${CarpetExtra}
curl -fsSL -O ${CarpetAddition}
curl -fsSL -O ${Lithium}
curl -fsSL -O ${Phosphor}
curl -fsSL -O ${Syncmatica}
# curl -fsSL -O ${litematicasp}
curl -fsSL -O ${Servux}
#curl -fsSL -o "$(echo ${WorldEdit} | sed -e 's|\?.*||g' -e 's|.*/||g' | nkf --url-input)" ${WorldEdit}

# GithubAPIのデータを表示
Rates=$(curl -fsSL -i -H "Authorization:${Token}" https://api.github.com)
Limit=$(echo "${Rates}" | grep "x-ratelimit-limit" | sed -e "s/.* //g" -e "s/\s//g")
Remaining=$(echo "${Rates}" | grep "x-ratelimit-remaining" | sed -e "s/.* //g" -e "s/\s//g")
Used=$(echo "${Rates}" | grep "x-ratelimit-used" | sed -e "s/.* //g" -e "s/\s//g")
Reset=$(echo "${Rates}" | grep "x-ratelimit-reset" | sed -e "s/.* //g" -e "s/\s//g")
Reset=$(( ${Reset} + 32400))
Reset=$(date --date="@${Reset}" +"%Y-%m-%d %H:%M:%S")
echo "##################################################"
echo "############### --- GitHub API Infomation ---"
echo "############### Used      : ${Used}/${Limit}"
echo "############### Remaining : ${Remaining}"
echo "############### Reset     : ${Reset}"
echo "##################################################"

# 結果を表示 & 権限を変更
chmod 755 /mods/*
chown -R ${UserID}:${GroupID} /mods/*
ls -lah /mods
