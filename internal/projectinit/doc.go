// Package projectinit implements `cco init`: writes/updates
// .claude/settings.json in a project directory and probes the daemon.
//
// Boundaries:
//   - Pure file-and-network operations.
//   - No dependency on internal/{receiver,service,repository,domain}.
//   - All I/O behind interfaces so tests stay hermetic.
package projectinit
