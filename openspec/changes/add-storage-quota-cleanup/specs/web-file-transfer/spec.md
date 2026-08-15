## MODIFIED Requirements

### Requirement: Automatic Retention Cleanup
The system SHALL remove uploaded files that exceed configured retention or storage limits after successful upload writes.

#### Scenario: Server starts
- **WHEN** any supported runtime starts
- **THEN** it creates the upload directory if needed
- **AND** it does not trigger retention or storage quota cleanup solely because the server started

#### Scenario: Server keeps running
- **WHEN** the server remains active without successful upload writes
- **THEN** it does not require periodic retention or storage quota cleanup

#### Scenario: Upload succeeds
- **WHEN** a `POST /api/upload` request successfully writes one or more files
- **THEN** the server schedules cleanup asynchronously after the response data is available
- **AND** the upload response is not blocked on the cleanup completing

#### Scenario: Retention limit is exceeded
- **GIVEN** the upload directory contains ordinary files older than `retentionDays`
- **WHEN** cleanup runs after a successful upload write
- **THEN** the server removes ordinary uploaded files older than `retentionDays`

#### Scenario: Storage quota is exceeded
- **GIVEN** the upload directory contains files whose total size exceeds the configured storage quota
- **WHEN** cleanup runs after a successful upload write
- **THEN** the server removes ordinary uploaded files by oldest modification time first
- **AND** cleanup stops when total ordinary file size is less than or equal to the configured storage quota or no removable ordinary files remain

#### Scenario: Newly uploaded file is larger than storage quota
- **GIVEN** an upload request contains a file larger than the configured storage quota
- **WHEN** the request is processed
- **THEN** the server rejects the upload with a client error JSON `error` message
- **AND** does not store the oversized file

#### Scenario: Cleanup does not overlap itself
- **GIVEN** a cleanup run is already in progress
- **WHEN** another upload succeeds
- **THEN** the server does not start a second concurrent cleanup run for the same upload directory
