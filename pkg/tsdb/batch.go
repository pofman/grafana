package tsdb

import "errors"

type Batch struct {
	DataSourceId int64
	Queries      QuerySlice
	Depends      map[string]bool
	Done         bool
	Started      bool
}

type BatchSlice []*Batch

func newBatch(dsId int64, queries QuerySlice) *Batch {
	return &Batch{
		DataSourceId: dsId,
		Queries:      queries,
		Depends:      make(map[string]bool),
	}
}

func (bg *Batch) process(context *QueryContext) {
	executor := getExecutorFor(bg.Queries[0].DataSource)

	if executor == nil {
		bg.Done = true
		result := &BatchResult{
			Error:        errors.New("Could not find executor for data source type: " + bg.Queries[0].DataSource.PluginId),
			QueryResults: make(map[string]*QueryResult),
		}
		for _, query := range bg.Queries {
			result.QueryResults[query.RefId] = &QueryResult{Error: result.Error}
		}
		context.ResultsChan <- result
		return
	}

	res := executor.Execute(bg.Queries, context)
	bg.Done = true
	context.ResultsChan <- res
}

func (bg *Batch) addQuery(query *Query) {
	bg.Queries = append(bg.Queries, query)
}

func (bg *Batch) allDependenciesAreIn(context *QueryContext) bool {
	for key := range bg.Depends {
		if _, exists := context.Results[key]; !exists {
			return false
		}
	}

	return true
}

func getBatches(req *Request) (BatchSlice, error) {
	batches := make(BatchSlice, 0)

	for _, query := range req.Queries {
		if foundBatch := findMatchingBatchGroup(query, batches); foundBatch != nil {
			foundBatch.addQuery(query)
		} else {
			newBatch := newBatch(query.DataSource.Id, QuerySlice{query})
			batches = append(batches, newBatch)

			for _, refId := range query.Depends {
				for _, batch := range batches {
					for _, batchQuery := range batch.Queries {
						if batchQuery.RefId == refId {
							newBatch.Depends[refId] = true
						}
					}
				}
			}
		}
	}

	return batches, nil
}

func findMatchingBatchGroup(query *Query, batches BatchSlice) *Batch {
	for _, batch := range batches {
		if batch.DataSourceId == query.DataSource.Id {
			return batch
		}
	}
	return nil
}
