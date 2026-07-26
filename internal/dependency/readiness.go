package dependency

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

const (
	DefaultStartupTimeout = 60 * time.Second
	DefaultProbeTimeout   = 3 * time.Second
	DefaultProbeInterval  = 5 * time.Second
	DefaultRetryMinimum   = 100 * time.Millisecond
	DefaultRetryMaximum   = 2 * time.Second
)

type ReadinessPolicy struct {
	StartupTimeout time.Duration
	ProbeTimeout   time.Duration
	ProbeInterval  time.Duration
	RetryMinimum   time.Duration
	RetryMaximum   time.Duration
}

func DefaultReadinessPolicy() ReadinessPolicy {
	return ReadinessPolicy{
		StartupTimeout: DefaultStartupTimeout,
		ProbeTimeout:   DefaultProbeTimeout,
		ProbeInterval:  DefaultProbeInterval,
		RetryMinimum:   DefaultRetryMinimum,
		RetryMaximum:   DefaultRetryMaximum,
	}
}

func (p ReadinessPolicy) validate() error {
	if p.StartupTimeout <= 0 || p.ProbeTimeout <= 0 || p.ProbeInterval <= 0 || p.RetryMinimum <= 0 || p.RetryMaximum < p.RetryMinimum {
		return errors.New("invalid dependency readiness policy")
	}
	return nil
}

type Probe struct {
	Name    string
	Ping    func(context.Context) error
	Observe func(string, time.Duration, error)
}

func Run(ctx context.Context, policy ReadinessPolicy, probes []Probe, setReady func(bool), serve func(context.Context) error) error {
	if err := policy.validate(); err != nil {
		return err
	}
	if len(probes) == 0 {
		return errors.New("dependency readiness requires at least one probe")
	}
	for _, probe := range probes {
		if probe.Name == "" || probe.Ping == nil {
			return errors.New("dependency readiness probe is invalid")
		}
	}

	setReady(false)
	defer setReady(false)
	serveContext, cancelServe := context.WithCancel(context.WithoutCancel(ctx))
	defer cancelServe()
	serveResult := make(chan error, 1)
	go func() { serveResult <- serve(serveContext) }()

	startupContext, cancelStartup := context.WithTimeout(ctx, policy.StartupTimeout)
	defer cancelStartup()
	retryDelay := policy.RetryMinimum
	for {
		failed := runProbes(startupContext, policy.ProbeTimeout, probes)
		if len(failed) == 0 {
			setReady(true)
			break
		}

		timer := time.NewTimer(retryDelay)
		select {
		case err := <-serveResult:
			timer.Stop()
			return err
		case <-startupContext.Done():
			timer.Stop()
			cancelServe()
			serveErr := <-serveResult
			if ctx.Err() != nil {
				return serveErr
			}
			if serveErr != nil {
				return serveErr
			}
			return fmt.Errorf("dependency readiness startup timeout: %s unavailable", strings.Join(failed, ", "))
		case <-timer.C:
		}
		retryDelay = min(retryDelay*2, policy.RetryMaximum)
	}

	ticker := time.NewTicker(policy.ProbeInterval)
	defer ticker.Stop()
	for {
		select {
		case err := <-serveResult:
			return err
		case <-ctx.Done():
			setReady(false)
			cancelServe()
			return <-serveResult
		case <-ticker.C:
			setReady(len(runProbes(ctx, policy.ProbeTimeout, probes)) == 0)
		}
	}
}

func runProbes(ctx context.Context, timeout time.Duration, probes []Probe) []string {
	var wait sync.WaitGroup
	failed := make([]bool, len(probes))
	for index, probe := range probes {
		wait.Add(1)
		go func() {
			defer wait.Done()
			probeContext, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			started := time.Now()
			err := probe.Ping(probeContext)
			failed[index] = err != nil
			if probe.Observe != nil {
				probe.Observe(probe.Name, time.Since(started), err)
			}
		}()
	}
	wait.Wait()

	names := make([]string, 0, len(probes))
	for index, isFailed := range failed {
		if isFailed {
			names = append(names, probes[index].Name)
		}
	}
	return names
}
