package main

import (
	"fmt"
	"net"
	s "server"
	st "storage"
	"strconv"
	"strings"
	"time"
)

func main(){
	waitSubmit(startServer())
}

func startServer() *s.Server {
	ready := make(chan interface{})
	storage := st.NewMapStorage()
	commitChannel := make(chan s.CommitEntry)
	serverIp, subnetMask := s.GetNetworkInfo()
	serverId := s.GetServerIdFromIp()
	defaultGateway := s.GetDefaultGateway()

	peersAddrs := s.GetPeersIp(serverIp, subnetMask)
	peersIds := []int{}
	peers := make(map[int]net.Addr)

	// Create all Servers in this cluster, assign ids and peer ids.
	for p := 0; p < len(peersAddrs); p++ {
		if peersAddrs[p] != serverIp && peersAddrs[p].String() != defaultGateway.String() {
			id, _ := strconv.Atoi(strings.Split(peersAddrs[p].String(), ".")[3])
			peers[id] = peersAddrs[p]
			peersIds = append(peersIds, id)
		}
	}
	
	server := s.NewServer(serverId, storage, ready, commitChannel)
	server.Serve(serverIp)
	// Connect all peers to each other.
	time.Sleep(1 * time.Second)
	
	for id, addr := range peers {
		error := server.ConnectToPeer(id, addr)
		fmt.Printf("Result: %v\n", error)
		if error != nil {
			server.DisconnectPeer(id)
			delete(peers, id)
		}
	}
	close(ready)

	go s.CheckNewPeers(server, &peers)

	return server
}


func waitSubmit(server *s.Server) {
	listener, err := net.Listen("tcp", ":9093")
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
		var mess string
		n, err := conn.Read(buf[0:])
	
		if err != nil {
			fmt.Printf("\n\nError: %v\n\n", err)
			return
		}
		if	n > 0 {
			//TODO: da modificare
			mess = strings.TrimSuffix(strings.ReplaceAll(string(buf[0:n]), "\r", ""), "\n")
			fmt.Printf("Received: %v, and error is %v\n", mess, err)
			command := s.NewService(mess, server)
			fmt.Printf("Received: %v, and error is %v\n", command.ServiceID, err)
			if err == nil {
				server.Submit(command)
			}
		}
	}
}
