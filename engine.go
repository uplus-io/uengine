/*
 * Copyright (c) 2019 uplus.io
 */

package uengine

import (
	"fmt"
	"math/rand"
	"sort"
	"time"
	"github.com/uplus-io/ugo/hash"
	log "github.com/uplus-io/ugo/logger"
	"github.com/uplus-io/uengine/model"
)

type Engine struct {
	config     UEngineConfig
	meta       StoreOperator
	partitions []StoreOperator
	partSize   int

	table *EngineTable
}

func NewEngine(config UEngineConfig) *Engine {
	storeType := StoreTypeOfValue(config.Engine)
	partSize := len(config.Partitions)
	stores := make([]StoreOperator, partSize)
	meta := NewStore(StoreConfig{Path: config.Meta, Type: storeType})
	for i, path := range config.Partitions {
		store := NewStore(StoreConfig{Path: path, Type: storeType})
		stores[i] = NewStoreOperatorKV(store)
	}
	engine := &Engine{config: config, meta: NewStoreOperatorKV(meta), partitions: stores, partSize: partSize}
	engine.table = NewEngineTable(engine)
	engine.makeTestData()
	return engine
}

func (p *Engine) makeTestData() {
	parts := p.Parts()
	for i, part := range parts {
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		key := []byte(fmt.Sprintf("%d", i))
		data := NewData(*NewIdentity("test-ns", "test-tab", key), key)
		data.Version = int32(r.Intn(10))
		p.SetData(part.Id, *data, true)
	}
}

func (p *Engine) ValidatePartition(nodeId int32) ([]model.Partition, error) {
	partitions := make([]model.Partition, 0)
	for i, store := range p.partitions {
		ring := int32(hash.GenerateConsistentRing(nodeId, i))
		partition := store.Part()
		if partition == nil {
			partition := model.Partition{}
			partition.Version = VERSION
			partition.Id = ring
			partition.Index = int32(i)
			_, err := store.PartIfAbsent(partition)
			if err != nil {
				return nil, err
			}
		} else {
			if partition.Id != ring {
				return nil, ErrPartRingVerifyFailed
			}
		}
		partitions = append(partitions, *partition)
		log.Debugf("validate partition[%d] ring[%d]", i, ring)
	}
	return partitions, nil
}

func (p *Engine) Close() {
	p.meta.Close()
	for _, store := range p.partitions {
		store.Close()
	}
}

func (p *Engine) Table() *EngineTable {
	return p.table
}

func (p *Engine) SetData(partId int32, data Data, incrementVersion bool) error {
	partition := p.Table().Partition(partId)
	if partition == nil {
		return ErrPartNotFound
	}
	p.meta.NSIfAbsent(data.Id.Namespace)
	p.meta.TABIfAbsent(data.Id.Namespace, data.Id.Table)
	return p.part(partition.Index).SetData(data, incrementVersion)
}

func (p *Engine) GetData(partId int32, id Identity) (*model.DataMeta, *model.DataContent, error) {
	partition := p.Table().Partition(partId)
	if partition == nil {
		return nil, nil, ErrPartNotFound
	}
	return p.part(partition.Index).GetData(id)
}

func (p *Engine) part(partIndex int32) StoreOperator {
	return p.partitions[partIndex]
}

func (p *Engine) PartSize() int {
	return p.partSize
}

func (p *Engine) GetPart(partId int32) *model.Partition {
	return p.Table().Partition(partId)
}

func (p *Engine) GetPartOfIndex(partIndex int32) *model.Partition {
	return p.Table().PartitionOfIndex(partIndex)
}

func (p *Engine) SimilarPart(ring int32) *model.Partition {
	parts := p.Parts()
	for i := len(parts) - 1; i > 0; i-- {
		part := parts[i]
		if ring <= part.Id {
			return &part
		}
	}
	return nil
}

func (p *Engine) AddPart(part model.Partition) error {
	return p.Table().AddPartition(part)
}

func (p *Engine) Parts() []model.Partition {
	partitions := make([]model.Partition, 0)
	for _, store := range p.partitions {
		partition := store.Part()
		partitions = append(partitions, *partition)
	}
	return partitions
}

func (p *Engine) PartRanges() []int {
	ranges := make([]int, p.PartSize())
	for i, part := range p.Parts() {
		ranges[i] = int(part.Id)
	}
	sort.Ints(ranges)
	return ranges
}
