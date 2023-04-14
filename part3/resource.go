package raft

import (
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"
	"time"
)

func getCPU() (total float64, idle float64) {
	contents, err := ioutil.ReadFile("/proc/stat")
	if err != nil {
		return 
	}
	lines := strings.Split(string(contents), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if fields[0] == "cpu" {
			numFields := len(fields)
			for i := 1; i < numFields; i++ {
				val, err := strconv.ParseFloat(fields[i], 64)
				if err != nil {
					fmt.Printf("error parsing cpu stat: %v\n", err)
				}
				total += val
				if i == 4 {
					idle = val
				}
			}
			return
		}
	}
	return
}

func getCPUPercent() float64 {
	total1, idle1 := getCPU()
	time.Sleep(200 * time.Millisecond)
	total2, idle2 := getCPU()
	return 100 * (1 - ((idle2 - idle1)) / (total2 - total1))
}

func getMem() (total float64, free float64) {
	contents, err := ioutil.ReadFile("/proc/meminfo")
	if err != nil {
		return 
	}
	lines := strings.Split(string(contents), "\n")
	for _, line := range lines {
		fields := strings.Split(line, ":")
		if len(fields) != 2 {
			continue
		}
		fields[1] = strings.Replace(fields[1], "kB", "", -1)
		fields[1] = strings.TrimSpace(fields[1])
		if fields[0] == "MemTotal" {
			total, err = strconv.ParseFloat(fields[1], 64)
			if err != nil {
				fmt.Printf("error parsing mem stat: %v\n", err)
			}
		} else if fields[0] == "MemAvailable" {
			free, err = strconv.ParseFloat(fields[1], 64)
			if err != nil {
				fmt.Printf("error parsing mem stat: %v\n", err)
			}
		} else {
			continue
		}
	}
	return
}

func getMemPercent() float64 {
	total, free := getMem()
	return 100 * (1 - free / total)
}

func getResources() (cpu float64, mem float64) {
	return getCPUPercent(), getMemPercent()
}

func WeightedSum(a float64, b float64, weightA float64, weightB float64) float64 {
	return (a * weightA + b * weightB) / (weightA + weightB)
}

func getLoadLevel() int {
	loadLevel := 0
	cpu, mem := getResources()
	sum := int(WeightedSum(cpu, mem, 0.5, 0.5) / 10)
	if sum < 1 {
		loadLevel = 1
	} else if sum > 10 {
		loadLevel = 10
	} else {
		loadLevel = sum
	}

	return loadLevel
}

//func main () {
//	fmt.Printf("Urgency: %d\n", getUrgency())
//}