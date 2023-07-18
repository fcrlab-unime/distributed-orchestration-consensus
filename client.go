package main

import (
	"os"
	"fmt"
	"net"
)

func main() {
	ip, port := os.Args[1], os.Args[2]

	fmt.Printf("IP: %v\nPort: %v\n", ip, port)

	conn, err := net.Dial("tcp", ip + ":" + port)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	message := 
	"ServiceType: Kubernetes\n" +
	"\n" +
	"prova"

	_, err = conn.Write([]byte(message))
	if err != nil {
		panic(err)
	}

	fmt.Printf("Sent: %v\n", message)

}
