#!/bin/bash

#$1: number of requests per second
#$2: number of gateways

HOSTS_FILE="hosts.txt"

#START THE DISTRIBUTED SYSTEM
echo "Starting the distributed system..."
UP_COMMAND="cd /home/pi/distributed-orchestration-consensus && docker compose up -d"
SLEEP_TIME=15

while IFS= read -r HOST || [[ -n "$HOST" ]]; do

  USER_HOST=$(echo "$HOST" | cut -d ':' -f 1)
  PORT=$(echo "$HOST" | cut -d ':' -f 2)

  echo "Executing command on $USER_HOST (port $PORT)..."
  { ssh -o StrictHostKeyChecking=no -p "$PORT" "$USER_HOST" "$UP_COMMAND"; } < /dev/null

  if [ $? -eq 0 ]; then
    echo "Command executed successfully on $USER_HOST"
  else
    echo "Failed to execute command on $USER_HOST"
  fi
  echo "-------------------------------------------"
  sleep $SLEEP_TIME
done < "$HOSTS_FILE"

#MOUNT THE DISTRIBUTED FILE SYSTEM
#sh $HOMR/distributed-orchestration-consensus/experiments/gluster.sh

#EXECUTE THE TESTS
echo "Executing the tests..."
IP_ADDRESSES=()
while IFS= read -r HOST || [[ -n "$HOST" ]]; do
  IP_ADDRESS=$(echo "$HOST" | cut -d ':' -f 1 | cut -d '@' -f 2)
  IP_ADDRESSES+=("$IP_ADDRESS")
done < "$HOSTS_FILE"
SELECTED_ADDRESSES=($(shuf -e "${IP_ADDRESSES[@]}" | head -n "$2"))
echo "Selected gateways: ${SELECTED_ADDRESSES[@]}"
go run ../client/client.go -f $1 "${SELECTED_ADDRESSES[@]}"
sleep $SLEEP_TIME

##STOP THE DISTRIBUTED SYSTEM
echo "Stopping the distributed system..."
DOWN_COMMAND="cd /home/pi/distributed-orchestration-consensus && docker compose down"

while IFS= read -r HOST || [[ -n "$HOST" ]]; do

  USER_HOST=$(echo "$HOST" | cut -d ':' -f 1)
  PORT=$(echo "$HOST" | cut -d ':' -f 2)

  echo "Executing command on $USER_HOST (port $PORT)..."
  { ssh -o StrictHostKeyChecking=no -p "$PORT" "$USER_HOST" "$DOWN_COMMAND"; } < /dev/null

  if [ $? -eq 0 ]; then
    echo "Command executed successfully on $USER_HOST"
  else
    echo "Failed to execute command on $USER_HOST"
  fi

  echo "-------------------------------------------"

done < "$HOSTS_FILE"

sleep $SLEEP_TIME

#SAVE THE RESULTS
echo "Saving the results..."

HOST_TO_SAVE=$(head -n 1 "$HOSTS_FILE")
USER_HOST=$(echo "$HOST_TO_SAVE" | cut -d ':' -f 1)
PORT=$(echo "$HOST_TO_SAVE" | cut -d ':' -f 2)

mkdir -p ./results/r$1g$2

SAVE_COMMAND="ssh -o StrictHostKeyChecking=no -p $PORT $USER_HOST 'sudo /home/pi/distributed-orchestration-consensus/experiments/backup_test.sh'"
{ $SAVE_COMMAND; } < /dev/null
if [ $? -eq 0 ]; then
  echo "Results saved successfully on $IP_ADDRESS"
else
  echo "Failed to save results on $IP_ADDRESS"
fi
echo "-------------------------------------------"

sleep $SLEEP_TIME
#DOWNLOAD THE RESULTS
echo "Downloading the results..."
scp -o StrictHostKeyChecking=no -P $PORT $USER_HOST:/home/pi/results/*.txt ./results/r$1g$2/.
scp -o StrictHostKeyChecking=no -P $PORT $USER_HOST:/home/pi/results/*.csv ./results/r$1g$2/.
scp -r -o StrictHostKeyChecking=no -P $PORT $USER_HOST:/home/pi/results/cpu ./results/r$1g$2/.
echo "Results downloaded successfully from $IP_ADDRESS"
echo "-------------------------------------------"

#CLEAN THE BACKUP
echo "Cleaning the backup..."
CLEAN_COMMAND="ssh -o StrictHostKeyChecking=no -p $PORT $USER_HOST '/home/pi/distributed-orchestration-consensus/experiments/delete_backup.sh'"
{ $CLEAN_COMMAND; } < /dev/null
if [ $? -eq 0 ]; then
  echo "Backup cleaned successfully on $IP_ADDRESS"
else
  echo "Failed to clean backup on $IP_ADDRESS"
fi
echo "-------------------------------------------"

#DELETE VOLUMES
#echo "Deleting volumes..."
#DELETE_COMMAND="docker volume rm distributed-orchestration-consensus_gluster"
#while IFS= read -r HOST || [[ -n "$HOST" ]]; do
#
#  USER_HOST=$(echo "$HOST" | cut -d ':' -f 1)
#  PORT=$(echo "$HOST" | cut -d ':' -f 2)
#
#  echo "Executing command on $USER_HOST (port $PORT)..."
#  { ssh -o StrictHostKeyChecking=no -p "$PORT" "$USER_HOST" "$DELETE_COMMAND"; } < /dev/null
#
#  if [ $? -eq 0 ]; then
#    echo "Command executed successfully on $USER_HOST"
#  else
#    echo "Failed to execute command on $USER_HOST"
#  fi
#
#  echo "-------------------------------------------"
#
#done < "$HOSTS_FILE"