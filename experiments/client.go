package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"time"
)

func main() {
	fmt.Println("Client module started.")
	reqN := 1.0
	//flag.IntVar(&reqN, "f", 1, "Number of requests per second to send")
	flag.Float64Var(&reqN, "f", 1, "Number of requests per second to send")
	flag.Parse()

	i, c := 0, 0
	timer := time.NewTimer(20 * time.Second)
	message, _ := os.ReadFile(os.Getenv("HOME") + "/distributed-orchestration-consensus/experiments/test.yaml")
	for {
		select {
		case <-time.After(time.Duration(1000/reqN-50) * time.Millisecond):
			i = (i + 1) % len(flag.Args())
			var conn net.Conn
			var err error
			//conn, err := net.Dial("tcp", flag.Arg(i)+":9093")
			for {
				conn, err = net.Dial("tcp", flag.Arg(i)+":9093")
				if err != nil {
					fmt.Println(err)
					continue // Keep retrying on error
				}
				break // Exit loop if no error
			}
			//if err != nil {
			//	fmt.Println(err)
			//	return
			//}
			defer conn.Close()
			conn.Write(message)
			fmt.Printf("Sent to %s\n", flag.Arg(i))
			c++
			time.Sleep(50 * time.Millisecond)
		case <-timer.C:
			fmt.Printf("\nSent %d messages", c)
			return
		}
	}
}
