## ADDED Requirements

### Requirement: Client Server URL Configuration
The system SHALL provide a client-facing server URL for the CLI without changing server listen address semantics.

#### Scenario: CLI server URL is provided by command line
- **GIVEN** the user passes `--server http://example.test:8282`
- **WHEN** the CLI uploads content
- **THEN** it sends the upload request to `http://example.test:8282/api/upload`

#### Scenario: CLI server URL is provided by environment
- **GIVEN** `COPY2SERVER_URL` is set and `--server` is not provided
- **WHEN** the CLI uploads content
- **THEN** it sends the upload request to `COPY2SERVER_URL` with `/api/upload`

#### Scenario: CLI server URL is provided by config file
- **GIVEN** `client.config.json` contains `serverUrl` and neither `--server` nor `COPY2SERVER_URL` is provided
- **WHEN** the CLI uploads content
- **THEN** it sends the upload request to `serverUrl` with `/api/upload`

#### Scenario: CLI server URL falls back to localhost
- **GIVEN** no CLI server URL is provided by command line, environment, or config file
- **WHEN** the CLI uploads content
- **THEN** it sends the upload request to `http://127.0.0.1:8282/api/upload`

#### Scenario: Server listen address remains independent
- **GIVEN** `addr` is configured for a server runtime
- **WHEN** the server runtime starts
- **THEN** `addr` controls the server listen address
- **AND** `serverUrl` does not change the listen address
