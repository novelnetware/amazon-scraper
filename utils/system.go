package utils

import (
	"log"
	"strconv"

	"github.com/shirou/gopsutil/v3/cpu"
)

// GetOptimalWorkerCount determines the number of workers based on config and system resources.
func GetOptimalWorkerCount(configValue string) int {
	// 1. Check for manual override
	if manualWorkers, err := strconv.Atoi(configValue); err == nil && manualWorkers > 0 {
		log.Printf("Using manually configured number of workers: %d", manualWorkers)
		return manualWorkers
	}

	// 2. If set to "auto" or invalid, calculate automatically
	if configValue != "auto" {
		log.Printf("WARN: Invalid workers value '%s'. Defaulting to 'auto' mode.", configValue)
	}

	// Get logical CPU core count
	// We use logical cores (true) because scraping is mostly I/O bound
	// and hyper-threading can be beneficial.
	cpuCores, err := cpu.Counts(true)
	if err != nil {
		// Fallback on error
		log.Printf("WARN: Could not detect CPU cores. Falling back to default: %d workers.", 2)
		return 2
	}

	// A safe formula: half of the available cores.
	// This prevents overwhelming the system with too many browser instances
	// and leaves resources for other tasks.
	optimalCount := cpuCores / 2

	// Ensure at least 1 worker, and cap at a reasonable max like 16.
	if optimalCount < 1 {
		optimalCount = 1
	}
	if optimalCount > 16 {
		optimalCount = 16
	}

	log.Printf("System has %d logical cores. Automatically setting number of workers to: %d", cpuCores, optimalCount)
	return optimalCount
}
