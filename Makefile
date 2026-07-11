BINARY := sysmon
PIDFILE := /tmp/$(BINARY).pid

.PHONY: build run start stop status clean

## build: build the binary
build:
	go build -o $(BINARY) .

## run: build and run in the current terminal (Ctrl+C to quit)
run: build
	./$(BINARY)

## start: build and run in the background with no window
start: build
	@nohup ./$(BINARY) -quiet >/dev/null 2>&1 & echo $$! > $(PIDFILE)
	@echo "sysmon started in the background (PID $$(cat $(PIDFILE)))"

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
