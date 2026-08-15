## MODIFIED Requirements

### Requirement: File Upload API
The system SHALL accept multipart file uploads from browser and non-browser clients and persist files to the configured upload directory.

#### Scenario: Multipart upload succeeds
- **GIVEN** a `POST /api/upload` request contains one or more `file` parts within the configured size limit
- **WHEN** the upload completes
- **THEN** each file is stored under a unique sanitized filename in the upload directory
- **AND** the response status is `201`
- **AND** the JSON response contains a `files` array with `name`, `size`, `modifiedAt`, `serverPath`, `downloadUrl`, and `isImage` for each stored file

#### Scenario: CLI multipart upload succeeds
- **GIVEN** a non-browser client sends `POST /api/upload` with one or more multipart `file` parts
- **WHEN** the upload completes
- **THEN** the server returns the same `201` response shape used by browser uploads
- **AND** each returned `serverPath` is suitable for CLI stdout and clipboard write-back

#### Scenario: Upload request has no files
- **GIVEN** a `POST /api/upload` request contains no usable `file` parts
- **WHEN** the request is processed
- **THEN** the response is a client error with a JSON `error` message

#### Scenario: Upload request is not multipart
- **GIVEN** a `POST /api/upload` request is not `multipart/form-data`
- **WHEN** the request is processed
- **THEN** the response is a client error with a JSON `error` message
