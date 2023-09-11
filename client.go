package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"time"
)

func main() {
	reqN := 1
	flag.IntVar(&reqN, "f", 1, "Number of requests to send")
	flag.Parse()

	i, c := 0, 0
	timer := time.NewTimer(3 * time.Second)
	for {
		select {
			case <-time.After(time.Duration(500 / reqN - 50) * time.Millisecond):
				i = (i + 1) % len(flag.Args())
				conn, err := net.Dial("tcp", flag.Arg(i) + ":9093")
				if err != nil {
					return
				}
				defer conn.Close()
				message, _ := os.ReadFile("test.yaml")
				conn.Write(message)
				fmt.Printf("Sent to %s\n",flag.Arg(i))
				c++
				time.Sleep(50 * time.Millisecond)
			case <-timer.C:
				fmt.Printf("\nSent %d messages", c)
				return
		}
	}
}
