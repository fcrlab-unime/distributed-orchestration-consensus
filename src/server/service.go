package server

import (
	"crypto/sha512"
	"fmt"
	"os"
	"strings"
	"time"
)

type SType string

type Service struct {
	// Unique ID of service
	ServiceID		string
	// Type of service
	Type 			SType

}

func NewService(command string, server *Server) *Service {
	
	service := &Service{}
	
	serviceMap := parseService(command)
	service.ServiceID = fmt.Sprintf("%x", sha512.Sum512([]byte(serviceMap["Command"] + time.Now().String())))
	if err := service.saveToFile(serviceMap["Command"]); err != nil {
		fmt.Printf("Error: %v\n", err)
	}
	service.Type = SType(serviceMap["Type"])

	return service
}

func parseService(command string) map[string]string {
	
	/* 	The first two lines of the command must be as follows:
		1. ServiceType: <Docker|Kubernetes>
	 	2. 

		Then, the rest of the command is the actual body of the command.
		...
	*/
	splitCommand := strings.SplitN(command, "\n\n", 2)
	Type, Command := strings.ReplaceAll(splitCommand[0], "\n", ""), splitCommand[1]

	service := make(map[string]string)
	service["Type"] = strings.Split(Type, ": ")[1]
	service["Command"] = Command
	return service
}

func (s *Service) saveToFile(command string) error {

	if _, err := os.Stat("services"); os.IsNotExist(err) {
		os.Mkdir("services", 0700)
	}

	return os.WriteFile("services/" + s.ServiceID[0:64], []byte(command), 0700)

}