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
	"golang.org/x/exp/slices"
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
	message = strings.TrimSuffix(strings.ReplaceAll(message, "\r", ""), "\n")

	var parseYml map[string]interface{}
	err := yaml.Unmarshal([]byte(message), &parseYml)
	if err != nil {
		return nil, err
	}

	networks := []string{}
	volumes := []string{}
	secrets := []string{}
	
	if _, ok := parseYml["networks"]; ok {
		for k := range parseYml["networks"].(map[string]interface{}) {
		networks = append(networks, k)
		}
	}

	if _, ok := parseYml["volumes"]; ok {
		for k := range parseYml["volumes"].(map[string]interface{}) {
		volumes = append(volumes, k)
		}
	}

	if _, ok := parseYml["secrets"]; ok {
		for k := range parseYml["secrets"].(map[string]interface{}) {
		secrets = append(secrets, k)
		}
	}
	var serviceNetworks []string
	for _, v := range parseYml["services"].(map[string]interface{}) {
		if _, ok := v.(map[string]interface{})["networks"]; ok {
			for _, k := range v.(map[string]interface{})["networks"].([]interface{}) {
				serviceNetworks = append(serviceNetworks, k.(string))
			}
		}

		if _, ok := v.(map[string]interface{})["volumes"]; ok {
			for _, k := range v.(map[string]interface{})["volumes"].([]interface{}) {
				volume := strings.Split(k.(string), ":")
				if !(strings.HasPrefix(volume[0], "/") || strings.HasPrefix(volume[0], ".")) {
					serviceNetworks = append(serviceNetworks, volume[0])
				}
			}
		}

		if _, ok := v.(map[string]interface{})["secrets"]; ok {
			for _, k := range v.(map[string]interface{})["secrets"].([]interface{}) {
				serviceNetworks = append(serviceNetworks, k.(string))
			}
		}
	}

	var servicesList []string
	for k, v := range parseYml["services"].(map[string]interface{}) {
		service := map[string]interface{}{
			"services": map[string]interface{}{k: v},
			"version": parseYml["version"],
		}
		if _, ok := v.(map[string]interface{})["networks"]; ok {
			for _, n := range networks {
				if slices.Contains(serviceNetworks, n) {
					service["networks"] = map[string]interface{}{n: parseYml["networks"].(map[string]interface{})[n]}
				}
			}
		}

		if _, ok := v.(map[string]interface{})["volumes"]; ok {
			for _, v := range volumes {
				if slices.Contains(serviceNetworks, v) {
					service["volumes"] = map[string]interface{}{v: parseYml["volumes"].(map[string]interface{})[v]}
					fmt.Println(service)
				}
			}
		}

		if _, ok := v.(map[string]interface{})["secrets"]; ok {
			for _, s := range secrets {
				if slices.Contains(serviceNetworks, s) {
					service["secrets"] = map[string]interface{}{s: parseYml["secrets"].(map[string]interface{})[s]}
				}
			}
		}
		yml, err := yaml.Marshal(service)
		if err != nil {
			return nil, err
		}
		servicesList = append(servicesList, "ServiceType: " + parseYml["ServiceType"].(string) + "\n\n" + string(yml))
	}


	return servicesList, nil
}
