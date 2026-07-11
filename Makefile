BINARY := sysmon
PIDFILE := /tmp/$(BINARY).pid

.PHONY: build run start stop status clean

## build: собрать бинарник
build:
	go build -o $(BINARY) .

## run: собрать и запустить в текущем терминале (Ctrl+C для выхода)
run: build
	./$(BINARY)

## start: собрать и запустить в фоне без окна
start: build
	@nohup ./$(BINARY) -quiet >/dev/null 2>&1 & echo $$! > $(PIDFILE)
	@echo "sysmon запущен в фоне (PID $$(cat $(PIDFILE)))"

## stop: остановить фоновый процесс (по PID-файлу — точно и безопасно)
stop:
	@if [ -f $(PIDFILE) ] && kill -0 $$(cat $(PIDFILE)) 2>/dev/null; then \
		kill $$(cat $(PIDFILE)) && echo "sysmon остановлен (PID $$(cat $(PIDFILE)))"; \
	else \
		echo "sysmon не запущен"; \
	fi
	@rm -f $(PIDFILE)

## status: проверить, работает ли
status:
	@if [ -f $(PIDFILE) ] && kill -0 $$(cat $(PIDFILE)) 2>/dev/null; then \
		echo "sysmon работает (PID $$(cat $(PIDFILE)))"; \
	else \
		echo "sysmon не работает"; \
	fi

## clean: удалить собранный бинарник
clean:
	@rm -f $(BINARY) $(PIDFILE)
	@echo "очищено"
