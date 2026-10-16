package main

import (
	"testing"
	"time"
)

func TestValidate(t *testing.T) {
	ok := Environment{DiscoveryInterval: time.Minute, PollInterval: time.Second, UnreachableThreshold: 1}
	if err := ok.validate(); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}

	for name, mutate := range map[string]func(*Environment){
		"zero poll interval":     func(e *Environment) { e.PollInterval = 0 },
		"negative discovery":     func(e *Environment) { e.DiscoveryInterval = -time.Second },
		"zero unreachable limit": func(e *Environment) { e.UnreachableThreshold = 0 },
	} {
		e := ok
		mutate(&e)
		if e.validate() == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}
