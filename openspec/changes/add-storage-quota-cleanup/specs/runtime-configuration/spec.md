## ADDED Requirements

### Requirement: Upload Storage Quota Configuration
The system SHALL allow operators to configure a maximum total storage size for uploaded files.

#### Scenario: Default upload storage quota is used
- **GIVEN** no upload storage quota is configured
- **WHEN** any supported server runtime starts
- **THEN** the upload storage quota defaults to `5G`

#### Scenario: Upload storage quota is configured by file
- **GIVEN** `server.config.json` contains `maxStorageGB`
- **WHEN** any supported server runtime starts
- **THEN** the runtime uses `maxStorageGB` as the upload directory storage quota in gigabytes

#### Scenario: Upload storage quota is configured by environment
- **GIVEN** `MAX_STORAGE_GB` is set
- **WHEN** any supported server runtime starts
- **THEN** the runtime uses `MAX_STORAGE_GB` instead of the file value

#### Scenario: Invalid upload storage quota falls back to default
- **GIVEN** the configured upload storage quota is missing, zero, negative, or not numeric
- **WHEN** any supported server runtime starts
- **THEN** the upload storage quota defaults to `5G`
