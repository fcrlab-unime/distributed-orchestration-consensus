package server

import (
	"crypto/sha256"
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type SType string

type Service struct {
	// Unique ID of service
	ServiceID string
	// Type of service
	Type SType
}

func NewService(command string, server *Server) Service {

	service := Service{}

	serviceMap := parseService(command)
	service.ServiceID = fmt.Sprintf("%x", sha256.Sum256([]byte(serviceMap["Command"]+time.Now().String())))
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
	parsedCommand := make(map[string]interface{})
	err := yaml.Unmarshal([]byte(command), &parsedCommand)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}
	Type := parsedCommand["ServiceType"].(string)
	delete(parsedCommand, "ServiceType")
	Command, err := yaml.Marshal(parsedCommand)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}

	service := make(map[string]string)
	service["Type"] = Type
	service["Command"] = string(Command)
	return service
}

func (s *Service) saveToFile(command string) error {

	if _, err := os.Stat("services"); os.IsNotExist(err) {
		os.Mkdir("services", 0700)
	}

	return os.WriteFile("services/"+s.ServiceID, []byte(command), 0700)

}
