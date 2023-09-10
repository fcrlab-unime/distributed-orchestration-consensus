package main

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"sync"
	"time"
)

func main() {
	dest := []string{}
	if len(os.Args) == 1 {
		dest = append(dest, "172.16.5.2", "172.16.5.80")//, "172.16.5.79")
	} else {
		dest = append(dest, os.Args[1:]...)
	}	
	
	b := 0
	c := 0
	mu := sync.Mutex{}
	i := 0
	timer := time.NewTimer(1 * time.Second)
	for b == 0 {
		select {
			case <-timer.C:
				b++
			case <-time.After(800 * time.Millisecond):
				i = (i + 1) % len(dest)
				mu.Lock()
				c++
				mu.Unlock()
				go func() {
					conn, err := net.Dial("tcp", dest[i] + ":9093")
					if err != nil {
						return
					}
					defer conn.Close()
					aaaaa := strconv.Itoa(c)
					var message string = 
					"ServiceType: Docker\n" +
					"\n" +
					"version: '3.5'\n" +
					"\n" +
					"services:\n" +
					"  raft"+ aaaaa +":\n" +
					"    image: hello-world\n" +
					"\n" +
					"networks:\n" +
					"  raft:\n" +
					"    external: true\n"
				
					_, err = conn.Write([]byte(message))
					if err != nil {
						return
						//panic(err)
					}
					fmt.Printf("%s",dest[i])
				}()
				time.Sleep(50 * time.Millisecond)
		}
	}
	fmt.Printf("\nSent %d messages", c)
}
