//go:build windows

package main

import (
	"os"
	"path/filepath"
	"strings"
)

// secureDynamogramOutputs moves generated preview/curve artifacts into the
// encrypted per-user vault. The UI materializes short-lived plaintext copies
// only while the current user session is unlocked.
func secureDynamogramOutputs(a *DynamogramAnalysis) error {
	if a == nil {
		return nil
	}
	if a.PreviewPath != "" && !isSecureAssetPath(a.PreviewPath) {
		if _, err := os.Stat(a.PreviewPath); err == nil {
			p, _, e := secureStoreFile(a.PreviewPath)
			if e != nil {
				return e
			}
			_ = os.Remove(a.PreviewPath)
			a.PreviewPath = p
		}
	}
	if a.ReconstructionPath != "" && !isSecureAssetPath(a.ReconstructionPath) {
		if _, err := os.Stat(a.ReconstructionPath); err == nil {
			p, _, e := secureStoreFile(a.ReconstructionPath)
			if e != nil {
				return e
			}
			_ = os.Remove(a.ReconstructionPath)
			a.ReconstructionPath = p
		}
	}
	if a.CSVPath != "" && !isSecureAssetPath(a.CSVPath) {
		if _, err := os.Stat(a.CSVPath); err == nil {
			p, _, e := secureStoreFile(a.CSVPath)
			if e != nil {
				return e
			}
			_ = os.Remove(a.CSVPath)
			a.CSVPath = p
		}
	}
	return nil
}

func writeSecureDynCSV(a *DynamogramAnalysis, pts []DynamogramPoint) error {
	if a == nil {
		return nil
	}
	if !isSecureAssetPath(a.CSVPath) {
		if err := writeDynCSV(*a, pts); err != nil {
			return err
		}
		return secureDynamogramOutputs(a)
	}
	td, err := ensureSessionTempDir()
	if err != nil {
		return err
	}
	tmp := filepath.Join(td, a.ID+"_curve.csv")
	ac := *a
	ac.CSVPath = tmp
	if err := writeDynCSV(ac, pts); err != nil {
		return err
	}
	stored, _, err := secureStoreFile(tmp)
	_ = os.Remove(tmp)
	if err != nil {
		return err
	}
	a.CSVPath = stored
	return nil
}

func writeSecureDynReconstruction(a *DynamogramAnalysis, pts []DynamogramPoint) error {
	if a == nil {
		return nil
	}
	td, err := ensureSessionTempDir()
	if err != nil {
		return err
	}
	tmp := filepath.Join(td, a.ID+"_axes.png")
	if err = writeDigitalDynPreview(a, pts, tmp); err != nil {
		return err
	}
	stored, _, err := secureStoreFile(tmp)
	_ = os.Remove(tmp)
	if err != nil {
		return err
	}
	if strings.Contains(strings.ToLower(a.DetectionMode), "цифров") {
		a.PreviewPath = stored
	} else {
		a.ReconstructionPath = stored
	}
	return nil
}

func materializeDynPreview(path string) (string, error) {
	return secureMaterialize(path, ".png")
}

func materializeDynCSV(path string) (string, error) {
	return secureMaterialize(path, ".csv")
}

func materializeDynReconstruction(path string) (string, error) {
	return secureMaterialize(path, ".png")
}

func materializeDynSource(a *DynamogramAnalysis) (string, error) {
	if a == nil {
		return "", os.ErrInvalid
	}
	p := a.SourceCopy
	if p == "" {
		p = a.SourceFile
	}
	ext := filepath.Ext(a.SourceFile)
	if ext == "" {
		ext = ".png"
	}
	return secureMaterialize(p, ext)
}
