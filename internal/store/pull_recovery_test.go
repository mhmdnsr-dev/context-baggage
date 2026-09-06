package store

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func recoveryStore(t *testing.T) Store {
	t.Helper()
	s := New(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	return s
}

func validRecoveryRecord() PullRecovery {
	return PullRecovery{
		Format:             PullRecoveryFormatVersion,
		DestinationType:    DestinationGitHub,
		DestinationID:      "dst_0123456789abcdef0123456789abcdef",
		RepositoryIdentity: "github.com/owner/repo",
		TargetCommitID:     strings.Repeat("a", 40),
		PreLocalHash:       strings.Repeat("b", 64),
		TargetPortableHash: strings.Repeat("c", 64),
	}
}

func TestPullRecoveryRoundTrip(t *testing.T) {
	s := recoveryStore(t)
	record := validRecoveryRecord()
	if err := s.WritePullRecovery(record); err != nil {
		t.Fatal(err)
	}
	got, err := s.ReadPullRecovery()
	if err != nil {
		t.Fatal(err)
	}
	if got != record {
		t.Fatalf("recovery round trip mismatch: %+v != %+v", got, record)
	}
	if exists, err := s.PullRecoveryExists(); err != nil || !exists {
		t.Fatalf("recovery exists = %t, err = %v", exists, err)
	}
	if err := s.RemovePullRecovery(); err != nil {
		t.Fatal(err)
	}
	if exists, err := s.PullRecoveryExists(); err != nil || exists {
		t.Fatalf("recovery exists after remove = %t, err = %v", exists, err)
	}
}

func TestPullRecoveryReadMissing(t *testing.T) {
	s := recoveryStore(t)
	_, err := s.ReadPullRecovery()
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected not exist, got %v", err)
	}
}

func TestPullRecoveryRejectsMalformed(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*PullRecovery)
	}{
		{name: "wrong format", mutate: func(r *PullRecovery) { r.Format = 2 }},
		{name: "bad destination type", mutate: func(r *PullRecovery) { r.DestinationType = DestinationFilesystem }},
		{name: "bad destination id", mutate: func(r *PullRecovery) { r.DestinationID = "dst_zzzz" }},
		{name: "empty repository", mutate: func(r *PullRecovery) { r.RepositoryIdentity = "" }},
		{name: "bad commit id", mutate: func(r *PullRecovery) { r.TargetCommitID = "not-a-sha" }},
		{name: "bad pre hash", mutate: func(r *PullRecovery) { r.PreLocalHash = "short" }},
		{name: "bad target hash", mutate: func(r *PullRecovery) { r.TargetPortableHash = "short" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			record := validRecoveryRecord()
			tt.mutate(&record)
			s := recoveryStore(t)
			if err := s.WritePullRecovery(record); !errors.Is(err, ErrPullRecoveryMalformed) {
				t.Fatalf("expected malformed error, got %v", err)
			}
		})
	}
}

func TestPullRecoveryRejectsOversized(t *testing.T) {
	s := recoveryStore(t)
	path := s.PullRecoveryPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	payload := strings.Repeat("x", maxPullRecoveryBytes+1)
	if err := os.WriteFile(path, []byte(payload), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := s.ReadPullRecovery()
	if !errors.Is(err, ErrPullRecoveryMalformed) {
		t.Fatalf("expected malformed oversized, got %v", err)
	}
}

func TestPullRecoveryDurabilityFailureRefusesInstall(t *testing.T) {
	s := recoveryStore(t)
	original := durableInstaller
	durableInstaller = func(tempPath, targetPath string) error {
		return errors.New("durable installation failed")
	}
	defer func() { durableInstaller = original }()
	if err := s.WritePullRecovery(validRecoveryRecord()); err == nil {
		t.Fatal("expected durability failure to abort installation")
	}
}

func TestPullRecoveryDurabilityRunsAfterInstall(t *testing.T) {
	s := recoveryStore(t)
	original := durableInstaller
	var installed bool
	durableInstaller = func(tempPath, targetPath string) error {
		installed = true
		if _, err := os.Stat(s.PullRecoveryPath()); err == nil {
			t.Fatalf("durability step ran before recovery file installed: %v", err)
		}
		if _, err := os.Stat(tempPath); err != nil {
			t.Fatalf("temporary file missing before install: %v", err)
		}
		if err := os.Rename(tempPath, targetPath); err != nil {
			t.Fatal(err)
		}
		return nil
	}
	defer func() { durableInstaller = original }()
	if err := s.WritePullRecovery(validRecoveryRecord()); err != nil {
		t.Fatal(err)
	}
	if !installed {
		t.Fatal("durable installation step was not invoked")
	}
	got, err := s.ReadPullRecovery()
	if err != nil {
		t.Fatal(err)
	}
	if got != validRecoveryRecord() {
		t.Fatalf("recovery record mismatch after durable install: %+v", got)
	}
}

func TestPullRecoveryRejectsUnknownField(t *testing.T) {
	s := recoveryStore(t)
	path := s.PullRecoveryPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	content := "format: 1\ndestinationType: github\ndestinationId: dst_0123456789abcdef0123456789abcdef\nrepositoryIdentity: github.com/owner/repo\ntargetCommitId: " + strings.Repeat("a", 40) + "\npreLocalHash: " + strings.Repeat("b", 64) + "\ntargetPortableHash: " + strings.Repeat("c", 64) + "\nunknown: value\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ReadPullRecovery(); !errors.Is(err, ErrPullRecoveryMalformed) {
		t.Fatalf("expected malformed unknown field, got %v", err)
	}
}
