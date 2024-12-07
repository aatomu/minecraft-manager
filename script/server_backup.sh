#! /bin/bash
cd $(dirname "$0")

# ${1} SERVER_DIR
# ${2} BACKUP_DIR / REMOTE_BACKUP_DIR
# ${3} RSYNC_ARGS    *Non Required
# ${4} RSYNC_COMMAND *Non Required
# ${5} WEBHOOK_URL   *Non Required
# ex. local ) ./server_backup.sh "/home/aatomu/servers/server" "/home/aatomu/backup/example" "-avhP" "https://...."
# ex. remote) ./server_backup.sh "/home/aatomu/servers/server" "minecraft@localhost:/home/minecraft/backup/example" "-avhP" "https://...."

readonly TIMESTAMP="$(date +%Y%m%d_%H%M%S)"
# Path
readonly SERVER_DIR="${1}"
readonly BACKUP_DIR="${2}"
readonly RSYNC_SOURCE="${SERVER_DIR}/"
readonly RSYNC_DEST="${BACKUP_DIR}/${TIMESTAMP}"
readonly RSYNC_LATEST="$(ls -d ${BACKUP_DIR}/*/ | tail -n 1)"
# Arg
readonly RSYNC_ARGS="-avhP --delete ${3}"
readonly RSYNC_COMMAND="${4}"
readonly WEBHOOK_URL="${5}"
# Color
readonly INFO_COLOR="49151"      #Hex: #00BFFF
readonly FAILED_COLOR="14423100" #Hex: #DC143C
# Exit Code
readonly SUCCESS=0
readonly ERROR=1
# Logging
exec 1> >(tee -a ${BACKUP_DIR}/${TIMESTAMP}.log)
exec 2> >(tee -a ${BACKUP_DIR}/${TIMESTAMP}.log)

# View Settings
echo "[INFO]: Backup rsync source           : \`${RSYNC_SOURCE}\`"
echo "[INFO]: Backup rsync dest             : \`${RSYNC_DEST}\`"
echo "[INFO]: Backup rsync latest(link-dest): \`${RSYNC_LATEST}\`"
echo "[INFO]: Backup rsync args             : \`${RSYNC_ARGS}\`"
echo "[INFO]: Backup rsync command          : \`rsync ${RSYNC_ARGS} \"${RSYNC_SOURCE}\" \"${RSYNC_DEST}\"\`"

# Start Log
echo "[INFO]: Backup rsync start"
curl -X POST -H 'Content-Type:application/json' -d "{\"embeds\":[{\"author\":{\"name\":\"Backup rsync starting...\"},\"color\":\"${INFO_COLOR}\"}]}" "${WEBHOOK_URL}"

# Create Dir
mkdir -p "${BACKUP_DIR}/${SERVER_NAME}"
# Rsync
if [ "${RSYNC_LATEST}" == "" ]; then
  if [ "${RSYNC_COMMAND}" == "" ]; then
    rsync ${RSYNC_ARGS} "${RSYNC_SOURCE}" "${RSYNC_DEST}"
  else
    rsync ${RSYNC_ARGS} -e "${RSYNC_COMMAND}" "${RSYNC_SOURCE}" "${RSYNC_DEST}"
  fi
else
  if [ "${RSYNC_COMMAND}" == "" ]; then
    rsync ${RSYNC_ARGS} --link-dest="${RSYNC_LATEST}" "${RSYNC_SOURCE}" "${RSYNC_DEST}"
  else
    rsync ${RSYNC_ARGS} -e "${RSYNC_COMMAND}" --link-dest="${RSYNC_LATEST}" "${RSYNC_SOURCE}" "${RSYNC_DEST}"
  fi
fi

# End Log(Failed)
if [ "$?" != "0" ]; then
  echo "[ERROR]: Backup rsync failed"
  curl -X POST -H 'Content-Type:application/json' -d "{\"embeds\":[{\"author\":{\"name\":\"Backup rsync failed\"},\"color\":\"${FAILED_COLOR}\"}]}" "${WEBHOOK_URL}"
  exit ${ERROR}
fi

# End Log(Success)
echo "[INFO]: Backup rsync finish"
curl -X POST -H 'Content-Type:application/json' -d "{\"embeds\":[{\"author\":{\"name\":\"Backup rsync success\"},\"color\":\"${INFO_COLOR}\"}]}" "${WEBHOOK_URL}"
exit ${SUCCESS}
