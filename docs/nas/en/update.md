# Update Hallido

Your scenes, schedules and your access key stay as they are – they are
stored separately from the program.

1. Download the new **`compose.yaml`** from
   [the latest release](https://github.com/ohm-meter/hallido/releases/latest).
   Each release has its own file; it points to exactly that version.
2. Replace the project file on your NAS:

## Synology (Container Manager)

1. **Container Manager → Project →** select `hallido` **→ Action → Stop**.
2. Open the project and go to **YAML configurations** (or **Action → Edit**).
3. Select everything in the text field and delete it. Open the new
   `compose.yaml` in a text editor, copy everything and paste it.
4. Click **Save**. When Container Manager asks whether to rebuild/start the
   project, confirm with **Yes** (otherwise **Action → Build**).

## QNAP (Container Station)

1. **Container Station → Applications →** `hallido` **→** the menu (⋯) **→ Edit**.
2. Replace the whole YAML text with the content of the new `compose.yaml`.
3. Click **Validate**, then **Update** (or **Apply**).

## Check

Open Hallido in the browser (reload the page once). Your scenes are still
there. The first line of the project file names the version
(`# Hallido v…`).

Old versions take up a little space (about 12 MB each). You can delete
unused images: Synology **Container Manager → Image** / QNAP **Container
Station → Images** → select the old `ghcr.io/ohm-meter/hallido` image →
**Delete**. Never delete the image that is in use.
