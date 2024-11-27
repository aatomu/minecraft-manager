#!/bin/bash

# 変数
readonly API_TOKEN="********"
readonly DOMAIN="example.com" # Target Domain/Record
readonly SUB_DOMAIN="minecraft"
readonly GLOBAL_IP=$(curl -Ss ipinfo.io/ip)

# Authentication
# Cloudflare API https://api.cloudflare.com/#zone-list-zones
# Generate @https://dash.cloudflare.com/profile/api-tokens
# トークンを作成
# =>API トークン テンプレート "ゾーン DNS を編集する"
#   => アクセス許可   : ゾーン,DNS,編集
#   => ゾーン リソース: 包含,特定のゾーン,任意のドメイン(下のTopDomainと同じに)
#   => TTL           : Start Date(特に設定なし),EndDate(ドメイン期限or 3,5,10年)
# => 概要に進む
# => トークンを作成
# => 表示されるトークンをAPITokenにコピペ

# Preview Setting
echo "GlobalIP: ${GLOBAL_IP} Target: ${SUB_DOMAIN}.${DOMAIN}"

# Exit if IP address does not change
CurrentARecord=$(dig $SUB_DOMAIN.$DOMAIN a +short)
if [ "${GLOBAL_IP}" = "${CurrentARecord}" ]; then
  echo "IP address does not change"
  exit 0
fi

echo "Wait 3Sec..."
sleep 3

# Get ZoneID
echo "Get ZoneID"
ZoneResult=$(curl -v -X GET "https://api.cloudflare.com/client/v4/zones?name=${DOMAIN}" \
  -H "Authorization: Bearer ${API_TOKEN}" \
  -H "Content-Type: application/json")
if [ $? != 0 ]; then
  echo "Failed Request ZoneID"
  exit 1
fi

ZoneID=$(echo "${ZoneResult}" | sed -e 's/[,{}]/\n/g' -e 's/"//g' | grep "id" | head -n 1 | cut -d ":" -f 2)
if [ "${ZoneID}" = "" ]; then
  echo "Failed Get ZoneID"
  echo "${ZoneResult}"
  exit 1
fi
echo "Got ZoneID: ${ZoneID}"

# Get RecordID/RecordType
echo "Get RecordID/RecordType"
RecordResult=$(curl -X GET "https://api.cloudflare.com/client/v4/zones/${ZoneID}/dns_records/?name=${SUB_DOMAIN}.${DOMAIN}" \
  -H "Authorization: Bearer ${API_TOKEN}" \
  -H "Content-Type:application/json")
if [ $? != 0 ]; then
  echo "Failed Request RecordID"
  exit 1
fi

RecordID=$(echo "${RecordResult}" | sed -e 's/[,{}]/\n/g' -e 's/"//g' | grep "id" | head -n 1 | cut -d ":" -f 2)
RecordType=$(echo "${RecordResult}" | sed -e 's/[,{}]/\n/g' -e 's/"//g' | grep "type" | head -n 1 | cut -d ":" -f 2)
if [ "${RecordID}" = "" ]; then
  echo "Failed Get RecordID/RecordType"
  echo "${RecordResult}"
  exit 1
fi
echo "Got RecordID: ${RecordID} ,RecordType: ${RecordType}"

# Update Record
echo "Update Record"
curl -X PATCH "https://api.cloudflare.com/client/v4/zones/${ZoneID}/dns_records/${RecordID}" \
  -H "Authorization: Bearer ${API_TOKEN}" \
  -H "Content-Type: application/json" \
  --data "{\"type\":\"${RecordType}\", \"name\":\"${SUB_DOMAIN}.${DOMAIN}\", \"content\":\"${GLOBAL_IP}\", \"proxied\":false}"
if [ $? != 0 ]; then
  echo "Failed Request Update Record"
  exit 1
fi
echo "Update Record Success!"
