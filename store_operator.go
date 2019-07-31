package uengine

import (
	"fmt"
	ggproto "github.com/golang/protobuf/proto"
	"github.com/uplus-io/uengine/model"
	"github.com/uplus-io/ugo/hash"
	log "github.com/uplus-io/ugo/logger"
	proto "github.com/uplus-io/ugo/proto"
	"github.com/uplus-io/ugo/utils"
)

type StoreOperator interface {
	Close() error
	//系统操作

	PartIfAbsent(part model.Partition) (*model.Partition, error)
	Part() *model.Partition

	MetaSeek(identity Identity, iter StoreMetaIterator) error
	MetaForEach(iter StoreMetaIterator) error
	DataSeek(identity Identity, iter StoreIterator) error
	DataForEach(iter StoreIterator) error

	NSs() []model.Namespace
	NS(namespace string) *model.Namespace
	NSValue(namespaceId int32) *model.Namespace
	NSIfAbsent(namespace string) *model.Namespace

	TABs(namespace string) []model.Table
	TAB(namespace, table string) *model.Table
	TABValue(namespace string, tableId int32) *model.Table
	TABIfAbsent(namespace, table string) *model.Table

	SysSet(table, key string, message ggproto.Message) error
	SysGet(table, key string, message ggproto.Message) error

	SetMeta(identity Identity, meta model.DataMeta) error
	GetMeta(identity Identity) (*model.DataMeta, error)
	SetData(data Data, incrementVersion bool) error
	GetData(identity Identity) (*model.DataMeta, *model.DataContent, error)
}

type StoreOperatorKV struct {
	store Store
}

func NewStoreOperatorKV(store Store) *StoreOperatorKV {
	return &StoreOperatorKV{store: store}
}

func (p *StoreOperatorKV) Close() error {
	return p.store.Close()
}

func (p *StoreOperatorKV) PartIfAbsent(part model.Partition) (*model.Partition, error) {
	partition := p.Part()
	if partition == nil {
		identity := NewIdOfPart([]byte(ENGINE_KEY_META_PART))
		bytes, err := proto.Marshal(&part)
		if err != nil {
			return nil, err
		}
		err = p.store.Set(identity.IdBytes(), bytes)
		if err != nil {
			return nil, err
		}
		return &part, nil
	}
	return partition, nil
}
func (p *StoreOperatorKV) Part() *model.Partition {
	identity := NewIdOfPart([]byte(ENGINE_KEY_META_PART))
	bytes, err := p.store.Get(identity.IdBytes())
	if err != nil {
		return nil
	}
	partition := &model.Partition{}
	err = proto.Unmarshal(bytes, partition)
	if err != nil {
		return nil
	}
	return partition
}

func (p *StoreOperatorKV) MetaSeek(identity Identity, iter StoreMetaIterator) error {
	return p.store.Seek(IdentityMetaId(identity), func(key, data []byte) bool {
		meta := model.DataMeta{}
		err := proto.Unmarshal(data, &meta)
		if err != nil {
			return false
		}
		return iter(key, meta)
	});
}
func (p *StoreOperatorKV) MetaForEach(iter StoreMetaIterator) error {
	return p.store.Seek(FLAG_META, func(key, data []byte) bool {
		meta := model.DataMeta{}
		err := proto.Unmarshal(data, &meta)
		if err != nil {
			return false
		}
		return iter(key, meta)
	})
}
func (p *StoreOperatorKV) DataSeek(identity Identity, iter StoreIterator) error {
	return p.store.Seek(IdentityDataId(identity), iter)
}
func (p *StoreOperatorKV) DataForEach(iter StoreIterator) error {
	return p.store.Seek(FLAG_DATA, iter)
}

func (p *StoreOperatorKV) NSs() []model.Namespace {
	nss := make([]model.Namespace, 0)
	identity := NewIdOfNs(EMPTY_KEY)
	p.store.Seek(identity.IdBytes(), func(key, data []byte) bool {
		namespace := &model.Namespace{}
		err := proto.Unmarshal(data, namespace)
		if err != nil {
			return false
		}
		nss = append(nss, *namespace)
		return true
	})
	return nss
}

func (p *StoreOperatorKV) NS(namespace string) *model.Namespace {
	return p.NSValue(hash.Int32Of(namespace))
}
func (p *StoreOperatorKV) NSValue(nsId int32) *model.Namespace {
	ns := &model.Namespace{}
	identity := NewIdOfNs(utils.LInt32ToBytes(nsId))
	data, err := p.store.Get(identity.IdBytes())
	if err != nil {
		log.Errorf("get namespace[%s] found error - %v", nsId, err)
		return nil
	}
	err = proto.Unmarshal(data, ns)
	if err != nil {
		log.Errorf("unmarshal namespace[%s] found error - %v", nsId, err)
		return nil
	}
	return ns
}
func (p *StoreOperatorKV) NSIfAbsent(namespace string) *model.Namespace {
	nsId := hash.Int32Of(namespace)
	ns := &model.Namespace{}
	//identity := NewIdOfNs(utils.LInt32ToBytes(nsId))
	identity := NewIdOfNs([]byte(fmt.Sprintf("%d", nsId)))
	data, err := p.store.Get(identity.IdBytes())
	if err == ErrDbKeyNotFound {
		ns.Id = nsId
		ns.Name = namespace
		ns.Desc = model.NewDescription(identity.NamespaceId, identity.TableId)
		nsData, _ := proto.Marshal(ns)
		err := p.store.Set(identity.IdBytes(), nsData)
		if err != nil {
			log.Errorf("create namespace[%s] fail - %v", namespace, err)
			return nil
		}
		return ns
	}
	if err != nil {
		log.Errorf("get namespace[%s] fail - %v", namespace, err)
		return nil
	}
	err = proto.Unmarshal(data, ns)
	if err != nil {
		log.Errorf("found namespace[%s],but unmarshal fail - %v", namespace, err)
		return nil
	}
	return ns
}

func (p *StoreOperatorKV) TABs(namespace string) []model.Table {
	tables := make([]model.Table, 0)
	identity := NewIdOfTab(namespace, EMPTY_KEY)
	p.store.Seek(identity.IdBytes(), func(key, data []byte) bool {
		table := &model.Table{}
		err := proto.Unmarshal(data, table)
		if err != nil {
			return false
		}
		tables = append(tables, *table)
		return true
	})
	return tables
}
func (p *StoreOperatorKV) TAB(namespace string, tab string) *model.Table {
	return p.TABValue(namespace, hash.Int32Of(tab))
}
func (p *StoreOperatorKV) TABValue(namespace string, tabId int32) *model.Table {
	table := &model.Table{}
	identity := NewIdOfTab(namespace, utils.LInt32ToBytes(tabId))
	data, err := p.store.Get(identity.IdBytes())
	if err != nil {
		log.Errorf("namespace[%s] get table[%s] found error - %v", namespace, tabId, err)
		return nil
	}
	err = proto.Unmarshal(data, table)
	if err != nil {
		log.Errorf("namespace[%s] unmarshal table[%s] found error - %v", namespace, tabId, err)
		return nil
	}
	return table
}
func (p *StoreOperatorKV) TABIfAbsent(namespace string, tab string) *model.Table {
	tabId := hash.Int32Of(tab)
	table := &model.Table{}
	//identity := NewIdOfTab(namespace, utils.LInt32ToBytes(tabId))
	identity := NewIdOfTab(namespace, []byte(fmt.Sprintf("%d", tabId)))
	data, err := p.store.Get(identity.IdBytes())
	if err == ErrDbKeyNotFound {
		table.Id = tabId
		table.Name = tab
		table.Desc = model.NewDescription(identity.NamespaceId, identity.TableId)
		nsData, _ := proto.Marshal(table)
		err := p.store.Set(identity.IdBytes(), nsData)
		if err != nil {
			log.Errorf("namespace[%s] create table[%s] fail - %v", namespace, tab, err)
			return nil
		}
		return table
	}
	if err != nil {
		log.Errorf("namespace[%s] get table[%s] fail - %v", namespace, tab, err)
		return nil
	}
	err = proto.Unmarshal(data, table)
	if err != nil {
		log.Errorf("namespace[%s] found table[%s],but unmarshal fail - %v", namespace, tab, err)
		return nil
	}
	return table
}

func (p *StoreOperatorKV) SysSet(table, key string, message ggproto.Message) error {
	identity := NewIdentity(ENGINE_NAMESPACE_SYSTEM, table, []byte(key))
	bytes, err := proto.Marshal(message)
	if err != nil {
		return err
	}
	return p.store.Set(identity.IdBytes(), bytes)
}

func (p *StoreOperatorKV) SysGet(table, key string, message ggproto.Message) error {
	identity := NewIdentity(ENGINE_NAMESPACE_SYSTEM, table, []byte(key))
	bytes, err := p.store.Get(identity.IdBytes())
	if err != nil {
		return err
	}
	return proto.Unmarshal(bytes, message)
}

func (p *StoreOperatorKV) SetMeta(dataId Identity, meta model.DataMeta) error {
	metaId := IdentityMetaId(dataId)
	bytes, err := proto.Marshal(&meta)
	if err != nil {
		return err
	}
	return p.store.Set(metaId, bytes)
}

func (p *StoreOperatorKV) GetMeta(dataId Identity) (meta *model.DataMeta, err error) {
	metaId := IdentityMetaId(dataId)
	meta = &model.DataMeta{}
	bytes, err := p.store.Get(metaId)
	if err !=nil {
		return nil, err
	}
	err = proto.Unmarshal(bytes, meta)
	return
}

func (p *StoreOperatorKV) SetData(data Data, incrementVersion bool) error {
	identity := data.Id
	meta, err := p.GetMeta(identity)
	if err != ErrDbKeyNotFound {
		return err
	}
	if meta == nil {
		meta = &model.DataMeta{
			Id:        IdentityVersionId(identity, 1),
			Version:   1,
			Namespace: identity.Namespace,
			Table:     identity.Table,
			Key:       identity.Key,
			Ring:      identity.ring,
		}
	} else {
		if incrementVersion {
			meta.Version = meta.Version + 1
		} else {
			meta.Version = data.Version
		}
		meta.Id = IdentityVersionId(identity, meta.Version)
	}
	p.SetMeta(identity, *meta)
	content := &model.DataContent{Deleted: false, Content: data.Content}
	bytes, err := proto.Marshal(content)
	if err != nil {
		return err
	}
	err = p.store.Set(meta.Id, bytes)
	if err != nil {
		//todo://rollback meta
		return err
	}
	return nil
}
func (p *StoreOperatorKV) GetData(identity Identity) (*model.DataMeta, *model.DataContent, error) {
	meta, err := p.GetMeta(identity)
	if err != nil {
		return nil, nil, err
	}
	bytes, err := p.store.Get(meta.Id)
	if err != nil {
		return nil, nil, err
	}
	data := &model.DataContent{}
	err = proto.Unmarshal(bytes, data)
	if err != nil {
		return nil, nil, err
	}
	return meta, data, nil
}
