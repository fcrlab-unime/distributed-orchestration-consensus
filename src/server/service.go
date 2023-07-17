package server

import (
	"fmt"
	"crypto/sha512"
	"strings"
	"time"
)

type SType string

const (
	Docker 		SType = "Docker"
	Kubernetes 	SType = "Kubernetes"
)

type Service struct {
	// Unique ID of service
	ServiceID		string
	// Type of service
	Type 			SType

}

func NewService(command string, server *Server) *Service {
	
	service := &Service{}
	
	serviceMap := parseService(command)
	service.ServiceID = fmt.Sprintf("%x", sha512.Sum512([]byte(command + time.Now().String())))
	service.Type = typeFromString(serviceMap["Type"])

	return service
}

func parseService(command string) map[string]string {
	
	/* 	The first two lines of the command must be as follows:
		1. ServiceType: <Docker|Kubernetes>
	 	2. 

		Then, the rest of the command is the actual body of the command.
		...
	*/
	splitCommand := strings.SplitN(command, "\n", 2)
	fmt.Printf("Command: %v\n", splitCommand)
	Type, Command := strings.ReplaceAll(splitCommand[0], "\n\n", ""), splitCommand[1]

	service := make(map[string]string)
	service["Type"] = strings.Split(Type, ": ")[1]
	service["Command"] = Command
	return service
}

func typeFromString (s string) SType {
	switch s {
		case "Docker":
			return Docker
		case "Kubernetes":
			return Kubernetes
		default:
			panic("Invalid type")
	}
}