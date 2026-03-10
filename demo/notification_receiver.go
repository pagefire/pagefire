//go:build ignore

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// Notification receiver -- stands in for Slack/email/PagerDuty.
// Prints incoming webhook notifications to the terminal.
func main() {
	http.HandleFunc("/notify", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		defer r.Body.Close()

		var payload map[string]any
		json.Unmarshal(body, &payload)

		subject, _ := payload["subject"].(string)
		alertBody, _ := payload["body"].(string)

		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "--- PAGE RECEIVED ---")
		fmt.Fprintf(os.Stderr, "  Time:    %s\n", time.Now().Format("15:04:05"))
		fmt.Fprintf(os.Stderr, "  Subject: %s\n", subject)
		fmt.Fprintf(os.Stderr, "  Body:    %s\n", alertBody)
		fmt.Fprintln(os.Stderr, "---------------------")
		fmt.Fprintln(os.Stderr)

		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{"status":"received"}`)
	})

	fmt.Println("Notification receiver listening on :9090")
	http.ListenAndServe(":9090", nil)
}
