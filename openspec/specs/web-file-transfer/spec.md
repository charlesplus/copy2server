# web-file-transfer Specification

## Purpose
Define the web upload, listing, download, and cleanup behavior for copy2server.

## Requirements

### Requirement: Web Upload Interface
The system SHALL serve a browser interface for selecting, dragging, dropping, or pasting files.

#### Scenario: User opens the root page
- **WHEN** the browser requests `GET /`
- **THEN** the server responds with the shared HTML upload interface
- **AND** the interface displays the configured upload limit and retention period

#### Scenario: User uploads through the browser
- **WHEN** the user selects, drops, or pastes one or more files
- **THEN** the browser submits them to `POST /api/upload` as multipart form data with field name `file`

### Requirement: File Upload API
The system SHALL accept multipart file uploads and persist files to the configured upload directory.

#### Scenario: Multipart upload succeeds
- **GIVEN** a `POST /api/upload` request contains one or more `file` parts within the configured size limit
- **WHEN** the upload completes
- **THEN** each file is stored under a unique sanitized filename in the upload directory
- **AND** the response status is `201`
- **AND** the JSON response contains a `files` array with `name`, `size`, `modifiedAt`, `serverPath`, `downloadUrl`, and `isImage` for each stored file

#### Scenario: Upload request has no files
- **GIVEN** a `POST /api/upload` request contains no usable `file` parts
- **WHEN** the request is processed
- **THEN** the response is a client error with a JSON `error` message

#### Scenario: Upload request is not multipart
- **GIVEN** a `POST /api/upload` request is not `multipart/form-data`
- **WHEN** the request is processed
- **THEN** the response is a client error with a JSON `error` message

### Requirement: File Listing API
The system SHALL list uploaded files with metadata needed by the browser interface.

#### Scenario: Files are listed
- **WHEN** the browser requests `GET /api/files`
- **THEN** the server responds with JSON containing a `files` array
- **AND** files are ordered by newest modification time first
- **AND** directories in the upload directory are excluded

### Requirement: File Download API
The system SHALL allow downloading previously uploaded files by safe filename only.

#### Scenario: Existing file is downloaded
- **GIVEN** a stored file exists in the upload directory
- **WHEN** the browser requests `GET /download/<name>` for that filename
- **THEN** the server returns the file contents

#### Scenario: Path traversal is rejected
- **GIVEN** a download request attempts to address a path outside the upload directory
- **WHEN** the request is processed
- **THEN** the server returns not found

### Requirement: Automatic Retention Cleanup
The system SHALL remove uploaded files older than the configured retention period.

#### Scenario: Server starts
- **WHEN** any supported runtime starts
- **THEN** it creates the upload directory if needed
- **AND** it attempts to remove files older than `retentionDays`

#### Scenario: Server keeps running
- **WHEN** the server remains active
- **THEN** it periodically attempts retention cleanup without requiring a restart
