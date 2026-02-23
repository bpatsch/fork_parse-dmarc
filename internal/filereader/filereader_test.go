package filereader

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/meysam81/parse-dmarc/internal/metrics"
	"github.com/meysam81/parse-dmarc/internal/parser"
	"github.com/meysam81/parse-dmarc/internal/storage"
	"github.com/rs/zerolog"

	_ "github.com/mattn/go-sqlite3" // Import the sqlite3 driver
)

const sampleXML = `<?xml version="1.0" encoding="UTF-8" ?>
<feedback>
  <report_metadata>
	<org_name>test-reporter.com</org_name>
	<email>noreply@test-reporter.com</email>
	<report_id>test-report-12345</report_id>
	<date_range>
	  <begin>1672531200</begin>
	  <end>1672617599</end>
	</date_range>
  </report_metadata>
  <policy_published>
	<domain>yourdomain.com</domain>
	<p>none</p>
	<sp>none</sp>
	<pct>100</pct>
  </policy_published>
  <record>
	<row>
	  <source_ip>1.2.3.4</source_ip>
	  <count>1</count>
	  <policy_evaluated>
		<disposition>none</disposition>
		<dkim>pass</dkim>
		<spf>pass</spf>
	  </policy_evaluated>
	</row>
	<identifiers>
	  <header_from>yourdomain.com</header_from>
	</identifiers>
	<auth_results>
	  <dkim>
		<domain>yourdomain.com</domain>
		<result>pass</result>
	  </dkim>
	  <spf>
		<domain>yourdomain.com</domain>
		<result>pass</result>
	  </spf>
	</auth_results>
  </record>
</feedback>
`

func TestProcessReports(t *testing.T) {
	// 1. Setup
	tempDir := t.TempDir()
	reportDir := filepath.Join(tempDir, "reports")
	processedDir := filepath.Join(reportDir, "processed")
	if err := os.Mkdir(reportDir, 0755); err != nil {
		t.Fatalf("Failed to create report dir: %v", err)
	}

	// Create sample files
	// a. XML
	xmlPath := filepath.Join(reportDir, "sample.xml")
	if err := os.WriteFile(xmlPath, []byte(sampleXML), 0644); err != nil {
		t.Fatalf("Failed to write sample.xml: %v", err)
	}

	// b. GZIP
	gzPath := filepath.Join(reportDir, "sample.xml.gz")
	var gzBuf bytes.Buffer
	gzWriter := gzip.NewWriter(&gzBuf)
	if _, err := gzWriter.Write([]byte(sampleXML)); err != nil {
		t.Fatalf("Failed to gzip sample xml: %v", err)
	}
	gzWriter.Close()
	if err := os.WriteFile(gzPath, gzBuf.Bytes(), 0644); err != nil {
		t.Fatalf("Failed to write sample.xml.gz: %v", err)
	}

	// c. ZIP
	zipPath := filepath.Join(reportDir, "sample.zip")
	zipFile, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("Failed to create sample.zip: %v", err)
	}
	defer zipFile.Close()
	zipWriter := zip.NewWriter(zipFile)
	w, err := zipWriter.Create("sample.xml")
	if err != nil {
		t.Fatalf("Failed to create entry in zip file: %v", err)
	}
	if _, err := w.Write([]byte(sampleXML)); err != nil {
		t.Fatalf("Failed to write content to zip entry: %v", err)
	}
	zipWriter.Close()

	// d. Non-report file
	nonReportPath := filepath.Join(reportDir, "notes.txt")
	if err := os.WriteFile(nonReportPath, []byte("ignore me"), 0644); err != nil {
		t.Fatalf("Failed to write notes.txt: %v", err)
	}




	// Create mock logger, storage, and processor
	log := zerolog.Nop()
	dbFile := filepath.Join(tempDir, "test.db")
	store, err := storage.NewStorage(dbFile)
	if err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}
	processor := NewProcessor(reportDir, store, nil, &log)

	// Define the save function to be passed to the processor
	saveFunc := func(feedback *parser.Feedback, m *metrics.Metrics, s *storage.Storage, log *zerolog.Logger) error {
		return s.SaveReport(feedback)
	}

	// 2. Execute
	if err := processor.ProcessReports(saveFunc); err != nil {
		t.Fatalf("ProcessReports returned an unexpected error: %v", err)
	}

	// 3. Assert
	// a. Assert files were moved
	originalFiles := []string{xmlPath, gzPath, zipPath}
	for _, p := range originalFiles {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("Expected original file %s to be moved, but it still exists", p)
		}

		processedPath := filepath.Join(processedDir, filepath.Base(p))
		if _, err := os.Stat(processedPath); os.IsNotExist(err) {
			t.Errorf("Expected processed file %s to exist, but it does not", processedPath)
		}
	}



	// b. Assert non-report file was ignored
	if _, err := os.Stat(nonReportPath); os.IsNotExist(err) {
		t.Errorf("Expected non-report file %s to be ignored, but it was moved or deleted", nonReportPath)
	}


	// c. Assert database content
	// Since all sample files have the same report_id, INSERT OR IGNORE will result in only one entry.
	// This correctly tests the idempotency logic.
	var reportCount int
	db, err := sql.Open("sqlite3", dbFile)
	if err != nil {
		t.Fatalf("Failed to open test DB for verification: %v", err)
	}
	defer db.Close()

	row := db.QueryRow("SELECT COUNT(*) FROM reports WHERE report_id = 'test-report-12345'")
	if err := row.Scan(&reportCount); err != nil {
		t.Fatalf("Failed to query database for report count: %v", err)
	}
	if reportCount != 1 {
		t.Errorf("Expected 1 report to be in the database, but found %d", reportCount)
	}
}
