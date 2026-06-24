package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/alanchenchen/suna/internal/config"
	"github.com/alanchenchen/suna/internal/update"
)

func updateCommand(args []string) {
	if len(args) > 0 {
		fmt.Fprintf(os.Stderr, "Unknown update option: %s\n", args[0])
		os.Exit(2)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	opts := update.Options{DataDir: config.DefaultDataDir(), Stdout: os.Stdout}
	latest, err := update.Check(ctx, opts)
	if err != nil {
		printUpdateError("Error checking update", err)
		os.Exit(1)
	}
	printUpdateStatus(latest)
	if !latest.UpdateNeeded {
		return
	}
	if !confirmUpdate(os.Stdin, os.Stdout) {
		fmt.Println(updateStyle().dim("Update cancelled."))
		return
	}

	probeCtx, probeCancel := context.WithTimeout(context.Background(), 2*time.Second)
	_, daemonErr := queryDaemonStatus(probeCtx)
	probeCancel()
	if daemonErr == nil {
		printUpdateBlocker("sunad is still running")
		fmt.Fprintln(os.Stderr, updateStyle().dim("Please exit the TUI and run `suna stop`, then retry `suna update`."))
		os.Exit(1)
	}

	installed, err := update.Install(ctx, opts)
	if err != nil {
		printUpdateError("Error updating Suna", err)
		os.Exit(1)
	}
	if !installed.UpdateNeeded {
		printUpdateStatus(installed)
		return
	}
	fmt.Printf("\n%s %s. %s\n", updateStyle().success("Suna updated to"), updateStyle().version(installed.LatestVersion), updateStyle().dim("Run `suna` to start the new version."))
}

func printUpdateStatus(latest update.Latest) {
	style := updateStyle()
	latestVersion := style.version(latest.LatestVersion)
	if latest.UpdateNeeded {
		latestVersion = style.warn(latest.LatestVersion)
	}
	fmt.Printf("%s %s\n", style.label("Current version:"), style.version(latest.CurrentVersion))
	fmt.Printf("%s  %s\n", style.label("Latest version:"), latestVersion)
	if latest.ReleaseURL != "" {
		fmt.Printf("%s         %s\n", style.label("Release:"), style.link(latest.ReleaseURL))
	}
	printReleaseNotes(latest.ReleaseNotes, style)
	if latest.UpdateNeeded {
		fmt.Println(style.warn("Update available."))
		return
	}
	fmt.Println(style.success("Already up to date."))
}

func printReleaseNotes(notes string, style updateCLIStyle) {
	notes = strings.TrimSpace(notes)
	if notes == "" {
		return
	}
	fmt.Println()
	fmt.Println(style.label("What's new:"))
	fmt.Println(limitReleaseNotes(notes, 4000))
	fmt.Println()
}

func limitReleaseNotes(notes string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	runes := []rune(notes)
	if len(runes) <= maxRunes {
		return notes
	}
	return strings.TrimSpace(string(runes[:maxRunes])) + "\n..."
}

func confirmUpdate(in io.Reader, out io.Writer) bool {
	fmt.Fprint(out, "Install this update? [y/N] ")
	reader := bufio.NewReader(in)
	line, err := reader.ReadString('\n')
	if err != nil && len(line) == 0 {
		return false
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes"
}

func printUpdateError(prefix string, err error) {
	style := updateStyleFor(os.Stderr)
	fmt.Fprintf(os.Stderr, "%s %s: %s\n", style.error("Error:"), prefix, err)
}

func printUpdateBlocker(message string) {
	style := updateStyleFor(os.Stderr)
	fmt.Fprintf(os.Stderr, "%s %s.\n", style.error("Error:"), message)
}

type updateCLIStyle struct {
	enabled bool
}

func updateStyle() updateCLIStyle {
	return updateStyleFor(os.Stdout)
}

func updateStyleFor(f *os.File) updateCLIStyle {
	return updateCLIStyle{enabled: shouldColor(f)}
}

func shouldColor(f *os.File) bool {
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func (s updateCLIStyle) label(v string) string   { return s.ansi("\x1b[2m", v) }
func (s updateCLIStyle) dim(v string) string     { return s.ansi("\x1b[2m", v) }
func (s updateCLIStyle) version(v string) string { return s.ansi("\x1b[36m", v) }
func (s updateCLIStyle) link(v string) string    { return s.ansi("\x1b[4m", v) }
func (s updateCLIStyle) success(v string) string { return s.ansi("\x1b[32m", v) }
func (s updateCLIStyle) warn(v string) string    { return s.ansi("\x1b[33m", v) }
func (s updateCLIStyle) error(v string) string   { return s.ansi("\x1b[31m", v) }

func (s updateCLIStyle) ansi(prefix, v string) string {
	if !s.enabled || v == "" {
		return v
	}
	return prefix + v + "\x1b[0m"
}
