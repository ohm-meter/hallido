# Install Hallido on your NAS

Hallido plays radio on your Sonos speakers – in groups, at the push of a
button or as a wake-up alarm. It runs on your NAS at home, needs no account
and no cloud.

This guide takes about **15 minutes**. You do not need any technical
knowledge. You only click in the NAS menus and in your browser.

## What you need

- A **Synology** NAS with DSM 7.2 or newer, or a **QNAP** NAS with QTS 5 or
  QuTS hero 5. Most models from 2018 onwards work (Intel/AMD or ARM).
- The NAS and the Sonos speakers are in the **same home network**
  (the same router).
- A phone or computer in your home network for the setup.
- About 50 MB of free space on the NAS.

## The three steps

1. **Download the project file.** Open
   [the latest release](https://github.com/ohm-meter/hallido/releases/latest)
   and download the file **`compose.yaml`** (under “Assets”). It contains
   everything your NAS needs – you do not have to change it.
2. **Install Hallido on your NAS** – choose your NAS:
   - [Synology (Container Manager)](synology.md)
   - [QNAP (Container Station)](qnap.md)
3. **Set up Hallido in your browser.** Open `http://<address-of-your-NAS>:8790`
   and follow the setup. It is described at the end of the guide for your NAS.

## Later

- [Update Hallido](update.md)
- [Uninstall Hallido](uninstall.md)
- [If something doesn't work](troubleshooting.md)

## Good to know

- **Your access key.** During setup Hallido creates a personal access key.
  It is shown **only once**. Keep it safe (password manager or on paper).
  Every phone or computer that should operate Hallido needs it once.
- **Only at home.** Hallido is reachable only in your home network. Do not
  open port 8790 on your router.
- **What exactly runs on your NAS?** One small program (about 12 MB) without
  any special rights. Everything is listed in
  [What Hallido does on your network and your NAS](../../explanation/what-hallido-does.md).
- **Want to check the download first?** See
  [Verify the image before installing](../../how-to/verify-image.md)
  (optional, for experienced users).
