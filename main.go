package main

import (
	"fmt"
	"net"
	"os"
	s "server"
	st "storage"
	"strings"
	"sync"
	"time"
	yaml "gopkg.in/yaml.v3"
)

func main(){
	waitSubmit(startServer())
}

func startServer() *s.Server {
	ready := make(chan interface{})
	storage := st.NewMapStorage()
	commitChannel := make(chan s.CommitEntry)
	serverIp, subnetMask := s.GetNetworkInfo()
	serverId := s.GetServerIdFromIp(serverIp, subnetMask)
	defaultGateway := s.GetDefaultGateway()

	peersAddrs := s.GetPeersIp(serverIp, subnetMask, nil, nil, false)
	peersIds := []int{}
	peers := make(map[int]net.Addr)

	// Create all Servers in this cluster, assign ids and peer ids.
	for p := 0; p < len(peersAddrs); p++ {
		if peersAddrs[p] != serverIp && peersAddrs[p].String() != defaultGateway.String() {
			id := s.GetServerIdFromIp(peersAddrs[p], subnetMask)
			peers[id] = peersAddrs[p]
			peersIds = append(peersIds, id)
		}
	}
	
	server := s.NewServer(serverId, storage, ready, commitChannel)
	
	wg := sync.WaitGroup{}
	wg.Add(1)
	go server.Serve(serverIp, &wg, ready)
	<-ready

	for id, addr := range peers {
		error := server.ConnectToPeer(id, addr)
		fmt.Printf("Result: %v\n", error)
		if error != nil {
			server.DisconnectPeer(id)
			delete(peers, id)
		}
	}
	// Connect all peers to each other.
	time.Sleep(1 * time.Second)
	close(ready)
	wg.Wait()

	go s.CheckNewPeers(server, &peers)

	return server
}


func waitSubmit(server *s.Server) {
	listener, err := net.Listen("tcp", ":" + os.Getenv("GATEWAY_PORT"))
	if err != nil {
		panic(err)
	}
	defer listener.Close()
	for {
		conn, err := listener.Accept()
		if err != nil {
			panic(err)
		}
		go handleConnection(conn, server)
	}
}

func handleConnection(conn net.Conn, server *s.Server) {
	defer conn.Close()
	for {
		buf := make([]byte, 4096) 
		n, err := conn.Read(buf[0:])
	
		if err != nil {
			return
		}

		services, err := parseMessage(string(buf[0:n]))
		if err != nil {
			fmt.Printf("Error: %v\n", err)
		}
		//TODO: da modificare
		for _, service := range services {
			command := s.NewService(service, server)
			if err == nil {
				server.Submit(command)
			}
		}
	}
}

func parseMessage(message string) ([]string, error) {
	var parsedMessage map[string]interface{}
	message = strings.TrimSuffix(strings.ReplaceAll(message, "\r", ""), "\n")

	err := yaml.Unmarshal([]byte(message), &parsedMessage)
	if err != nil {
		return nil, err
	}
	servicesList := []string{}
	for key, service := range parsedMessage["services"].(map[string]interface{}) {
		tmpService := make(map[string]interface{})
		for key, field := range parsedMessage {
			if key != "services" {
				tmpService[key] = field
			}
		}
		tmpService["services"] = map[string]interface{}{key: service}
		serviceYaml, err := yaml.Marshal(tmpService)
		if err != nil {
			return nil, err
		}
		servicesList = append(servicesList, string(serviceYaml))
	}
	
	return servicesList, nil
}
