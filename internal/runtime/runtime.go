package runtime

import (
	"context"
	"os/signal"
	"syscall"
)

// Run owns process signal registration and passes the resulting cancellation
// context to the process runtime.
func Run(run func(context.Context) error) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	return run(ctx)
}

// RunWithoutReadiness serves an operational process that has not implemented
// its required dependency initialization yet. It deliberately keeps readiness
// false while still allowing liveness and other operational HTTP traffic.
func RunWithoutReadiness(ctx context.Context, setReady func(bool), run func(context.Context) error) error {
	setReady(false)
	defer setReady(false)
	return run(ctx)
}

// RunInitialized gives the process ownership of readiness. The caller performs
// initialization before calling this function; cancellation clears readiness
// before it reaches the transport's child context.
func RunInitialized(ctx context.Context, setReady func(bool), run func(context.Context) error) error {
	if err := ctx.Err(); err != nil {
		setReady(false)
		return err
	}

	serveContext, cancel := context.WithCancel(context.WithoutCancel(ctx))
	finished := make(chan struct{})
	setReady(true)
	go func() {
		select {
		case <-ctx.Done():
			setReady(false)
			cancel()
		case <-finished:
		}
	}()

	err := run(serveContext)
	close(finished)
	setReady(false)
	cancel()
	return err
}
