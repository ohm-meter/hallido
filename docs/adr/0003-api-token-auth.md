# ADR 0003: API token for access protection

Status: Accepted · Date: 2026-05-28

## Context
The agent controls home devices and will later be called by Homey. Even on the
LAN, not every request should control devices unchecked (OWASP).

## Decision
Bearer token for all state-changing and reading application endpoints. Health
and the GUI page itself stay open. Token comparison in constant time
(crypto/subtle). The token comes from the config or env (SRA_API_TOKEN), never
from code. The placeholder value is rejected at startup.

## Rationale
Simple, sufficient for LAN operation, and easy for Homey to use (one header).
HTTPS can be added later if access over untrusted networks becomes necessary
(separate ADR).

## Consequences
The GUI stores the token locally in the browser and sends it as a Bearer
header. On a 401 it asks for the token again.
