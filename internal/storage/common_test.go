package storage

import (
	"testing"
)

func TestGetStatistics_HasData(t *testing.T) {
	// Create an in-memory SQLite database for testing
	storage, err := NewStorage(":memory:")
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	// Test case 1: Empty database - HasData should be false
	stats, err := storage.GetStatistics()
	if err != nil {
		t.Fatalf("Failed to get statistics: %v", err)
	}

	if stats.HasData {
		t.Errorf("Expected HasData to be false for empty database, got true")
	}

	if stats.TotalReports != 0 {
		t.Errorf("Expected TotalReports to be 0, got %d", stats.TotalReports)
	}

	if stats.TotalMessages != 0 {
		t.Errorf("Expected TotalMessages to be 0, got %d", stats.TotalMessages)
	}

	if stats.CompliantMessages != 0 {
		t.Errorf("Expected CompliantMessages to be 0, got %d", stats.CompliantMessages)
	}

	if stats.ComplianceRate != 0 {
		t.Errorf("Expected ComplianceRate to be 0, got %f", stats.ComplianceRate)
	}

	// Note: We would need to add a report to test HasData = true,
	// but that would require more complex setup with the parser.Feedback structure.
	// For now, this test validates the empty database case which is the core issue we fixed.
}
