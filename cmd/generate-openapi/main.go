package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"gopkg.in/yaml.v3"
)

// generateOpenAPI converts the OpenAPI YAML spec to JSON format
func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: generate-openapi <input.yaml> [output.json]")
		fmt.Println("  input.yaml: Path to OpenAPI YAML file")
		fmt.Println("  output.json: Optional output JSON file (default: openapi.json)")
		os.Exit(1)
	}

	inputFile := os.Args[1]
	outputFile := "openapi.json"
	if len(os.Args) > 2 {
		outputFile = os.Args[2]
	}

	// Read YAML file
	yamlData, err := os.ReadFile(inputFile)
	if err != nil {
		log.Fatalf("Failed to read YAML file: %v", err)
	}

	// Parse YAML
	var spec map[string]interface{}
	if err := yaml.Unmarshal(yamlData, &spec); err != nil {
		log.Fatalf("Failed to parse YAML: %v", err)
	}

	// Convert to JSON
	jsonData, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal JSON: %v", err)
	}

	// Write JSON file
	if err := os.WriteFile(outputFile, jsonData, 0644); err != nil {
		log.Fatalf("Failed to write JSON file: %v", err)
	}

	fmt.Printf("✓ Successfully converted %s to %s\n", inputFile, outputFile)
}

