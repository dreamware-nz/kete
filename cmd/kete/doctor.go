package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"text/tabwriter"
	"time"

	"github.com/dreamware-nz/kete/internal/store"
	"github.com/spf13/cobra"
)

// check is one diagnostic row. status is "PASS" or "FAIL".
type check struct {
	name   string
	status string
	detail string
}

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose kete setup",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDoctor(cmd.OutOrStdout())
		},
	}
}

func runDoctor(w io.Writer) error {
	checks := []check{
		checkDotdir(),
		checkUpstream(),
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	defer tw.Flush()
	failed := 0
	for _, c := range checks {
		fmt.Fprintf(tw, "%s\t%s\t%s\n", c.status, c.name, c.detail)
		if c.status == "FAIL" {
			failed++
		}
	}
	if failed > 0 {
		return fmt.Errorf("%d check(s) failed", failed)
	}
	return nil
}

// checkDotdir asserts ~/.kete exists with mode 0700 (ADR 0004).
func checkDotdir() check {
	dir, err := store.DefaultDir()
	if err != nil {
		return check{"dotdir", "FAIL", err.Error()}
	}
	info, err := os.Stat(dir)
	if errors.Is(err, os.ErrNotExist) {
		return check{"dotdir", "FAIL", "missing: " + dir}
	}
	if err != nil {
		return check{"dotdir", "FAIL", err.Error()}
	}
	if !info.IsDir() {
		return check{"dotdir", "FAIL", "not a directory: " + dir}
	}
	if info.Mode().Perm() != 0o700 {
		return check{"dotdir", "FAIL",
			fmt.Sprintf("%s mode %o, want 700", dir, info.Mode().Perm())}
	}
	return check{"dotdir", "PASS", dir}
}

// upstreamURL returns the URL the proxy will dial, derived from
// KETE_UPSTREAM (anthropic|cc-proxy|bedrock; default anthropic) and the
// matching *_URL override env var. ADR 0015.
func upstreamURL() (string, error) {
	switch up := os.Getenv("KETE_UPSTREAM"); up {
	case "", "anthropic":
		if u := os.Getenv("KETE_ANTHROPIC_URL"); u != "" {
			return u, nil
		}
		return "https://api.anthropic.com", nil
	case "cc-proxy":
		if u := os.Getenv("KETE_CC_PROXY_URL"); u != "" {
			return u, nil
		}
		return "", errors.New("KETE_UPSTREAM=cc-proxy requires KETE_CC_PROXY_URL")
	case "bedrock":
		region := os.Getenv("AWS_REGION")
		if region == "" {
			return "", errors.New("KETE_UPSTREAM=bedrock requires AWS_REGION")
		}
		return "https://bedrock-runtime." + region + ".amazonaws.com", nil
	default:
		return "", fmt.Errorf("KETE_UPSTREAM=%q (want anthropic|cc-proxy|bedrock)", up)
	}
}

// checkUpstream HEAD-pings the configured upstream URL.
// Any 2xx, 4xx (auth-shaped), or 405 counts as reachable; only network
// errors and 5xx count as FAIL.
func checkUpstream() check {
	url, err := upstreamURL()
	if err != nil {
		return check{"upstream", "FAIL", err.Error()}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return check{"upstream", "FAIL", url + " — " + err.Error()}
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		return check{"upstream", "FAIL",
			fmt.Sprintf("%s — HTTP %d", url, resp.StatusCode)}
	}
	return check{"upstream", "PASS",
		fmt.Sprintf("%s — HTTP %d", url, resp.StatusCode)}
}
