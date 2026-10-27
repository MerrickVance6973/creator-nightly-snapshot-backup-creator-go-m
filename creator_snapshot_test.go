package main

import (
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"
)

func TestBuildSnapshotCutoff(t *testing.T) {
	cutoff := time.Date(2026, time.September, 25, 2, 0, 0, 0, time.UTC)
	tests := []struct {
		name            string
		input           CreatorExport
		wantRecords     int
		wantDeliveries  []string
		wantSubscribers []string
		wantProcessing  []string
	}{
		{
			name: "keeps records at or before cutoff and sorts stable output",
			input: CreatorExport{
				CreatorID: "creator-42", AsOf: cutoff,
				AssetDeliveries: []AssetDelivery{
					{DeliveryID: "d-2", AssetID: "course-pdf", DeliveredAt: cutoff},
					{DeliveryID: "d-1", AssetID: "preset-pack", DeliveredAt: cutoff.Add(-time.Hour)},
					{DeliveryID: "d-3", AssetID: "future-file", DeliveredAt: cutoff.Add(time.Second)},
				},
				SubscriberEvents: []SubscriberUpdate{
					{SubscriberID: "s-2", State: "active", UpdatedAt: cutoff.Add(-time.Minute)},
					{SubscriberID: "s-1", State: "paused", UpdatedAt: cutoff.Add(-time.Hour)},
				},
				ProcessingEvents: []ProcessingOutcome{
					{ContentID: "c-1", Stage: "transcode", Status: "complete", CompletedAt: cutoff.Add(-time.Minute)},
					{ContentID: "c-2", Stage: "caption", Status: "complete", CompletedAt: cutoff.Add(time.Minute)},
				},
			},
			wantRecords: 5, wantDeliveries: []string{"d-1", "d-2"},
			wantSubscribers: []string{"s-1", "s-2"}, wantProcessing: []string{"c-1"},
		},
		{
			name:        "allows an empty nightly partition",
			input:       CreatorExport{CreatorID: "creator-quiet", AsOf: cutoff},
			wantRecords: 0, wantDeliveries: []string{}, wantSubscribers: []string{}, wantProcessing: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			artifact, err := BuildSnapshot(tt.input)
			if err != nil {
				t.Fatal(err)
			}
			if artifact.Records != tt.wantRecords {
				t.Fatalf("records = %d, want %d", artifact.Records, tt.wantRecords)
			}
			data, err := base64.StdEncoding.DecodeString(artifact.DataBase64)
			if err != nil {
				t.Fatal(err)
			}
			var got Snapshot
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatal(err)
			}
			assertIDs(t, deliveryIDs(got.AssetDeliveries), tt.wantDeliveries)
			assertIDs(t, subscriberIDs(got.SubscriberEvents), tt.wantSubscribers)
			assertIDs(t, contentIDs(got.ProcessingEvents), tt.wantProcessing)

			again, err := BuildSnapshot(tt.input)
			if err != nil {
				t.Fatal(err)
			}
			if artifact.Key != again.Key {
				t.Fatal("identical input did not produce stable upload identity")
			}
		})
	}
}

func deliveryIDs(items []AssetDelivery) []string {
	ids := make([]string, len(items))
	for i := range items {
		ids[i] = items[i].DeliveryID
	}
	return ids
}

func subscriberIDs(items []SubscriberUpdate) []string {
	ids := make([]string, len(items))
	for i := range items {
		ids[i] = items[i].SubscriberID
	}
	return ids
}

func contentIDs(items []ProcessingOutcome) []string {
	ids := make([]string, len(items))
	for i := range items {
		ids[i] = items[i].ContentID
	}
	return ids
}

func assertIDs(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("ids = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ids = %v, want %v", got, want)
		}
	}
}
