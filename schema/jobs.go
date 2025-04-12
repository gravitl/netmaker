package schema

import (
	"context"
	"github.com/gravitl/netmaker/db"
	"time"
)

// Job represents a task that netmaker server
// wants to do.
//
// Ideally, a jobs table should have details
// about its type, status, who initiated it,
// etc. But, for now, the table only contains
// records of jobs that have been done, so
// that it is easier to prevent a task from
// being executed again.
type Job struct {
	ID        string    `gorm:"id;primary_key"`
	CreatedAt time.Time `gorm:"created_at"`
}

// TableName returns the name of the jobs table.
func (j *Job) TableName() string {
	return "jobs"
}

// Create creates a job record in the jobs table.
func (j *Job) Create(ctx context.Context) error {
	return db.FromContext(ctx).Table(j.TableName()).Create(j).Error
}

// Get returns a job record with the given Job.ID.
func (j *Job) Get(ctx context.Context) error {
	return db.FromContext(ctx).Table(j.TableName()).Where("id = ?", j.ID).First(j).Error
}
