package main

/*
#cgo LDFLAGS: -framework ApplicationServices -framework CoreGraphics
#include <ApplicationServices/ApplicationServices.h>

// nudge двигает курсор на (dx, dy) от текущей позиции и постит настоящее
// событие MouseMoved — именно оно сбрасывает системный таймер бездействия,
// в отличие от простого перемещения курсора.
static void nudge(int dx, int dy) {
    CGEventRef cur = CGEventCreate(NULL);
    CGPoint p = CGEventGetLocation(cur);
    CFRelease(cur);

    CGPoint np = CGPointMake(p.x + dx, p.y + dy);
    CGEventRef move = CGEventCreateMouseEvent(NULL, kCGEventMouseMoved, np, kCGMouseButtonLeft);
    CGEventPost(kCGHIDEventTap, move);
    CFRelease(move);
}
*/
import "C"

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	interval := flag.Duration("interval", 10*time.Second, "как часто дёргать курсор (напр. 10s, 30s, 1m)")
	quiet := flag.Bool("quiet", false, "не писать логи в stdout")
	flag.Parse()

	if *quiet {
		log.SetOutput(os.Stderr)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	ticker := time.NewTicker(*interval)
	defer ticker.Stop()

	if !*quiet {
		log.Printf("sysmon: активность каждые %s (Ctrl+C для выхода)", *interval)
	}

	for {
		select {
		case <-ticker.C:
			// Небольшое смещение туда-обратно: курсор фактически на месте,
			// но система видит движение.
			C.nudge(1, 0)
			time.Sleep(40 * time.Millisecond)
			C.nudge(-1, 0)
		case <-sig:
			if !*quiet {
				log.Println("sysmon: остановлен")
			}
			return
		}
	}
}
