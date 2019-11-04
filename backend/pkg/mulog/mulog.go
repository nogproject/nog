// Package `mulog` provides minimal Zap-Sugar-like loggers with convenient
// structured logging `Levelw(msg, kv...)` functions.
package mulog

import (
	"fmt"
	"log"
	"os"
)

// `Logger` prints messages with timestamps, using package `log`.
type Logger struct{}

func (Logger) Infow(msg string, kv ...interface{}) {
	log.Printf("info: %s %v\n", msg, kv)
}

func (Logger) Warnw(msg string, kv ...interface{}) {
	log.Printf("warning: %s %v\n", msg, kv)
}

func (Logger) Errorw(msg string, kv ...interface{}) {
	log.Printf("error: %s %v\n", msg, kv)
}

func (Logger) Panicw(msg string, kv ...interface{}) {
	log.Panicf("panic: %s %v\n", msg, kv)
}

func (Logger) Fatalw(msg string, kv ...interface{}) {
	log.Fatalf("fatal: %s %v\n", msg, kv)
}

// `Printer` prints undecorated messages, using package `fmt`.
type Printer struct{}

func (Printer) Infow(msg string, kv ...interface{}) {
	fmt.Fprintf(os.Stderr, "info: %s %v\n", msg, kv)
}

func (Printer) Warnw(msg string, kv ...interface{}) {
	fmt.Fprintf(os.Stderr, "warning: %s %v\n", msg, kv)
}

func (Printer) Errorw(msg string, kv ...interface{}) {
	fmt.Fprintf(os.Stderr, "error: %s %v\n", msg, kv)
}

func (Printer) Panicw(msg string, kv ...interface{}) {
	msg = fmt.Sprintf("%s %v", msg, kv)
	fmt.Fprintf(os.Stderr, "panic: %s\n", msg)
	panic(msg)
}

func (Printer) Fatalw(msg string, kv ...interface{}) {
	fmt.Fprintf(os.Stderr, "fatal: %s %v\n", msg, kv)
	os.Exit(1)
}
