package models

import (
	"time"

	"github.com/uyloal/baihu-panel/internal/constant"
)

// Dependency 依赖包模型 (Node.js & pnpm 专属)
type Dependency struct {
	ID        string    `json:"id" gorm:"primaryKey;size:20"`
	Name      string    `json:"name" gorm:"size:100;not null;index"`
	Version   string    `json:"version" gorm:"size:50"`
	Language  string    `json:"language" gorm:"size:100;default:'node'"`
	Remark    string    `json:"remark" gorm:"size:255"`
	Log       BigText   `json:"log"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (Dependency) TableName() string {
	return constant.TablePrefix + "deps"
}
