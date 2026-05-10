// Package dashboard implements the M3.1 dashboard view: today / 7-day /
// 30-day rollups plus the top-3 most expensive sessions today. It polls
// every TickMsg (1 s) with in-flight de-dup.
package dashboard
