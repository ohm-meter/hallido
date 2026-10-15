# ADR 0002: Own lightweight SOAP layer for Sonos

Status: Accepted · Date: 2026-05-28

## Context
Sonos is controlled locally over UPnP/SOAP. Existing Go libraries
(ianr0bkny/go-sonos, ninjasphere/go-sonos) have been unmaintained since
2017-2020 and cover the entire UPnP surface, which is more than needed.

## Decision
A custom, lightweight SOAP client (increment 2) that covers only the required
commands: SSDP discovery, grouping (AVTransport), volume (RenderingControl),
starting playback, and reading status.

## Rationale
Fully maintainable, easy to oversee, no legacy baggage, and a good fit for the
existing SOAP/WSDL knowledge. The old libraries serve only as a reference for
the exact SOAP envelopes.

## Consequences
Somewhat more initial effort than a ready-made library, but more stable in the
long run and free of external dependencies. The adapter sits behind
SpeakerController and can therefore be tested in isolation (SOAP mock).
