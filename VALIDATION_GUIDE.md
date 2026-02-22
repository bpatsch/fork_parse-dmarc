# Validation Guide for Filesystem DMARC Importer

This document provides a comprehensive guide for an AI agent to validate the recent changes that introduced the filesystem-based DMARC report processing feature.

## 1. Objective

The goal of the recent changes was to allow the `parse-dmarc` application to read, parse, and process DMARC aggregate reports from a local filesystem directory, as an alternative to fetching them from an IMAP server.

## 2. Core Requirements to Validate

1.  **Configurable Directory:** The application must be configurable to read from a local directory via a new `ReportPath` setting in `config.json`.
2.  **Conditional Execution:** When `ReportPath` is set, the IMAP fetching logic must be skipped. The application should only process files from the specified directory.
3.  **Mutual Exclusivity:** The configuration should enforce that either IMAP settings *or* `ReportPath` are provided, but not both. An error should be raised if this rule is violated.
4.  **One-Shot Processing:** In filesystem mode, the application should process all valid report files in the directory once and then keep the web server running without continuous monitoring.
5.  **File Handling:** The processor must correctly handle `.xml`, `.gz`, and `.zip` files.
6.  **Archiving:** Successfully processed files must be moved to a `processed` subdirectory to prevent re-ingestion.
7.  **Documentation:** The new feature and its usage (specifically for Docker) must be documented in the `README.md`. A guide for creating a Pull Request should also be present.

## 3. Verification Steps

To validate the implementation, follow these steps methodically.

### Step 3.1: Code Review

Review the following files to ensure the changes align with the requirements.

1.  **`internal/config/config.go`**
    - **[ ] Verify:** The `Config` struct contains a new `ReportPath` field.
    - **[ ] Verify:** The `Validate()` function has been updated. It should return an error if both `ReportPath` and IMAP settings are present, and also if neither are present.
    - **[ ] Verify:** The `GenerateSample()` function includes `ReportPath` in the example configuration.

2.  **`main.go`**
    - **[ ] Verify:** A `reportSource` variable is determined based on whether `cfg.ReportPath` is set.
    - **[ ] Verify:** The main processing logic contains an `if reportSource == "filesystem"` block.
    - **[ ] Verify:** The filesystem block initializes a `filereader.Processor` and calls its `ProcessReports` method **once**. It does *not* contain a `time.Ticker` loop.
    - **[ ] Verify:** After the single processing run, the filesystem block enters a `select` loop to keep the server alive.
    - **[ ] Verify:** The `else` block retains the original IMAP fetching logic, including the `time.Ticker` loop for continuous mode.

3.  **`internal/filereader/filereader.go`**
    - **[ ] Verify:** The file exists and defines a `Processor` struct.
    - **[ ] Verify:** The `ProcessReports` method iterates through files in the configured directory.
    - **[ ] Verify:** It correctly identifies files ending in `.xml`, `.gz`, and `.zip`.
    - **[ ] Verify:** It reuses the `parser.ParseReport` function to handle file content.
    - **[ ] Verify:** It calls the `archiveFile` helper function to move files to a `processed` subdirectory upon success.

4.  **`README.md`**
    - **[ ] Verify:** A new section named "Building the Docker Image" exists.
    - **[ ] Verify:** This section contains instructions and examples for running a locally-built Docker container in the new filesystem mode.

5.  **`HOWTO_pull-request.md`**
    - **[ ] Verify:** The file exists and contains a comprehensive guide to forking, configuring remotes, and creating a pull request.

### Step 3.2: Functional Testing

This requires executing the application.

#### Test Setup

1.  **Create a test directory for reports:**
    ```bash
    mkdir -p ./test_reports/processed
    ```

2.  **Create a sample DMARC report file (`sample.xml`):**
    Use the following content for a file named `test_reports/sample.xml`:
    ```xml
    <?xml version="1.0" encoding="UTF-8" ?>
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
    ```

3.  **Create compressed versions:**
    ```bash
    gzip -k ./test_reports/sample.xml
    zip ./test_reports/sample.zip ./test_reports/sample.xml
    ```
    You should now have `sample.xml`, `sample.xml.gz`, and `sample.zip` in the `test_reports` directory.

4.  **Create a `test_config.json` file:**
    ```json
    {
      "log_level": "debug",
      "report_path": "./test_reports",
      "database": {
        "path": "./test_db.sqlite"
      }
    }
    ```

#### Test Execution & Verification

1.  **Run the application in filesystem mode:**
    ```bash
    go run . --config test_config.json
    ```

2.  **[ ] Verify Application Logs:**
    Check the console output. You should see logs indicating:
    - `using report source` with `source=filesystem`
    - `scanning for DMARC reports in directory` with `path=./test_reports`
    - `processing file` for `sample.xml`, `sample.xml.gz`, and `sample.zip`.
    - Three `saved report` messages, one for each file.
    - `filesystem reports processed` with `count=3`.
    - `filesystem processing complete. Server is running.`

3.  **[ ] Verify File Archiving:**
    List the contents of the `test_reports` directory and its `processed` subdirectory.
    - The `test_reports` directory should be empty (or contain only the `processed` directory).
    - The `test_reports/processed` directory should now contain `sample.xml`, `sample.xml.gz`, and `sample.zip`.
    ```bash
    ls -l ./test_reports/processed
    ```

4.  **[ ] Verify Database Content (Optional but Recommended):**
    Check the SQLite database to confirm the three reports were saved.
    ```bash
    sqlite3 ./test_db.sqlite "SELECT report_id FROM reports;"
    ```
    The output should contain `test-report-12345` (it will only appear once due to `INSERT OR IGNORE` and the identical `report_id` in the samples, which also correctly validates that duplicate reports are not inserted).

5.  **Cleanup:**
    ```bash
    rm -rf ./test_reports ./test_config.json ./test_db.sqlite
    ```

### Step 3.3: Regression Testing

1.  **[ ] Verify IMAP Mode:**
    - Temporarily modify your primary `config.json` to remove the `report_path` key and ensure your IMAP credentials are valid.
    - Run the application: `go run .`.
    - **[ ] Verify:** The application logs should show `using report source` with `source=imap` and proceed with fetching reports from the IMAP server as it did previously. This confirms the original functionality is not broken.
