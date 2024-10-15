package main

import (
	"fmt"
	ng "namesgenerator"
	"net"
	"os"
	s "server"
	st "storage"
	"strings"
	"sync"
	"test"
	"time"

	"golang.org/x/exp/slices"
	yaml "gopkg.in/yaml.v3"
)

func main() {
	fmt.Println("Orchestration module started.")
	waitStart(startServer())
}

func startServer() *s.Server {
	// Creates a new server and other network info.
	ready := make(chan interface{})
	storage := st.NewMapStorage()
	commitChannel := make(chan s.CommitEntry)
	serverIp, subnetMask := s.GetNetworkInfo()
	if os.Getenv("DEBUG") == "1" {
		fmt.Println("Network Info:")
		fmt.Println("Server IP:" + serverIp.String())
		fmt.Println("Subnet Mask:" + subnetMask)
	}
	serverId := s.GetServerIdFromIp(serverIp, subnetMask)
	defaultGateway := s.GetDefaultGateway()

	// Gets all peers in the cluster.
	peersAddrs := s.GetPeersIp(serverIp, subnetMask, nil, false)
	peersIds := []int{}
	peers := make(map[int]net.Addr)

	// Assigns ids to peers.
	for p := 0; p < len(peersAddrs); p++ {
		if peersAddrs[p] != serverIp && peersAddrs[p].String() != defaultGateway.String() {
			id := s.GetServerIdFromIp(peersAddrs[p], subnetMask)
			peers[id] = peersAddrs[p]
			peersIds = append(peersIds, id)
		}
	}

	// Creates the server.
	server := s.NewServer(serverId, storage, ready, commitChannel)

	wg := sync.WaitGroup{}
	wg.Add(1)

	// Wraps the server in a RPC server.
	go server.Serve(serverIp, &wg, ready)
	<-ready

	// Connect to all the detected peers.
	for id, addr := range peers {
		error := server.ConnectToPeer(id, addr)
		fmt.Printf("Result: %v\n", error)
		if error != nil {
			server.DisconnectPeer(id)
			delete(peers, id)
		}
	}

	time.Sleep(1 * time.Second)
	close(ready)
	wg.Wait()

	// Starts monitoring the workload.
	go server.GetConsensusModule().MonitorLoad()
	// Starts checking for new peers.
	go s.CheckNewPeers(server, &peers)

	return server
}

func waitStart(server *s.Server) {
	// Create a listening socket
	listener, err := net.Listen("tcp", ":"+os.Getenv("GATEWAY_PORT"))
	if err != nil {
		panic(err)
	}
	defer listener.Close()

	// START CPU MONITORING
	if os.Getenv("TIME") == "1" {
		server.GetConsensusModule().CPUChan <- struct{}{}
	}

	// Counter of the accepted connections
	index := 1

	for {
		conn, err := listener.Accept()
		if err != nil {
			panic(err)
		}
		if os.Getenv("TIME") == "1" {
			server.Times[index] = test.NewTimesStruct(server.GetId())
			server.Times[index].SetStartTime("RE")
			go handleConnection(conn, server, index)
			index++
		} else {
			go handleConnection(conn, server)
		}
	}
	/* for {
		conn, err := listener.Accept()
		if err != nil {
			panic(err)
		}
		go handleConnection(conn, server)
	} */
}

func handleConnection(conn net.Conn, server *s.Server, index ...int) {
	defer conn.Close()
	buf := make([]byte, 4096)
	n, err := conn.Read(buf[0:])

	if err != nil {
		return
	}

	/* parseTime := time.Now() */
	// Parses the request
	services, err := parseMessage(string(buf[0:n]))
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}
	/* parseDuration := time.Since(parseTime)

	f, err := os.OpenFile("/log/requestElab.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		panic(err)
	}
	f.WriteString(fmt.Sprintf("%v,%v\n", parseTime, parseDuration))
	f.Close() */

	for _, service := range services {
		// Creates different instances for each request
		command := s.NewService(service, server)
		server.AddService(command)
		if err == nil {
			if os.Getenv("TIME") == "1" {
				server.Times[index[0]].SetDurationAndWrite(index[0], "RE", server.GetConsensusModule().StartTime)
				server.Submit(command, index[0])
			} else {
				server.Submit(command)
			}
			/* server.Submit(command) */
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
	for _, v := range parseYml["services"].(map[string]interface{}) {
		service := map[string]interface{}{
			"services": map[string]interface{}{ng.GetRandomName(1): v},
			"version":  parseYml["version"],
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
		servicesList = append(servicesList, "ServiceType: "+parseYml["ServiceType"].(string)+"\n\n"+string(yml))
	}

	return servicesList, nil
}
