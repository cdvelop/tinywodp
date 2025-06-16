package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// BenchmarkResult stores benchmark results for memory analysis
type BenchmarkResult struct {
	Name        string
	Library     string
	Iterations  int64
	NsPerOp     int64
	BytesPerOp  int64
	AllocsPerOp int64
	Description string
}

// MemoryComparison stores comparison data between implementations
type MemoryComparison struct {
	Standard   BenchmarkResult
	TinyString BenchmarkResult
	Category   string
}

// JSONComparison stores JSON benchmark comparison data
type JSONComparison struct {
	Operation   string // "Marshal" or "Unmarshal"
	BatchSize   int    // 1, 100, 1000, 10000
	IsErrorCase bool
	Standard    BenchmarkResult
	TinyString  BenchmarkResult
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run analyzer.go [binary|memory|json|all]")
		fmt.Println("  binary  - Analyze binary sizes")
		fmt.Println("  memory  - Analyze memory allocations")
		fmt.Println("  json    - Analyze JSON operations")
		fmt.Println("  all     - Run all analyses")
		return
	}

	mode := os.Args[1]

	switch mode {
	case "binary":
		analyzeBinarySizes()
	case "memory":
		analyzeMemoryAllocations()
	case "json":
		analyzeJSONOperations()
	case "all":
		analyzeBinarySizes()
		fmt.Println()
		analyzeMemoryAllocations()
		fmt.Println()
		analyzeJSONOperations()
	default:
		LogError(fmt.Sprintf("Unknown mode: %s", mode))
		return
	}
}

// analyzeBinarySizes analyzes and reports binary size comparisons
func analyzeBinarySizes() {
	LogStep("Analyzing binary sizes with multiple optimization levels...")

	binaries := measureBinarySizes()
	if len(binaries) == 0 {
		LogError("No binaries found to analyze")
		return
	}

	displayBinaryResults(binaries)
	displayOptimizationTable(binaries)
	updateREADMEWithBinaryData(binaries)

	LogSuccess("Binary size analysis completed and README updated")
}

// analyzeMemoryAllocations analyzes and reports memory allocation comparisons
func analyzeMemoryAllocations() {
	LogStep("Starting memory allocation benchmark...")

	// Check if we can run benchmarks
	if !checkGoBenchAvailable() {
		LogError("Cannot run Go benchmarks")
		return
	}

	// Run memory benchmarks
	comparisons := runMemoryBenchmarks()
	if len(comparisons) == 0 {
		LogError("No benchmark results available. Make sure Go benchmarks can run successfully.")
		return
	}

	// Display results
	displayMemoryResults(comparisons)

	// Update README
	updateREADMEWithMemoryData(comparisons)

	LogSuccess("Memory benchmark completed and README updated")
}

// analyzeJSONOperations analyzes and reports JSON operation comparisons
func analyzeJSONOperations() {
	LogStep("Starting JSON operations benchmark...")

	// Check if we can run benchmarks
	if !checkGoBenchAvailable() {
		LogError("Cannot run Go benchmarks")
		return
	}

	// Run JSON benchmarks
	comparisons, err := runJSONBenchmarks()
	if err != nil {
		LogError(fmt.Sprintf("Error running JSON benchmarks: %v", err))
		return
	}

	if len(comparisons) == 0 {
		LogError("No JSON benchmark results available")
		return
	}

	// Display results
	displayJSONResults(comparisons)

	// Update README
	updateREADMEWithJSONData(comparisons)

	LogSuccess("JSON benchmark completed and README updated")
}

// measureBinarySizes scans for and measures all binary files
func measureBinarySizes() []BinaryInfo {
	var allBinaries []BinaryInfo

	binaryDir := "bench-binary-size"
	if !FileExists(binaryDir) {
		LogError(fmt.Sprintf("Binary directory %s not found", binaryDir))
		return nil
	}

	// Define patterns to search for
	patterns := []string{"standard", "tinystring"}

	// Search for binaries
	for _, pattern := range patterns {
		binaries, err := FindBinaries(binaryDir, []string{pattern})
		if err != nil {
			LogError(fmt.Sprintf("Error finding binaries: %v", err))
			continue
		}
		allBinaries = append(allBinaries, binaries...)
	}

	return allBinaries
}

// displayBinaryResults shows binary size results in a table format
func displayBinaryResults(binaries []BinaryInfo) {
	fmt.Println("\n📊 Binary Size Results:")
	fmt.Println("========================")
	fmt.Printf("%-20s %-8s %-12s %-10s\n", "File", "Type", "Library", "Size")
	fmt.Println(strings.Repeat("-", 55))

	for _, binary := range binaries {
		fmt.Printf("%-20s %-8s %-12s %-10s\n",
			binary.Name, binary.Type, binary.Library, binary.SizeStr)
	}
	fmt.Println()
}

// displayOptimizationTable shows optimization comparison table
func displayOptimizationTable(binaries []BinaryInfo) {
	optimizations := getOptimizationConfigs()

	fmt.Println("📊 Optimization Level Comparison:")
	fmt.Println("==================================")

	for _, opt := range optimizations {
		fmt.Printf("\n%s Optimization (%s):\n", opt.Name, opt.Description)
		fmt.Printf("%-15s %-15s %-15s %-15s\n", "", "Standard", "TinyString", "Improvement")
		fmt.Println(strings.Repeat("-", 65))

		// Find matching binaries for this optimization level
		standardNative := findBinaryByPattern(binaries, "standard", "native", opt.Suffix)
		tinystringNative := findBinaryByPattern(binaries, "tinystring", "native", opt.Suffix)
		standardWasm := findBinaryByPattern(binaries, "standard", "wasm", opt.Suffix)
		tinystringWasm := findBinaryByPattern(binaries, "tinystring", "wasm", opt.Suffix)

		if standardNative.Name != "" && tinystringNative.Name != "" {
			improvement := calculateImprovement(standardNative.Size, tinystringNative.Size)
			fmt.Printf("%-15s %-15s %-15s %-15s\n", "Native",
				standardNative.SizeStr, tinystringNative.SizeStr, improvement)
		}

		if standardWasm.Name != "" && tinystringWasm.Name != "" {
			improvement := calculateImprovement(standardWasm.Size, tinystringWasm.Size)
			fmt.Printf("%-15s %-15s %-15s %-15s\n", "WebAssembly",
				standardWasm.SizeStr, tinystringWasm.SizeStr, improvement)
		}
	}
}

// findBinaryByPattern finds a binary matching the specified criteria
func findBinaryByPattern(binaries []BinaryInfo, library, binaryType, optSuffix string) BinaryInfo {
	for _, binary := range binaries {
		if binary.Library == library && binary.Type == binaryType && binary.OptLevel == extractOptLevel(binary.Name) {
			if optSuffix == "" && binary.OptLevel == "default" {
				return binary
			}
			if optSuffix != "" && strings.Contains(binary.Name, optSuffix) {
				return binary
			}
		}
	}
	return BinaryInfo{}
}

// calculateImprovement calculates percentage improvement
func calculateImprovement(original, improved int64) string {
	if original == 0 {
		return "N/A"
	}

	improvement := float64(original-improved) / float64(original) * 100
	if improvement > 0 {
		return fmt.Sprintf("%.1f%% smaller", improvement)
	} else if improvement < 0 {
		return fmt.Sprintf("%.1f%% larger", -improvement)
	}
	return "Same size"
}

// getOptimizationConfigs returns TinyGo optimization configurations
func getOptimizationConfigs() []OptimizationConfig {
	return []OptimizationConfig{
		{
			Name:        "Default",
			Flags:       "",
			Description: "Default TinyGo optimization (-opt=z)",
			Suffix:      "",
		},
		{
			Name:        "Ultra",
			Flags:       "-opt=z -gc=leaking -scheduler=none",
			Description: "Ultra size optimization",
			Suffix:      "-ultra",
		},
		{
			Name:        "Speed",
			Flags:       "-opt=2",
			Description: "Speed optimization",
			Suffix:      "-speed",
		},
		{
			Name:        "Debug",
			Flags:       "-opt=1",
			Description: "Debug build",
			Suffix:      "-debug",
		},
	}
}

// checkGoBenchAvailable checks if Go benchmarks can be run
func checkGoBenchAvailable() bool {
	_, err := exec.LookPath("go")
	return err == nil
}

// runMemoryBenchmarks executes memory benchmarks and returns comparisons
func runMemoryBenchmarks() []MemoryComparison {
	var comparisons []MemoryComparison

	// Run standard library benchmarks
	LogInfo("Running standard library memory benchmarks...")
	standardResults := runBenchmarks("standard")

	// Run TinyString benchmarks
	LogInfo("Running TinyString memory benchmarks...")
	tinystringResults := runBenchmarks("tinystring")

	// Create comparisons
	comparisons = append(comparisons, createComparison(
		"String Processing",
		findBenchmark(standardResults, "BenchmarkStringProcessing"),
		findBenchmark(tinystringResults, "BenchmarkStringProcessing"),
	))

	comparisons = append(comparisons, createComparison(
		"Number Processing",
		findBenchmark(standardResults, "BenchmarkNumberProcessing"),
		findBenchmark(tinystringResults, "BenchmarkNumberProcessing"),
	))

	comparisons = append(comparisons, createComparison(
		"Mixed Operations",
		findBenchmark(standardResults, "BenchmarkMixedOperations"),
		findBenchmark(tinystringResults, "BenchmarkMixedOperations"),
	))

	// Check for pointer optimization benchmark (TinyString only)
	pointerBench := findBenchmark(tinystringResults, "BenchmarkStringProcessingWithPointers")
	if pointerBench.Name != "" {
		standardEquivalent := findBenchmark(standardResults, "BenchmarkStringProcessing")
		comparisons = append(comparisons, createComparison(
			"String Processing (Pointer Optimization)",
			standardEquivalent,
			pointerBench,
		))
	}

	return comparisons
}

// runBenchmarks executes benchmarks for a specific library implementation
func runBenchmarks(library string) []BenchmarkResult {
	var results []BenchmarkResult

	benchDir := filepath.Join("bench-memory-alloc", library)
	if !FileExists(benchDir) {
		LogError(fmt.Sprintf("Benchmark directory %s not found", benchDir))
		return results
	}
	cmd := exec.Command("go", "test", "-bench=.", "-benchmem", "-run=^$")
	cmd.Dir = benchDir

	output, err := cmd.Output()
	if err != nil {
		LogError(fmt.Sprintf("Failed to run benchmarks in %s: %v", benchDir, err))
		return results
	}

	return parseBenchmarkOutput(string(output), library)
}

// parseBenchmarkOutput parses Go benchmark output into structured results
func parseBenchmarkOutput(output, library string) []BenchmarkResult {
	var results []BenchmarkResult

	scanner := bufio.NewScanner(strings.NewReader(output))
	benchmarkRegex := regexp.MustCompile(`^(Benchmark\w+)(?:-\d+)?\s+(\d+)\s+(\d+)\s+ns/op\s+(\d+)\s+B/op\s+(\d+)\s+allocs/op`)
	for scanner.Scan() {
		line := scanner.Text()
		matches := benchmarkRegex.FindStringSubmatch(line)

		if len(matches) == 6 {
			iterations, _ := strconv.ParseInt(matches[2], 10, 64)
			nsPerOp, _ := strconv.ParseInt(matches[3], 10, 64)
			bytesPerOp, _ := strconv.ParseInt(matches[4], 10, 64)
			allocsPerOp, _ := strconv.ParseInt(matches[5], 10, 64)

			result := BenchmarkResult{
				Name:        matches[1],
				Library:     library,
				Iterations:  iterations,
				NsPerOp:     nsPerOp,
				BytesPerOp:  bytesPerOp,
				AllocsPerOp: allocsPerOp,
			}

			results = append(results, result)
		}
	}

	return results
}

// createComparison creates a memory comparison between two benchmark results
func createComparison(category string, standard, tinystring BenchmarkResult) MemoryComparison {
	return MemoryComparison{
		Standard:   standard,
		TinyString: tinystring,
		Category:   category,
	}
}

// findBenchmark finds a benchmark result by name
func findBenchmark(results []BenchmarkResult, name string) BenchmarkResult {
	for _, result := range results {
		if result.Name == name {
			return result
		}
	}
	return BenchmarkResult{}
}

// displayMemoryResults shows memory benchmark results in a table format
func displayMemoryResults(comparisons []MemoryComparison) {
	fmt.Println("\n🧠 Memory Allocation Results:")
	fmt.Println("============================")
	fmt.Printf("%-35s %-12s %-15s %-15s %-15s\n",
		"Category", "Library", "Bytes/Op", "Allocs/Op", "Time/Op")
	fmt.Println(strings.Repeat("-", 95))

	for _, comparison := range comparisons {
		if comparison.Standard.Name != "" {
			fmt.Printf("%-35s %-12s %-15s %-15d %-15s\n",
				comparison.Category, "standard",
				FormatSize(comparison.Standard.BytesPerOp),
				comparison.Standard.AllocsPerOp,
				formatNanoTime(comparison.Standard.NsPerOp))
		}

		if comparison.TinyString.Name != "" {
			fmt.Printf("%-35s %-12s %-15s %-15d %-15s\n",
				"", "tinystring",
				FormatSize(comparison.TinyString.BytesPerOp),
				comparison.TinyString.AllocsPerOp,
				formatNanoTime(comparison.TinyString.NsPerOp))

			// Show improvement
			if comparison.Standard.Name != "" && comparison.TinyString.Name != "" {
				memImprovement := calculateMemoryImprovement(
					comparison.Standard.BytesPerOp, comparison.TinyString.BytesPerOp)
				allocImprovement := calculateMemoryImprovement(
					comparison.Standard.AllocsPerOp, comparison.TinyString.AllocsPerOp)

				fmt.Printf("%-35s %-12s %-15s %-15s %-15s\n",
					"  → Improvement", "", memImprovement, allocImprovement, "")
			}
		}
		fmt.Println()
	}
}

// formatNanoTime formats nanoseconds to readable time units
func formatNanoTime(ns int64) string {
	if ns < 1000 {
		return fmt.Sprintf("%dns", ns)
	} else if ns < 1000000 {
		return fmt.Sprintf("%.1fμs", float64(ns)/1000)
	} else {
		return fmt.Sprintf("%.1fms", float64(ns)/1000000)
	}
}

// calculateMemoryImprovement calculates percentage improvement for memory metrics
func calculateMemoryImprovement(original, improved int64) string {
	if original == 0 {
		return "N/A"
	}

	improvement := float64(original-improved) / float64(original) * 100
	if improvement > 0 {
		return fmt.Sprintf("%.1f%% less", improvement)
	} else if improvement < 0 {
		return fmt.Sprintf("%.1f%% more", -improvement)
	}
	return "Same"
}

// updateREADMEWithBinaryData updates README with binary size analysis
func updateREADMEWithBinaryData(binaries []BinaryInfo) {
	reporter := NewReportGenerator("../README.md")
	if err := reporter.UpdateBinaryData(binaries); err != nil {
		LogError(fmt.Sprintf("Failed to update README with binary data: %v", err))
	}
}

// updateREADMEWithMemoryData updates README with memory benchmark data
func updateREADMEWithMemoryData(comparisons []MemoryComparison) {
	reporter := NewReportGenerator("../README.md")
	if err := reporter.UpdateMemoryData(comparisons); err != nil {
		LogError(fmt.Sprintf("Failed to update README with memory data: %v", err))
	}
}

// updateREADMEWithJSONData actualiza el README con los resultados de los benchmarks JSON
func updateREADMEWithJSONData(comparisons []JSONComparison) error {
	reporter := NewReportGenerator("README.md")
	err := reporter.UpdateJSONData(comparisons)
	if err != nil {
		return fmt.Errorf("failed to update README with JSON data: %v", err)
	}
	return nil
}

// runJSONBenchmarks executes JSON benchmarks and returns the results
func runJSONBenchmarks() ([]JSONComparison, error) {
	LogInfo("Running JSON benchmarks...")

	comparisons := make([]JSONComparison, 0)
	jsonDir := filepath.Join("bench-memory-alloc", "json-comparison")

	// Execute benchmarks
	cmd := exec.Command("go", "test", "-bench=.", "-benchmem")
	cmd.Dir = jsonDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("error running benchmarks: %v", err)
	}

	// Process results
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "Benchmark") {
			continue
		}

		// Extract benchmark data
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}

		name := fields[0]
		nsPerOp, _ := strconv.ParseInt(fields[2], 10, 64)
		bytesPerOp, _ := strconv.ParseInt(fields[3], 10, 64)
		allocsPerOp, _ := strconv.ParseInt(fields[4], 10, 64)

		result := BenchmarkResult{
			Name:        name,
			NsPerOp:     nsPerOp,
			BytesPerOp:  bytesPerOp,
			AllocsPerOp: allocsPerOp,
		}

		// Determine operation type and batch size
		operation := getJSONOperation(name)
		batchSize := getJSONBatchSize(name)
		isError := strings.Contains(name, "Errors")

		// Find corresponding pair or create new comparison
		found := false
		for i := range comparisons {
			if comparisons[i].Operation == operation &&
				comparisons[i].BatchSize == batchSize &&
				comparisons[i].IsErrorCase == isError {
				if strings.Contains(name, "Standard") {
					comparisons[i].Standard = result
				} else {
					comparisons[i].TinyString = result
				}
				found = true
				break
			}
		}

		if !found {
			comparison := JSONComparison{
				Operation:   operation,
				BatchSize:   batchSize,
				IsErrorCase: isError,
			}
			if strings.Contains(name, "Standard") {
				comparison.Standard = result
			} else {
				comparison.TinyString = result
			}
			comparisons = append(comparisons, comparison)
		}
	}

	return comparisons, nil
}

// displayJSONResults shows the results of the JSON benchmarks
func displayJSONResults(comparisons []JSONComparison) {
	fmt.Println("\nJSON Performance Results:")
	fmt.Println("=========================")

	for _, comp := range comparisons {
		batchDesc := ""
		if comp.IsErrorCase {
			batchDesc = "Error Cases"
		} else if comp.BatchSize == 1 {
			batchDesc = "Single"
		} else {
			batchDesc = fmt.Sprintf("Batch-%d", comp.BatchSize)
		}

		fmt.Printf("\n%s (%s):\n", comp.Operation, batchDesc)
		fmt.Printf("  Standard:   %d ns/op, %d B/op, %d allocs/op\n",
			comp.Standard.NsPerOp, comp.Standard.BytesPerOp, comp.Standard.AllocsPerOp)
		fmt.Printf("  TinyString: %d ns/op, %d B/op, %d allocs/op\n",
			comp.TinyString.NsPerOp, comp.TinyString.BytesPerOp, comp.TinyString.AllocsPerOp)
	}
}

// getJSONOperation extracts the operation type from the benchmark name
func getJSONOperation(name string) string {
	if strings.Contains(name, "Marshal") {
		return "Marshal"
	}
	return "Unmarshal"
}

// getJSONBatchSize extracts the batch size from the benchmark name
func getJSONBatchSize(name string) int {
	if strings.Contains(name, "Single") {
		return 1
	}
	re := regexp.MustCompile(`Batch(\d+)`)
	matches := re.FindStringSubmatch(name)
	if len(matches) < 2 {
		return 0 // For error cases
	}
	size, _ := strconv.Atoi(matches[1])
	return size
}
