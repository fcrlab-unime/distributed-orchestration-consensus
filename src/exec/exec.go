package exec

import (
	"os/exec"
	"fmt"
)

func Exec(service string) {
	exec.Command("docker-compose", "-f", "/home/raft/services/" + service, "up", "-d").Start()
	fmt.Printf("Eseguito %s\n", service)
}