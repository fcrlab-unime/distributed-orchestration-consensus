#!/bin/bash

#$1: number of requests per second
#$2: number of gateways

HOSTS_FILE="hosts.txt"

mapfile -t HOSTS_LIST < "$HOSTS_FILE"

#START THE DISTRIBUTED SYSTEM
echo "Starting the distributed system..."
UP_COMMAND="cd /home/pi/distributed-orchestration-consensus && docker compose up -d"
SLEEP_TIME=25

#while IFS= read -r HOST || [[ -n "$HOST" ]]; do
for HOST in "${HOSTS_LIST[@]}"; do

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
#done < "$HOSTS_FILE"
done
#MOUNT THE DISTRIBUTED FILE SYSTEM
#sh $HOMR/distributed-orchestration-consensus/experiments/gluster.sh

#EXECUTE THE TESTS
echo "Executing the tests..."
IP_ADDRESSES=()
#while IFS= read -r HOST || [[ -n "$HOST" ]]; do
for HOST in "${HOSTS_LIST[@]}"; do
  IP_ADDRESS=$(echo "$HOST" | cut -d ':' -f 1 | cut -d '@' -f 2)
  IP_ADDRESSES+=("$IP_ADDRESS")
done
SELECTED_ADDRESSES=($(shuf -e "${IP_ADDRESSES[@]}" | head -n "$2"))
echo "Selected gateways: ${SELECTED_ADDRESSES[@]}"
go run ./client.go -f $1 "${SELECTED_ADDRESSES[@]}"
sleep $SLEEP_TIME

echo "\n-------------------------------------------"

##STOP THE DISTRIBUTED SYSTEM
echo "Stopping the distributed system..."
DOWN_COMMAND="cd /home/pi/distributed-orchestration-consensus && docker compose down"

#while IFS= read -r HOST || [[ -n "$HOST" ]]; do
for HOST in "${HOSTS_LIST[@]}"; do
  USER_HOST=$(echo "$HOST" | cut -d ':' -f 1)
  PORT=$(echo "$HOST" | cut -d ':' -f 2)

  echo "Executing command on $USER_HOST (port $PORT)..."
  { ssh -o StrictHostKeyChecking=no -p "$PORT" "$USER_HOST" "$DOWN_COMMAND"; } < /dev/null

  if [ $? -eq 0 ]; then
    echo "Command executed successfully on $USER_HOST"
  else
    echo "Failed to execute command on $USER_HOST"
    exit 1
  fi

  echo "-------------------------------------------"

#done < "$HOSTS_FILE"
done

sleep $SLEEP_TIME

#SAVE THE RESULTS
echo "Saving the results..."

#HOST_TO_SAVE=$(head -n 1 "$HOSTS_FILE")
#USER_HOST=$(echo "$HOST_TO_SAVE" | cut -d ':' -f 1)
#PORT=$(echo "$HOST_TO_SAVE" | cut -d ':' -f 2)

mkdir -p ./results/r$1g$2

#SAVE_COMMAND="ssh -o StrictHostKeyChecking=no -p $PORT $USER_HOST 'sudo sh /home/pi/distributed-orchestration-consensus/experiments/backup_test.sh'"
#{ $SAVE_COMMAND; } < /dev/null
#if [ $? -eq 0 ]; then
#  echo "Results saved successfully on $IP_ADDRESS"
#else
#  echo "Failed to save results on $IP_ADDRESS"
#  exit 1
#fi
#while IFS= read -r HOST || [[ -n "$HOST" ]]; do
for HOST in "${HOSTS_LIST[@]}"; do

  USER_HOST=$(echo "$HOST" | cut -d ':' -f 1)
  PORT=$(echo "$HOST" | cut -d ':' -f 2)

  ssh -o StrictHostKeyChecking=no -p $PORT $USER_HOST 'sudo sh /home/pi/distributed-orchestration-consensus/experiments/backup_test.sh'

  
  #DOWNLOAD THE RESULTS
  echo "Downloading the results from $USER_HOST..."
  #scp -o StrictHostKeyChecking=no -P $PORT $USER_HOST:/home/pi/results/*.txt ./results/r$1g$2/.
  #scp -o StrictHostKeyChecking=no -P $PORT $USER_HOST:/home/pi/results/*.csv ./results/r$1g$2/.
  scp -r -o StrictHostKeyChecking=no -P $PORT $USER_HOST:/home/pi/results/resources ./results/r$1g$2/.
  scp -r -o StrictHostKeyChecking=no -P $PORT $USER_HOST:/home/pi/results/times ./results/r$1g$2/.

  #CLEAN THE BACKUP
  echo "Cleaning the backup on $USER_HOST..."
  ssh -o StrictHostKeyChecking=no -p $PORT $USER_HOST 'sudo sh /home/pi/distributed-orchestration-consensus/experiments/delete_backup.sh'
  echo "Cleaned the backup on $USER_HOST"
#done < "$HOSTS_FILE"
done


#CLEAN_COMMAND="ssh -o StrictHostKeyChecking=no -p $PORT $USER_HOST 'sudo sh/home/pi/distributed-orchestration-consensus/experiments/delete_backup.sh'"
#{ $CLEAN_COMMAND; } < /dev/null
#if [ $? -eq 0 ]; then
#  echo "Backup cleaned successfully on $IP_ADDRESS"
#else
#  echo "Failed to clean backup on $IP_ADDRESS"
#  exit 1
#fi
echo "-------------------------------------------"

#DELETE VOLUMES
echo "Deleting volumes..."
DELETE_COMMAND="docker volume rm distributed-orchestration-consensus_gluster distributed-orchestration-consensus_test"
for HOST in "${HOSTS_LIST[@]}"; do
#while IFS= read -r HOST || [[ -n "$HOST" ]]; do

  USER_HOST=$(echo "$HOST" | cut -d ':' -f 1)
  PORT=$(echo "$HOST" | cut -d ':' -f 2)

  echo "Executing command on $USER_HOST (port $PORT)..."
  { ssh -o StrictHostKeyChecking=no -p "$PORT" "$USER_HOST" "$DELETE_COMMAND"; } < /dev/null

  if [ $? -eq 0 ]; then
    echo "Command executed successfully on $USER_HOST"
  else
    echo "Failed to execute command on $USER_HOST"
    exit 1
  fi

  echo "-------------------------------------------"

#done < "$HOSTS_FILE"
done