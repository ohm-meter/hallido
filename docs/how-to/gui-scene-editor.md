# GUI: Create scenes and save speakers

The web GUI can create new **scenes** and save discovered speakers to `config.json` — without editing the file by hand.

## Prerequisites

- Agent is running (`./scripts/dev.sh start` or systemd)
- Valid `api_token` in `config.json` (not `BITTE-AENDERN`)
- Token entered in the browser dialog
- At least one existing scene in the config (validated at startup)

## New scene

1. Click **"New scene"** at the top.
2. Enter a **name** — the **ID (slug)** is generated automatically (ä → ae, spaces → `-`).
3. Check **speakers** from the discovery list and set the **volume** 0–100 for each speaker.
4. **Source:**
   - **Stream from speaker:** Choose a speaker that is currently playing a stream (the now-playing URI is taken over and normalized).
   - **Custom stream URL:** Enter `https://…` (`file://` is blocked).
5. **Save** — the entry lands in `config.json`, and the scene is playable in the GUI **immediately** (hot reload).

## Edit a scene

1. Click **"Edit"** on the scene card (bottom right, next to Delete) — not on the card itself, which plays the scene.
2. The **"Edit scene"** dialog opens with prefilled values (name, speakers, stream URL).
3. The **ID (slug)** is read-only — to rename the technical ID, delete the scene and create it again.
4. To keep your changes, click **Save** (PUT, hot reload). **Cancel** closes without saving.

## Delete a scene

1. Click the **delete icon** (top right) on the scene card — not on the card itself, which plays the scene.
2. Confirm in the dialog: **"Really delete scene '…'?"** → **Delete** (or **Cancel**).
3. The scene disappears from the GUI; the entry is removed from `config.json` (hot reload).

## Save speakers to config

In the **Found** speakers panel → **"Save speakers to config"**:

- Writes the name + IP of all currently found speakers to `speakers` (merge; existing scenes/token are kept).
- Fixed speaker IPs are **active immediately** (hot reload in the Sonos driver). A restart is only needed when `listen` or `--driver` changes: `./scripts/dev.sh restart`

## API (for scripts/Homey)

| Method | Path | Purpose |
|--------|------|---------|
| `GET` | `/api/v1/config` | Overview without the token in the response |
| `POST` | `/api/v1/config/speakers/sync` | Body: `{ "speakers": [{ "name", "ip" }] }` |
| `POST` | `/api/v1/config/scenes` | Body: `{ "scene": { id, name, source, members } }` |
| `PUT` | `/api/v1/config/scenes/{id}` | Body: `{ name, source, members }` — ID only in the path |
| `GET` | `/api/v1/sonos/favorites` | Sonos favorites (Sonos driver) |
| `POST` | `/api/v1/config/scenes/from-favorite` | Stream scene from a favorite (`favorite_id`, importable only) |
| `DELETE` | `/api/v1/config/scenes/{id}` | Remove a scene from `config.json` (slug ID) |

All with `Authorization: Bearer <token>`.

## Errors

- **401** — Check the token in the dialog.
- **400** — Invalid slug, URI, duplicate members, `file://`, unknown JSON fields.
- **404** — Scene not found on delete or edit.
- Config writing is disabled when the server runs without a `-config` path (special tests only).

## See also

- [Create a scene (manually)](create-scene.md)
- [Speaker config](speaker-config-and-refresh.md)
- [Music services & now playing](music-services-and-now-playing.md)
