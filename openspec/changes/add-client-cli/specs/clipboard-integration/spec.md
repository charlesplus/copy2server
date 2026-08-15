## Purpose

Define how the CLI reads uploadable data from the operating system clipboard and writes returned server paths back to the clipboard across supported desktop platforms.

## ADDED Requirements

### Requirement: Clipboard Text Support
The clipboard integration SHALL support plain text clipboard content.

#### Scenario: Plain text is uploaded as a text file
- **GIVEN** the clipboard contains plain text
- **WHEN** the CLI reads the clipboard for upload
- **THEN** it produces an uploadable text file payload
- **AND** the payload preserves the clipboard text content

### Requirement: Clipboard File Reference Support
The clipboard integration SHALL support file references exposed by the operating system clipboard.

#### Scenario: Clipboard references local files
- **GIVEN** the clipboard contains one or more readable local file references
- **WHEN** the CLI reads the clipboard for upload
- **THEN** it produces upload payloads from the referenced files

#### Scenario: Clipboard references an unreadable file
- **GIVEN** the clipboard contains a file reference that cannot be read
- **WHEN** the CLI reads the clipboard for upload
- **THEN** it reports an error instead of silently skipping the referenced file

### Requirement: Clipboard Image Support
The clipboard integration SHALL support image data exposed by the operating system clipboard.

#### Scenario: Clipboard contains image data
- **GIVEN** the clipboard contains readable image data
- **WHEN** the CLI reads the clipboard for upload
- **THEN** it produces an uploadable image payload
- **AND** assigns a filename with an image extension matching the readable image format when known

### Requirement: Clipboard Rich and Binary Format Support
The clipboard integration SHALL support non-text formats that the operating system exposes as readable data.

#### Scenario: Clipboard contains readable rich text
- **GIVEN** the clipboard exposes HTML or RTF content
- **WHEN** the CLI reads the clipboard for upload
- **THEN** it produces an uploadable payload with a matching rich text extension when the format is identifiable

#### Scenario: Clipboard contains readable binary data
- **GIVEN** the clipboard exposes binary data with a known MIME type or format name
- **WHEN** the CLI reads the clipboard for upload
- **THEN** it produces an uploadable binary payload
- **AND** uses a filename extension inferred from the content type when possible

#### Scenario: Clipboard contains unsupported private data
- **GIVEN** the clipboard contains only proprietary or private formats not exposed as readable data
- **WHEN** the CLI reads the clipboard for upload
- **THEN** it reports that no uploadable clipboard content is available

### Requirement: Clipboard Path Write-Back
The clipboard integration SHALL write uploaded server paths as plain text.

#### Scenario: Single path is written
- **WHEN** the CLI requests clipboard write-back for one uploaded file
- **THEN** the clipboard contains that server path as plain text

#### Scenario: Multiple paths are written
- **WHEN** the CLI requests clipboard write-back for multiple uploaded files
- **THEN** the clipboard contains the server paths as newline-separated plain text
