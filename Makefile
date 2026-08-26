BINARY := sysmon
PIDFILE := /tmp/$(BINARY).pid
# INTERVAL controls how often the cursor is nudged. Override on the command
# line, e.g. `make start INTERVAL=3s`. Must be a positive Go duration.
INTERVAL ?= 10s
# PGREP is the single liveness source of truth for start/stop/status. `-x`
# matches the process name exactly, so the unrelated macOS daemon whose name
# is the binary name plus a trailing "d" (/usr/libexec/sysmond) is never
# matched. `-U` restricts the scan to processes owned by the invoking user.
PGREP = pgrep -x -U $$(id -u) $(BINARY)

.PHONY: build run start stop status clean

## build: build the binary
build:
	go build -o $(BINARY) ./cmd/sysmon

## run: build and run in the current terminal (Ctrl+C to quit)
run: build
	./$(BINARY) -interval $(INTERVAL)

## start: build and run in the background with no window
start: build
	@nohup ./$(BINARY) -quiet -interval $(INTERVAL) >/dev/null 2>&1 & echo $$! > $(PIDFILE)
	@echo "sysmon started in the background (PID $$(cat $(PIDFILE)), interval $(INTERVAL))"

## stop: stop the background process, falling back to a live scan when the
##        pidfile is missing or stale
stop:
	@live=$$($(PGREP) 2>/dev/null || true); \
	tracked=$$(cat $(PIDFILE) 2>/dev/null || true); \
	if [ -z "$$live" ]; then \
		rm -f $(PIDFILE); \
		if [ -z "$$tracked" ]; then \
			echo "sysmon: not running (nothing to stop)"; \
		else \
			echo "sysmon: not running (removed stale pidfile -> PID $$tracked)"; \
		fi; \
		exit 0; \
	fi; \
	if [ -n "$$tracked" ] && echo "$$live" | grep -qxF "$$tracked"; then \
		echo "sysmon: stopping tracked PID $$tracked"; \
	else \
		livelist=$$(echo $$live); \
		echo "sysmon: pidfile missing or stale -- stopping PID(s) found by live scan: $$livelist"; \
	fi; \
	kill $$live 2>/dev/null || true; \
	remaining=""; \
	for i in 1 2 3 4 5 6 7 8 9 10; do \
		remaining=$$($(PGREP) 2>/dev/null || true); \
		if [ -z "$$remaining" ]; then break; fi; \
		sleep 0.2; \
	done; \
	if [ -n "$$remaining" ]; then \
		echo "sysmon: FAILED to stop PID(s) $$remaining" >&2; \
		exit 1; \
	fi; \
	livelist=$$(echo $$live); \
	rm -f $(PIDFILE); \
	echo "sysmon: stopped (PID(s) $$livelist)"

## status: check whether it is running
status:
	@if [ -f $(PIDFILE) ] && kill -0 $$(cat $(PIDFILE)) 2>/dev/null; then \
		echo "sysmon is running (PID $$(cat $(PIDFILE)))"; \
	else \
		echo "sysmon is not running"; \
	fi

## clean: remove the built binary
clean: stop
	@rm -f $(BINARY) $(PIDFILE)
	@echo "cleaned"
