# norns-sc-installer

a Go script to facilitate installing zip bundles of compiled SC synths.

```
./norns-sc-installer --check /usr/local/share/SuperCollider/Extensions \
    --check /home/we/dust/code \
    --check /home/we/.local/share/SuperCollider/Extensions \
    --to /home/we/.local/share/SuperCollider/Extensions/portedplugins \
    --url https://github.com/schollz/portedplugins/releases/download/v0.4.5/PortedPlugins-Linux.zip
```