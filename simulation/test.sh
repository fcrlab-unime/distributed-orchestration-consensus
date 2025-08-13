#!/bin/bash

#$1: number of requests per second
#$2: number of gateways

#START THE DISTRIBUTED SYSTEM
#echo "Starting the distributed system..."

sh ./launch.sh $2

ORCHESTRATION_IPS=()

for i in $(seq 1 $2); do
  # Nome del container orchestration generato da Compose
  CONTAINER_NAME="instance_${i}-orchestration-1"

  # Prende l'IP sulla rete gluster_net
  IP=$(docker inspect -f '{{.NetworkSettings.Networks.gluster_net.IPAddress}}' $CONTAINER_NAME)

  # Aggiunge l'IP alla lista
  ORCHESTRATION_IPS+=("$IP")
done

# Stampa la lista completa
echo "Lista IP dei container orchestration:"
printf '%s\n' "${ORCHESTRATION_IPS[@]}"

sleep 10

#EXECUTE THE TESTS
echo "Executing the tests..."
#go run ./client.go -f $1 "${ORCHESTRATION_IPS[@]}"
docker run --network gluster_net client-go -f $1 "${ORCHESTRATION_IPS[@]}"

echo "\n-------------------------------------------"
