package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRootCommand_HasExpectedSubcommands(t *testing.T) {
	root := newRootCmd()
	registerSubcommands(root)

	want := map[string]bool{
		"serve":           false,
		"tui":             false,
		"init":            false,
		"rebuild-rollups": false,
		"version":         false,
	}
	for _, c := range root.Commands() {
		if _, ok := want[c.Name()]; ok {
			want[c.Name()] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("missing subcommand: %s", name)
		}
	}
}

func TestVersionCommand_PrintsVersion(t *testing.T) {
	root := newRootCmd()
	registerSubcommands(root)

	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"version"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "claude-code-observer") {
		t.Errorf("output missing binary name: %q", got)
	}
}

func TestRoot_HomeFlag_OverridesDefault(t *testing.T) {
	homeDir = ""
	t.Setenv("CCO_HOME", "")
	root := newRootCmd()
	registerSubcommands(root)
	root.SetArgs([]string{"--home", "/tmp/cco-test", "version"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if homeDir != "/tmp/cco-test" {
		t.Errorf("homeDir = %q, want /tmp/cco-test", homeDir)
	}
}

func TestRoot_HomeFromEnv(t *testing.T) {
	homeDir = ""
	t.Setenv("CCO_HOME", "/tmp/from-env")
	root := newRootCmd()
	registerSubcommands(root)
	root.SetArgs([]string{"version"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if homeDir != "/tmp/from-env" {
		t.Errorf("homeDir = %q, want /tmp/from-env", homeDir)
	}
}

func TestRoot_ThemeFlag(t *testing.T) {
	themeName = ""
	t.Setenv("CCO_THEME", "")
	root := newRootCmd()
	registerSubcommands(root)
	root.SetArgs([]string{"--theme", "latte", "version"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if themeName != "latte" {
		t.Errorf("themeName = %q, want latte", themeName)
	}
}

func TestRoot_IconsFlag(t *testing.T) {
	iconsName = ""
	t.Setenv("CCO_ICONS", "")
	root := newRootCmd()
	registerSubcommands(root)
	root.SetArgs([]string{"--icons", "nerd", "version"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if iconsName != "nerd" {
		t.Errorf("iconsName = %q, want nerd", iconsName)
	}
}
