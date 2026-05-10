package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInit_PrintEmitsJSON(t *testing.T) {
	homeDir = "" // reset global from main.go
	t.Setenv("CCO_HOME", "")

	dir := t.TempDir()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)

	root := newRootCmd()
	registerSubcommands(root)
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"init", "--print"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out.String(), "OTEL_EXPORTER_OTLP_ENDPOINT") {
		t.Errorf("--print missing owned keys: %s", out.String())
	}
	if _, err := os.Stat(filepath.Join(dir, ".claude", "settings.json")); err == nil {
		t.Error("--print created the file")
	}
}

func TestInit_FreshDir_WritesFile(t *testing.T) {
	homeDir = ""
	t.Setenv("CCO_HOME", "")

	dir := t.TempDir()
	cwd, _ := os.Getwd()
	_ = os.Chdir(dir)
	defer os.Chdir(cwd)

	root := newRootCmd()
	registerSubcommands(root)
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"init", "--force"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(dir, ".claude", "settings.json"))
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}
	if !strings.Contains(string(b), `"OTEL_EXPORTER_OTLP_ENDPOINT": "http://localhost:4317"`) {
		t.Errorf("settings missing endpoint: %s", string(b))
	}
}
