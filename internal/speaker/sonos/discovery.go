package sonos

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"sonos-radio-agent/internal/apperr"
)

// sonosPort ist der feste lokale Steuerport jedes Sonos-Geraets.
const sonosPort = 1400

// fixedIPProbeTimeout begrenzt Wartezeit pro feste IP. Falsche oder veraltete
// Eintraege duerfen SSDP nicht blockieren (frueher: volle Client-Timeout 5 s).
const fixedIPProbeTimeout = 800 * time.Millisecond

// ssdpSearchTimeout ist die SSDP-Multicast-Wartezeit.
const ssdpSearchTimeout = 2 * time.Second

// ssdpFetchConcurrency limitiert parallele Geraetebeschreibungs-Abrufe nach SSDP.
const ssdpFetchConcurrency = 6

// ssdpSearchHook ersetzt ssdpSearch in Tests (nil = Produktion).
var ssdpSearchHook func(time.Duration) ([]string, error)

// ssdpSearch sucht per SSDP-Multicast nach Sonos-ZonePlayern und liefert die
// LOCATION-URLs der Geraetebeschreibungen.
//
// Multicast kann je nach Router/VLAN scheitern -- in dem Fall liefert die
// Funktion einfach eine leere Liste, und die Fixed-IP-Overrides aus der Config
// uebernehmen.
func ssdpSearch(timeout time.Duration) ([]string, error) {
	mcast, err := net.ResolveUDPAddr("udp4", "239.255.255.250:1900")
	if err != nil {
		return nil, err
	}
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	msg := strings.Join([]string{
		"M-SEARCH * HTTP/1.1",
		"HOST: 239.255.255.250:1900",
		`MAN: "ssdp:discover"`,
		"MX: 1",
		"ST: urn:schemas-upnp-org:device:ZonePlayer:1",
		"", "",
	}, "\r\n")
	if _, err := conn.WriteToUDP([]byte(msg), mcast); err != nil {
		return nil, err
	}

	_ = conn.SetReadDeadline(time.Now().Add(timeout))
	seen := map[string]bool{}
	var locations []string
	buf := make([]byte, 2048)
	for {
		n, from, err := conn.ReadFromUDP(buf)
		if err != nil {
			break // Lese-Timeout -> Suche beendet
		}
		resp := string(buf[:n])
		if !strings.Contains(strings.ToLower(headerValue(resp, "ST")), "zoneplayer") {
			continue
		}
		if loc := headerValue(resp, "LOCATION"); loc != "" && !seen[loc] && locationMatchesSender(loc, from) {
			seen[loc] = true
			locations = append(locations, loc)
		}
	}
	return locations, nil
}

// locationMatchesSender stellt sicher, dass die LOCATION auf den antwortenden
// Host zeigt -- sonst koennte ein LAN-Teilnehmer den Agenten auf fremde URLs lenken.
func locationMatchesSender(loc string, from *net.UDPAddr) bool {
	u, err := url.Parse(loc)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || from == nil {
		return false
	}
	return u.Hostname() == from.IP.String()
}

// headerValue liest einen HTTP-Header-Wert aus einer rohen SSDP-Antwort.
func headerValue(resp, key string) string {
	for _, line := range strings.Split(resp, "\r\n") {
		if i := strings.IndexByte(line, ':'); i > 0 {
			if strings.EqualFold(strings.TrimSpace(line[:i]), key) {
				return strings.TrimSpace(line[i+1:])
			}
		}
	}
	return ""
}

// deviceDescription ist der relevante Ausschnitt von device_description.xml.
type deviceDescription struct {
	Room string `xml:"device>roomName"`
	UDN  string `xml:"device>UDN"`
}

// fetchDevice laedt die Geraetebeschreibung von baseURL und baut ein device.
func fetchDevice(ctx context.Context, client *http.Client, baseURL string) (*device, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/xml/device_description.xml", nil)
	if err != nil {
		return nil, apperr.New(apperr.CodeInternal, "build request: "+err.Error())
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, apperr.New(apperr.CodeSpeakerUnreachable,
			"Device description unreachable: "+err.Error())
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, apperr.New(apperr.CodeSpeakerUnreachable,
			fmt.Sprintf("Device description: HTTP %d", resp.StatusCode))
	}
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	var d deviceDescription
	if err := xml.Unmarshal(raw, &d); err != nil {
		return nil, apperr.New(apperr.CodeInternal, "parse device description: "+err.Error())
	}
	if d.Room == "" {
		return nil, apperr.New(apperr.CodeInternal,
			"device description without roomName -- is this really a Sonos device?")
	}
	return &device{
		name:    d.Room,
		baseURL: baseURL,
		uid:     strings.TrimPrefix(d.UDN, "uuid:"),
	}, nil
}

// baseURLFromLocation macht aus einer SSDP-LOCATION die Geraete-Basis-URL.
// Beispiel: http://192.168.0.10:1400/xml/device_description.xml -> http://192.168.0.10:1400
func baseURLFromLocation(loc string) string {
	if i := strings.Index(loc, "/xml/"); i > 0 {
		return loc[:i]
	}
	return loc
}

// baseURLFromIP baut die Basis-URL aus einer festen IP (Config-Override).
func baseURLFromIP(ip string) string {
	return fmt.Sprintf("http://%s:%d", ip, sonosPort)
}

func runSSDPSearch(timeout time.Duration) ([]string, error) {
	if ssdpSearchHook != nil {
		return ssdpSearchHook(timeout)
	}
	return ssdpSearch(timeout)
}

// probeFixedIPs prueft alle Config-Overrides parallel mit kurzem Timeout.
func (c *Controller) probeFixedIPs(ctx context.Context) map[string]*device {
	overrides := c.fixedOverrides()
	if len(overrides) == 0 {
		return map[string]*device{}
	}

	found := make(map[string]*device, len(overrides))
	var mu sync.Mutex
	var wg sync.WaitGroup
	for name, ip := range overrides {
		if ip == "" {
			continue
		}
		wg.Add(1)
		go func(name, ip string) {
			defer wg.Done()
			probeCtx, cancel := context.WithTimeout(ctx, fixedIPProbeTimeout)
			defer cancel()
			d, err := fetchDevice(probeCtx, c.client, baseURLFromIP(ip))
			if err != nil {
				c.noteProbeFailed(name, ip, err)
				return
			}
			c.noteProbeOK(name, ip)
			d.name = name
			mu.Lock()
			found[name] = d
			mu.Unlock()
		}(name, ip)
	}
	wg.Wait()
	return found
}

// fetchDevicesFromLocations laedt Geraetebeschreibungen parallel (begrenzt).
func fetchDevicesFromLocations(ctx context.Context, client *http.Client, locations []string) map[string]*device {
	if len(locations) == 0 {
		return map[string]*device{}
	}

	found := make(map[string]*device, len(locations))
	var mu sync.Mutex
	sem := make(chan struct{}, ssdpFetchConcurrency)
	var wg sync.WaitGroup

	for _, loc := range locations {
		wg.Add(1)
		go func(loc string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			d, err := fetchDevice(ctx, client, baseURLFromLocation(loc))
			if err != nil {
				return
			}
			mu.Lock()
			if _, exists := found[d.name]; !exists {
				found[d.name] = d
			}
			mu.Unlock()
		}(loc)
	}
	wg.Wait()
	return found
}
