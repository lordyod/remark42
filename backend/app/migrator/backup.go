package migrator

import (
	"compress/gzip"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strings"
	"time"

	log "github.com/go-pkgz/lgr"
	"github.com/pkg/errors"
)

// AutoBackup struct handles daily backups params for siteID
type AutoBackup struct {
	Exporter       Exporter
	BackupLocation string
	SiteID         string
	KeepMax        int
	Duration       time.Duration
}

// Do runs daily export to local files, keeps up to keepMax backups for given siteID
func (ab AutoBackup) Do(ctx context.Context) {
	log.Printf("[INFO] activate auto-backup for %s under %s, duration %s", ab.SiteID, ab.BackupLocation, ab.Duration)
	tick := time.NewTicker(ab.Duration)
	defer tick.Stop()
	log.Printf("[DEBUG] first backup for %s at %s", ab.SiteID, time.Now().Add(ab.Duration))

	for {
		select {
		case <-tick.C:
			if _, err := ab.makeBackup(); err != nil {
				log.Printf("[WARN] auto-backup for %s failed, %s", ab.SiteID, err)
				continue
			}
			ab.removeOldBackupFiles()
			log.Printf("[DEBUG] next backup for %s at %s", ab.SiteID, time.Now().Add(ab.Duration))
		case <-ctx.Done():
			log.Printf("[WARN] terminated autobackup for %s", ab.SiteID)
			return
		}
	}
}

func (ab AutoBackup) makeBackup() (string, error) {
	log.Printf("[DEBUG] make backup for %s", ab.SiteID)
	backupFile := fmt.Sprintf("%s/backup-%s-%s.gz", ab.BackupLocation, ab.SiteID, time.Now().Format("20060102"))
	fh, err := os.Create(backupFile)
	if err != nil {
		return "", errors.Wrapf(err, "can't create backup file %s", backupFile)
	}
	gz := gzip.NewWriter(fh)

	if _, err = ab.Exporter.Export(gz, ab.SiteID); err != nil {
		return "", errors.Wrapf(err, "export failed for %s", ab.SiteID)
	}
	if err = gz.Close(); err != nil {
		return "", errors.Wrapf(err, "can't close gz for %s", backupFile)
	}
	if err = fh.Close(); err != nil {
		return "", errors.Wrapf(err, "can't close file handler for %s", backupFile)
	}
	log.Printf("[DEBUG] created backup file %s", backupFile)
	return backupFile, nil
}

func (ab AutoBackup) removeOldBackupFiles() {
	files, err := ioutil.ReadDir(ab.BackupLocation)
	if err != nil {
		log.Printf("[WARN] can't read files in backup directory %s, %s", ab.BackupLocation, err)
		return
	}
	backFiles := []os.FileInfo{}
	for _, file := range files {
		if strings.HasPrefix(file.Name(), "backup-"+ab.SiteID) {
			backFiles = append(backFiles, file)
		}
	}
	sort.Slice(backFiles, func(i int, j int) bool { return backFiles[i].Name() < backFiles[j].Name() })

	if len(backFiles) > ab.KeepMax {
		for i := 0; i < len(backFiles)-ab.KeepMax; i++ {
			fpath := ab.BackupLocation + "/" + backFiles[i].Name()
			if e := os.Remove(fpath); e != nil {
				log.Printf("[WARN] can't delete %s, %s", fpath, err)
				continue
			}
			log.Printf("[DEBUG] removed %s", fpath)
		}
	}
}
