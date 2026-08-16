package docker

import (
	"crypto/rand"
	"encoding/json"
	"time"
)

func cryptoRandRead(b []byte) (int, error) { return rand.Read(b) }

type ContainerStats struct {
	CPUCores float64   `json:"cpu_cores"`
	MemBytes float64   `json:"mem_bytes"`
	MemLimit int64     `json:"mem_limit"`
	Time     time.Time `json:"time"`
}

type rawStats struct {
	CPUStats struct {
		CPUUsage struct {
			TotalUsage uint64 `json:"total_usage"`
		} `json:"cpu_usage"`
		SystemUsage uint64 `json:"system_cpu_usage"`
		OnlineCPUs  uint64 `json:"online_cpus"`
	} `json:"cpu_stats"`
	PreCPUStats struct {
		CPUUsage struct {
			TotalUsage uint64 `json:"total_usage"`
		} `json:"cpu_usage"`
		SystemUsage uint64 `json:"system_cpu_usage"`
	} `json:"precpu_stats"`
	MemoryStats struct {
		Usage uint64 `json:"usage"`
	} `json:"memory_stats"`
}

func parseStatsJSON(b []byte) *ContainerStats {
	var s rawStats
	_ = json.Unmarshal(b, &s)
	cs := &ContainerStats{Time: time.Now()}
	cs.MemBytes = float64(s.MemoryStats.Usage)
	cpuDelta := float64(s.CPUStats.CPUUsage.TotalUsage - s.PreCPUStats.CPUUsage.TotalUsage)
	sysDelta := float64(s.CPUStats.SystemUsage - s.PreCPUStats.SystemUsage)
	ncpu := s.CPUStats.OnlineCPUs
	if ncpu == 0 {
		ncpu = 1
	}
	if sysDelta > 0 {
		cs.CPUCores = cpuDelta / sysDelta * float64(ncpu)
	}
	return cs
}
