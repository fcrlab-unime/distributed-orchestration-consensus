FROM golang:1.22

WORKDIR /app

# Copia il codice sorgente
COPY client.go ./
COPY test.yaml ./

# Compila senza moduli
RUN GO111MODULE=off go build -o client client.go

# Comando di default
ENTRYPOINT ["./client"]
