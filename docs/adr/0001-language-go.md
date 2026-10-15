# ADR 0001: Go as the backend language

Status: Accepted · Date: 2026-05-28

## Context
The agent runs on a Raspberry Pi as a long-running service, provides a GUI and
a REST API, and will later be used by home automation tools. A single backend
language was a requirement. Candidates: Go, PHP, Node.js.

## Decision
Go.

## Rationale
A static binary without a runtime is ideal on the Pi (low resource use, no
PHP-FPM/web server, no node_modules). Cross-compiling for the Pi is a build
flag. Go ships with a test runner, formatter, and vet, which suits multiple
agents and consistent standards. Local Sonos control runs over UPnP/SOAP,
which is language-independent and easy to implement with existing SOAP
knowledge.

## Consequences
PHP (the operator's strongest language) is not used here; instead, the project
doubles as a Go learning vehicle for Pi/IoT. The GUI still has to be
HTML/CSS/JS, but without a build pipeline and served from the Go binary.
