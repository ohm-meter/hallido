# How-to: Migrate the install layout (code owned by root, config in /var/lib)

Applies to system installs (`scripts/install.sh --system --systemd`) created
before the split layout ([ADR 0017](../adr/0017-install-layout.md)):
`config.json` still lives under `/usr/local/sonos-agent/`, and the directory is
owned by the service user `sonos`. User installs
(`~/.local/share/sonos-agent`) are not affected.

## Detect

```bash
sonos-agent doctor
```

`FAIL Old install layout` or the message `… .install-meta is not loaded` mean a
migration is needed.

## Migrate

Back up first:

```bash
sudo cp -a /usr/local/sonos-agent/config.json ~/config.json.pre-migration
sudo cp -a /etc/systemd/system/sonos-agent.service ~/sonos-agent.service.pre-migration
```

**Automatically on update** — with the new CLI, `sonos-agent update` detects the
old layout and runs `sudo sonos-agent migrate` before building. If the old CLI
is still installed, the first `update` run only installs the new one; then run:

```bash
sudo sonos-agent migrate
```

The command:

1. stops the service (if it is running)
2. creates `/var/lib/sonos-agent` (`sonos:sonos`, `0750`) and moves
   `config.json` and `config.json.bak` there (`0600`); temp files are
   deleted, symlinks are not carried over
3. makes `/usr/local/sonos-agent` owned by root and not writable by others
4. rewrites `.install-meta` (`DATA_DIR`, `CONFIG_PATH`, `LAYOUT=2`)
5. adjusts the unit (`-config /var/lib/sonos-agent/config.json`,
   `WorkingDirectory`, `StateDirectory` instead of `ReadWritePaths`) — your own lines
   such as `Environment=SRA_API_TOKEN=…` are kept; backup:
   `/etc/systemd/system/sonos-agent.service.pre-migrate`
6. runs `systemctl daemon-reload` and starts the service again

A second run reports "Already on the new layout — nothing to do".

If `migrate` aborts with "suspicious: git hook …", the clone
`/usr/local/sonos-agent/src` was owned by the service and contains hooks or executable
git config. Nothing was changed. Inspect or delete the clone and run the
installer again — it then clones fresh as root.

Fresh install instead of migration: `sudo ./scripts/install.sh --system --systemd`
also detects the old layout and runs `migrate`.

## Verify

```bash
sonos-agent doctor                      # "Layout: Code root-eigen …"
sudo ls -la /var/lib/sonos-agent        # config.json sonos:sonos -rw-------
stat -c '%U %a' /usr/local/sonos-agent /usr/local/sonos-agent/.install-meta
systemctl status sonos-agent
curl -s http://127.0.0.1:8080/api/v1/health
```

Save a scene in the web app (this writes `config.json` and `.bak` in the new
directory).

## Rollback

```bash
sudo systemctl stop sonos-agent
sudo cp /etc/systemd/system/sonos-agent.service.pre-migrate /etc/systemd/system/sonos-agent.service
sudo install -m 0640 -o sonos -g sonos /var/lib/sonos-agent/config.json /usr/local/sonos-agent/config.json
sudo chown sonos:sonos /usr/local/sonos-agent
sudo systemctl daemon-reload && sudo systemctl start sonos-agent
```

The old CLI, or any CLI from before ADR 0017, still loads a root-owned
`.install-meta`; the new CLI would migrate again on the next `update`.

## Test without a real system

```bash
make test-install-docker
```

Runs migration, fresh install, the symlink trap and a tampered clone with
real paths in a Debian container.
