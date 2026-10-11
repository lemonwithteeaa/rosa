//go:build windows

package main

import "fmt"

type sqliteLabPassRepository struct{ store *rosaSQLiteStore }

func (r sqliteLabPassRepository) Save(pass LabPassRecord) error {
	if r.store == nil {
		return fmt.Errorf("SQLite Repository не активен")
	}
	return r.store.insertLabPass(pass)
}
func (r sqliteLabPassRepository) SaveBatch(passes []LabPassRecord) error {
	if r.store == nil {
		return fmt.Errorf("SQLite Repository не активен")
	}
	return r.store.insertLabPassBatch(passes)
}
func (r sqliteLabPassRepository) LoadAll() ([]LabPassRecord, error) {
	if r.store == nil {
		return nil, fmt.Errorf("SQLite Repository не активен")
	}
	return r.store.loadLabPasses()
}
func (r sqliteLabPassRepository) LoadForObject(key string) ([]LabPassRecord, error) {
	if r.store == nil {
		return nil, fmt.Errorf("SQLite Repository не активен")
	}
	return r.store.loadLabPassesForObject(key)
}
func (r sqliteLabPassRepository) LoadLatestSaved() ([]LabPassRecord, error) {
	if r.store == nil {
		return nil, fmt.Errorf("SQLite Repository не активен")
	}
	return r.store.loadLatestSavedLabPasses()
}

func activeLabPassRepository() LabPassRepository {
	if persistentStore == nil {
		return nil
	}
	return sqliteLabPassRepository{store: persistentStore}
}
