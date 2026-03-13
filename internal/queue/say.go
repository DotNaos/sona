package queue

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	queuePollInterval = 100 * time.Millisecond
	queuePause        = 500 * time.Millisecond
)

type SayQueue struct {
	baseDir string
	now     func() time.Time
	sleep   func(context.Context, time.Duration) error
}

type SaySlot struct {
	ticketPath string
	lockFile   *os.File
	queue      *SayQueue
}

func NewSayQueue() (*SayQueue, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return nil, fmt.Errorf("resolve user cache dir: %w", err)
	}

	baseDir := filepath.Join(cacheDir, "sona", "say-queue")
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return nil, fmt.Errorf("create say queue dir: %w", err)
	}

	return &SayQueue{
		baseDir: baseDir,
		now:     time.Now,
		sleep: func(ctx context.Context, d time.Duration) error {
			timer := time.NewTimer(d)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
				return nil
			}
		},
	}, nil
}

func (q *SayQueue) Acquire(ctx context.Context) (*SaySlot, error) {
	ticketPath, err := q.createTicket()
	if err != nil {
		return nil, err
	}

	for {
		if err := ctx.Err(); err != nil {
			_ = os.Remove(ticketPath)
			return nil, err
		}

		if err := q.cleanupStaleTickets(ticketPath); err != nil {
			_ = os.Remove(ticketPath)
			return nil, err
		}

		firstPath, err := q.firstTicketPath()
		if err != nil {
			_ = os.Remove(ticketPath)
			return nil, err
		}
		if firstPath != ticketPath {
			if err := q.sleep(ctx, queuePollInterval); err != nil {
				_ = os.Remove(ticketPath)
				return nil, err
			}
			continue
		}

		lockFile, err := os.OpenFile(filepath.Join(q.baseDir, "active.lock"), os.O_CREATE|os.O_RDWR, 0o644)
		if err != nil {
			_ = os.Remove(ticketPath)
			return nil, fmt.Errorf("open say queue lock: %w", err)
		}

		if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX); err != nil {
			lockFile.Close()
			_ = os.Remove(ticketPath)
			return nil, fmt.Errorf("lock say queue: %w", err)
		}

		firstPath, err = q.firstTicketPath()
		if err != nil {
			_ = syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)
			_ = lockFile.Close()
			_ = os.Remove(ticketPath)
			return nil, err
		}

		if firstPath == ticketPath {
			return &SaySlot{
				ticketPath: ticketPath,
				lockFile:   lockFile,
				queue:      q,
			}, nil
		}

		_ = syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)
		_ = lockFile.Close()
		if err := q.sleep(ctx, queuePollInterval); err != nil {
			_ = os.Remove(ticketPath)
			return nil, err
		}
	}
}

func (s *SaySlot) Release() error {
	var releaseErr error

	if err := s.queue.sleep(context.Background(), queuePause); err != nil {
		releaseErr = err
	}

	if err := os.Remove(s.ticketPath); err != nil && !os.IsNotExist(err) && releaseErr == nil {
		releaseErr = fmt.Errorf("remove say queue ticket: %w", err)
	}

	if s.lockFile != nil {
		if err := syscall.Flock(int(s.lockFile.Fd()), syscall.LOCK_UN); err != nil && releaseErr == nil {
			releaseErr = fmt.Errorf("unlock say queue: %w", err)
		}
		if err := s.lockFile.Close(); err != nil && releaseErr == nil {
			releaseErr = fmt.Errorf("close say queue lock: %w", err)
		}
	}

	return releaseErr
}

func (q *SayQueue) createTicket() (string, error) {
	now := q.now().UTC()
	randomSuffix := make([]byte, 4)
	if _, err := rand.Read(randomSuffix); err != nil {
		return "", fmt.Errorf("generate say queue suffix: %w", err)
	}

	name := fmt.Sprintf("%020d-%06d-%s.ticket", now.UnixNano(), os.Getpid(), hex.EncodeToString(randomSuffix))
	path := filepath.Join(q.baseDir, name)

	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return "", fmt.Errorf("create say queue ticket: %w", err)
	}
	defer file.Close()

	if _, err := file.WriteString(strconv.Itoa(os.Getpid())); err != nil {
		return "", fmt.Errorf("write say queue ticket: %w", err)
	}

	return path, nil
}

func (q *SayQueue) firstTicketPath() (string, error) {
	entries, err := q.ticketEntries()
	if err != nil {
		return "", err
	}
	if len(entries) == 0 {
		return "", fmt.Errorf("say queue is unexpectedly empty")
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})
	return filepath.Join(q.baseDir, entries[0].Name()), nil
}

func (q *SayQueue) ticketEntries() ([]fs.DirEntry, error) {
	entries, err := os.ReadDir(q.baseDir)
	if err != nil {
		return nil, fmt.Errorf("read say queue dir: %w", err)
	}

	var tickets []fs.DirEntry
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.HasSuffix(entry.Name(), ".ticket") {
			tickets = append(tickets, entry)
		}
	}
	return tickets, nil
}

func (q *SayQueue) cleanupStaleTickets(ownTicketPath string) error {
	entries, err := q.ticketEntries()
	if err != nil {
		return err
	}

	for _, entry := range entries {
		ticketPath := filepath.Join(q.baseDir, entry.Name())
		if ticketPath == ownTicketPath {
			continue
		}

		pid, err := pidFromTicketName(entry.Name())
		if err != nil {
			continue
		}

		stale, err := processMissing(pid)
		if err != nil {
			return err
		}
		if stale {
			if err := os.Remove(ticketPath); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("remove stale say queue ticket: %w", err)
			}
		}
	}

	return nil
}

func pidFromTicketName(name string) (int, error) {
	parts := strings.Split(name, "-")
	if len(parts) < 3 {
		return 0, fmt.Errorf("invalid ticket name %q", name)
	}
	return strconv.Atoi(parts[1])
}

func processMissing(pid int) (bool, error) {
	err := syscall.Kill(pid, 0)
	switch err {
	case nil:
		return false, nil
	case syscall.ESRCH:
		return true, nil
	case syscall.EPERM:
		return false, nil
	default:
		return false, fmt.Errorf("check process %d: %w", pid, err)
	}
}
