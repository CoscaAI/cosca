package codeanalyzer

import (
	"testing"
)

const testCode = `package main

import (
	"database/sql"
	"fmt"
)

// User represents a user in the system.
type User struct {
	ID   int
	Name string
}

// GetUser fetches a user by ID.
func GetUser(db *sql.DB, id int) (*User, error) {
	// TODO: add caching
	if id <= 0 {
		return nil, fmt.Errorf("invalid id")
	}

	var user User
	err := db.QueryRow("SELECT id, name FROM users WHERE id = ?", id).
		Scan(&user.ID, &user.Name)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

// ListUsers fetches all users.
func ListUsers(db *sql.DB) ([]*User, error) {
	rows, err := db.Query("SELECT id, name FROM users")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*User
	for rows.Next() {
		var user User
		if err := rows.Scan(&user.ID, &user.Name); err != nil {
			return nil, err
		}
		users = append(users, &user)
	}

	return users, nil
}
`

// TestAnalyzeGo tests Go code analysis.
func TestAnalyzeGo(t *testing.T) {
	analyzer := NewAnalyzer()

	result, err := analyzer.AnalyzeGo("/test/main.go", testCode)
	if err != nil {
		t.Fatalf("Failed to analyze code: %v", err)
	}

	if result.Language != "go" {
		t.Errorf("Expected language go, got %s", result.Language)
	}

	if result.FunctionCount != 2 {
		t.Errorf("Expected 2 functions, got %d", result.FunctionCount)
	}

	if result.StructCount != 1 {
		t.Errorf("Expected 1 struct, got %d", result.StructCount)
	}

	if result.ImportCount != 2 {
		t.Errorf("Expected 2 imports, got %d", result.ImportCount)
	}

	if result.TodoComments != 1 {
		t.Errorf("Expected 1 TODO comment, got %d", result.TodoComments)
	}

	// Check metrics
	if result.Metrics["function_count"] != 2 {
		t.Errorf("Expected function_count metric 2, got %v", result.Metrics["function_count"])
	}

	// Check function details
	for _, fn := range result.Functions {
		if fn.Name == "GetUser" {
			if !fn.HasErrorReturn {
				t.Error("GetUser should have error return")
			}
			if fn.Parameters != 2 {
				t.Errorf("GetUser should have 2 parameters, got %d", fn.Parameters)
			}
		}
	}
}

// TestAnalyzeLargeFile tests large file detection.
func TestAnalyzeLargeFile(t *testing.T) {
	analyzer := NewAnalyzer()

	// Generate a large file
	largeCode := "package main\n\n"
	for i := 0; i < 520; i++ {
		largeCode += "var x" + itoa(i) + " = " + itoa(i) + "\n"
	}

	result, err := analyzer.AnalyzeGo("/test/large.go", largeCode)
	if err != nil {
		t.Fatalf("Failed to analyze code: %v", err)
	}

	if result.Lines < 500 {
		t.Errorf("Expected large file, got %d lines", result.Lines)
	}

	// Check pattern detection
	found := false
	for _, p := range result.Patterns {
		if p.Pattern == "god_object" {
			found = true
		}
	}
	if !found {
		t.Error("Expected god_object pattern")
	}
}

// TestAnalyzeComplexity tests complexity detection.
func TestAnalyzeComplexity(t *testing.T) {
	analyzer := NewAnalyzer()

	complexCode := `package main

func complexFunction(a, b, c, d int) int {
	if a > 0 {
		if b > 0 {
			if c > 0 {
				if d > 0 {
					for i := 0; i < 10; i++ {
						if i%2 == 0 {
							return i
						}
					}
				}
			}
		}
	}
	return 0
}
`

	result, err := analyzer.AnalyzeGo("/test/complex.go", complexCode)
	if err != nil {
		t.Fatalf("Failed to analyze code: %v", err)
	}

	if result.MaxComplexity < 5 {
		t.Errorf("Expected complexity >= 5, got %d", result.MaxComplexity)
	}

	if result.MaxNestingDepth < 4 {
		t.Errorf("Expected nesting depth >= 4, got %d", result.MaxNestingDepth)
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	digits := ""
	for i > 0 {
		digits = string(rune('0'+i%10)) + digits
		i /= 10
	}
	return digits
}