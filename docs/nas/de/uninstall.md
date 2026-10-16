# Hallido deinstallieren

Damit entfernst du Hallido von deiner NAS. Deine Sonos-Boxen werden nicht
verändert.

## Synology (Container Manager)

1. **Container Manager → Projekt →** `hallido` auswählen **→ Aktion → Stoppen**.
2. **Aktion → Löschen.** Wirst du gefragt, ob auch die Daten (Volumes)
   gelöscht werden sollen, hake das an – sonst bleiben deine Einstellungen auf
   der NAS (siehe unten).
3. **Container Manager → Image →** `ghcr.io/ohm-meter/hallido` auswählen **→ Löschen**.
4. **File Station →** Ordner **docker → hallido →** Ordner löschen.
5. Nutzt du gar keine Container mehr, kannst du den Container Manager im
   Paket-Zentrum deinstallieren.

## QNAP (Container Station)

1. **Container Station → Anwendungen →** `hallido` **→** Menü (⋯) **→ Löschen**.
   Hake, falls angeboten, an, dass auch die **Volumes** gelöscht werden.
2. **Container Station → Images →** `ghcr.io/ohm-meter/hallido` auswählen **→ Entfernen**.
3. Falls angeboten: unter **Volumes** `hallido_hallido-data` löschen.

## Einstellungen, die übrig bleiben

Hallido speichert seine Einstellungen (Zugangsschlüssel, Szenen, Zeitpläne,
Protokoll) in einem Speicherbereich namens `hallido_hallido-data`. Er ist
kleiner als 5 MB. Installierst du Hallido **mit demselben Projektnamen**
wieder, findet es sie wieder, und die Einrichtung entfällt.

Um ihn ganz zu entfernen, lösche das Volume `hallido_hallido-data` dort, wo
deine Container-App Volumes auflistet. Hat sie keine solche Liste, bleibt es
liegen; es schadet nicht.

## Im Browser

Auf jedem Handy und Computer, den du benutzt hast, kannst du das
Hallido-Symbol vom Startbildschirm entfernen. Um auch den gespeicherten
Zugangsschlüssel zu entfernen, lösche in den Browser-Einstellungen die
Website-Daten für die Adresse der NAS.
