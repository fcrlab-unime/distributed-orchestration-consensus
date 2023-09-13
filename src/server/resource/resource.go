package load

import (
	"github.com/shirou/gopsutil/cpu"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

func getCPUPercent() float64 {
	perc, err :=  cpu.Percent(8 * time.Millisecond, true)
	sum := 0.0
	for _,core := range perc {
		sum += core
	}
	if err != nil {
		return 0
	}
	return sum / float64(len(perc))
}

func getMem() (total float64, free float64) {
	contents, err := os.ReadFile("/proc/meminfo")
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
	return total, free
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

func GetLoadLevel() (int, float64) {
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

	return loadLevel, cpu
}