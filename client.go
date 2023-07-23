package main

import (
	"math/rand"
	"fmt"
	"net"
	"os"
	"time"
)

func main() {
	dest := []string{}
	if len(os.Args) == 1 {
		dest = append(dest, "192.168.0.2", "192.168.0.3", "192.168.0.4")
	} else {
		dest = append(dest, os.Args[1:]...)
	}	
	
	timer := time.NewTimer(60 * time.Second)
	b := 0
	for b == 0 {
		select {
			case <-timer.C:
				b++
			default:
				i := rand.Intn(len(dest))
				go func() {
					conn, err := net.Dial("tcp", dest[i] + ":9093")
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
				
					fmt.Printf("Sent: %v\nto %s", message, dest[i])
				}()
				time.Sleep(75 * time.Millisecond)
		}
	}
}
