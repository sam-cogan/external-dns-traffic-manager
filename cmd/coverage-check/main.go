package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
)

const module = "github.com/sam-cogan/external-dns-traffic-manager/"

type requirement struct {
	File     string  `json:"file"`
	Function string  `json:"function"`
	Minimum  float64 `json:"minimum"`
	Risk     string  `json:"risk"`
}

func check(policy io.Reader, report io.Reader, output io.Writer) error {
	var requirements []requirement
	decoder := json.NewDecoder(policy)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&requirements); err != nil {
		return fmt.Errorf("decode policy: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("policy must contain exactly one JSON document")
	}
	if len(requirements) == 0 {
		return fmt.Errorf("coverage policy is empty")
	}
	coverage := make(map[string]float64)
	scanner := bufio.NewScanner(report)
	totalFound := false
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 0 {
			continue
		}
		if len(fields) != 3 {
			return fmt.Errorf("malformed coverage row: %s", scanner.Text())
		}
		value, err := strconv.ParseFloat(strings.TrimSuffix(fields[2], "%"), 64)
		if err != nil || math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > 100 || !strings.HasSuffix(fields[2], "%") {
			return fmt.Errorf("invalid coverage percentage: %q", fields[2])
		}
		if fields[0] == "total:" {
			if totalFound || fields[1] != "(statements)" {
				return fmt.Errorf("invalid coverage total")
			}
			totalFound = true
			fmt.Fprintf(output, "Repository total: %.1f%% (informational)\n", value)
			continue
		}
		location := strings.TrimSuffix(fields[0], ":")
		colon := strings.LastIndex(location, ":")
		if colon < 0 {
			return fmt.Errorf("invalid coverage location: %q", fields[0])
		}
		line, err := strconv.Atoi(location[colon+1:])
		if err != nil || line <= 0 {
			return fmt.Errorf("invalid source line: %q", fields[0])
		}
		file, ok := strings.CutPrefix(location[:colon], module)
		if !ok {
			return fmt.Errorf("unexpected coverage module: %q", location[:colon])
		}
		key := file + ":" + fields[1]
		// Different receivers may have the same method name. A policy for such
		// a name protects the least-covered implementation, not just the last.
		if previous, exists := coverage[key]; !exists || value < previous {
			coverage[key] = value
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read coverage: %w", err)
	}
	if !totalFound {
		return fmt.Errorf("coverage report is missing its total")
	}
	seen := make(map[string]bool)
	var failures []string
	for _, rule := range requirements {
		key := rule.File + ":" + rule.Function
		if rule.File == "" || rule.Function == "" || rule.Risk == "" || rule.Minimum <= 0 ||
			rule.Minimum > 100 || math.IsNaN(rule.Minimum) || seen[key] {
			return fmt.Errorf("invalid or duplicate policy rule: %s", key)
		}
		seen[key] = true
		value, exists := coverage[key]
		if !exists {
			failures = append(failures, key+" missing from coverage")
			continue
		}
		fmt.Fprintf(output, "%s: %s %.1f%% (minimum %.1f%%)\n", rule.Risk, key, value, rule.Minimum)
		if value < rule.Minimum {
			failures = append(failures, fmt.Sprintf("%s %.1f%% < %.1f%%", key, value, rule.Minimum))
		}
	}
	if len(failures) > 0 {
		return fmt.Errorf("risk coverage failed:\n%s", strings.Join(failures, "\n"))
	}
	return nil
}

func run() error {
	policyPath := flag.String("policy", "coverage-policy.json", "risk coverage policy")
	reportPath := flag.String("report", "coverage-functions.txt", "go tool cover -func output")
	flag.Parse()
	policy, err := os.Open(*policyPath)
	if err != nil {
		return err
	}
	defer policy.Close()
	report, err := os.Open(*reportPath)
	if err != nil {
		return err
	}
	defer report.Close()
	return check(policy, report, os.Stdout)
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
