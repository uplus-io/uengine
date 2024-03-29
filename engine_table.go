/*
 * Copyright (c) 2019 uplus.io 
 */

package uengine

import (
	"github.com/uplus-io/uengine/model"
	log "github.com/uplus-io/ugo/logger"

)

const (
	ENGINE_NAMESPACE_SYSTEM = "_sys"  // 系统命名空间
	ENGINE_NAMESPACE_USER   = "_user" // 用户命名空间

	ENGINE_TABLE_NAMESPACES = "_ns"      //系统命名空间表名
	ENGINE_TABLE_TABLES     = "_tab"     //系统表名
	ENGINE_TABLE_PARTITIONS = "_part"    //系统分区表名
	ENGINE_TABLE_CLUSTERS   = "_cluster" //集群信息存储表
	ENGINE_TABLE_METAS      = "_meta"    //系统元数据表名

	ENGINE_KEY_META_STORAGE    = "storage" //存储元数据主键
	ENGINE_KEY_META_PART       = "part"    //分区元数据主键
	ENGINE_KEY_META_REPOSITORY = "repo"    //分区元数据主键
)

var (
	EMPTY_KEY = []byte{}
)

type EngineTable struct {
	engine *Engine
	parts  map[int32]*model.Partition
}

func NewEngineTable(engine *Engine) *EngineTable {
	table := &EngineTable{engine: engine, parts: make(map[int32]*model.Partition)}
	table.recoverPartition()
	return table
}

func NewIdOfNs(key []byte) *Identity {
	return NewIdentity(ENGINE_NAMESPACE_SYSTEM, ENGINE_TABLE_NAMESPACES, key)
}

func NewIdOfPart(key []byte) *Identity {
	return NewIdentity(ENGINE_NAMESPACE_SYSTEM, ENGINE_TABLE_PARTITIONS, key)
}

func NewIdOfTab(namespace string, key []byte) *Identity {
	return NewIdentity(namespace, ENGINE_TABLE_TABLES, key)
}

func NewIdOfData(namespace, table string, key []byte) *Identity {
	return NewIdentity(namespace, table, key)
}

func (p *EngineTable) recoverPartition() {
	partSize := len(p.engine.config.Partitions)
	for i := 0; i < partSize; i++ {
		operator := p.engine.partitions[i]
		partition := operator.Part()
		if partition != nil {
			p.parts[partition.Id] = partition
		}
	}
}

func (p *EngineTable) Repository() *model.Repository {
	repo := &model.Repository{}
	err := p.engine.meta.SysGet(ENGINE_TABLE_CLUSTERS, ENGINE_KEY_META_REPOSITORY, repo)
	if err != nil && err != ErrDbKeyNotFound {
		log.Errorf("get cluster repository meta")
		return nil
	}
	return repo
}

func (p *EngineTable) RepositoryUpdate(repository model.Repository) error {
	return p.engine.meta.SysSet(ENGINE_TABLE_CLUSTERS, ENGINE_KEY_META_REPOSITORY, &repository)
}

func (p *EngineTable) PartitionOfIndex(partIndex int32) *model.Partition {
	if int(partIndex) >= len(p.parts) {
		partIndex = 0
	}
	return p.engine.part(partIndex).Part()
}

func (p *EngineTable) Partition(partId int32) *model.Partition {
	partition := p.parts[partId]
	return partition
}

func (p *EngineTable) AddPartition(part model.Partition) error {
	newPart, err := p.engine.part(part.Index).PartIfAbsent(part)
	if err != nil {
		return err
	}
	p.parts[newPart.Id] = newPart
	return nil
}
