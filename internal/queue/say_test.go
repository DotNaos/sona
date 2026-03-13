package queue

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPidFromTicketName(t *testing.T) {
	pid, err := pidFromTicketName("00000000000000000001-012345-abcdef01.ticket")
	if err != nil {
		t.Fatalf("pidFromTicketName() error = %v", err)
	}
	if pid != 12345 {
		t.Fatalf("pidFromTicketName() = %d, want 12345", pid)
	}
}

func TestCleanupStaleTicketsRemovesDeadProcess(t *testing.T) {
	baseDir := t.TempDir()
	queue := &SayQueue{
		baseDir: baseDir,
		now:     time.Now,
		sleep: func(context.Context, time.Duration) error {
			return nil
		},
	}

	staleTicket := filepath.Join(baseDir, "00000000000000000001-999999-abcdef01.ticket")
	if err := os.WriteFile(staleTicket, []byte("999999"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	ownTicket := filepath.Join(baseDir, "00000000000000000002-000001-bcdef012.ticket")
	if err := os.WriteFile(ownTicket, []byte("1"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if err := queue.cleanupStaleTickets(ownTicket); err != nil {
		t.Fatalf("cleanupStaleTickets() error = %v", err)
	}

	if _, err := os.Stat(staleTicket); !os.IsNotExist(err) {
		t.Fatalf("stale ticket still exists, stat err = %v", err)
	}
}
