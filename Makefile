BINARY := sysmon
PIDFILE := /tmp/$(BINARY).pid
# INTERVAL controls how often the cursor is nudged. Override on the command
# line, e.g. `make start INTERVAL=3s`. Must be a positive Go duration.
INTERVAL ?= 10s

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

## stop: stop the background process (by PID file -- precise and safe)
stop:
	@if [ -f $(PIDFILE) ] && kill -0 $$(cat $(PIDFILE)) 2>/dev/null; then \
		kill $$(cat $(PIDFILE)) && echo "sysmon stopped (PID $$(cat $(PIDFILE)))"; \
	else \
		echo "sysmon is not running"; \
	fi
	@rm -f $(PIDFILE)

## status: check whether it is running
status:
	@if [ -f $(PIDFILE) ] && kill -0 $$(cat $(PIDFILE)) 2>/dev/null; then \
		echo "sysmon is running (PID $$(cat $(PIDFILE)))"; \
	else \
		echo "sysmon is not running"; \
	fi

## clean: remove the built binary
clean:
	@rm -f $(BINARY) $(PIDFILE)
	@echo "cleaned"
