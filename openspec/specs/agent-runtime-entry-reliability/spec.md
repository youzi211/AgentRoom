# agent-runtime-entry-reliability Specification

## Purpose
TBD - created by archiving change fix-agent-runtime-entry-bugs. Update Purpose after archive.
## Requirements
### Requirement: Runtime artifacts persist in all dialogue modes
The system SHALL persist and expose runtime-produced message artifacts for agent replies generated through both mention fanout and guided dialogue modes.

#### Scenario: Guided dialogue stores DeepAgent report artifact
- **WHEN** a guided dialogue agent runtime returns a response with a Markdown report artifact
- **THEN** the saved agent message includes that artifact in its `artifacts` collection

#### Scenario: Mention fanout continues storing runtime artifacts
- **WHEN** a mention fanout agent runtime returns a response with one or more artifacts
- **THEN** the saved agent message includes those artifacts unchanged except for normal defaulting of missing artifact IDs or filenames

### Requirement: Entry page uses non-admin recent room discovery
The system SHALL allow the entry page to retrieve a limited list of recent active rooms without requiring admin credentials.

#### Scenario: Ordinary user loads entry page recent rooms
- **WHEN** the entry page requests recent active rooms without an admin API key
- **THEN** the backend returns a successful response containing only safe public room summary fields

#### Scenario: Public room summary matches entry page fields
- **WHEN** the entry page renders a recent room returned by the public listing response
- **THEN** room name, room ID, dialogue mode, agent count, passcode status, and room status can be read from fields present in the response

#### Scenario: Admin room listing stays protected
- **WHEN** `ADMIN_API_KEY` is configured and a request calls the admin room listing route without the admin key
- **THEN** the backend rejects that admin listing request

### Requirement: DeepAgent question text is isolated from CLI options
The system SHALL pass user-controlled DeepAgent question text to the Python CLI so it cannot be interpreted as CLI options.

#### Scenario: Question starts with option-like text
- **WHEN** a DeepAgent runtime receives a question beginning with `--`
- **THEN** the subprocess argv separates runtime options from the question positional argument

### Requirement: DeepAgent subprocess execution is concurrency bounded
The system SHALL bound concurrent DeepAgent subprocess executions inside a backend process.

#### Scenario: Multiple DeepAgent responses start concurrently
- **WHEN** more DeepAgent runtime calls are requested than the configured concurrency limit allows
- **THEN** excess calls wait for capacity instead of starting additional subprocesses immediately

#### Scenario: Waiting runtime call observes cancellation
- **WHEN** a DeepAgent runtime call is waiting for concurrency capacity and its context is canceled or times out
- **THEN** the call returns without starting a subprocess

