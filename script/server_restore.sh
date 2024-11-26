#! /bin/bash
cd $(dirname "$0")
readonly SCRIPTPATH="${PWD}"

# ${1} SERVER_DIR
# ${2} BACKUP_DIR
# ${3} TARGET_TIMESTAMP
# ${4} WEBHOOK_URL   *Non Required
# ex. ./server_restore.sh "/home/aatomu/server/example" "/home/aatomu/backup/example" "20231225_212304" "https://...."

# Path
readonly SERVER_DIR="${1}"
readonly BACKUP_DIR="${2}"
readonly TARGET_TIMESTAMP="${3}"
readonly RSYNC_SOURCE="${BACKUP_DIR}/${TARGET_TIMESTAMP}/"
readonly RSYNC_DEST="${SERVER_DIR}/"
# Arg
readonly WEBHOOK_URL="${4}"
# Color
readonly INFO_COLOR="49151"      #Hex: #00BFFF
readonly FAILED_COLOR="14423100" #Hex: #DC143C
# Exit code
readonly SUCCESS=0
readonly ERROR=1

# Arg check
if [ "${TARGET_TIMESTAMP}" == "" ]; then
  echo "[ERROR]: Ivaild timestamp value: \"${TARGET_TIMESTAMP}\""
  exit ${ERROR}
fi
if [ ! -d "${RSYNC_SOURCE}" ]; then
  echo "[ERROR]: This timestamp not found: \"${TARGET_TIMESTAMP}\""
  echo "[ERROR]: Backuped timestamp list"
  cd ${BACKUP_DIR}
  ls -d */
  curl -X POST -H 'Content-Type:application/json' -d "{\"embeds\":[{\"author\":{\"name\":\"Backup timestamp:\\\"${TARGET_TIMESTAMP}\\\" not found\"},\"description\":\"**Backuped timestamps**\n$(ls -d */)\",\"color\":\"${FAILED_COLOR}\"}]}" "${WEBHOOK_URL}"
  exit ${ERROR}
fi

# View settings
echo "[INFO]: Restore rsync source           : \`${RSYNC_SOURCE}\`"
echo "[INFO]: Restore rsync dest             : \`${RSYNC_DEST}\`"
echo "[INFO]: Restore rsync command          : \`rsync -avhP --delete \"${RSYNC_SOURCE}\" \"${RSYNC_DEST}\"\`"

# Start log
echo "[INFO]: Restore rsync start"
curl -X POST -H 'Content-Type:application/json' -d "{\"embeds\":[{\"author\":{\"name\":\"Restore rsync starting...\"},\"color\":\"${INFO_COLOR}\"}]}" "${WEBHOOK_URL}"

# Rsync
rsync -avhP --delete "${RSYNC_SOURCE}" "${RSYNC_DEST}"

# End log(failed)
if [ "$?" != "0" ]; then
  echo "[ERROR]: Rsync failed"
  curl -X POST -H 'Content-Type:application/json' -d "{\"embeds\":[{\"author\":{\"name\":\"Restore rsync failed\"},\"color\":\"${FAILED_COLOR}\"}]}" "${WEBHOOK_URL}"
  exit ${ERROR}
fi

# End log(success)
echo "[INFO]: Rsync finish"
curl -X POST -H 'Content-Type:application/json' -d "{\"embeds\":[{\"author\":{\"name\":\"Restore rsync finish\"},\"color\":\"${INFO_COLOR}\"}]}" "${WEBHOOK_URL}"
exit ${SUCCESS}
