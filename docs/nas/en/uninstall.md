# Uninstall Hallido

This removes Hallido from your NAS. Your Sonos speakers are not changed.

## Synology (Container Manager)

1. **Container Manager → Project →** select `hallido` **→ Action → Stop**.
2. **Action → Delete.** If you are asked whether to delete the data
   (volumes) too, tick it – otherwise your settings stay on the NAS (see
   below).
3. **Container Manager → Image →** select `ghcr.io/ohm-meter/hallido` **→ Delete**.
4. **File Station →** folder **docker → hallido →** delete the folder.
5. If you no longer use containers at all, you can uninstall Container
   Manager in Package Center.

## QNAP (Container Station)

1. **Container Station → Applications →** `hallido` **→** menu (⋯) **→ Delete**.
   Tick the option to also delete the **volumes**, if it is offered.
2. **Container Station → Images →** select `ghcr.io/ohm-meter/hallido` **→ Remove**.
3. If offered, under **Volumes**, delete `hallido_hallido-data`.

## Settings that remain

Hallido keeps its settings (access key, scenes, schedules, log) in a storage
area called `hallido_hallido-data`. It is smaller than 5 MB. If you install
Hallido again **with the same project name**, it finds them again and the
setup is not needed.

To remove it completely, delete the volume `hallido_hallido-data` where your
container app lists volumes. If your NAS app has no such list, it stays; it
does no harm.

## In your browser

On every phone or computer you used, you can remove the Hallido home-screen
icon. To also remove the stored access key, delete the website data for the
NAS address in the browser settings.
