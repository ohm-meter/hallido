# Wenn etwas nicht klappt

Fang hier an: Öffne Hallido und schau unter **Boxen → Protokoll**. Dort
steht, was Hallido gemacht hat und welche Probleme es gab.

## Die Seite öffnet sich nicht

- Nimm `http://` (nicht `https://`), die **Adresse deiner NAS** und am Ende
  **`:8790`**, z. B. `http://192.168.1.20:8790`.
- Läuft Hallido? Synology: **Container Manager → Projekt** – `hallido` muss
  grün/**Wird ausgeführt** sein. QNAP: **Container Station → Anwendungen** –
  `hallido` **Wird ausgeführt**. Wenn nicht, starte es.
- Ist dein Handy im WLAN zu Hause (nicht mobile Daten, nicht Gäste-WLAN)?
- **Firewall der NAS:** Hast du sie eingeschaltet, erlaube dein Heimnetz.
  Synology: **Systemsteuerung → Sicherheit → Firewall → Regeln bearbeiten →
  Erstellen**: Ports **Alle**, Quell-IP **Subnetz** deines Heimnetzes
  (z. B. `192.168.1.0` / `255.255.255.0`), Aktion **Zulassen**. QNAP: in der
  **QuFirewall** das lokale Netz zulassen.

## „Einrichtungszeit abgelaufen“

Zu deiner Sicherheit geht die Einrichtung nur in der ersten Stunde nach dem
Start von Hallido. Starte es neu – Synology: **Projekt → `hallido` → Aktion →
Neu starten**; QNAP: **Anwendungen → `hallido` → Neu starten** – und lade die
Seite neu.

## „Nur aus dem Heimnetz“

Du hast Hallido über QuickConnect, myQNAPcloud, ein VPN oder eine andere
Fernzugriffs-Adresse geöffnet. Öffne es im Heimnetz mit der lokalen Adresse
deiner NAS (`http://192.168.x.x:8790`).

## „Bereits eingerichtet“, aber du hast den Zugangsschlüssel nicht

Jemand (vielleicht du auf einem anderen Gerät) hat den Schlüssel schon
erstellt.

- Funktioniert Hallido auf einem Gerät noch, nutze es dort weiter; den
  Schlüssel kann Hallido aber nicht noch einmal anzeigen.
- Hat ihn kein Gerät mehr: deinstalliere Hallido **mitsamt den Daten** (siehe
  [Deinstallieren](uninstall.md)) und installiere es neu. Deine Szenen musst
  du dann neu anlegen.
- Warst du es nicht selbst, hat vielleicht ein anderes Gerät in deinem Netz
  die Einrichtung gemacht. Schau nach der Neuinstallation unter **Boxen →
  Protokoll**: Der Eintrag zur Einrichtung zeigt die Adresse des Geräts.

## „Token ungültig — bitte neu eingeben“

Der Zugangsschlüssel in diesem Browser fehlt oder ist falsch. Geh zu **Boxen
→ API-Token ändern** und füge deinen Schlüssel genau ein (mit den
Bindestrichen).

## Keine Boxen gefunden

- Sind die Boxen eingeschaltet und spielen in der Sonos-App normal?
- Hängt die NAS am **selben Router/Netz** wie die Boxen? Ein Gäste-Netz oder
  ein eigenes VLAN funktioniert nicht.
- Firewall der NAS: siehe oben – die Antworten der Boxen müssen
  hereingelassen werden.
- Die Projektdatei muss `network_mode: host` enthalten (das tut sie, wenn du
  die originale `compose.yaml` verwendet hast).
- Tippe im Tab **Boxen** auf **Aktualisieren** und warte bis zu 10 Sekunden.

## Die Sendersuche findet nichts

Die Suche nutzt das freie Internet-Verzeichnis radio-browser.info. Prüfe die
Internetverbindung der NAS. Hast du die Suche abgeschaltet
(`HALLIDO_RADIO_SEARCH: 0`), gib stattdessen die Stream-Adresse des Senders
direkt ein.

## Port 8790 ist schon belegt

Dann startet Hallido nicht (im Log steht „address already in use“). Ändere in
der Projektdatei die Zeile

```yaml
      SRA_LISTEN: ":${HALLIDO_PORT:-8790}"
```

zum Beispiel in `SRA_LISTEN: ":8791"` und starte Hallido neu (wie bei einer
[Aktualisierung](update.md)). Nimm dann `:8791` in der Adresse.

## Hallido startet ständig neu oder zeigt „unhealthy“

Schau ins Container-Log: Synology **Container Manager → Container → hallido →
Protokoll**; QNAP **Container Station → Container → hallido → Logs**.
Typische Ursachen: Port schon belegt (siehe oben), kein freier
Speicherplatz.

## Immer noch nicht?

Leg ein Issue an unter
[github.com/ohm-meter/hallido/issues](https://github.com/ohm-meter/hallido/issues)
mit deinem NAS-Modell, der Hallido-Version und den passenden Zeilen aus
**Boxen → Protokoll**. Poste nie deinen Zugangsschlüssel.
