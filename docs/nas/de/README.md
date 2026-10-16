# Hallido auf deiner NAS installieren

Hallido spielt Radio auf deinen Sonos-Boxen – als Gruppe, per Knopfdruck
oder als Wecker. Es läuft bei dir zu Hause auf der NAS, braucht kein Konto
und keine Cloud.

Diese Anleitung dauert etwa **15 Minuten**. Du brauchst keine
Technik-Kenntnisse. Du klickst nur in den Menüs der NAS und im Browser.

## Das brauchst du

- Eine **Synology**-NAS mit DSM 7.2 oder neuer oder eine **QNAP**-NAS mit
  QTS 5 bzw. QuTS hero 5. Die meisten Modelle ab 2018 funktionieren
  (Intel/AMD oder ARM).
- Die NAS und die Sonos-Boxen sind im **selben Heimnetz** (am selben Router).
- Ein Handy oder einen Computer im Heimnetz für die Einrichtung.
- Etwa 50 MB freien Speicher auf der NAS.

## Die drei Schritte

1. **Projektdatei herunterladen.** Öffne
   [das neueste Release](https://github.com/ohm-meter/hallido/releases/latest)
   und lade die Datei **`compose.yaml`** herunter (unter „Assets“). Sie
   enthält alles, was deine NAS braucht – du musst nichts darin ändern.
2. **Hallido auf der NAS installieren** – wähle deine NAS:
   - [Synology (Container Manager)](synology.md)
   - [QNAP (Container Station)](qnap.md)
3. **Hallido im Browser einrichten.** Öffne
   `http://<Adresse-deiner-NAS>:8790` und folge der Einrichtung. Sie steht am
   Ende der Anleitung für deine NAS.

## Später

- [Hallido aktualisieren](update.md)
- [Hallido deinstallieren](uninstall.md)
- [Wenn etwas nicht klappt](troubleshooting.md)

## Gut zu wissen

- **Dein Zugangsschlüssel.** Bei der Einrichtung erstellt Hallido einen
  persönlichen Zugangsschlüssel. Er wird **nur einmal** angezeigt. Bewahre
  ihn sicher auf (Passwort-Manager oder auf Papier). Jedes Handy und jeder
  Computer, mit dem du Hallido bedienen willst, braucht ihn einmal.
- **Nur zu Hause.** Hallido ist nur im Heimnetz erreichbar. Gib Port 8790
  im Router nicht frei.
- **Was genau läuft auf deiner NAS?** Ein kleines Programm (etwa 12 MB) ohne
  besondere Rechte. Alles steht in
  [What Hallido does on your network and your NAS](../../explanation/what-hallido-does.md#kurzfassung-deutsch)
  (mit deutscher Kurzfassung).
- **Download vorher prüfen?** Siehe
  [Verify the image before installing](../../how-to/verify-image.md)
  (optional, für Erfahrene, auf Englisch).
