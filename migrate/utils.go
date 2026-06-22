package migrate

import (
	"context"
	"encoding/json"

	"github.com/gravitl/netmaker/db"
)

type KVRecord struct {
	Key   string `gorm:"column:key;primaryKey"`
	Value string `gorm:"column:value"`
}

func kvInsert(ctx context.Context, tableName, key string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	return db.FromContext(ctx).Table(tableName).Save(&KVRecord{Key: key, Value: string(data)}).Error
}

func kvList(ctx context.Context, tableName string) (map[string]string, error) {
	var records []KVRecord
	err := db.FromContext(ctx).Table(tableName).Order("key").Find(&records).Error
	if err != nil {
		return nil, err
	}

	var list = make(map[string]string)
	for _, record := range records {
		list[record.Key] = record.Value
	}

	return list, nil
}
