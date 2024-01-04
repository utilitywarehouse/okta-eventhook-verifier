package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	oktaVerificationHeader = "x-okta-verification-challenge"
)

var (
	log            = slog.Default()
	address        string
	eventHookPaths string
	exitWhenDone   bool
	timeOutHours   int
)

func usage() {
	fmt.Fprintf(os.Stderr, "NAME:\n")
	fmt.Fprintf(os.Stderr, "\tokta-eventhook-verifier\n")

	fmt.Fprintf(os.Stderr, "DESCRIPTION:\n")
	fmt.Fprintf(os.Stderr, "\tA okta event hook handler which only performs initial verification\n")

	fmt.Fprintf(os.Stderr, "OPTIONS:\n")
	fmt.Fprintf(os.Stderr, "\t--listen-address      (default: :9000) [$LISTEN_ADDRESS]\n")
	fmt.Fprintf(os.Stderr, "\t--event-hook-paths    (default: /)     [$EVENT_HOOK_PATHS]\n")
	fmt.Fprintf(os.Stderr, "\t--exit-when-done      (default: true)  [$EXIT_WHEN_DONE]\n")
	fmt.Fprintf(os.Stderr, "\t--time-out-hours      (default: 0)     [$TIME_OUT_HOURS]\n")
	os.Exit(2)
}

func main() {
	flag.StringVar(&address, "listen-address", ":9000", "address the web server binds to")
	flag.StringVar(&eventHookPaths, "event-hook-paths", "/", "comma separated string of event-hook paths")
	flag.BoolVar(&exitWhenDone, "exit-when-done", true, "if true service will exit once all event hooks (paths) are verified")
	flag.IntVar(&timeOutHours, "time-out-hours", 0, "if value is greater then 0 then service will exit in given hours even if event hooks are not verified")

	flag.Usage = usage
	flag.Parse()

	// Check environment variables for overrides
	if env := os.Getenv("LISTEN_ADDRESS"); env != "" {
		address = env
	}
	if env := os.Getenv("EVENT_HOOK_PATHS"); env != "" {
		eventHookPaths = env
	}
	if env := os.Getenv("EXIT_WHEN_DONE"); env != "" {
		if strings.EqualFold(env, "true") {
			exitWhenDone = true
		}
	}
	if env := os.Getenv("TIME_OUT_HOURS"); env != "" {
		if v, err := strconv.Atoi(env); err == nil {
			timeOutHours = v
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	if timeOutHours > 0 {
		ctx, cancel = context.WithTimeout(ctx, time.Duration(timeOutHours)*time.Hour)
	}
	defer cancel()

	done := make(chan bool)
	wg := &sync.WaitGroup{}

	mux := http.NewServeMux()
	server := &http.Server{
		Addr:              address,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       5 * time.Second,
		ReadHeaderTimeout: 1 * time.Second,
		Handler:           mux,
	}

	go gracefulShutdown(ctx, cancel, done, server)

	// register verification handler
	for _, path := range strings.Split(eventHookPaths, ",") {
		log.Info("registering event-hook verifier", "path", path)
		wg.Add(1)
		mux.Handle(path, verificationHandler(wg))
	}

	// Start a goroutine to wait for the completion
	go func() {
		wg.Wait()
		close(done)
	}()

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Error("unable to start server", "err", err)
		os.Exit(1)
	}
}

func verificationHandler(wg *sync.WaitGroup) http.Handler {
	var once sync.Once
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			log.Error("invalid verification request received", "method", r.Method, "path", r.URL.Path)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if strings.TrimSpace(r.Header.Get(oktaVerificationHeader)) == "" {
			log.Error("verification header missing", "method", r.Method, "path", r.URL.Path)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// its valid One-Time Verification Request
		b := `{"verification" : "` + r.Header.Get(oktaVerificationHeader) + `"}`

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := fmt.Fprintf(w, "%s", b); err != nil {
			log.Error("could not write one-time verification response", "path", r.URL.Path, "err", err)
			return
		}
		log.Info("one-time verification response sent", "path", r.URL.Path)

		if exitWhenDone {
			once.Do(func() {
				wg.Done()
			})
		}
	})
}

// gracefulShutdown will terminate when terminate signal received or when all event hooks are verified
func gracefulShutdown(ctx context.Context, cancel context.CancelFunc, done chan bool, server *http.Server) {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	for {
		select {
		case <-stop:
			log.Info("received stop signal")
			shutdownServer(server)
		case <-done:
			log.Info("all event hooks are verified")
			shutdownServer(server)
		case <-ctx.Done():
			log.Info("timed out")
			shutdownServer(server)
		}
	}
}

func shutdownServer(server *http.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	log.Info("shutting down server...")
	err := server.Shutdown(ctx)
	if err != nil {
		log.Error("failed to shutdown http server", "err", err)
	}
}
