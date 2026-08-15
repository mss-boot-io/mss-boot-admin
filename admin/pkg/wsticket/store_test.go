package wsticket

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func validRecord() Record {
	return Record{
		UserID:    "user-1",
		RoleID:    "role-1",
		SessionID: "session-1",
		Origin:    "https://admin.example",
	}
}

func TestStoreIssueConsumeAndReplayLocal(t *testing.T) {
	store := New()
	ticket, issued, err := store.Issue(context.Background(), nil, validRecord(), 30*time.Second)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	if len(ticket) != 43 || issued.ExpiresAt.IsZero() {
		t.Fatalf("Issue() = (%q, %#v)", ticket, issued)
	}
	if !Valid(ticket) || Valid("short") || Valid(ticket+"=") {
		t.Fatalf("ticket canonical validation failed for %q", ticket)
	}
	record, err := store.Consume(context.Background(), nil, ticket)
	if err != nil || record.UserID != "user-1" {
		t.Fatalf("Consume() = (%#v, %v)", record, err)
	}
	if _, err = store.Consume(context.Background(), nil, ticket); err != ErrNotFound {
		t.Fatalf("replay error = %v, want %v", err, ErrNotFound)
	}
}

func TestStoreExpiredTicketIsBurned(t *testing.T) {
	store := New()
	now := time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }
	ticket, _, err := store.Issue(context.Background(), nil, validRecord(), 5*time.Second)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	now = now.Add(6 * time.Second)
	if _, err = store.Consume(context.Background(), nil, ticket); err != ErrExpired {
		t.Fatalf("expired Consume() error = %v, want %v", err, ErrExpired)
	}
	if _, err = store.Consume(context.Background(), nil, ticket); err != ErrNotFound {
		t.Fatalf("expired replay error = %v, want %v", err, ErrNotFound)
	}
}

func TestStoreConsumeIsSingleUseAcrossRedisConsumers(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	store := New()
	ticket, _, err := store.Issue(context.Background(), client, validRecord(), time.Minute)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}

	const consumers = 8
	var wait sync.WaitGroup
	wait.Add(consumers)
	results := make(chan error, consumers)
	for range consumers {
		go func() {
			defer wait.Done()
			_, consumeErr := store.Consume(context.Background(), client, ticket)
			results <- consumeErr
		}()
	}
	wait.Wait()
	close(results)
	successes := 0
	for consumeErr := range results {
		if consumeErr == nil {
			successes++
		} else if consumeErr != ErrNotFound {
			t.Fatalf("Consume() error = %v", consumeErr)
		}
	}
	if successes != 1 {
		t.Fatalf("successful consumers = %d, want 1", successes)
	}
}

func TestStoreRejectsIncompleteRecord(t *testing.T) {
	if _, _, err := New().Issue(context.Background(), nil, Record{}, time.Minute); err == nil {
		t.Fatal("Issue() accepted an incomplete record")
	}
	record := validRecord()
	record.Origin = "https://admin.example.attacker.test/path"
	if _, _, err := New().Issue(context.Background(), nil, record, time.Minute); err == nil {
		t.Fatal("Issue() accepted an invalid origin")
	}
}
