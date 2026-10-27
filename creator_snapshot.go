package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

type CreatorExport struct {
	CreatorID        string              `json:"creator_id"`
	AsOf             time.Time           `json:"as_of"`
	AssetDeliveries  []AssetDelivery     `json:"asset_deliveries"`
	SubscriberEvents []SubscriberUpdate  `json:"subscriber_updates"`
	ProcessingEvents []ProcessingOutcome `json:"processing_outcomes"`
}

type AssetDelivery struct {
	DeliveryID  string    `json:"delivery_id"`
	AssetID     string    `json:"asset_id"`
	DeliveredAt time.Time `json:"delivered_at"`
}

type SubscriberUpdate struct {
	SubscriberID string    `json:"subscriber_id"`
	State        string    `json:"state"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type ProcessingOutcome struct {
	ContentID   string    `json:"content_id"`
	Stage       string    `json:"stage"`
	Status      string    `json:"status"`
	CompletedAt time.Time `json:"completed_at"`
}

type Snapshot struct {
	SchemaVersion    int                 `json:"schema_version"`
	CreatorID        string              `json:"creator_id"`
	AsOf             time.Time           `json:"as_of"`
	AssetDeliveries  []AssetDelivery     `json:"asset_deliveries"`
	SubscriberEvents []SubscriberUpdate  `json:"subscriber_updates"`
	ProcessingEvents []ProcessingOutcome `json:"processing_outcomes"`
}

type SnapshotArtifact struct {
	Key        string
	DataBase64 string
	Records    int
}

func BuildSnapshot(input CreatorExport) (SnapshotArtifact, error) {
	if input.CreatorID == "" {
		return SnapshotArtifact{}, fmt.Errorf("creator_id is required")
	}
	if input.AsOf.IsZero() {
		return SnapshotArtifact{}, fmt.Errorf("as_of is required")
	}

	snapshot := Snapshot{SchemaVersion: 1, CreatorID: input.CreatorID, AsOf: input.AsOf.UTC()}
	for _, item := range input.AssetDeliveries {
		if !item.DeliveredAt.After(input.AsOf) {
			snapshot.AssetDeliveries = append(snapshot.AssetDeliveries, item)
		}
	}
	for _, item := range input.SubscriberEvents {
		if !item.UpdatedAt.After(input.AsOf) {
			snapshot.SubscriberEvents = append(snapshot.SubscriberEvents, item)
		}
	}
	for _, item := range input.ProcessingEvents {
		if !item.CompletedAt.After(input.AsOf) {
			snapshot.ProcessingEvents = append(snapshot.ProcessingEvents, item)
		}
	}

	sort.Slice(snapshot.AssetDeliveries, func(i, j int) bool {
		return snapshot.AssetDeliveries[i].DeliveryID < snapshot.AssetDeliveries[j].DeliveryID
	})
	sort.Slice(snapshot.SubscriberEvents, func(i, j int) bool {
		return snapshot.SubscriberEvents[i].SubscriberID < snapshot.SubscriberEvents[j].SubscriberID
	})
	sort.Slice(snapshot.ProcessingEvents, func(i, j int) bool {
		return snapshot.ProcessingEvents[i].ContentID < snapshot.ProcessingEvents[j].ContentID
	})

	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return SnapshotArtifact{}, fmt.Errorf("encode snapshot: %w", err)
	}
	digest := sha256.Sum256(data)
	shortDigest := hex.EncodeToString(digest[:8])
	date := snapshot.AsOf.Format("2006-01-02")
	key := fmt.Sprintf("creator-snapshots/date=%s/creator=%s/snapshot-%s.json", date, input.CreatorID, shortDigest)

	return SnapshotArtifact{
		Key:        key,
		DataBase64: base64.StdEncoding.EncodeToString(data),
		Records:    len(snapshot.AssetDeliveries) + len(snapshot.SubscriberEvents) + len(snapshot.ProcessingEvents),
	}, nil
}
