package sonos

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"unicode"

	"sonos-radio-agent/internal/apperr"
)

var (
	reSOAPFaultString = regexp.MustCompile(`(?i)<(?:[\w-]+:)?faultstring[^>]*>([^<]+)</`)
	reSOAPErrorCode   = regexp.MustCompile(`(?i)<(?:[\w-]+:)?errorCode[^>]*>([^<]+)</`)
)

// service beschreibt einen UPnP-Service eines Sonos-Geraets.
type service struct {
	controlPath string // SOAP-Endpoint unterhalb http://IP:1400
	serviceType string // UPnP urn (Teil von SOAPACTION)
}

var (
	// avTransport steuert Wiedergabe und Gruppierung.
	avTransport = service{
		controlPath: "/MediaRenderer/AVTransport/Control",
		serviceType: "urn:schemas-upnp-org:service:AVTransport:1",
	}
	// renderingControl steuert die Lautstaerke.
	renderingControl = service{
		controlPath: "/MediaRenderer/RenderingControl/Control",
		serviceType: "urn:schemas-upnp-org:service:RenderingControl:1",
	}
	contentDirectory = service{
		controlPath: "/MediaServer/ContentDirectory/Control",
		serviceType: "urn:schemas-upnp-org:service:ContentDirectory:1",
	}
	zoneGroupTopology = service{
		controlPath: "/ZoneGroupTopology/Control",
		serviceType: "urn:schemas-upnp-org:service:ZoneGroupTopology:1",
	}
	deviceProperties = service{
		controlPath: "/DeviceProperties/Control",
		serviceType: "urn:schemas-upnp-org:service:DeviceProperties:1",
	}
)

// arg ist ein SOAP-Argument. Die Reihenfolge ist relevant (UPnP erwartet die
// Argumente in der definierten Folge), daher ein Slice statt einer Map.
type arg struct{ Name, Value string }

// soapCall fuehrt eine UPnP-SOAP-Aktion auf einem Geraet aus und liefert den
// Antwort-Body. Transportfehler werden als "unreachable", abgelehnte Aktionen
// als "playback_failed" zurueckgegeben -- damit der Aufrufer sinnvoll
// unterscheiden kann.
func (c *Controller) soapCall(ctx context.Context, d *device, svc service, action string, args []arg) ([]byte, error) {
	return c.soapCallClient(ctx, c.client, d, svc, action, args)
}

// soapCallClient wie soapCall, mit eigenem HTTP-Client (Zeitlimit).
func (c *Controller) soapCallClient(ctx context.Context, client *http.Client, d *device, svc service, action string, args []arg) ([]byte, error) {
	body := buildEnvelope(svc.serviceType, action, args)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.baseURL+svc.controlPath, strings.NewReader(body))
	if err != nil {
		return nil, apperr.New(apperr.CodeInternal, "build soap request: "+err.Error())
	}
	req.Header.Set("Content-Type", `text/xml; charset="utf-8"`)
	req.Header.Set("SOAPACTION", fmt.Sprintf(`"%s#%s"`, svc.serviceType, action))

	resp, err := client.Do(req)
	if err != nil {
		ip := ipFromBaseURL(d.baseURL)
		hint := fmt.Sprintf("No SOAP response from %s:1400 — speaker powered on and on the same network?", ip)
		switch {
		case errors.Is(err, context.DeadlineExceeded):
			hint = fmt.Sprintf("Timeout at %s:1400 — speaker may be asleep, try again.", ip)
		case errors.Is(err, context.Canceled):
			hint = fmt.Sprintf("Request to %s:1400 aborted (server timeout) — with many speakers, restart the scene.", ip)
		}
		return nil, apperr.New(apperr.CodeSpeakerUnreachable,
			"Speaker is not responding").
			WithSpeaker(d.name).
			WithHint(hint)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	if resp.StatusCode != http.StatusOK {
		return nil, apperr.New(apperr.CodePlaybackFailed,
			fmt.Sprintf("Device rejected %s (HTTP %d): %s", action, resp.StatusCode, soapFault(respBody))).
			WithSpeaker(d.name)
	}
	return respBody, nil
}

// buildEnvelope erzeugt einen SOAP-1.1-Envelope. ALLE Argumentwerte werden
// XML-escaped -- das verhindert Injection ueber Quellen wie Stream-URLs, die
// & oder < enthalten koennen (OWASP: Injection).
func buildEnvelope(serviceType, action string, args []arg) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="utf-8"?>`)
	b.WriteString(`<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/"><s:Body>`)
	fmt.Fprintf(&b, `<u:%s xmlns:u="%s">`, action, serviceType)
	for _, a := range args {
		fmt.Fprintf(&b, "<%s>%s</%s>", a.Name, escapeXML(a.Value), a.Name)
	}
	fmt.Fprintf(&b, `</u:%s></s:Body></s:Envelope>`, action)
	return b.String()
}

// escapeXML escaped einen Wert fuer die Verwendung als XML-Textinhalt.
func escapeXML(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

// soapFault zieht eine knappe Ursache aus einer UPnP-Fault-Antwort.
func soapFault(body []byte) string {
	var f struct {
		ErrorCode   string `xml:"Body>Fault>detail>UPnPError>errorCode"`
		FaultString string `xml:"Body>Fault>faultstring"`
	}
	if err := xml.Unmarshal(body, &f); err == nil {
		if f.ErrorCode != "" {
			msg := "UPnP error code " + f.ErrorCode
			if fs := strings.TrimSpace(f.FaultString); fs != "" {
				return msg + ": " + fs
			}
			return msg
		}
		if fs := strings.TrimSpace(f.FaultString); fs != "" {
			return fs
		}
	}
	if m := reSOAPFaultString.FindSubmatch(body); len(m) > 1 {
		if fs := strings.TrimSpace(string(m[1])); fs != "" {
			return fs
		}
	}
	if m := reSOAPErrorCode.FindSubmatch(body); len(m) > 1 {
		code := strings.TrimSpace(string(m[1]))
		if code != "" {
			return "UPnP error code " + code
		}
	}
	if snippet := soapBodySnippet(body); snippet != "" {
		return "Response: " + snippet
	}
	return "unexpected device response"
}

func soapBodySnippet(body []byte) string {
	s := strings.TrimSpace(string(body))
	if s == "" {
		return ""
	}
	s = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == '\t' {
			return ' '
		}
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
	s = strings.Join(strings.Fields(s), " ")
	const maxLen = 200
	if len(s) > maxLen {
		return s[:maxLen] + "…"
	}
	return s
}

// isSoftStatusSOAPError ist true, wenn ein Geraet eine Statusabfrage ablehnt
// (z. B. TV im Leerlauf mit HTTP 500), aber erreichbar ist.
func isSoftStatusSOAPError(err error) bool {
	return apperr.As(err).Code == apperr.CodePlaybackFailed
}
