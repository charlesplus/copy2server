# runtime-configuration Specification

## Purpose
Define how copy2server is configured and how the supported runtimes stay aligned.

## Requirements

### Requirement: Shared Configuration File
The system SHALL read default runtime settings from a JSON configuration file.

#### Scenario: Default config file is used
- **GIVEN** no `CONFIG` environment variable is set
- **WHEN** any supported runtime starts
- **THEN** it reads `config.json` from the working directory when that file exists

#### Scenario: Alternate config file is selected
- **GIVEN** `CONFIG` is set to a JSON file path
- **WHEN** any supported runtime starts
- **THEN** it reads configuration from that file path

#### Scenario: Missing config falls back to defaults
- **GIVEN** the selected config file does not exist
- **WHEN** any supported runtime starts
- **THEN** it uses built-in defaults for all configuration keys

### Requirement: Environment Overrides
The system SHALL allow environment variables to override file-based configuration.

#### Scenario: Network address is overridden
- **GIVEN** `ADDR` is set
- **WHEN** any supported runtime starts
- **THEN** it listens on the configured address instead of the file value

#### Scenario: Upload settings are overridden
- **GIVEN** `UPLOAD_DIR`, `MAX_UPLOAD_MB`, or `RETENTION_DAYS` is set
- **WHEN** any supported runtime starts
- **THEN** the matching upload directory, upload limit, or retention period uses the environment value

#### Scenario: Template path is overridden
- **GIVEN** `INDEX_HTML` is set
- **WHEN** any supported runtime serves `GET /`
- **THEN** it renders the template at that path

### Requirement: Runtime Parity
The system SHALL expose the same user-facing behavior from the Go, Python3, and Node.js implementations.

#### Scenario: A runtime is selected
- **GIVEN** the user starts `go run .`, `python3 server.py`, or `node server.js`
- **WHEN** the server is ready
- **THEN** the same HTTP routes, JSON response shapes, upload directory behavior, and template variables are available

#### Scenario: No package manager dependencies are required for fallback runtimes
- **WHEN** the user starts the Python3 or Node.js implementation
- **THEN** the implementation runs without installing pip or npm packages
