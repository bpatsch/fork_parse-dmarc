package filereader

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/meysam81/parse-dmarc/internal/metrics"
	"github.com/meysam81/parse-dmarc/internal/parser"
	"github.com/meysam81/parse-dmarc/internal/storage"
	"github.com/rs/zerolog"
)

// Processor handles reading DMARC reports from the filesystem.
type Processor struct {
	reportPath string
	store      *storage.Storage
	metrics    *metrics.Metrics
	log        *zerolog.Logger
}

// SaveReportFunc is a function signature for a function that saves a parsed report.
// This allows decoupling the filereader from the main application's saving logic.
type SaveReportFunc func(feedback *parser.Feedback, m *metrics.Metrics, store *storage.Storage, log *zerolog.Logger) error

// NewProcessor creates a new filesystem report processor.
func NewProcessor(reportPath string, store *storage.Storage, m *metrics.Metrics, log *zerolog.Logger) *Processor {
	return &Processor{
		reportPath: reportPath,
		store:      store,
		metrics:    m,
		log:        log,
	}
}

// ProcessReports scans the configured directory for DMARC reports, parses them,
// saves them to storage, and moves processed files to an archive directory.
func (p *Processor) ProcessReports(saveFunc SaveReportFunc) error {
	p.log.Info().Str("path", p.reportPath).Msg("scanning for DMARC reports in directory")
	fetchStart := time.Now()
	if p.metrics != nil {
		p.metrics.FetchCyclesTotal.Inc()
	}

	files, err := os.ReadDir(p.reportPath)
	if err != nil {
		if p.metrics != nil {
			p.metrics.FetchErrors.Inc()
		}
		return fmt.Errorf("failed to read report directory %s: %w", p.reportPath, err)
	}

	if len(files) == 0 {
		p.log.Info().Msg("no report files found in directory")
		return nil
	}

	processedCount := 0
	for _, file := range files {
		if file.IsDir() || !isDMARCReportFile(file.Name()) {
			continue
		}

		filePath := filepath.Join(p.reportPath, file.Name())
		p.log.Debug().Str("file", filePath).Msg("processing file")
		if p.metrics != nil {
			p.metrics.AttachmentsTotal.Inc() // Reusing 'AttachmentsTotal' for files
		}

		// Special handling for .msg files
		if strings.HasSuffix(strings.ToLower(file.Name()), ".msg") {
			if err := p.processMSGFile(filePath, saveFunc); err != nil {
				p.log.Warn().Err(err).Str("file", filePath).Msg("failed to process .msg file")
				if p.metrics != nil {
					p.metrics.ReportParseErrors.Inc()
				}
				continue
			}
			processedCount++
			
			// Move processed .msg file to archive
			if err := p.archiveFile(filePath); err != nil {
				p.log.Error().Err(err).Str("file", filePath).Msg("failed to archive processed .msg file")
				// Continue even if archiving fails, as attachments are already processed
			}
			continue
		}

		// Regular file processing for .xml, .xml.gz, .zip
		fileData, err := os.ReadFile(filePath)
		if err != nil {
			p.log.Warn().Err(err).Str("file", filePath).Msg("failed to read report file")
			continue
		}

		feedback, err := parser.ParseReport(fileData)
		if err != nil {
			p.log.Warn().Err(err).Str("filename", file.Name()).Msg("failed to parse report")
			if p.metrics != nil {
				p.metrics.ReportParseErrors.Inc()
			}
			continue
		}

		if err := saveFunc(feedback, p.metrics, p.store, p.log); err != nil {
			p.log.Error().Err(err).Msg("failed to save report from file")
			if p.metrics != nil {
				p.metrics.ReportStoreErrors.Inc()
			}
			continue
		}

		processedCount++

		// Move processed file to archive
		if err := p.archiveFile(filePath); err != nil {
			p.log.Error().Err(err).Str("file", filePath).Msg("failed to archive processed report file")
			// Continue even if archiving fails, as the report is already saved
		}
	}

	if p.metrics != nil {
		p.metrics.ReportsFetched.Add(float64(processedCount))
		p.metrics.RecordFetchDuration(time.Since(fetchStart))
		p.metrics.LastFetchTimestamp.SetToCurrentTime()
	}

	p.log.Info().Int("count", processedCount).Msg("filesystem reports processed")
	return nil
}

// processMSGFile processes an Outlook .msg file and extracts DMARC report attachments
func (p *Processor) processMSGFile(filePath string, saveFunc SaveReportFunc) error {
	msgProcessor := NewMSGFileProcessor(p.log)
	attachments, err := msgProcessor.ProcessMSGFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to process .msg file: %w", err)
	}

	if len(attachments) == 0 {
		p.log.Info().Str("file", filePath).Msg("no DMARC report attachments found in .msg file")
		return nil
	}

	// Process each DMARC attachment
	for _, attachment := range attachments {
		p.log.Info().Str("attachment", attachment.Filename).Str("file", filePath).Msg("processing DMARC attachment from .msg")
		
		feedback, err := parser.ParseReport(attachment.Data)
		if err != nil {
			p.log.Warn().Err(err).Str("attachment", attachment.Filename).Msg("failed to parse DMARC report from .msg attachment")
			if p.metrics != nil {
				p.metrics.ReportParseErrors.Inc()
			}
			continue
		}

		if err := saveFunc(feedback, p.metrics, p.store, p.log); err != nil {
			p.log.Error().Err(err).Str("attachment", attachment.Filename).Msg("failed to save DMARC report from .msg attachment")
			if p.metrics != nil {
				p.metrics.ReportStoreErrors.Inc()
			}
			continue
		}
	}

	return nil
}

// archiveFile moves a successfully processed file to a 'processed' subdirectory.
func (p *Processor) archiveFile(filePath string) error {
	archiveDir := filepath.Join(p.reportPath, "processed")
	if err := os.MkdirAll(archiveDir, 0755); err != nil {
		return fmt.Errorf("failed to create archive directory %s: %w", archiveDir, err)
	}

	newFilePath := filepath.Join(archiveDir, filepath.Base(filePath))
	p.log.Debug().Str("from", filePath).Str("to", newFilePath).Msg("archiving file")
	return os.Rename(filePath, newFilePath)
}

// isDMARCReportFile checks if a filename is likely a DMARC report.
func isDMARCReportFile(filename string) bool {
	lower := strings.ToLower(filename)
	return strings.HasSuffix(lower, ".xml") ||
		strings.HasSuffix(lower, ".xml.gz") ||
		strings.HasSuffix(lower, ".zip") ||
		strings.HasSuffix(lower, ".msg")
}
