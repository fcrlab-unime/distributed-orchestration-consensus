#!/bin/bash

# Numero di repliche
REPLICAS=${1:-3}
NETWORK_NAME="cluster_net"

docker network rm -f "$NETWORK_NAME" 2>/dev/null || true
docker network create \
  --driver bridge \
  --subnet 192.168.100.0/25 \
  "$NETWORK_NAME"

# Loop per avviare n istanze
for i in $(seq 1 $REPLICAS); do
  echo "🔹 Avvio istanza $i..."

  # Crea directory dati per volumi isolati
  mkdir -p "./data/gluster-$i" "./data/test-$i"

  # Genera un file docker-compose specifico per l'istanza
  COMPOSE_FILE="docker-compose.$i.yaml"
  sed "s/__IDX__/$i/g" docker-compose.template.yaml > $COMPOSE_FILE

  # Avvia la compose isolata su network condivisa
  docker compose -p instance_$i -f $COMPOSE_FILE up -d
  sleep 10
done
