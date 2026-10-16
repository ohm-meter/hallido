# If something doesn't work

Start here: open Hallido and look at **Speakers → Log**. It lists what
Hallido did and any problems, in plain sentences.

## The page does not open

- Use `http://` (not `https://`), the **address of your NAS** and
  **`:8790`** at the end, e.g. `http://192.168.1.20:8790`.
- Is Hallido running? Synology: **Container Manager → Project** – `hallido`
  must be green/**Running**. QNAP: **Container Station → Applications** –
  `hallido` **Running**. If not, start it.
- Is your phone in the home Wi-Fi (not mobile data, not guest Wi-Fi)?
- **Firewall on the NAS:** if you switched it on, allow your home network.
  Synology: **Control Panel → Security → Firewall → Edit Rules → Create**:
  Ports **All**, Source IP **Subnet** of your home network (e.g.
  `192.168.1.0` / `255.255.255.0`), Action **Allow**. QNAP: in **QuFirewall**
  allow the local network.

## “Setup time is over”

For your security, the setup is only possible during the first hour after
Hallido starts. Restart it – Synology: **Project → `hallido` → Action →
Restart**; QNAP: **Applications → `hallido` → Restart** – then reload the
page.

## “Only from your home network”

You opened Hallido through QuickConnect, myQNAPcloud, a VPN or another
remote address. Open it in your home network with the local address of your
NAS (`http://192.168.x.x:8790`).

## “Already set up”, but you do not have the access key

Someone (perhaps you, on another device) has already created the key.

- If a device still has Hallido open and working, keep using it there; the
  key cannot be shown again, though.
- If no device has it: uninstall Hallido **including its data** (see
  [Uninstall](uninstall.md)), then install it again. Your scenes need to be
  created again.
- If you did not create it yourself, that may mean another device in your
  network set it up. Check **Speakers → Log** after reinstalling: the setup
  entry shows the address of the device.

## “Invalid token — please enter it again”

The access key in this browser is missing or wrong. Go to **Speakers →
Change API token** and paste your key exactly (with the dashes).

## No speakers found

- Are the speakers on and playing normally in the Sonos app?
- Is the NAS connected to the **same router/network** as the speakers? A
  guest network or a separate VLAN does not work.
- NAS firewall: see above – the speakers' answers must be allowed in.
- The project file must contain `network_mode: host` (it does if you used
  the original `compose.yaml`).
- Tap **Refresh** in the **Speakers** tab and wait up to 10 seconds.

## The radio station search does not find anything

The search uses the free internet directory radio-browser.info. Check the
NAS's internet connection. If you switched the search off
(`HALLIDO_RADIO_SEARCH: 0`), enter the stream address of the station
directly instead.

## Port 8790 is already in use

Then Hallido does not start (the log says “address already in use”). In the
project file change the line

```yaml
      SRA_LISTEN: ":${HALLIDO_PORT:-8790}"
```

to, for example, `SRA_LISTEN: ":8791"` and start Hallido again (like an
[update](update.md)). Then use `:8791` in the address.

## Hallido keeps restarting or shows “unhealthy”

Look at the container log: Synology **Container Manager → Container →
hallido → Log**; QNAP **Container Station → Containers → hallido → Logs**.
Typical causes: port already in use (see above), no free disk space.

## Still stuck?

Open an issue at
[github.com/ohm-meter/hallido/issues](https://github.com/ohm-meter/hallido/issues)
with your NAS model, the Hallido version and the relevant lines from
**Speakers → Log**. Never post your access key.
