package main

import "etcura/internal/model"

func (a *App) StartCompare(req model.CompareRequest) (model.CompareInfo, error) {
	return a.workbench.StartCompare(a.ctx, req)
}

func (a *App) CancelCompare(id string) { a.workbench.CancelCompare(id) }

func (a *App) ComparePage(id, cursor, filter string) (model.ComparePage, error) {
	return a.workbench.ComparePage(id, cursor, filter)
}

func (a *App) CompareDetail(id, key string) (model.CompareDetail, error) {
	return a.workbench.CompareDetail(id, key)
}

func (a *App) PreviewSync(id string, keys []string) (model.SyncPlan, error) {
	return a.workbench.PreviewSync(id, keys)
}

func (a *App) ApplySync(id string, permanentKeys []string) ([]model.SyncResult, error) {
	return a.workbench.ApplySync(id, permanentKeys)
}

func (a *App) StartWatch(req model.WatchStart) (string, error) {
	return a.workbench.StartWatch(a.ctx, req)
}

func (a *App) PollWatch(id string) (model.WatchBatch, error) { return a.workbench.PollWatch(id) }
func (a *App) StopWatch(id string)                           { a.workbench.StopWatch(id) }
