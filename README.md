# sysmon

A small Go utility for macOS: it keeps the system "active" by posting a real
mouse-move event every N seconds (which resets the idle timer and prevents the
machine from going idle / into the screensaver). The cursor stays effectively in
place.

## Build

```sh
go build -o sysmon .
```

You need the Xcode Command Line Tools (`xcode-select --install`) -- it uses cgo
and the ApplicationServices framework.

## Run

Normal run (with logs):

```sh
./sysmon
```

Custom interval:

```sh
./sysmon -interval 30s
```

In the background, with no output (there is no window -- it is a console
program):

```sh
nohup ./sysmon -quiet >/dev/null 2>&1 &
```

Stop the background process:

```sh
pkill -f sysmon
```

## Permissions (important)

macOS requires permission to post system events. On first run the system will
ask you to add the **terminal** (Terminal.app / iTerm) that `sysmon` is launched
from to:

**System Settings -> Privacy & Security -> Accessibility**

Without this permission the mouse event will not be delivered.
