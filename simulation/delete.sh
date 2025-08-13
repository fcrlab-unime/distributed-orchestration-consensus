#!/bin/bash

# Numero di repliche (default: 3)
REPLICAS=${1:-3}
NETWORK_NAME="cluster_net"

echo "🔹 Arresto e rimozione delle istanze..."
for i in $(seq 1 $REPLICAS); do
  echo "  ➜ Instance $i"

  COMPOSE_FILE="docker-compose.$i.yaml"
  
  # Se esiste il file compose, chiudi e rimuovi
  if [ -f "$COMPOSE_FILE" ]; then
    docker compose -p instance_$i -f "$COMPOSE_FILE" down --volumes --remove-orphans
    rm -f "$COMPOSE_FILE"
  fi

  # Rimuovi le directory dati
  rm -rf "./data/gluster-$i" "./data/test-$i"
done

echo "🔹 Rimozione rete Docker..."
docker network rm "$NETWORK_NAME" 2>/dev/null || true

echo "✅ Tutto eliminato."
