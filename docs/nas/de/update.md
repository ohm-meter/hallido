# Hallido aktualisieren

Deine Szenen, Zeitpläne und dein Zugangsschlüssel bleiben erhalten – sie
liegen getrennt vom Programm.

1. Lade die neue **`compose.yaml`** aus
   [dem neuesten Release](https://github.com/ohm-meter/hallido/releases/latest)
   herunter. Jedes Release hat seine eigene Datei; sie zeigt genau auf diese
   Version.
2. Ersetze die Projektdatei auf deiner NAS:

## Synology (Container Manager)

1. **Container Manager → Projekt →** `hallido` auswählen **→ Aktion → Stoppen**.
2. Öffne das Projekt und geh zu **YAML-Konfigurationen** (oder **Aktion →
   Bearbeiten**).
3. Markiere den ganzen Text im Feld und lösche ihn. Öffne die neue
   `compose.yaml` in einem Texteditor, kopiere alles und füge es ein.
4. Klicke auf **Speichern**. Fragt der Container Manager, ob das Projekt neu
   erstellt/gestartet werden soll, bestätige mit **Ja** (sonst **Aktion →
   Erstellen**).

## QNAP (Container Station)

1. **Container Station → Anwendungen →** `hallido` **→** Menü (⋯) **→ Bearbeiten**.
2. Ersetze den ganzen YAML-Text durch den Inhalt der neuen `compose.yaml`.
3. Klicke auf **Prüfen**, dann auf **Aktualisieren** (bzw. **Übernehmen**).

## Kontrolle

Öffne Hallido im Browser (Seite einmal neu laden). Deine Szenen sind noch
da. Die erste Zeile der Projektdatei nennt die Version (`# Hallido v…`).

Alte Versionen belegen etwas Platz (je etwa 12 MB). Nicht mehr genutzte
Images kannst du löschen: Synology **Container Manager → Image** / QNAP
**Container Station → Images** → das alte Image `ghcr.io/ohm-meter/hallido`
auswählen → **Löschen**. Lösche nie das Image, das gerade verwendet wird.
