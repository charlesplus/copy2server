## Purpose

Provide a terminal-first client that uploads clipboard content or explicit local files to copy2server and returns server-side file paths without using the browser UI.

## ADDED Requirements

### Requirement: Default Clipboard Upload
The CLI SHALL read the system clipboard and upload readable clipboard content when no explicit file path is provided.

#### Scenario: Clipboard upload succeeds
- **GIVEN** the clipboard contains readable content
- **WHEN** the user runs `copy2server` with no file arguments
- **THEN** the CLI uploads that content to `POST /api/upload`
- **AND** prints each returned `serverPath` to stdout
- **AND** writes the printed server paths back to the clipboard by default

#### Scenario: Clipboard has no uploadable content
- **GIVEN** the clipboard is empty or contains no format the CLI can read
- **WHEN** the user runs `copy2server` with no file arguments
- **THEN** the CLI prints an error to stderr
- **AND** exits with a non-zero status
- **AND** does not overwrite the clipboard

### Requirement: Explicit File Upload
The CLI SHALL upload explicit file paths instead of reading the clipboard.

#### Scenario: Positional file is uploaded
- **GIVEN** `./a.png` exists
- **WHEN** the user runs `copy2server ./a.png`
- **THEN** the CLI uploads `./a.png` to `POST /api/upload`
- **AND** prints the returned `serverPath` to stdout
- **AND** writes the returned `serverPath` back to the clipboard by default

#### Scenario: File flag is uploaded
- **GIVEN** `./a.png` exists
- **WHEN** the user runs `copy2server --file ./a.png`
- **THEN** the CLI uploads `./a.png` to `POST /api/upload`
- **AND** does not read the clipboard

#### Scenario: Multiple files are uploaded
- **GIVEN** all provided file paths exist
- **WHEN** the user runs `copy2server a.png b.txt`
- **THEN** the CLI uploads all provided files in one multipart request when possible
- **AND** prints one returned `serverPath` per line
- **AND** writes the same newline-separated paths back to the clipboard by default

### Requirement: CLI Path Output and Clipboard Write-Back
The CLI SHALL make uploaded server paths available to both scripts and interactive users.

#### Scenario: Copy is enabled by default
- **WHEN** an upload succeeds without `--no-copy`
- **THEN** stdout contains the returned server paths
- **AND** the system clipboard is replaced with those same paths

#### Scenario: Copy is disabled
- **WHEN** an upload succeeds with `--no-copy`
- **THEN** stdout contains the returned server paths
- **AND** the CLI does not replace the system clipboard

#### Scenario: Copy is requested explicitly
- **WHEN** an upload succeeds with `--copy`
- **THEN** stdout contains the returned server paths
- **AND** the system clipboard is replaced with those same paths

### Requirement: CLI Upload Naming
The CLI SHALL allow users to provide a filename for clipboard-derived uploads.

#### Scenario: Clipboard text uses provided name
- **GIVEN** the clipboard contains text
- **WHEN** the user runs `copy2server --name note.txt`
- **THEN** the CLI uploads the clipboard text using `note.txt` as the multipart filename

#### Scenario: Clipboard content uses generated name
- **GIVEN** the clipboard contains readable content and no `--name` value is provided
- **WHEN** the user runs `copy2server`
- **THEN** the CLI uploads the content with a generated filename appropriate for the detected content type

### Requirement: CLI Failure Behavior
The CLI SHALL fail predictably without destroying user clipboard data.

#### Scenario: Server upload fails
- **GIVEN** the selected server returns an upload error
- **WHEN** the CLI uploads content
- **THEN** the CLI prints the server error to stderr
- **AND** exits with a non-zero status
- **AND** does not overwrite the clipboard

#### Scenario: File path is invalid
- **GIVEN** a provided file path does not exist or is not readable
- **WHEN** the user runs `copy2server <path>`
- **THEN** the CLI prints an error to stderr
- **AND** exits with a non-zero status
- **AND** does not make an upload request
