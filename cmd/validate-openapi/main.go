package main

import (
	"fmt"
	"log"
	"os"

	"gopkg.in/yaml.v3"
)

// validateOpenAPI validates the structure of an OpenAPI specification
func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: validate-openapi <spec.yaml>")
		fmt.Println("  spec.yaml: Path to OpenAPI YAML file")
		os.Exit(1)
	}

	specFile := os.Args[1]

	// Read YAML file
	yamlData, err := os.ReadFile(specFile)
	if err != nil {
		log.Fatalf("Failed to read YAML file: %v", err)
	}

	// Parse YAML
	var spec map[string]interface{}
	if err := yaml.Unmarshal(yamlData, &spec); err != nil {
		log.Fatalf("Failed to parse YAML: %v", err)
	}

	// Basic validation
	errors := []string{}

	// Check required top-level fields
	requiredFields := []string{"openapi", "info", "paths"}
	for _, field := range requiredFields {
		if _, ok := spec[field]; !ok {
			errors = append(errors, fmt.Sprintf("missing required field: %s", field))
		}
	}

	// Check OpenAPI version
	if version, ok := spec["openapi"].(string); ok {
		if version < "3.0.0" {
			errors = append(errors, fmt.Sprintf("unsupported OpenAPI version: %s (requires 3.0.0+)", version))
		}
	}

	// Check info object
	if info, ok := spec["info"].(map[string]interface{}); ok {
		requiredInfoFields := []string{"title", "version"}
		for _, field := range requiredInfoFields {
			if _, ok := info[field]; !ok {
				errors = append(errors, fmt.Sprintf("missing required info field: %s", field))
			}
		}
	} else {
		errors = append(errors, "info must be an object")
	}

	// Check paths object
	if paths, ok := spec["paths"].(map[string]interface{}); ok {
		if len(paths) == 0 {
			errors = append(errors, "paths object is empty")
		}
	} else {
		errors = append(errors, "paths must be an object")
	}

	// Report results
	if len(errors) > 0 {
		fmt.Println("✗ Validation failed:")
		for _, err := range errors {
			fmt.Printf("  - %s\n", err)
		}
		os.Exit(1)
	}

	fmt.Println("✓ OpenAPI specification is valid")
	fmt.Printf("  Version: %v\n", spec["openapi"])
	if info, ok := spec["info"].(map[string]interface{}); ok {
		fmt.Printf("  Title: %v\n", info["title"])
		fmt.Printf("  Version: %v\n", info["version"])
	}
	if paths, ok := spec["paths"].(map[string]interface{}); ok {
		fmt.Printf("  Paths: %d\n", len(paths))
	}
}

