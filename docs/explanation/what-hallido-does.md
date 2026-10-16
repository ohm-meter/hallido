# What Hallido does on your network and your NAS

This page lists everything Hallido does on your machine and in your network:
open ports, outgoing connections, files it writes, rights it needs and data
it stores. It describes the container image (NAS, Docker); the systemd
installation is mentioned where it differs. A short German version is at the
[end of this page](#kurzfassung-deutsch).

Background: [ADR 0021](../adr/0021-container-image-supply-chain.md) (image,
signature), [ADR 0022](../adr/0022-first-run-setup.md) (setup),
[ADR 0023](../adr/0023-operations-log.md) (log),
[SECURITY.md](https://github.com/ohm-meter/hallido/blob/master/SECURITY.md).

## At a glance

| Question | Answer |
|---|---|
| Where does the software come from? | Only from our releases: image `ghcr.io/ohm-meter/hallido`, pinned by digest and signed — [verify it](../how-to/verify-image.md) |
| Does it need the internet? | No. Only for the optional station search and to check stream addresses you entered |
| Does it phone home, send telemetry or check for updates? | No |
| Does it need an account (Sonos, cloud, ours)? | No |
| Which rights in the container? | Normal user 65532, no capabilities, read-only system, only `/data` writable |
| Reachable from the internet? | Not by itself. Do not forward its port on your router |

## Open ports (listening)

| Port | Protocol | Where | Purpose |
|---|---|---|---|
| 8790 | TCP (HTTP) | all network interfaces of the NAS (host network) | Web app and REST API. Change with `HALLIDO_PORT` / `SRA_LISTEN` (systemd: `listen` in `config.json`, default 8080) |
| random high port | UDP | all interfaces, only during a speaker search | Receives the answers of the Sonos speakers to the discovery request (SSDP); closed after a few seconds |

Why host networking: Sonos speakers are found with a multicast message
(SSDP), which does not pass the default Docker bridge network. With host
networking the container uses the NAS's network directly, therefore also
port 8790 directly on the NAS.

Without the access key only these answer: the web app files, `GET
/api/v1/health` (status and the names of the speakers found, meant for
monitoring), and during first-run setup `GET /api/v1/setup` and — once, only
from your home network, at most one hour after start — `POST
/api/v1/setup/token`. Everything else needs the key.

## Outgoing connections

| Destination | Protocol / port | When | Can I turn it off? |
|---|---|---|---|
| Multicast `239.255.255.250` | UDP 1900 (SSDP) | Speaker search: when the web app lists speakers, when you search again, when a scene needs a speaker that is not known yet | No — this is how speakers are found. Speakers with a fixed IP in the config are contacted directly |
| Your Sonos speakers in the LAN | HTTP TCP 1400 (UPnP/SOAP) | Every action: play, group, volume, status (the web app polls every few seconds while it is open) | No — this is the control channel |
| Radio Browser directory: `de1.api.radio-browser.info`, `de2.api.radio-browser.info`, `fi1.api.radio-browser.info` | HTTPS 443 | Only when you type into the station search of the scene editor | Yes: `HALLIDO_RADIO_SEARCH=0` (compose) / `SRA_RADIO_SEARCH=0`. Then enter stream addresses yourself |
| The radio station of a scene (the stream address you chose) | HTTP/HTTPS | Shortly before a scene starts: Hallido reads the first bytes to recognise the format (MP3/AAC/HLS) and, if playback fails, to explain why | Not separately; it only contacts addresses you put into your own scenes |

The audio itself is fetched by the **Sonos speakers**, not by Hallido.
Hallido never contacts Sonos's cloud, our servers, analytics, ad networks,
update servers or CDNs. The web app loads no external files (fonts, scripts,
images); a Content Security Policy (`default-src 'self'`) enforces this in the
browser. Station logos from the search are not loaded.

Installing and updating downloads the image from `ghcr.io` (GitHub's
container registry). That is done by Container Manager / Container Station,
not by Hallido.

## Files Hallido writes

In the container only the data volume `/data` (named volume `hallido-data`)
is writable:

| File | Content | Rights |
|---|---|---|
| `/data/config.json` | Access key, scenes (names, stream addresses, speakers, volumes), schedules, time zone, optional fixed speaker IPs, listen address | `0600`, user 65532 |
| `/data/config.json.bak` | The previous version of `config.json` (one generation) | `0600` |
| `/data/.config-*.tmp` | Short-lived temporary file while saving (atomic replace) | `0600` |
| `/data/logs/hallido.log` and `.1` to `.3` | Setup and operations log, JSON lines, max. 1 MiB per file (≈ 4 MiB total) | `0600`, folder `0700` |

Nothing else is written: the root filesystem of the container is read-only.
The systemd installation writes the same files to `/var/lib/sonos-agent`
(without `logs/`; its log goes to journald).

The container platform additionally keeps its own console log of the
container (the same lines as the log file, without secrets).

## Rights

| | Container (NAS) | systemd |
|---|---|---|
| User | `65532:65532` (no root) | own system user `sonos` |
| Capabilities | none (`cap_drop: ALL`) | none (empty capability set) |
| Gaining rights | blocked (`no-new-privileges`) | blocked (`NoNewPrivileges`) |
| File system | read-only, except `/data` | `ProtectSystem=strict`, except `/var/lib/sonos-agent` |
| Limits | 128 MB memory, 64 processes, lower CPU weight | — |
| Docker socket, NAS folders, devices | none | — |
| Network | host network (needed for speaker discovery) | host |

## Data stored and who can see it

| Data | Where | Notes |
|---|---|---|
| Access key | `config.json` on the NAS; in the browser of each device you use (`localStorage`, key `sra_token`) | Generated on the NAS, shown once. Never written to the log |
| Scenes, schedules, speaker names, volumes, stream addresses | `config.json` | Visible to everyone with the access key (web app, API) |
| Speaker names | also via `GET /api/v1/health` | Readable without the key in your home network |
| Log | `/data/logs`, web app (Speakers → Log) | Time, event, scene/speaker names, errors; during setup the IP address of the device that set it up. Secrets are replaced by `[redacted]` |
| Browser settings | `localStorage`: theme (`sonos-theme`), window size flag (`sonos-window-sized`); the service worker caches the app files | Stays on your device |

Hallido stores no personal data beyond this, no listening history and no
statistics. All data stays on your NAS and your devices.

## How to check this yourself

- **Signature and digest of the image:** [Verify the image before
  installing](../how-to/verify-image.md).
- **Settings of the container:** the project file `compose.yaml` is short
  and commented; Container Manager / Container Station show user, network and
  volume of the running container.
- **What happened:** web app → Speakers → Log.
- **Source code:** everything is open source (MIT) at
  `github.com/ohm-meter/hallido`; one Go program without third-party
  libraries.

---

## Kurzfassung (Deutsch)

**Herkunft:** nur unsere Releases (`ghcr.io/ohm-meter/hallido`), per Digest
festgelegt und signiert – [prüfbar](../how-to/verify-image.md). Keine
Telemetrie, keine Update-Prüfung, kein Konto.

**Offene Ports:** TCP **8790** (Web-App und API, auf allen Netzwerkanschlüssen
der NAS, wegen Host-Netzwerk); kurzzeitig ein zufälliger UDP-Port für die
Antworten der Boxen bei der Suche.

**Ausgehende Verbindungen:** Multicast `239.255.255.250:1900` (Boxen suchen),
die Sonos-Boxen im Heimnetz auf TCP 1400 (Steuerung), **optional** die
Senderliste `*.api.radio-browser.info` über HTTPS (abschaltbar mit
`HALLIDO_RADIO_SEARCH=0`) und kurz vor dem Start einer Szene der Radiosender,
den du selbst eingetragen hast (Formatprüfung). Den Ton holen die Boxen
selbst.

**Geschriebene Dateien:** nur im Daten-Volume `/data`: `config.json`
(Zugangsschlüssel, Szenen, Zeitpläne, Zeitzone), `config.json.bak`, kurz eine
Temp-Datei beim Speichern, `logs/hallido.log` (+ bis zu 3 ältere Dateien,
zusammen ≈ 4 MB). Alles mit Rechten `0600`. Das übrige Dateisystem ist
schreibgeschützt.

**Rechte:** normaler Benutzer 65532, keine Capabilities, keine
Rechteausweitung, 128 MB Speicher, 64 Prozesse; kein Zugriff auf
Docker-Socket, NAS-Ordner oder Geräte.

**Gespeicherte Daten:** Zugangsschlüssel (auf der NAS und im Browser deiner
Geräte), Szenen, Zeitpläne, Boxennamen, Protokoll (ohne Geheimnisse; bei der
Einrichtung die IP-Adresse des einrichtenden Geräts). Alles bleibt auf deiner
NAS und deinen Geräten.
