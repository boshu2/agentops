package main

import (
	"context"
	"testing"
)

func TestAppDirectInvocationHasProductionPorts(t *testing.T) {
	app := AppFromContext(context.Background())
	if app == nil || app.ExecCommand == nil || app.LookPath == nil || app.Stdout == nil || app.Stderr == nil {
		t.Fatalf("direct command invocation has no usable App ports: %+v", app)
	}
}

func TestAppNilContextHasProductionPorts(t *testing.T) {
	var nilContext context.Context
	app := AppFromContext(nilContext)
	if app == nil || app.ExecCommand == nil || app.LookPath == nil || app.Stdout == nil || app.Stderr == nil {
		t.Fatalf("nil command context has no usable App ports: %+v", app)
	}
}
