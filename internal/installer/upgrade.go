package installer

import (
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/lubaskinc0de/beatstash/internal/installer/config"
)

func (s *setup) upgrade(ctx context.Context) error {
	s.t.Banner("Upgrade beatstash",
		fmt.Sprintf("Move an installation to beatstash v%s.", s.version),
		"Each change is shown before it is made.")
	s.t.Stages(4)
	s.t.Stage("Find the installation")
	if err := s.locate(ctx); err != nil {
		return err
	}
	installed, err := parseVersion(s.project.Env().Get("BEATSTASH_VERSION"))
	if err != nil {
		return fmt.Errorf("cannot read BEATSTASH_VERSION in %s: %w", s.project.Env().Path, err)
	}
	switch order := s.version.compare(installed); {
	case order == 0:
		s.t.Done("%s already runs beatstash v%s.", s.project.Dir, installed)
		return nil
	case order < 0:
		return fmt.Errorf("%s runs v%s, newer than v%s; database migrations cannot be undone, so downgrades are not supported", s.project.Dir, installed, s.version)
	}
	s.t.Say("Upgrading %s from v%s to v%s.", s.project.Dir, installed, s.version)

	s.t.Stage("Review deployment files")
	if err := s.offerFile("compose.yml", s.composeFile(), "Changes you made to compose.yml by hand are lost; the diff shows them."); err != nil {
		return err
	}
	if exists(s.project.Path(proxyFile)) {
		if err := s.offerFile(proxyFile, s.ProxyTemplate, "Changes you made to it by hand are lost; the diff shows them."); err != nil {
			return err
		}
	}
	doc, err := s.readConfig()
	if err != nil {
		return err
	}
	merged, err := config.Merge(doc, s.ConfigTemplate)
	if err != nil {
		return err
	}
	for _, key := range merged.Unknown {
		s.t.Warn("config.toml has %s, which v%s no longer uses; remove it when convenient.", key, s.version)
	}
	if err := s.offerFile("config.toml", merged.Text, "New settings get their default values; your values and comments stay."); err != nil {
		return err
	}

	s.t.Stage("Back up the database")
	if err := s.backUp(ctx, installed); err != nil {
		return err
	}

	s.t.Stage("Start the new version")
	s.t.Say("This downloads v%s and restarts the stack. The bot migrates its database when it starts.", s.version)
	if !s.t.Confirm(fmt.Sprintf("Switch to v%s and restart", s.version), false) {
		s.t.Warn("The installation stays on v%s.", installed)
		return nil
	}
	if err := s.project.Env().Set("BEATSTASH_VERSION", s.version.String()); err != nil {
		return err
	}
	if err := s.project.Run(ctx, "pull"); err != nil {
		return err
	}
	if err := s.project.Run(ctx, "up", "-d"); err != nil {
		return err
	}
	s.t.Done("beatstash v%s is running.", s.version)
	return nil
}

// offerFile shows how the release changes a deployment file and replaces it
// once the person agrees, keeping the old one beside it.
func (s *setup) offerFile(name, next, note string) error {
	path := s.project.Path(name)
	data, err := os.ReadFile(path) //nolint:gosec // G304: files of the installation
	if err != nil {
		return err
	}
	if string(data) == next {
		s.t.Done("%s needs no changes.", name)
		return nil
	}
	s.t.Diff(path, string(data), next)
	s.t.Note("%s", note)
	if !s.t.Confirm(fmt.Sprintf("Update %s (the current one is kept as %s.bak)", name, name), false) {
		s.t.Warn("Kept %s; the new version may need these changes.", name)
		return nil
	}
	if err := config.WriteFile(path+".bak", string(data)); err != nil {
		return err
	}
	if err := config.WriteFile(path, next); err != nil {
		return err
	}
	s.t.Done("Updated %s.", name)
	return nil
}

func (s *setup) backUp(ctx context.Context, installed version) error {
	s.t.Warn("Database migrations cannot be undone: without a backup there is no way back to v%s.", installed)
	if !s.t.Confirm("Back up the database first", false) {
		return nil
	}
	if err := s.project.Run(ctx, "up", "-d", "--wait", "postgres"); err != nil {
		return err
	}
	dir := s.project.Path("backups")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	path := filepath.Join(dir, fmt.Sprintf("beatstash-%s-%s.sql.gz", installed, time.Now().Format("20060102-150405")))
	if err := s.dump(ctx, path); err != nil {
		_ = os.Remove(path)
		return err
	}
	s.t.Done("Saved %s.", path)
	s.t.Note("To restore it: gunzip -c %s | docker compose exec -T postgres psql -U beatstash beatstash", path)
	return nil
}

func (s *setup) dump(ctx context.Context, path string) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600) //nolint:gosec // G304: a new file in the installation
	if err != nil {
		return err
	}
	archive := gzip.NewWriter(file)
	err = s.project.Pipe(ctx, archive, "exec", "-T", "postgres", "pg_dump", "-U", "beatstash", "beatstash")
	return errors.Join(err, archive.Close(), file.Close())
}
