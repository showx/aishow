package metrics

import (
	"bufio"
	"bytes"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"aishow/internal/models"
)

type Collector struct {
	mu   sync.RWMutex
	snap models.Hardware
}

func Start() *Collector {
	c := &Collector{}
	c.refresh()
	go func() {
		t := time.NewTicker(2 * time.Second)
		defer t.Stop()
		for range t.C {
			c.refresh()
		}
	}()
	return c
}

func (c *Collector) Snapshot() models.Hardware {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := c.snap
	out.GPUs = append([]models.GPUStat(nil), c.snap.GPUs...)
	return out
}

func (c *Collector) refresh() {
	hw := models.Hardware{
		CPUCores: runtime.NumCPU(),
	}
	hw.CPUPercent = sampleCPU()
	hw.RAMUsedGB, hw.RAMTotalGB, hw.RAMPercent = sampleRAM()
	hw.GPUs = sampleGPUs()
	if len(hw.GPUs) > 0 {
		hw.GPUPercent = hw.GPUs[0].Util
		hw.GPUMemUsedMB = hw.GPUs[0].MemUsedMB
		hw.GPUMemTotalMB = hw.GPUs[0].MemTotalMB
		hw.GPUName = hw.GPUs[0].Name
		hw.GPUTemp = hw.GPUs[0].Temp
	}
	c.mu.Lock()
	c.snap = hw
	c.mu.Unlock()
}

func sampleCPU() float64 {
	out, err := exec.Command("powershell", "-NoProfile", "-Command",
		`(Get-CimInstance Win32_Processor | Measure-Object -Property LoadPercentage -Average).Average`,
	).Output()
	if err != nil {
		return 0
	}
	v, _ := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	return v
}

func sampleRAM() (used, total, pct float64) {
	out, err := exec.Command("powershell", "-NoProfile", "-Command",
		`$o=Get-CimInstance Win32_OperatingSystem; '{0} {1}' -f $o.TotalVisibleMemorySize,$o.FreePhysicalMemory`,
	).Output()
	if err != nil {
		return 0, 0, 0
	}
	parts := strings.Fields(strings.TrimSpace(string(out)))
	if len(parts) < 2 {
		return 0, 0, 0
	}
	totKB, _ := strconv.ParseFloat(parts[0], 64)
	freeKB, _ := strconv.ParseFloat(parts[1], 64)
	if totKB <= 0 {
		return 0, 0, 0
	}
	total = totKB / 1024 / 1024
	used = (totKB - freeKB) / 1024 / 1024
	pct = used / total * 100
	return used, total, pct
}

func sampleGPUs() []models.GPUStat {
	out, err := exec.Command("nvidia-smi",
		"--query-gpu=name,utilization.gpu,memory.used,memory.total,temperature.gpu",
		"--format=csv,noheader,nounits",
	).Output()
	if err != nil {
		return nil
	}
	var gpus []models.GPUStat
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		parts := strings.Split(line, ",")
		if len(parts) < 5 {
			continue
		}
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		util, _ := strconv.Atoi(parts[1])
		used, _ := strconv.Atoi(parts[2])
		total, _ := strconv.Atoi(parts[3])
		temp, _ := strconv.Atoi(parts[4])
		gpus = append(gpus, models.GPUStat{
			Name:       parts[0],
			Util:       util,
			MemUsedMB:  used,
			MemTotalMB: total,
			Temp:       temp,
		})
	}
	return gpus
}
