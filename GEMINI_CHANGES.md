# DMARC Report Processing from Filesystem

This document outlines the requirements and implementation details for adding a new feature to `parse-dmarc` that allows it to process DMARC reports from a local filesystem directory instead of an IMAP server.

## 1. Requirements

The user requested the following modifications:
- DMARC reports (in all currently supported formats: `.xml`, `.gz`, `.zip`) should be readable from a configurable local directory.
- When processing from a local directory is configured, the IMAP/email fetching code should not be executed.
- For the filesystem processing mode, a continuous monitoring loop (like the one for IMAP) is not required. The application should process the files once and then keep the web server running to display the results.

## 2. Implementation Plan

A three-step plan was devised to implement this feature:

1.  **Update Configuration:**
    -   Introduce a new `ReportPath` setting in `internal/config/config.go`.
    -   Update the configuration validation (`Validate`) to enforce that either `ReportPath` or IMAP settings are provided, but not both.
    -   Update the sample configuration file (`GenerateSample`) to include the new setting.

2.  **Modify Main Application Logic:**
    -   In `main.go`, determine the report source ("filesystem" or "imap") based on the loaded configuration.
    -   Refactor the core report-saving logic into a reusable `saveParsedReport` helper function.
    -   Replace the main processing loop with a conditional block that executes different logic based on the determined report source.
    -   The "filesystem" path will process files once and then wait for a shutdown signal.
    -   The "imap" path will retain its existing continuous fetch and `--fetch-once` behavior.

3.  **Implement Filesystem Reading Logic:**
    -   Create a new file: `internal/filereader/filereader.go`.
    -   Define a `Processor` struct to manage filesystem-based report processing.
    -   Implement a `ProcessReports` method that:
        -   Scans the configured directory for report files.
        -   Reads each file's content.
        -   Reuses the existing `parser.ParseReport` function.
        -   Calls the `saveParsedReport` helper function passed from `main.go`.
        -   Moves successfully processed files into a `processed` subdirectory to prevent reprocessing.

## 3. Detailed Changes

The following changes were applied to the codebase:

### `internal/config/config.go`

-   **Added `ReportPath` to `Config` struct:**
    ```go
    type Config struct {
        // ... other fields
        ReportPath  string         `json:"report_path" env:"REPORT_PATH"`
        // ... other fields
    }
    ```
-   **Updated `Validate()` function:** The function was rewritten to check for the mutual exclusivity of `ReportPath` and IMAP settings. It now returns an error if both are set, or if neither is set (in a context where a report source is required).
-   **Updated `GenerateSample()` function:** Added the `ReportPath` field to the sample configuration output, with a comment explaining its use.

### `main.go`

-   **Added `filereader` import:**
    ```go
    import (
        // ...
        "github.com/meysam81/parse-dmarc/internal/filereader"
        // ...
    )
    ```
-   **Determined `reportSource`:** Added logic after configuration loading to set a `reportSource` variable to either "filesystem" or "imap".
-   **Created `saveParsedReport` helper:** The report saving and metrics logic was extracted from `fetchReports` into a new `saveParsedReport` function to be reused by both IMAP and filesystem processors.
-   **Implemented Conditional Processing Loop:** The main report processing block was replaced with an `if reportSource == "filesystem"` / `else` block.
    -   The `filesystem` block initializes `filereader.NewProcessor`, calls its `ProcessReports` method once, and then enters a `select` loop to wait for shutdown, keeping the server alive.
    -   The `else` block contains the original IMAP processing logic with its `fetchOnce` and `ticker` functionality.

### `internal/filereader/filereader.go`

-   **Created new file:** This file contains all the logic for the filesystem-based report processing.
-   **`Processor` struct:** Holds dependencies like the report path, storage, metrics, and logger.
-   **`NewProcessor()` function:** A constructor for the `Processor`.
-   **`ProcessReports()` method:**
    -   Reads the directory specified by `reportPath`.
    -   Identifies valid report files (`.xml`, `.gz`, `.zip`).
    -   For each file, it reads the data, calls `parser.ParseReport`, and then calls the `saveFunc` (which is `saveParsedReport` from `main.go`).
    -   Upon successful processing, calls a helper `archiveFile` to move the source file to a `processed` subdirectory.
