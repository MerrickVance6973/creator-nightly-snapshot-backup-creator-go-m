package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

type snapshotResult struct {
	Bucket  string `json:"bucket"`
	Key     string `json:"key"`
	Records int    `json:"records"`
}

func main() {
	log.SetFlags(0)
	if err := run(context.Background(), os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("creator-snapshot", flag.ContinueOnError)
	bucket := flags.String("bucket", "creator-nightly-snapshots", "object storage bucket")
	listen := flags.String("listen", ":8080", "HTTP listen address")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 1 {
		return fmt.Errorf("command required: init, snapshot, or serve")
	}
	apiKey := os.Getenv("INFRAI_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("INFRAI_API_KEY is required")
	}
	client := NewInfraiClient(apiKey)

	switch flags.Arg(0) {
	case "init":
		if err := client.CreateBucket(ctx, *bucket); err != nil {
			return err
		}
		fmt.Printf("bucket ready: %s\n", *bucket)
		return nil
	case "snapshot":
		result, err := snapshotFromReader(ctx, client, *bucket, os.Stdin)
		if err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(result)
	case "serve":
		handler := snapshotHandler(client, *bucket)
		log.Printf("snapshot service listening on %s", *listen)
		return http.ListenAndServe(*listen, handler)
	default:
		return fmt.Errorf("unknown command %q", flags.Arg(0))
	}
}

func snapshotFromReader(ctx context.Context, client *InfraiClient, bucket string, reader io.Reader) (snapshotResult, error) {
	var input CreatorExport
	decoder := json.NewDecoder(io.LimitReader(reader, 4<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		return snapshotResult{}, fmt.Errorf("decode creator export: %w", err)
	}
	artifact, err := BuildSnapshot(input)
	if err != nil {
		return snapshotResult{}, err
	}
	if err := client.PutSnapshot(ctx, bucket, artifact.Key, artifact.DataBase64); err != nil {
		return snapshotResult{}, err
	}
	return snapshotResult{Bucket: bucket, Key: artifact.Key, Records: artifact.Records}, nil
}

func snapshotHandler(client *InfraiClient, bucket string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/snapshots" {
			http.NotFound(w, r)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
		defer cancel()
		result, err := snapshotFromReader(ctx, client, bucket, r.Body)
		if err != nil {
			status := http.StatusBadRequest
			var apiErr *InfraiError
			if errors.As(err, &apiErr) {
				status = apiErr.HTTPStatus
				if status < 400 || status > 499 {
					status = http.StatusBadGateway
				}
			}
			writeJSON(w, status, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, result)
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
